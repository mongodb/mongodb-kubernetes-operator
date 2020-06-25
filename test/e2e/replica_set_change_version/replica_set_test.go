package replica_set

import (
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
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

	mdb := e2eutil.NewTestMongoDB("mdb0")
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb))
	t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb))
	t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
		mdbv1.MongoDBStatus{
			MongoURI: mdb.MongoURI(),
			Phase:    mdbv1.Running,
		}))

	// Upgrade version to 4.0.8
	t.Run("MongoDB is reachable while version is upgraded", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.0.8"))
			t.Run("StatefulSet has OnDelete update strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsUpdated(&mdb))
			t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 2))

		},
	))
	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))

	// Downgrade version back to 4.0.6
	t.Run("MongoDB is reachable while version is downgraded", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test Version can be downgraded", mongodbtests.ChangeVersion(&mdb, "4.0.6"))
			t.Run("StatefulSet has OnDelete restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsUpdated(&mdb))
			t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 3))

		},
	))
	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))
}
