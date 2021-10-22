package replica_set_cross_namespace_deploy

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
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

func TestCrossNamespaceDeploy(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	postfix, err := generate.RandomValidDNS1123Label(5)
	if err != nil {
		t.Fatal(err)
	}
	namespace := fmt.Sprintf("clusterwide-test-%s", postfix)

	err = e2eutil.EnsureNamespace(ctx, namespace)
	if err != nil {
		t.Fatal(err)
	}

	if err := createDatabaseServiceAccountRoleAndRoleBinding(t, namespace); err != nil {
		t.Fatal(err)
	}

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", namespace)

	_, err = setup.GeneratePasswordForUser(ctx, user, namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
}

// createDatabaseServiceAccountRoleAndRoleBinding creates the ServiceAccount, Role and RoleBinding required
// for the database StatefulSet in the other namespace.
func createDatabaseServiceAccountRoleAndRoleBinding(t *testing.T, namespace string) error {
	sa := corev1.ServiceAccount{}
	err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: "mongodb-database", Namespace: e2eutil.OperatorNamespace}, &sa)
	if err != nil {
		t.Fatal(err)
	}

	sa.Namespace = namespace
	sa.ObjectMeta.ResourceVersion = ""

	err = e2eutil.TestClient.Create(context.TODO(), &sa, &e2eutil.CleanupOptions{})
	if err != nil {
		t.Fatal(err)
	}

	role := rbacv1.Role{}
	err = e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: "mongodb-database", Namespace: e2eutil.OperatorNamespace}, &role)
	if err != nil {
		t.Fatal(err)
	}

	role.Namespace = namespace
	role.ObjectMeta.ResourceVersion = ""

	err = e2eutil.TestClient.Create(context.TODO(), &role, &e2eutil.CleanupOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rolebinding := rbacv1.RoleBinding{}
	err = e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: "mongodb-database", Namespace: e2eutil.OperatorNamespace}, &rolebinding)
	if err != nil {
		t.Fatal(err)
	}

	rolebinding.Namespace = namespace
	rolebinding.ObjectMeta.ResourceVersion = ""

	err = e2eutil.TestClient.Create(context.TODO(), &rolebinding, &e2eutil.CleanupOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return nil
}
