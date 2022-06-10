package replica_set_mongod_config

import (
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

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

func TestReplicaSetMongodPortChangeWithArbiters(t *testing.T) {
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

	connectivityTests := func(t *testing.T) {
		fmt.Printf("connectionStringForUser: %s\n", mongodbtests.GetConnectionStringForUser(mdb, scramUser))
		t.Run("Test Basic Connectivity with generated connection string secret",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))

		// FIXME after port change in the service mongodb+srv connection stopped working!
		//t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		//t.Run("Test SRV Connectivity with generated connection string secret",
		//	tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(automationconfig.DefaultDBPort)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(&mdb, int32(automationconfig.DefaultDBPort)))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(&mdb))
	t.Run("Wait for MongoDB to finish setup cluster", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Connectivity tests", connectivityTests)

	t.Run("Scale to 1 Arbiter", mongodbtests.ScaleArbiters(&mdb, 1))
	t.Run("Wait for MongoDB to start scaling arbiters", mongodbtests.MongoDBReachesPendingPhase(&mdb))
	t.Run("Wait for MongoDB to finish scaling arbiters", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Automation config has expecter arbiter", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(&mdb, 1))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(&mdb))
	t.Run("Arbiters Stateful Set becomes ready", mongodbtests.ArbitersStatefulSetBecomesReady(&mdb))
	t.Run("Connectivity tests", connectivityTests)

	t.Run("Change port of running cluster", mongodbtests.ChangePort(&mdb, 40333))
	t.Run("Wait for MongoDB to start changing port", mongodbtests.MongoDBReachesPendingPhase(&mdb))
	t.Run("Wait for MongoDB to finish changing port", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(&mdb))
	t.Run("Arbiters Stateful Set becomes ready", mongodbtests.ArbitersStatefulSetBecomesReady(&mdb))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(40333)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(&mdb, int32(40333)))
	t.Run("Connectivity tests", connectivityTests)
}
