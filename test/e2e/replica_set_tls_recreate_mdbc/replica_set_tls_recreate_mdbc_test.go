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
	resourceName := "mdb-tls"

	ctx, testConfig := setup.SetupWithTLS(t, resourceName)
	defer ctx.Teardown()

	mdb1, user := e2eutil.NewTestMongoDB(ctx, resourceName, testConfig.Namespace)
	scramUser := mdb1.GetScramUsers()[0]
	mdb1.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(ctx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb1, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb1))

	if err := e2eutil.TestClient.Delete(context.TODO(), &mdb1); err != nil {
		t.Fatalf("Failed to delete first test MongoDB: %s", err)
	}
	t.Run("Stateful Set Is Deleted", mongodbtests.StatefulSetIsDeleted(&mdb1))

	mdb2, _ := e2eutil.NewTestMongoDB(ctx, resourceName, testConfig.Namespace)
	mdb2.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)
	tester1, err := FromResource(t, mdb2)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb2, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb2))
	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester1.HasTlsMode("requireSSL", 60, WithTls(mdb2)))
		t.Run("Basic Connectivity Succeeds", tester1.ConnectivitySucceeds(WithTls(mdb2)))
		t.Run("SRV Connectivity Succeeds", tester1.ConnectivitySucceeds(WithURI(mdb2.MongoSRVURI("")), WithTls(mdb2)))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester1.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb2, scramUser)), WithTls(mdb2)))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester1.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb2, scramUser)), WithTls(mdb2)))
		t.Run("Connectivity Fails", tester1.ConnectivityFails(WithoutTls()))
		t.Run("Ensure authentication is configured", tester1.EnsureAuthenticationIsConfigured(3, WithTls(mdb2)))
	})
	t.Run("TLS is disabled", mongodbtests.DisableTLS(&mdb2))
	t.Run("MongoDB Reaches Failed Phase", mongodbtests.MongoDBReachesFailedPhase(&mdb2))
	t.Run("TLS is enabled", mongodbtests.EnableTLS(&mdb2))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb2))
}
