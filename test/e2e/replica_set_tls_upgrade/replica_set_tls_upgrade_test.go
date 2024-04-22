package replica_set_tls

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/tlstests"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetTLSUpgrade(t *testing.T) {
	ctx := context.Background()
	resourceName := "mdb-tls"

	testCtx, testConfig := setup.SetupWithTLS(ctx, t, resourceName)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, resourceName, testConfig.Namespace)
	_, err := setup.GeneratePasswordForUser(testCtx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds(WithoutTls()))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))

	// Enable TLS as optional
	t.Run("MongoDB is reachable while TLS is being enabled", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*15, WithoutTls())()
		t.Run("Upgrade to TLS", tlstests.EnableTLS(ctx, &mdb, true))
		t.Run("Stateful Set Leaves Ready State, after setting TLS to preferSSL", mongodbtests.StatefulSetBecomesUnready(ctx, &mdb))
		t.Run("Stateful Set Reaches Ready State, after setting TLS to preferSSL", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("Wait for TLS to be enabled", tester.HasTlsMode("preferSSL", 60, WithoutTls()))
	})

	// Ensure MongoDB is reachable both with and without TLS
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds(WithoutTls()))
	t.Run("Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls(ctx, mdb)))
	t.Run("Internal cluster keyfile authentication is enabled", tester.HasKeyfileAuth(3, WithTls(ctx, mdb)))

	// Make TLS required
	t.Run("MongoDB is reachable over TLS while making TLS required", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10, WithTls(ctx, mdb))()
		t.Run("Make TLS required", tlstests.EnableTLS(ctx, &mdb, false))
		t.Run("Stateful Set Reaches Ready State, after setting TLS to requireSSL", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("Wait for TLS to be required", tester.HasTlsMode("requireSSL", 120, WithTls(ctx, mdb)))
	})

	// Ensure MongoDB is reachable only over TLS
	t.Run("Test Basic TLS Connectivity", tester.ConnectivitySucceeds(WithTls(ctx, mdb)))
	t.Run("Test TLS Required For Connectivity", tester.ConnectivityFails(WithoutTls()))
}
