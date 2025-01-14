package replica_set_multiple_namespaces

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

const (
	testWatchNamespaceEnvName = "TEST_WATCH_NAMESPACE"
	testMongoDBNamespaces     = "ns1,ns2"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// TestReplicaSetMultipleNamespaces creates two MongoDB resources in separate
// namespaces to be processed by the Operator simultaneously.
func TestReplicaSetMultipleNamespaces(t *testing.T) {
	t.Setenv(testWatchNamespaceEnvName, testMongoDBNamespaces)
	ctx := context.Background()

	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	for _, namespace := range strings.Split(testMongoDBNamespaces, ",") {
		mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb", namespace)

		_, err := setup.GeneratePasswordForUser(testCtx, user, namespace)
		if err != nil {
			t.Fatal(err)
		}

		tester, err := mongotester.FromResource(ctx, t, mdb)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Create MongoDB Resource mdb", mongodbtests.CreateMongoDBResource(&mdb, testCtx))

		t.Run("mdb: Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))

		t.Run("mdb: Test Basic Connectivity", tester.ConnectivitySucceeds())

		t.Run("mdb: AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))

		t.Run("mdb: Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	}
}
