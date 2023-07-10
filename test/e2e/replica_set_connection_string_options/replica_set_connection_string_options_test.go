package replica_set_connection_string_options

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

func TestReplicaSetWithConnectionString(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	scramUser := mdb.GetScramUsers()[0]

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

	/**
	User options only.
	*/
	t.Run("Connection String With User Options Only", func(t *testing.T) {
		t.Run("Test Add New Connection String Option to User", mongodbtests.AddConnectionStringOptionToUser(&mdb, "readPreference", "primary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		scramUser = mdb.GetScramUsers()[0]
		t.Run("Test Basic Connectivity With User Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With User Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with user options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with user options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	})

	/**
	General options only.
	*/
	t.Run("Connection String With General Options Only", func(t *testing.T) {
		t.Run("Resetting Connection String Options", mongodbtests.ResetConnectionStringOptions(&mdb))
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(&mdb, "readPreference", "primary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		scramUser = mdb.GetScramUsers()[0]
		t.Run("Test Basic Connectivity With Resource Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With Resource Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with resource options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with resource options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	})

	/**
	Overwritten options.
	*/
	t.Run("Connection String With Overwritten Options", func(t *testing.T) {
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(&mdb, "readPreference", "primary"))
		t.Run("Test Add New Connection String Option to User", mongodbtests.AddConnectionStringOptionToUser(&mdb, "readPreference", "secondary"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		scramUser = mdb.GetScramUsers()[0]
		t.Run("Test Basic Connectivity With Overwritten Options", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity With Overwritten Options", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret with overwritten options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret with overwritten options",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	})

	/**
	Wrong options.
	*/
	t.Run("Connection String With Wrong Options", func(t *testing.T) {
		t.Run("Resetting Connection String Options", mongodbtests.ResetConnectionStringOptions(&mdb))
		t.Run("Test Add New Connection String Option to Resource", mongodbtests.AddConnectionStringOption(&mdb, "readPreference", "wrong"))
		t.Run("Test Secrets Are Updated", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		scramUser = mdb.GetScramUsers()[0]
		t.Run("Test Basic Connectivity", tester.ConnectivityRejected(WithURI(mdb.MongoURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test SRV Connectivity", tester.ConnectivityRejected(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret",
			tester.ConnectivityRejected(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret",
			tester.ConnectivityRejected(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	})

}
