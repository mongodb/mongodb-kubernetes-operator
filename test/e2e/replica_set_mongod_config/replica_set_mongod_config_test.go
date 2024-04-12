package replica_set_mongod_config

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/stretchr/objx"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSet(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	settings := []string{
		"storage.wiredTiger.engineConfig.journalCompressor",
		"storage.dbPath",
	}

	values := []string{
		"zlib",
		"/some/path/db",
	}

	// Override the journal compressor and dbPath settings
	mongodConfig := objx.New(map[string]interface{}{})
	for i := range settings {
		mongodConfig.Set(settings[i], values[i])
	}

	// Override the net.port setting
	mongodConfig.Set("net.port", 40333.)

	mdb.Spec.AdditionalMongodConfig.Object = mongodConfig

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	for i := range settings {
		t.Run(fmt.Sprintf("Mongod setting %s has been set", settings[i]), tester.EnsureMongodConfig(settings[i], values[i]))
	}
	t.Run("Mongod setting net.port has been set", tester.EnsureMongodConfig("net.port", int32(40333)))
	t.Run("Service has the correct port", mongodbtests.ServiceUsesCorrectPort(ctx, &mdb, 40333))
}
