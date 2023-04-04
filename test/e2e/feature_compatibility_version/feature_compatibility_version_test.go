package feature_compatibility_version

import (
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestFeatureCompatibilityVersion(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	const lowestMDBVersion = "4.4.18"
	const highestMDBVersion = "5.0.15"
	const featureCompatibility = "4.0"
	const upgradedFeatureCompatibility = "4.4"

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	mdb.Spec.Version = lowestMDBVersion
	mdb.Spec.FeatureCompatibilityVersion = featureCompatibility

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
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Upgrade while keeping the Feature Compatibility intact
	t.Run("MongoDB is reachable while version is upgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*20)()
		t.Run("Test Version can be upgraded", mongodbtests.ChangeVersion(&mdb, highestMDBVersion))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
	})

	t.Run("Test Basic Connectivity after upgrade has completed", tester.ConnectivitySucceeds())
	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after upgrade, is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Downgrade while keeping the Feature Compatibility intact
	t.Run("MongoDB is reachable while version is downgraded", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		t.Run("Test Version can be downgraded", mongodbtests.ChangeVersion(&mdb, lowestMDBVersion))
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
	})

	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after downgrade, is %s", featureCompatibility), tester.HasFCV(featureCompatibility, 3))

	// Upgrade the Feature Compatibility keeping the MongoDB version the same
	t.Run("Test FeatureCompatibilityVersion can be upgraded", func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(&mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.FeatureCompatibilityVersion = upgradedFeatureCompatibility
		})
		assert.NoError(t, err)
		t.Run("Stateful Set Reaches Ready State, after Upgrading", mongodbtests.StatefulSetBecomesReady(&mdb))
	})

	t.Run(fmt.Sprintf("Test FeatureCompatibilityVersion, after downgrade, is %s", upgradedFeatureCompatibility), tester.HasFCV(upgradedFeatureCompatibility, 3))
}
