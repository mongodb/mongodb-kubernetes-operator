package replica_set_recovery

import (
	"context"
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
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func TestReplicaSetRecovery(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")
	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := mongotester.FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(ctx, &mdb, 1))
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))

	t.Run("MongoDB is reachable", func(t *testing.T) {
		defer tester.StartBackgroundConnectivityTest(t, time.Second*10)()
		n, err := rand.Int(rand.Reader, big.NewInt(int64(mdb.Spec.Members)))
		if err != nil {
			t.Fatal(err)
		}
		t.Run("Delete Random Pod", mongodbtests.DeletePod(ctx, &mdb, int(n.Int64())))
		t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		t.Run("Test Status Was Updated", mongodbtests.Status(ctx, &mdb, mdbv1.MongoDBCommunityStatus{
			MongoURI:                   mdb.MongoURI(""),
			Phase:                      mdbv1.Running,
			Version:                    mdb.GetMongoDBVersion(),
			CurrentMongoDBMembers:      3,
			CurrentStatefulSetReplicas: 3,
		}))

	})
}
