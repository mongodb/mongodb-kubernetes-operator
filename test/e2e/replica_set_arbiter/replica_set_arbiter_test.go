package replica_set

import (
	"fmt"
	"os"
	"testing"
	"time"

	//. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

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
	desiredStatus := fmt.Sprintf("Error ensuring Arbiter config: number of arbiters specified (%v) is greater or equal than the number of members in the replicaset (%v). At least one member must not be an arbiter", numberArbiters, numberMembers)
	mdb, user := e2eutil.NewTestMongoDBArbiter(ctx, "mdb0", "", numberMembers, numberArbiters)
	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check status for case 1", mongodbtests.CheckMessageStatusMdb(ctx, &mdb, desiredStatus))

	// Invalid case 2
	numberArbiters = -1
	numberMembers = 3
	desiredStatus = "Error ensuring Arbiter config: number of arbiters must be greater or equal than 0"
	mdb, user = e2eutil.NewTestMongoDBArbiter(ctx, "mdb1", "", numberMembers, numberArbiters)
	_, err = setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check status for case 2", mongodbtests.CheckMessageStatusMdb(ctx, &mdb, desiredStatus))

	// Valid case 1
	numberArbiters = 1
	numberMembers = 3
	mdb, user = e2eutil.NewTestMongoDBArbiter(ctx, "mdb2", "", numberMembers, numberArbiters)
	_, err = setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Check status for case 3", mongodbtests.StatefulSetBecomesReady(&mdb, wait.Timeout(20*time.Minute)))
	t.Run("Check the number of arbiters", mongodbtests.AutomationConfigReplicaSetsHaveExpectedArbiters(&mdb, numberArbiters))

}
