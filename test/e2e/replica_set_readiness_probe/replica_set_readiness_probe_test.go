package replica_set_readiness_probe

import (
	"math/rand"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetReadinessProbeScaling(t *testing.T) {

	rand.Seed(time.Now().Unix())

	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, _ := e2eutil.NewTestMongoDB("mdb0")
	//password, err := setup.GeneratePasswordForUser(user, ctx)
	//if err != nil {
	//	t.Fatal(err)
	//}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", mongodbtests.Connectivity(&mdb))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("MongoDB is reachable", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Delete Random Pod", mongodbtests.DeletePod(&mdb, rand.Intn(mdb.Spec.Members)))
			t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetIsReady(&mdb))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
			t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
				mdbv1.MongoDBStatus{
					MongoURI: mdb.MongoURI(),
					Phase:    mdbv1.Running,
				}))
		},
	))
}
