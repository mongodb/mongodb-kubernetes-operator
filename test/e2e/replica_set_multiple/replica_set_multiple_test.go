package replica_set_multiple

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// TestReplicaSetMultiple creates two MongoDB resources that are handled by the Operator at the
// same time. One of them is scaled to 5 and then back to 3
func TestReplicaSetMultiple(t *testing.T) {
	ctx := context.Background()

	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb0, user0 := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	mdb1, user1 := e2eutil.NewTestMongoDB(testCtx, "mdb1", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user0, "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = setup.GeneratePasswordForUser(testCtx, user1, "")
	if err != nil {
		t.Fatal(err)
	}

	tester0, err := mongotester.FromResource(ctx, t, mdb0)
	if err != nil {
		t.Fatal(err)
	}
	tester1, err := mongotester.FromResource(ctx, t, mdb1)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource mdb0", mongodbtests.CreateMongoDBResource(&mdb0, testCtx))
	t.Run("Create MongoDB Resource mdb1", mongodbtests.CreateMongoDBResource(&mdb1, testCtx))

	t.Run("mdb0: Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb0))
	t.Run("mdb1: Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb1))

	t.Run("mdb0: Test Basic Connectivity", tester0.ConnectivitySucceeds())
	t.Run("mdb1: Test Basic Connectivity", tester1.ConnectivitySucceeds())

	t.Run("mdb0: AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb0, 1))
	t.Run("mdb1: AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb1, 1))

	t.Run("mdb0: Ensure Authentication", tester0.EnsureAuthenticationIsConfigured(3))
	t.Run("mdb1: Ensure Authentication", tester1.EnsureAuthenticationIsConfigured(3))

	t.Run("MongoDB is reachable while being scaled up", func(t *testing.T) {
		defer tester0.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Scale MongoDB Resource Up", mongodbtests.Scale(ctx, &mdb0, 5))
		t.Run("Stateful Set Scaled Up Correctly", mongodbtests.StatefulSetBecomesReady(ctx, &mdb0))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb0))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb0, 3))
		t.Run("Test Status Was Updated", mongodbtests.Status(ctx, &mdb0, mdbv1.MongoDBCommunityStatus{
			MongoURI:                   mdb0.MongoURI(""),
			Phase:                      mdbv1.Running,
			CurrentMongoDBMembers:      5,
			CurrentStatefulSetReplicas: 5,
		}))

		// TODO: Currently the scale down process takes too long to reasonably include this in the test
		//t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(&mdb0, 3))
		//t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReadyAfterScaleDown(&mdb0))
		//t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb0))
		//t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb0, 3))
		//t.Run("Test Status Was Updated", mongodbtests.Status(&mdb0,
		//	mdbv1.MongoDBStatus{
		//		MongoURI:                   mdb0.MongoURI(""),
		//		Phase:                      mdbv1.Running,
		//		CurrentMongoDBMembers:   5,
		//		CurrentStatefulSetReplicas: 5,
		//	}))

	})

	// One last check that mdb1 was not altered.
	t.Run("mdb1: Test Basic Connectivity", tester1.ConnectivitySucceeds())
}
