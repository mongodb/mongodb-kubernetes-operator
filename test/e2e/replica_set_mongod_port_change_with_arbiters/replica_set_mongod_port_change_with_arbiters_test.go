package replica_set_mongod_config

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

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

func TestReplicaSetMongodPortChangeWithArbiters(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	// FIXME: This behavior has been changed in 6.x timeline and now the arbiter (nor the RS) can't reach the goal state.
	mdb.Spec.Version = "4.4.19"
	scramUser := mdb.GetAuthUsers()[0]

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	connectivityTests := func(t *testing.T) {
		fmt.Printf("connectionStringForUser: %s\n", mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))
		t.Run("Test Basic Connectivity with generated connection string secret",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(ctx, mdb, scramUser))))

		// FIXME after port change in the service mongodb+srv connection stopped working!
		//t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		//t.Run("Test SRV Connectivity with generated connection string secret",
		//	tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(automationconfig.DefaultDBPort)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(ctx, &mdb, int32(automationconfig.DefaultDBPort)))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
	t.Run("Wait for MongoDB to finish setup cluster", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	t.Run("Connectivity tests", connectivityTests)

	t.Run("Scale to 1 Arbiter", mongodbtests.ScaleArbiters(ctx, &mdb, 1))
	t.Run("Wait for MongoDB to start scaling arbiters", mongodbtests.MongoDBReachesPendingPhase(ctx, &mdb))
	t.Run("Wait for MongoDB to finish scaling arbiters", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	t.Run("Automation config has expecter arbiter", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(ctx, &mdb, 1))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
	t.Run("Arbiters Stateful Set becomes ready", mongodbtests.ArbitersStatefulSetBecomesReady(ctx, &mdb))
	t.Run("Connectivity tests", connectivityTests)

	t.Run("Change port of running cluster", mongodbtests.ChangePort(ctx, &mdb, 40333))
	t.Run("Wait for MongoDB to start changing port", mongodbtests.MongoDBReachesPendingPhase(ctx, &mdb))
	t.Run("Wait for MongoDB to finish changing port", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	t.Run("Stateful Set becomes ready", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
	t.Run("Arbiters Stateful Set becomes ready", mongodbtests.ArbitersStatefulSetBecomesReady(ctx, &mdb))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(40333)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(ctx, &mdb, int32(40333)))
	t.Run("Connectivity tests", connectivityTests)
}
