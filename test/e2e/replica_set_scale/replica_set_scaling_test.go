package replica_set_readiness_probe

import (
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetScale(t *testing.T) {
	ctx := f.NewTestCtx(t)
	defer ctx.Cleanup()

	// register our types with the testing framework
	if err := e2eutil.RegisterTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	mdb := e2eutil.NewTestMongoDB()
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb))
	t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb))
	t.Run("MongoDB is reachable", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Scale MongoDB Resource Up", mongodbtests.Scale(&mdb, 5))
			t.Run("Stateful Set Scaled Up Correctly", mongodbtests.StatefulSetIsReady(&mdb))
			t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
				mdbv1.MongoDBStatus{
					MongoURI: mdb.MongoURI(),
					Phase:    mdbv1.Running,
				}))
			t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(&mdb, 3))
			t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReady(&mdb))
			t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
				mdbv1.MongoDBStatus{
					MongoURI: mdb.MongoURI(),
					Phase:    mdbv1.Running,
				}))
		},
	))
}
