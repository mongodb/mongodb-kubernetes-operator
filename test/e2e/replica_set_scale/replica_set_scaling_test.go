package replica_set_readiness_probe

import (
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongotester"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetScale(t *testing.T) {

	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0")
	_, err := setup.GeneratePasswordForUser(user, ctx)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic ConnectivitySucceeds", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("MongoDB is reachable", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Scale MongoDB Resource Up", mongodbtests.Scale(&mdb, 5))
		t.Run("Stateful Set Scaled Up Correctly", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 2))
		t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
			mdbv1.MongoDBStatus{
				MongoURI: mdb.MongoURI(),
				Phase:    mdbv1.Running,
			}))
		t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(&mdb, 3))
		t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 3))
		t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
			mdbv1.MongoDBStatus{
				MongoURI: mdb.MongoURI(),
				Phase:    mdbv1.Running,
			}))
	})
}
