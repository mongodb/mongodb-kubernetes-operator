package replica_set_tls

import (
	"fmt"
	"os"
	"testing"

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

func TestReplicaSetTLS(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb-tls", "")
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := setup.CreateTLSResources(mdb.Namespace, ctx); err != nil {
		t.Fatalf("Failed to set up TLS resources: %s", err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	mongodbtests.SkipTestIfLocal(t, "Wait for TLS to be enabled", tester.HasTlsMode("requireSSL", 60, WithTls()))
	mongodbtests.SkipTestIfLocal(t, "Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls()))
	mongodbtests.SkipTestIfLocal(t, "Test TLS required", tester.ConnectivityFails(WithoutTls()))
	mongodbtests.SkipTestIfLocal(t, "Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3, WithTls()))
	t.Run("TLS is disabled", mongodbtests.DisableTLS(&mdb))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(&mdb))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
}
