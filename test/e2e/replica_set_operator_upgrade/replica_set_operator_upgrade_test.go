package replica_set_operator_upgrade

import (
	"fmt"
	"os"
	"testing"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetOperatorUpgrade(t *testing.T) {
	resourceName := "mdb0"
	testConfig := setup.LoadTestConfigFromEnv()
	ctx := setup.SetupWithTestConfig(t, testConfig, true, true, resourceName)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, resourceName, testConfig.Namespace)
	scramUser := mdb.GetScramUsers()[0]
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)
	mdb.Spec.Arbiters = 1
	mdb.Spec.Members = 2

	_, err := setup.GeneratePasswordForUser(ctx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb, true))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB TLS Configuration", func(t *testing.T) {
		t.Run("Has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls(mdb)))
		t.Run("Basic Connectivity Succeeds", tester.ConnectivitySucceeds(WithTls(mdb)))
		t.Run("SRV Connectivity Succeeds", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithTls(mdb)))
		t.Run("Basic Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser)), WithTls(mdb)))
		t.Run("SRV Connectivity With Generated Connection String Secret Succeeds",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser)), WithTls(mdb)))
		t.Run("Connectivity Fails", tester.ConnectivityFails(WithoutTls()))
		t.Run("Ensure authentication is configured", tester.EnsureAuthenticationIsConfigured(3, WithTls(mdb)))
	})

	// upgrade the operator to master
	config := setup.LoadTestConfigFromEnv()
	err = setup.DeployOperator(config, resourceName, true, false)
	assert.NoError(t, err)

	// Perform the basic tests
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb, true))
}

// TestReplicaSetOperatorUpgradeFrom0_7_2 is intended to be run locally not in CI.
// It simulates deploying cluster using community operator 0.7.2 and then upgrading it using newer version.
func TestReplicaSetOperatorUpgradeFrom0_7_2(t *testing.T) {
	t.Skip("Supporting this test in CI requires installing also CRDs from release v0.7.2")
	resourceName := "mdb-upg"
	testConfig := setup.LoadTestConfigFromEnv()

	// deploy operator and other components as it was at version 0.7.2
	testConfig.OperatorImage = "quay.io/mongodb/mongodb-kubernetes-operator:0.7.2"
	testConfig.VersionUpgradeHookImage = "quay.io/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.3"
	testConfig.ReadinessProbeImage = "quay.io/mongodb/mongodb-kubernetes-readinessprobe:1.0.6"
	testConfig.AgentImage = "quay.io/mongodb/mongodb-agent:11.0.5.6963-1"

	ctx := setup.SetupWithTestConfig(t, testConfig, true, false, resourceName)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, resourceName, "")
	scramUser := mdb.GetScramUsers()[0]
	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	runTests := func(t *testing.T) {
		t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
		t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb, true))
		t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
		t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
		t.Run("Has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls(mdb)))
		t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
		t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
		t.Run("Test Basic Connectivity with generated connection string secret",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetConnectionStringForUser(mdb, scramUser))))
		t.Run("Test SRV Connectivity with generated connection string secret",
			tester.ConnectivitySucceeds(WithURI(mongodbtests.GetSrvConnectionStringForUser(mdb, scramUser))))
		t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	}

	runTests(t)

	// When running against local operator we could stop here,
	// rescale helm operator deployment to zero and run local operator then.

	testConfig = setup.LoadTestConfigFromEnv()
	err = setup.DeployOperator(testConfig, resourceName, true, false)
	assert.NoError(t, err)

	runTests(t)
}
