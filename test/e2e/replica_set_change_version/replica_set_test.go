package replica_set

import (
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"

	appsv1 "k8s.io/api/apps/v1"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetUpgradeVersion(t *testing.T) {

	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0")

	_, err := setup.GeneratePasswordForUser(user, ctx)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", tester.Connectivity())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

	// Upgrade version to 4.0.8
	t.Run("MongoDB is reachable while version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.0.8"))
		t.Run("StatefulSet has OnDelete update strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 2))
	})

	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))

	// Downgrade version back to 4.0.6
	t.Run("MongoDB is reachable while version is downgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Version can be downgraded", mongodbtests.ChangeVersion(&mdb, "4.0.6"))
		t.Run("StatefulSet has OnDelete restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 3))
	})

	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))
}
