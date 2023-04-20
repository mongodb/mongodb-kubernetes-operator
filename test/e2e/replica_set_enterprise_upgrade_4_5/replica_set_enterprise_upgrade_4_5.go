package replica_set

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

var (
	versionsForUpgrades = []string{"4.4.19", "5.0.15"}
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSet(t *testing.T) {
	DeployEnterpriseAndUpgradeTest(t, versionsForUpgrades)
}

func DeployEnterpriseAndUpgradeTest(t *testing.T, versionsToBeTested []string) {
	t.Setenv(construct.MongodbName, "mongodb-enterprise-server")
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	mdb.Spec.Version = versionsToBeTested[0]

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
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

	for i := 1; i < len(versionsToBeTested); i++ {
		t.Run(fmt.Sprintf("Testing upgrade from %s to %s", versionsForUpgrades[i-1], versionsForUpgrades[i]), func(t *testing.T) {
			defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
			t.Run(fmt.Sprintf("Upgrading to %s", versionsForUpgrades[i]), mongodbtests.ChangeVersion(&mdb, versionsForUpgrades[i]))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
			t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, i+1))
		})
	}
}
