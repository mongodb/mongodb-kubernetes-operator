package statefulset_delete

import (
	"fmt"
	"os"
	"testing"

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

func TestStatefulSetDelete(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))

	t.Run("Operator recreates StatefulSet", func(t *testing.T) {
		t.Run("Delete Statefulset", mongodbtests.DeleteStatefulSet(&mdb))
		t.Run("Test Replica Set Recovers", mongodbtests.StatefulSetBecomesReady(&mdb))
		t.Run("MongoDB Reaches Running Phase", mongodbtests.MongoDBReachesRunningPhase(&mdb))
		t.Run("Test Status Was Updated", mongodbtests.Status(&mdb,
			mdbv1.MongoDBCommunityStatus{
				MongoURI:              mdb.MongoURI(""),
				Phase:                 mdbv1.Running,
				Version:               mdb.GetMongoDBVersion(),
				CurrentMongoDBMembers: mdb.DesiredReplicas(),
			}))
	})
}
