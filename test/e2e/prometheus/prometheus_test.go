package prometheus

import (
	"fmt"
	"os"
	"testing"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestPrometheus(t *testing.T) {
	resourceName := "mdb0"
	ctx, testConfig := setup.SetupWithTLS(t, resourceName)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, resourceName, testConfig.Namespace)

	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)
	mdb.Spec.Prometheus = e2eutil.NewPrometheusConfig(mdb.Namespace)

	_, err := setup.GeneratePasswordForUser(ctx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))

	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB with Prometheus configuration", func(t *testing.T) {
		t.Run("Resource has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls(mdb)))
		t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds(WithTls(mdb)))
		t.Run("Test Prometheus endpoint is active", tester.PrometheusEndpointIsReachable("prom-user", "prom-password", false))
		t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3, WithTls(mdb)))
		t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

		t.Run("Enabling HTTPS on the Prometheus endpoint", func(t *testing.T) {
			err = e2eutil.UpdateMongoDBResource(&mdb, func(mdb *v1.MongoDBCommunity) {
				mdb.Spec.Prometheus.TLSSecretRef.Name = "tls-certificate"
			})
			assert.NoError(t, err)

			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
			t.Run("Test Prometheus HTTPS endpoint is active", tester.PrometheusEndpointIsReachable("prom-user", "prom-password", true))
			t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 2))
		})
	})
}
