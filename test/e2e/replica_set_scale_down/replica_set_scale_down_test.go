package replica_set_scale_down

import (
	"fmt"
	"os"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetScaleDown(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "replica-set-scale-down", "")

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(ctx, &mdb))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))

	t.Run("MongoDB is reachable", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, ctx, time.Second*10)()
		t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(ctx, &mdb, 1))
		t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReadyAfterScaleDown(ctx, &mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 3))
		t.Run("Test Status Was Updated", mongodbtests.Status(ctx, &mdb,
			mdbv1.MongoDBCommunityStatus{
				MongoURI:                   mdb.MongoURI(""),
				Phase:                      mdbv1.Running,
				Version:                    mdb.GetMongoDBVersion(),
				CurrentMongoDBMembers:      1,
				CurrentStatefulSetReplicas: 1,
			}))
	})
}
