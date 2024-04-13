package replica_set_mongod_readiness

import (
	"context"
	"fmt"
	"os"
	"testing"

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

func TestReplicaSet(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Ensure Agent container is marked as non-ready", func(t *testing.T) {
		t.Run("Break mongod data files", mongodbtests.ExecInContainer(ctx, &mdb, 0, "mongod", "mkdir /data/tmp; mv /data/WiredTiger.wt /data/tmp"))
		// Just moving the file doesn't fail the mongod until any data is written - the easiest way is to kill the mongod
		// and in this case it won't restart
		t.Run("Kill mongod process", mongodbtests.ExecInContainer(ctx, &mdb, 0, "mongod", "kill 1"))
		// CLOUDP-89260: mongod uptime 1 minute and readiness probe failureThreshold 40 (40 * 5 -> 200 seconds)
		// note, that this may take much longer on evergreen than locally
		t.Run("Pod agent container becomes not-ready", mongodbtests.PodContainerBecomesNotReady(ctx, &mdb, 0, "mongodb-agent"))
	})
	t.Run("Ensure Agent container gets fixed", func(t *testing.T) {
		// Note, that we call this command on the 'mongodb-agent' container as the 'mongod' container is down and we cannot
		// execute shell there. But both containers share the same /data directory so we can do it from any of them.
		t.Run("Fix mongod data files", mongodbtests.ExecInContainer(ctx, &mdb, 0, "mongodb-agent", "mv /data/tmp/WiredTiger.wt /data/"))
		// Eventually the agent will start mongod again
		t.Run("Pod agent container becomes ready", mongodbtests.PodContainerBecomesReady(ctx, &mdb, 0, "mongodb-agent"))
	})
}
