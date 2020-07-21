package replica_set_tls

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetTLS(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)
	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb-tls")
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(user, ctx)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	if err := setup.CreateTLSResources(mdb.Namespace, ctx); err != nil {
		t.Fatalf("Failed to set up TLS resources: %+v", err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Wait for TLS to be enabled", tester.WaitForTLSMode("requireSSL", WithTls()))
	t.Run("Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls()))
	t.Run("Test TLS required", tester.ConnectivityFails(WithoutTls()))
}
