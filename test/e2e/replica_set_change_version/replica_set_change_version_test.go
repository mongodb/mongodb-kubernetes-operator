package replica_set

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"

	appsv1 "k8s.io/api/apps/v1"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetUpgradeVersion(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	const initialMDBVersion = "4.4.18"
	const upgradedMDBVersion = "5.0.12"
	const upgradedWithIncreasedPatchMDBVersion = "5.0.15"

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	mdb.Spec.Version = initialMDBVersion

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))

	// Upgrade minor version to upgradedMDBVersion
	t.Run("MongoDB is reachable while minor version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Minor Version can be upgraded", mongodbtests.ChangeVersion(ctx, &mdb, upgradedMDBVersion))
		t.Run("StatefulSet has OnDelete update strategy", mongodbtests.StatefulSetHasUpdateStrategy(ctx, &mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 2))
	})

	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(ctx, &mdb, appsv1.RollingUpdateStatefulSetStrategyType))

	// Upgrade patch version to upgradedWithIncreasedPatchMDBVersion
	t.Run("MongoDB is reachable while patch version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Patch Version can be upgraded", mongodbtests.ChangeVersion(ctx, &mdb, upgradedWithIncreasedPatchMDBVersion))
		t.Run("StatefulSet has OnDelete restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(ctx, &mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after upgrading", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 3))
	})
	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(ctx, &mdb, appsv1.RollingUpdateStatefulSetStrategyType))
}
