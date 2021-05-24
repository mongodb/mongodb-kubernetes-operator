package replica_set

import (
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"

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

	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	mdb.Spec.Version = "4.2.0"

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
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))

	// Upgrade minor version to 4.4.0
	t.Run("MongoDB is reachable while minor version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Minor Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.4.0"))
		t.Run("StatefulSet has OnDelete update strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 2))
	})

	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))

	// Upgrade patch version to 4.4.1
	t.Run("MongoDB is reachable while patch version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Patch Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.4.1"))
		t.Run("StatefulSet has OnDelete restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.OnDeleteStatefulSetStrategyType))
		t.Run("Stateful Set Reaches Ready State, after upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
		t.Run("AutomationConfig's version has been increased", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 3))
	})
	t.Run("StatefulSet has RollingUpgrade restart strategy", mongodbtests.StatefulSetHasUpdateStrategy(&mdb, appsv1.RollingUpdateStatefulSetStrategyType))
}
