package replica_set_tls

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/tlstests"
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

func TestReplicaSetTLSRotate(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)
	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb-tls", "")
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(user, ctx, "")
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
	t.Run("Wait for TLS to be enabled", tester.HasTlsMode("requireSSL", 60, WithTls()))
	t.Run("Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls()))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3, WithTls()))
	t.Run("Test TLS required", tester.ConnectivityFails(WithoutTls()))

	t.Run("MongoDB is reachable while certificate is rotated", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10, WithTls())()
		t.Run("Update certificate secret", tlstests.RotateCertificate(&mdb))
		t.Run("Wait for certificate to be rotated", tester.WaitForRotatedCertificate())
	})
}
