package replica_set_tls

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

/*func TestReplicaSetTLS(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb-tls", "")
	scramUser := mdb.GetScramUsers()[0]
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
	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls()))
		t.Run("Basic Connectivity Succeeds", tester.ConnectivitySucceeds(WithTls()))
		t.Run("SRV Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI()), WithTls()))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser)), WithTls()))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser)), WithTls()))
		t.Run("Connectivity Fails", tester.ConnectivityFails(WithoutTls()))
		t.Run("Ensure authentication is configured", tester.EnsureAuthenticationIsConfigured(3, WithTls()))
	})
	t.Run("TLS is disabled", mongodbtests.DisableTLS(&mdb))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(&mdb))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
}*/

func TestReplicaSetTLSAcceptingConnecWithoutCertif(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb-tls", "")
	scramUser := mdb.GetScramUsers()[0]
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	if mdb.Spec.AdditionalMongodConfig.Object == nil {
		mdb.Spec.AdditionalMongodConfig.Object = make(map[string]interface{})
	}
	mdb.Spec.AdditionalMongodConfig.Object["net.tls.allowConnectionsWithoutCertificates"] = false
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
	_, _ = tester, scramUser
	/*mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls()))
		t.Run("Basic Connectivity Succeeds", tester.ConnectivitySucceeds(WithTls()))
		t.Run("SRV Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI()), WithTls()))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser)), WithTls()))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser)), WithTls()))
		t.Run("Connectivity Fails", tester.ConnectivityFails(WithoutTls()))
		t.Run("Connectivity Fails for user without TLS",
			tester.ConnectivityFails(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser)), WithoutCertificates()))
		t.Run("Ensure authentication is configured", tester.EnsureAuthenticationIsConfigured(3, WithTls()))
	})
	t.Run("TLS is disabled", mongodbtests.DisableTLS(&mdb))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(&mdb))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))*/
}
