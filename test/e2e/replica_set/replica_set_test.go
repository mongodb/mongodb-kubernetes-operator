package replica_set

import (
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestMongoDB() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-mongodb",
			Namespace: f.Global.Namespace,
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Type:    "ReplicaSet",
			Version: "4.0.6",
		},
	}
}

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSet(t *testing.T) {
	ctx := f.NewTestCtx(t)
	defer ctx.Cleanup()

	// register our types with the testing framework
	if err := e2eutil.RegisterTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	mdb := newTestMongoDB()
	t.Run("Create MongoDB Resource", mongodbtests.CreateResource(mdb, ctx))
	t.Run("Perform Basic Functionality Checks", mongodbtests.BasicFunctionality(mdb))
	t.Run("Test Basic Connectivity", mongodbtests.BasicConnectivity(mdb))
}
