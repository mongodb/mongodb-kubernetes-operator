package replica_set_tls

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

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

func TestReplicaSetTLS(t *testing.T) {
	ctx := context.Background()
	resourceName := "mdb-tls"

	testCtx, testConfig := setup.SetupWithTLS(ctx, t, resourceName)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, resourceName, testConfig.Namespace)
	scramUser := mdb.GetAuthUsers()[0]
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

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
	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls(ctx, mdb)))
		t.Run("Basic Connectivity Succeeds", tester.ConnectivitySucceeds(WithTls(ctx, mdb)))
		t.Run("SRV Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithTls(ctx, mdb)))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser)), WithTls(ctx, mdb)))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser)), WithTls(ctx, mdb)))
		t.Run("Connectivity Fails", tester.ConnectivityFails(WithoutTls()))
		t.Run("Ensure authentication is configured", tester.EnsureAuthenticationIsConfigured(3, WithTls(ctx, mdb)))
	})
	t.Run("TLS is disabled", mongodbtests.DisableTLS(ctx, &mdb))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(ctx, &mdb))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(ctx, &mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
}
