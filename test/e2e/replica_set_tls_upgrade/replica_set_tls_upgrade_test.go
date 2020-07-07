package replica_set_tls

import (
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
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

	mdb := e2eutil.NewTestMongoDB("mdb0")
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Config Map Was Correctly Created", mongodbtests.AutomationConfigConfigMapExists(&mdb))
	t.Run("Stateful Set Reaches Ready State", mongodbtests.StatefulSetIsReady(&mdb))
	t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
	t.Run("Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb))
	t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
		mdbv1.MongoDBStatus{
			MongoURI: mdb.MongoURI(),
			Phase:    mdbv1.Running,
		}))

	// Enable TLS as optional
	t.Run("MongoDB is reachable while TLS is being enabled", mongodbtests.IsReachableDuring(&mdb, time.Second*10,
		func() {
			t.Run("Create TLS Resources", mongodbtests.CreateTLSResources(&mdb, ctx))
			t.Run("Upgrade to TLS", mongodbtests.EnableTLS(&mdb, true))
			t.Run("Stateful Set Reaches Ready State, after enabling TLS", mongodbtests.StatefulSetIsReady(&mdb))
			t.Run("Wait for TLS to be enabled", mongodbtests.WaitForSetting(&mdb, "sslMode", "preferSSL"))
		},
	))

	// Ensure MongoDB is reachable both with and without TLS
	t.Run("Test Basic Connectivity", mongodbtests.BasicConnectivity(&mdb))
	t.Run("Test Basic TLS Connectivity", mongodbtests.BasicConnectivityWithTLS(&mdb))

	// Make TLS required
	t.Run("MongoDB is reachable while making TLS required", mongodbtests.IsReachableDuring(&mdb, time.Second*10, func() {
		t.Run("MongoDB is reachable over TLS while making TLS required", mongodbtests.IsReachableOverTLSDuring(&mdb, time.Second*10,
			func() {
				t.Run("Make TLS required", mongodbtests.EnableTLS(&mdb, false))
				t.Run("Wait for TLS to be required", mongodbtests.WaitForSetting(&mdb, "sslMode", "requireSSL"))
			},
		))
	}))

	// Ensure MongoDB is reachable only over TLS
	t.Run("Test Basic TLS Connectivity", mongodbtests.BasicConnectivityWithTLS(&mdb))
	t.Run("Test TLS required", mongodbtests.BasicConnectivityTLSRequired(&mdb))
}
