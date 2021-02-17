package replica_set_scale_up

import (
	"fmt"
	"os"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetScaleUp(t *testing.T) {

	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0", "")

	_, err := setup.GeneratePasswordForUser(user, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("MongoDB is reachable", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Scale MongoDB Resource Up", mongodbtests.Scale(&mdb, 5))
		t.Run("Stateful Set Scaled Up Correctly", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 3))
		t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
			mdbv1.MongoDBCommunityStatus{
				MongoURI:                   mdb.MongoURI(),
				Phase:                      mdbv1.Running,
				CurrentMongoDBMembers:      5,
				CurrentStatefulSetReplicas: 5,
			}))

		// TODO: Currently the scale down process takes too long to reasonably include this in the test
		//t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(&mdb, 3))
		//t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReadyAfterScaleDown(&mdb))
		//t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		//t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 5))
		//t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
		//	mdbv1.MongoDBStatus{
		//		MongoURI:                   mdb.MongoURI(),
		//		Phase:                      mdbv1.Running,
		//		CurrentMongoDBMembers:   3,
		//		CurrentStatefulSetReplicas: 3,
		//	}))
	})
}
