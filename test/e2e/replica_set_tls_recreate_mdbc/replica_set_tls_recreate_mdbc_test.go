package replica_set_tls

import (
	"context"
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

func TestReplicaSetTLSRecreateMdbc(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb1, user := e2eutil.NewTestMongoDB(ctx, "mdb-tls", "")
	scramUser := mdb1.GetScramUsers()[0]
	mdb1.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := setup.CreateTLSResources(mdb1.Namespace, ctx, setup.CertKeyPair); err != nil {
		t.Fatalf("Failed to set up TLS resources: %s", err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb1, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb1))

	if err := e2eutil.TestClient.Delete(context.TODO(), &mdb1); err != nil {
		t.Fatalf("Failed to delete first test MongoDB: %s", err)
	}

	mdb2, _ := e2eutil.NewTestMongoDB(ctx, "mdb-tls", "")
	mdb2.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)
	tester1, err := FromResource(t, mdb2)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb2, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb2))
	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester1.HasTlsMode("requireSSL", 60, WithTls()))
		t.Run("Basic Connectivity Succeeds", tester1.ConnectivitySucceeds(WithTls()))
		t.Run("SRV Connectivity Succeeds", tester1.ConnectivitySucceeds(WithURI(mdb2.MongoSRVURI()), WithTls()))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester1.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb2, scramUser)), WithTls()))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester1.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb2, scramUser)), WithTls()))
		t.Run("Connectivity Fails", tester1.ConnectivityFails(WithoutTls()))
		t.Run("Ensure authentication is configured", tester1.EnsureAuthenticationIsConfigured(3, WithTls()))
	})
	t.Run("TLS is disabled", mongodbtests.DisableTLS(&mdb2))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(&mdb2))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(&mdb2))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb2))
}
