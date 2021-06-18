package replica_set

import (
	"context"
	"fmt"
	"os"
	"testing"

	//. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllers "github.com/mongodb/mongodb-kubernetes-operator/controllers"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
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

func TestReplicaSetArbiterV2(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	//mdb, user := e2eutil.NewTestMongoDBArbiter(ctx, "mdb0", "", 3, 3)
	mdb, _ := e2eutil.NewTestMongoDBArbiter(ctx, "mdb0", "", 3, 4)

	mgr := client.NewManager(&mdb)

	r := controllers.NewReconciler(mgr)
	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	//assertReconciliationSuccessful(t, res, err)
	print("output:", " and err:", err, " status: Message: ", mdb.Status.Message, "\nPHASE: ",
		mdb.Status.Phase, "\n")
	t.Logf("Mongodb spec %v", mdb.Spec)
	//t.logf("mongodb spec: %v", )
}

func TestReplicaSetArbiter(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDBArbiter(ctx, "mdb0", "", 3, 3)
	//scramUser := mdb.GetScramUsers()[0]

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	//print("BEFORE ERROR\n\n\n\n\n")
	if err != nil {
		print("ERROR 1 \n\n\n\n\n")
		t.Fatal(err)
	}

	//tester, err := FromResource(t, mdb)
	/*if err != nil {
		print("ERROR 2 \n\n\n\n\n")
		t.Fatal(err)
	}*/

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	//mdb.Status.Message
	err = e2eutil.TestClient.Create(context.TODO(), &mdb, &e2eutil.CleanupOptions{TestContext: ctx})
	assert.Error(t, err)
	t.Logf("Error successfully generated %s/%s", mdb.Name, mdb.Namespace)
	print("Status:", mdb.Status.Message)
	print("error: ", err)
}
