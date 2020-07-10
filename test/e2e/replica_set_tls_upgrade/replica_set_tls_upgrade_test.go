package replica_set_tls

import (
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/tlstests"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetTLSUpgrade(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)
	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb := e2eutil.NewTestMongoDB("mdb-tls")
	if err := setup.CreateTLSResources(mdb.Namespace, ctx); err != nil {
		t.Fatalf("Failed to set up TLS resources: %+v", err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", mongodbtests.Connectivity(&mdb))

	// Enable TLS as optional
	t.Run("MongoDB is reachable while TLS is being enabled", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Upgrade to TLS", tlstests.EnableTLS(&mdb, true))
			t.Run("Stateful Set Reaches Ready State, after enabling TLS", mongodbtests.StatefulSetIsReady(&mdb))
			t.Run("Wait for TLS to be enabled", tlstests.WaitForTLSMode(&mdb, "preferSSL"))
		},
	))

	// Ensure MongoDB is reachable both with and without TLS
	t.Run("Test Basic Connectivity", mongodbtests.Connectivity(&mdb))
	t.Run("Test Basic TLS Connectivity", tlstests.ConnectivityWithTLS(&mdb))

	// Make TLS required
	t.Run("MongoDB is reachable over TLS while making TLS required", tlstests.IsReachableOverTLSDuring(&mdb, time.Second*10,
		func() {
			t.Run("Make TLS required", tlstests.EnableTLS(&mdb, false))
			t.Run("Wait for TLS to be required", tlstests.WaitForTLSMode(&mdb, "requireSSL"))
		},
	))

	// Ensure MongoDB is reachable only over TLS
	t.Run("Test Basic TLS Connectivity", tlstests.ConnectivityWithTLS(&mdb))
	t.Run("Test TLS Required For Connectivity", tlstests.ConnectivityWithoutTLSShouldFail(&mdb))
}
