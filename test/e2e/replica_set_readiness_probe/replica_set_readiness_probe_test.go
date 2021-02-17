package replica_set_readiness_probe

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
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

func TestReplicaSetReadinessProbeScaling(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0", "")
	_, err := setup.GeneratePasswordForUser(user, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))

	t.Run("MongoDB is reachable", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		n, err := rand.Int(rand.Reader, big.NewInt(int64(mdb.Spec.Members)))
		if err != nil {
			t.Fatal(err)
		}
		t.Run("Delete Random Pod", mongodbtests.DeletePod(&mdb, int(n.Int64())))
		t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetIsReady(&mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
			mdbv1.MongoDBCommunityStatus{
				MongoURI:                   mdb.MongoURI(),
				Phase:                      mdbv1.Running,
				CurrentMongoDBMembers:      3,
				CurrentStatefulSetReplicas: 3,
			}))

	})
}
