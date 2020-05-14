package feature_compatibility_version_upgrade

import (
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestFeatureCompatibilityVersion(t *testing.T) {
	ctx := f.NewContext(t)
	defer ctx.Cleanup()
	if err := e2eutil.RegisterTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	mdb := e2eutil.NewTestMongoDB()
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))

	t.Run("Test FeatureCompatibilityVersion is 4.0", mongodbtests.FeatureCompatibilityVersion(&mdb, "4.0", 1))
	// Upgrade version to 4.2.6 while keeping the FCV set to 4.0
	t.Run("MongoDB is reachable", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(&mdb, "4.2.6"))
			t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetIsUpdated(&mdb))
			t.Run("Test Basic Connectivity after upgrade has completed", mongodbtests.BasicConnectivity(&mdb))
		},
	))
	t.Run("Test FeatureCompatibilityVersion, after upgrade, is 4.0", mongodbtests.FeatureCompatibilityVersion(&mdb, "4.0", 1))

	t.Run("MongoDB is reachable", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Test FCV can be upgraded", func(t *testing.T) {
				err := e2eutil.UpdateMongoDBResource(&mdb, func(db *mdbv1.MongoDB) {
					db.Spec.FeatureCompatibilityVersion = "4.2"
				})
				assert.NoError(t, err)
			})
			t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsUpdated(&mdb))
			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		},
	))

	t.Run("Test FeatureCompatibilityVersion, after upgrade, is 4.2", mongodbtests.FeatureCompatibilityVersion(&mdb, "4.2", 3))
}
