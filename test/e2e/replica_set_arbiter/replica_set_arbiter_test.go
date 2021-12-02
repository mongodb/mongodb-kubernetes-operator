package replica_set

import (
	"fmt"
	"os"
	"testing"
	"time"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/wait"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetArbiter(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	// Invalid case 1
	numberArbiters := 3
	numberMembers := 3
	desiredStatus := fmt.Sprintf("error validating new Spec: number of arbiters specified (%v) is greater or equal than the number of members in the replicaset (%v). At least one member must not be an arbiter", numberArbiters, numberMembers)
	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	mdb.Spec.Arbiters = numberArbiters
	mdb.Spec.Members = numberMembers
	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check status for case 1", mongodbtests.StatefulSetMessageIsReceived(&mdb, ctx, desiredStatus))

	// Invalid case 2
	numberArbiters = -1
	numberMembers = 3
	desiredStatus = "error validating new Spec: number of arbiters must be greater or equal than 0"
	mdb, user = e2eutil.NewTestMongoDB(ctx, "mdb1", "")
	mdb.Spec.Arbiters = numberArbiters
	mdb.Spec.Members = numberMembers
	_, err = setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check status for case 2", mongodbtests.StatefulSetMessageIsReceived(&mdb, ctx, desiredStatus))

	numberArbiters = 0
	numberMembers = 3
	mdb, user = e2eutil.NewTestMongoDB(ctx, "mdb2", "")
	mdb.Spec.Arbiters = numberArbiters
	mdb.Spec.Members = numberMembers
	_, err = setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check that the stateful set becomes ready", mongodbtests.StatefulSetBecomesReady(&mdb, wait.Timeout(20*time.Minute)))
	t.Run("Check the number of arbiters", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(&mdb, numberArbiters))

	// Arbiters need to be less than regular members
	t.Run("Scale MongoDB Up to 2 Arbiters", mongodbtests.ScaleArbiters(&mdb, 2))
	t.Run("Arbiters Stateful Set Scaled Up Correctly", mongodbtests.ArbitersStatefulSetBecomesReady(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))

	t.Run("Scale MongoDB Up to 0 Arbiters", mongodbtests.ScaleArbiters(&mdb, 0))
	t.Run("Arbiters Stateful Set Scaled Up Correctly", mongodbtests.ArbitersStatefulSetBecomesReady(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Check the number of arbiters", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(&mdb, 0))
}
