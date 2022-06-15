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
	resourceName := "mdb-tls"

	ctx, testConfig := setup.SetupWithTLS(t, resourceName)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, resourceName, testConfig.Namespace)
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(ctx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, mdb)
	if err != nil {
		t.Fatal(err)
	}

	clientCert, err := GetClientCert(ctx, mdb)
	if err != nil {
		t.Fatal(err)
	}
	initialCertSerialNumber := clientCert.SerialNumber

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(ctx, &mdb))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Wait for TLS to be enabled", tester.HasTlsMode("requireSSL", 60, WithTls(ctx, mdb)))
	t.Run("Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls(ctx, mdb)))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3, WithTls(ctx, mdb)))
	t.Run("Test TLS required", tester.ConnectivityFails(WithoutTls()))

	t.Run("MongoDB is reachable while certificate is rotated", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, ctx, time.Second*10, WithTls(ctx, mdb))()
		t.Run("Update certificate secret", tlstests.RotateCertificate(ctx, &mdb))
		t.Run("Wait for certificate to be rotated", tester.WaitForRotatedCertificate(ctx, mdb, initialCertSerialNumber))
		t.Run("Wait for MongoDB to reach Running Phase after rotating server cert", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		t.Run("Extend CA certificate validity", tlstests.ExtendCACertificate(ctx, &mdb))
		t.Run("Wait for MongoDB to start reconciling after extending CA", mongodbtests.MongoDBReachesPendingPhase(ctx, &mdb))
		t.Run("Wait for MongoDB to reach Running Phase after extending CA", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	})
}
