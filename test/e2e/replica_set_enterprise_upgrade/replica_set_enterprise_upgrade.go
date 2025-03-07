package replica_set_enterprise_upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
)

func DeployEnterpriseAndUpgradeTest(ctx context.Context, t *testing.T, versionsToBeTested []string) {
	t.Setenv(construct.MongodbRepoUrlEnv, "docker.io/mongodb")
	t.Setenv(construct.MongodbImageEnv, "mongodb-enterprise-server")
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	mdb.Spec.Version = versionsToBeTested[0]

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))

	for i := 1; i < len(versionsToBeTested); i++ {
		t.Run(fmt.Sprintf("Testing upgrade from %s to %s", versionsToBeTested[i-1], versionsToBeTested[i]), func(t *testing.T) {
			defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
			t.Run(fmt.Sprintf("Upgrading to %s", versionsToBeTested[i]), mongodbtests.ChangeVersion(ctx, &mdb, versionsToBeTested[i]))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
			t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, i+1))
		})
	}
}
