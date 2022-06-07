package replica_set_mongod_config

import (
	"fmt"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
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

func TestReplicaSetMongodPortChange(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")

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
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(automationconfig.DefaultDBPort)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(&mdb, int32(automationconfig.DefaultDBPort)))
	t.Run("Wait for MongoDB to finish setup cluster", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Change port of running cluster", mongodbtests.ChangePort(&mdb, 40333))
	t.Run("Wait for MongoDB to start changing port", mongodbtests.MongoDBReachesPendingPhase(&mdb))
	t.Run("Wait for MongoDB to finish changing port", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(40333)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(&mdb, int32(40333)))
}
