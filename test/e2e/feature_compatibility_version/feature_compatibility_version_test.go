package feature_compatibility_version

import (
	"testing"
	"time"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestFeatureCompatibilityVersion(t *testing.T) {

	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, _ := e2eutil.NewTestMongoDB("mdb0")

	//password, err := setup.GeneratePasswordForUser(user, ctx)
	//if err != nil {
	//	t.Fatal(err)
	//}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))

	t.Run("Test FeatureCompatibilityVersion is 4.0", mongodbtests.HasFeatureCompatibilityVersion(&mdb, "4.0", 3))
	// Upgrade version to 4.2.6 while keeping the FCV set to 4.0
	t.Run("MongoDB is reachable while version is upgraded", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.2.6"))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsReady(&mdb))
		},
	))
	t.Run("Test Basic Connectivity after upgrade has completed", mongodbtests.Connectivity(&mdb))
	t.Run("Test FeatureCompatibilityVersion, after upgrade, is 4.0", mongodbtests.HasFeatureCompatibilityVersion(&mdb, "4.0", 3))

	// Downgrade version back to 4.0.6, checks that the FeatureCompatibilityVersion stayed at 4.0
	t.Run("MongoDB is reachable while version is downgraded", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test Version can be downgraded", mongodbtests.ChangeVersion(&mdb, "4.0.6"))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsReady(&mdb))
		},
	))
	t.Run("Test FeatureCompatibilityVersion, after downgrade, is 4.0", mongodbtests.HasFeatureCompatibilityVersion(&mdb, "4.0", 3))
}
