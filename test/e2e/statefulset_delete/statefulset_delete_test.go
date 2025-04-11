package statefulset_delete

import (
	"context"
	"fmt"
	"os"
	"testing"

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

func TestStatefulSetDelete(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))

	t.Run("Operator recreates StatefulSet", func(t *testing.T) {
		t.Run("Delete Statefulset", mongodbtests.DeleteStatefulSet(ctx, &mdb))
		t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetBecomesReady(ctx, &mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
		t.Run("Test Status Was Updated", mongodbtests.Status(ctx, &mdb, mdbv1.MongoDBCommunityStatus{
			MongoURI:              mdb.MongoURI(""),
			Phase:                 mdbv1.Running,
			Version:               mdb.GetMongoDBVersion(),
			CurrentMongoDBMembers: mdb.DesiredReplicas(),
		}))
	})
}
