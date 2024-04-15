package prometheus

import (
	"context"
	"fmt"
	"os"
	"testing"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
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
	ctx := context.Background()
	resourceName := "mdb0"
	testCtx, testConfig := setup.SetupWithTLS(ctx, t, resourceName)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, resourceName, testConfig.Namespace)

	mdb.Spec.Security.TLS = e2eutil.NewTestTLSConfig(false)
	mdb.Spec.Prometheus = e2eutil.NewPrometheusConfig(ctx, mdb.Namespace)

	_, err := setup.GeneratePasswordForUser(testCtx, user, testConfig.Namespace)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))

	mongodbtests.SkipTestIfLocal(t, "Ensure MongoDB with Prometheus configuration", func(t *testing.T) {
		t.Run("Resource has TLS Mode", tester.HasTlsMode("requireSSL", 60, WithTls(ctx, mdb)))
		t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds(WithTls(ctx, mdb)))
		t.Run("Test Prometheus endpoint is active", tester.PrometheusEndpointIsReachable("prom-user", "prom-password", false))
		t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3, WithTls(ctx, mdb)))
		t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))

		t.Run("Enabling HTTPS on the Prometheus endpoint", func(t *testing.T) {
			err = e2eutil.UpdateMongoDBResource(ctx, &mdb, func(mdb *v1.MongoDBCommunity) {
				mdb.Spec.Prometheus.TLSSecretRef.Name = "tls-certificate"
			})
			assert.NoError(t, err)

			t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
			t.Run("Test Prometheus HTTPS endpoint is active", tester.PrometheusEndpointIsReachable("prom-user", "prom-password", true))
			t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 2))
		})
	})
}
