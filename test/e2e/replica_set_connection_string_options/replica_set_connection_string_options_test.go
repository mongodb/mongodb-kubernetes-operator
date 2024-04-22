package replica_set_connection_string_options

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

func TestReplicaSetWithConnectionString(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	scramUser := mdb.GetAuthUsers()[0]

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))

	/**
	User options only.
	*/
	t.Run("Connection String With User Options Only", func(t *testing.T) {
		t.Run("Test Add New Connection String Option to User", mongodbtests.AddConnectionStringOptionToUser(ctx, &mdb, "readPreference", "primary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		scramUser = mdb.GetAuthUsers()[0]
		t.Run("Test Basic Connectivity With User Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With User Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with user options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with user options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser))))
	})

	/**
	General options only.
	*/
	t.Run("Connection String With General Options Only", func(t *testing.T) {
		t.Run("Resetting Connection String Options", mongodbtests.ResetConnectionStringOptions(ctx, &mdb))
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(ctx, &mdb, "readPreference", "primary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		scramUser = mdb.GetAuthUsers()[0]
		t.Run("Test Basic Connectivity With Resource Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With Resource Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with resource options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with resource options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser))))
	})

	/**
	Overwritten options.
	*/
	t.Run("Connection String With Overwritten Options", func(t *testing.T) {
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(ctx, &mdb, "readPreference", "primary"))
		t.Run("Test Add New Connection String Option to User", mongodbtests.AddConnectionStringOptionToUser(ctx, &mdb, "readPreference", "secondary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		scramUser = mdb.GetAuthUsers()[0]
		t.Run("Test Basic Connectivity With Overwritten Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With Overwritten Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with overwritten options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with overwritten options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser))))
	})

	/**
	Wrong options.
	*/
	t.Run("Connection String With Wrong Options", func(t *testing.T) {
		t.Run("Resetting Connection String Options", mongodbtests.ResetConnectionStringOptions(ctx, &mdb))
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(ctx, &mdb, "readPreference", "wrong"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		scramUser = mdb.GetAuthUsers()[0]
		t.Run("Test Basic Connectivity", tester.ConnectivityRejected(ctx, WithURI(mdb.MongoURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test SRV Connectivity", tester.ConnectivityRejected(ctx, WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret",
			tester.ConnectivityRejected(ctx, WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret",
			tester.ConnectivityRejected(ctx, WithURI(mongodbtests.GetSrvConnectionStringForUser(ctx, mdb, scramUser))))
	})

}
