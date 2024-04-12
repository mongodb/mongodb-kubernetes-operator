package feature_compatibility_version

import (
	"context"
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// TestFeatureCompatibilityVersion test different scenarios of upgrading both FCV and image version. Note, that
// 4.4 images are the most convenient for this test as they support both FCV 4.2 and 4.4 and the underlying storage
// format remains the same. Versions 5 and 6 are one way upgrade only.
// See: https://www.mongodb.com/docs/manual/reference/command/setFeatureCompatibilityVersion/
func TestFeatureCompatibilityVersion(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	// This is the lowest version available for the official images
	const lowestMDBVersion = "4.4.16"
	const highestMDBVersion = "4.4.19"
	const featureCompatibility = "4.2"
	const upgradedFeatureCompatibility = "4.4"

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	mdb.Spec.Version = lowestMDBVersion
	mdb.Spec.FeatureCompatibilityVersion = featureCompatibility

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
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Upgrade while keeping the Feature Compatibility intact
	t.Run("MongoDB is reachable while version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*20)()
		t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(ctx, &mdb, highestMDBVersion))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
	})

	t.Run("Test Basic Connectivity after upgrade has completed", tester.ConnectivitySucceeds())
	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after upgrade, is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Downgrade while keeping the Feature Compatibility intact
	t.Run("MongoDB is reachable while version is downgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Version can be downgraded", mongodbtests.ChangeVersion(ctx, &mdb, lowestMDBVersion))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
	})

	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after downgrade, is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Upgrade the Feature Compatibility keeping the MongoDB version the same
	t.Run("Test FeatureCompatibilityVersion can be upgraded", func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(ctx, &mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.FeatureCompatibilityVersion = upgradedFeatureCompatibility
		})
		assert.NoError(t, err)
		t.Run("Stateful Set Reaches Ready State, after Upgrading FeatureCompatibilityVersion", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("MongoDB Reaches Running Phase, after Upgrading FeatureCompatibilityVersion", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	})

	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after upgrading FeatureCompatibilityVersion, is %s", upgradedFeatureCompatibility), tester.HasFCV(upgradedFeatureCompatibility, 10))
}
