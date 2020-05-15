package replica_set_multiple

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

// TestReplicaSet creates two MongoDB resources that are handled by the Operator at the
// same time. One of them is scaled to 5 and then back to 3
func TestReplicaSet(t *testing.T) {
	ctx := f.NewContext(t)
	defer ctx.Cleanup()
	if err := e2eutil.RegisterTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	mdb0 := e2eutil.NewTestMongoDB("mdb0")
	mdb1 := e2eutil.NewTestMongoDB("mdb1")
	t.Run("Create MongoDB Resource mdb0", mongodbtests.CreateMongoDBResource(&mdb0, ctx))
	t.Run("Create MongoDB Resource mdb1", mongodbtests.CreateMongoDBResource(&mdb1, ctx))

	t.Run("mdb0: Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb0))
	t.Run("mdb1: Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb1))

	t.Run("mdb0: Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb0))
	t.Run("mdb1: Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb1))

	t.Run("mdb0: MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb0))
	t.Run("mdb1: MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb1))

	t.Run("mdb0: Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb0))
	t.Run("mdb1: Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb1))

	t.Run("mdb0: Test Status Was Updated", mongodbtests.Status(&mdb0,
		mdbv1.MongoDBStatus{
			MongoURI: mdb0.MongoURI(),
			Phase:    mdbv1.Running,
		}))
	t.Run("mdb1: Test Status Was Updated", mongodbtests.Status(&mdb1,
		mdbv1.MongoDBStatus{
			MongoURI: mdb1.MongoURI(),
			Phase:    mdbv1.Running,
		}))

	t.Run("MongoDB is reachable while being scaled up", mongodbtests.IsReachableDuring(&mdb0, time.Second*10,
		func() {
			t.Run("Scale MongoDB Resource Up", mongodbtests.Scale(&mdb0, 5))
			t.Run("Stateful Set Scaled Up Correctly", mongodbtests.StatefulSetIsReady(&mdb0))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb0))
			t.Run("Test Status Was Updated", mongodbtests.Status(&mdb0,
				mdbv1.MongoDBStatus{
					MongoURI: mdb0.MongoURI(),
					Phase:    mdbv1.Running,
				}))
			t.Run("Scale MongoDB Resource Down", mongodbtests.Scale(&mdb0, 3))
			t.Run("Stateful Set Scaled Down Correctly", mongodbtests.StatefulSetIsReady(&mdb0))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb0))
			t.Run("Test Status Was Updated", mongodbtests.Status(&mdb0,
				mdbv1.MongoDBStatus{
					MongoURI: mdb0.MongoURI(),
					Phase:    mdbv1.Running,
				}))
		},
	))

	// One last check that mdb1 was not altered.
	t.Run("mdb1: Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb1))
}
