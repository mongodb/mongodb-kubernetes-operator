package replica_set_cross_namespace_deploy

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestCrossNamespaceDeploy(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	postfix, err := generate.RandomValidDNS1123Label(5)
	if err != nil {
		t.Fatal(err)
	}
	namespace := fmt.Sprintf("clusterwide-test-%s", postfix)

	err = e2eutil.EnsureNamespace(ctx, namespace)
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.EnsureServiceAccount(ctx, namespace, "mongodb-kubernetes-operator")
	if err != nil {
		t.Fatal(err)
	}

	// Create a role in the test namespace
	err = e2eutil.EnsureRole(ctx, namespace, "mongodb-kubernetes-operator", []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "services/finalizers", "endpoints", "persistentvolumeclaims", "events", "configmaps", "secrets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "daemonsets", "replicasets", "statefulsets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"monitoring.coreos.com"},
			Resources: []string{"servicemonitors"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups:     []string{"apps"},
			ResourceNames: []string{"mongodb-kubernetes-operator"},
			Resources:     []string{"deployments/finalizers"},
			Verbs:         []string{"update"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets", "deployments"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{"mongodbcommunity.mongodb.com"},
			Resources: []string{"mongodbcommunity", "mongodbcommunity/status", "mongodbcommunity/spec"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.EnsureRoleBinding(ctx, namespace, "mongodb-kubernetes-operator",
		[]rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: "mongodb-kubernetes-operator",
		}}, rbacv1.RoleRef{
			Kind:     "Role",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "mongodb-kubernetes-operator",
		})
	if err != nil {
		t.Fatal(err)
	}

	// Create a cluster role in the default (operator) namespace
	err = e2eutil.EnsureClusterRole(ctx, e2eutil.OperatorNamespace, "mongodb-kubernetes-operator", []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "services/finalizers", "endpoints", "persistentvolumeclaims", "events", "configmaps", "secrets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "daemonsets", "replicasets", "statefulsets"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
		{
			APIGroups: []string{"monitoring.coreos.com"},
			Resources: []string{"servicemonitors"},
			Verbs:     []string{"get", "create"},
		},
		{
			APIGroups:     []string{"apps"},
			ResourceNames: []string{"mongodb-kubernetes-operator"},
			Resources:     []string{"deployments/finalizers"},
			Verbs:         []string{"update"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets", "deployments"},
			Verbs:     []string{"get"},
		},
		{
			APIGroups: []string{"mongodbcommunity.mongodb.com"},
			Resources: []string{"mongodbcommunity", "mongodbcommunity/status", "mongodbcommunity/spec"},
			Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = e2eutil.EnsureClusterRoleBinding(ctx, e2eutil.OperatorNamespace, "mongodb-kubernetes-operator",
		[]rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      "mongodb-kubernetes-operator",
			Namespace: e2eutil.OperatorNamespace,
		}}, rbacv1.RoleRef{
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
			Name:     "mongodb-kubernetes-operator",
		})
	if err != nil {
		t.Fatal(err)
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0", namespace)

	_, err = setup.GeneratePasswordForUser(user, ctx, namespace)
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
