package replica_set_readiness_probe

import (
	"math/rand"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetReadinessProbeScaling(t *testing.T) {
	ctx := f.NewTestCtx(t)
	defer ctx.Cleanup()

	// register our types with the testing framework
	if err := e2eutil.RegisterTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	mdb := e2eutil.NewTestMongoDB()
<<<<<<< HEAD
	t.Run("Create MongoDB Resource", mongodbtests.CreateOrUpdateResource(&mdb, ctx))
	t.Run("Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb))
	t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb))
	t.Run("MongoDB is reachable", mongodbtests.IsReachableDuring(&mdb,
		func() {
			t.Run("Delete Random Pod", mongodbtests.DeletePod(&mdb, rand.Intn(mdb.Spec.Members-1)))
			t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetIsReady(&mdb))
		},
	))

=======
	t.Run("Create MongoDB Resource", mongodbtests.CreateResource(mdb, ctx))
	t.Run("Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(mdb))
	t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(mdb))
	t.Run("Test Basic Connectivity", mongodbtests.BasicConnectivity(mdb))
	t.Run("Delete Random Pod", mongodbtests.DeletePod(mdb, rand.Intn(mdb.Spec.Members-1)))
	t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetIsReady(mdb))
	t.Run("Test Recovered Replica Set Connectivity", mongodbtests.BasicConnectivity(mdb))
>>>>>>> master
}
