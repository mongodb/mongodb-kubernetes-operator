package replica_set_remove_user

import (
	"context"
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code, err := e2eutil.RunTest(m)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

func intPtr(x int) *int       { return &x }
func strPtr(s string) *string { return &s }

func TestCleanupUsers(t *testing.T) {
	ctx := context.Background()
	testCtx := setup.Setup(ctx, t)
	defer testCtx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(testCtx, "mdb0", "")

	_, err := setup.GeneratePasswordForUser(testCtx, user, "")
	if err != nil {
		t.Fatal(err)
	}

	// config member options
	memberOptions := []automationconfig.MemberOptions{
		{
			Votes:    intPtr(1),
			Tags:     map[string]string{"foo1": "bar1"},
			Priority: strPtr("1.5"),
		},
		{
			Votes:    intPtr(1),
			Tags:     map[string]string{"foo2": "bar2"},
			Priority: strPtr("1"),
		},
		{
			Votes:    intPtr(1),
			Tags:     map[string]string{"foo3": "bar3"},
			Priority: strPtr("2.5"),
		},
	}
	mdb.Spec.MemberConfig = memberOptions

	settings := map[string]interface{}{
		"electionTimeoutMillis": float64(20),
	}
	mdb.Spec.AutomationConfigOverride = &mdbv1.AutomationConfigOverride{
		ReplicaSet: mdbv1.OverrideReplicaSet{Settings: mdbv1.MapWrapper{Object: settings}},
	}

	newUser := mdbv1.MongoDBUser{
		Name: fmt.Sprintf("%s-user-2", "mdb-0"),
		PasswordSecretRef: mdbv1.SecretKeyReference{
			Key:  fmt.Sprintf("%s-password-2", "mdb-0"),
			Name: fmt.Sprintf("%s-%s-password-secret-2", "mdb-0", testCtx.ExecutionId),
		},
		Roles: []mdbv1.Role{
			// roles on testing db for general connectivity
			{
				DB:   "testing",
				Name: "readWrite",
			},
			{
				DB:   "testing",
				Name: "clusterAdmin",
			},
			// admin roles for reading FCV
			{
				DB:   "admin",
				Name: "readWrite",
			},
			{
				DB:   "admin",
				Name: "clusterAdmin",
			},
			{
				DB:   "admin",
				Name: "userAdmin",
			},
		},
		ScramCredentialsSecretName: fmt.Sprintf("%s-my-scram-2", "mdb-0"),
	}

	_, err = setup.GeneratePasswordForUser(testCtx, newUser, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(ctx, t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, testCtx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(ctx, &mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Test SRV Connectivity", tester.ConnectivitySucceeds(WithURI(mdb.MongoSRVURI("")), WithoutTls(), WithReplicaSet(mdb.Name)))
	t.Run("Add new user to MongoDB Resource", mongodbtests.AddUserToMongoDBCommunity(ctx, &mdb, newUser))
	t.Run("MongoDB reaches Running phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	editedUser := mdb.Spec.Users[1]
	t.Run("Edit connection string secret name of the added user", mongodbtests.EditConnectionStringSecretNameOfLastUser(ctx, &mdb, "other-secret-name"))
	t.Run("MongoDB reaches Running phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	t.Run("Old connection string secret is cleaned up", mongodbtests.ConnectionStringSecretIsCleanedUp(ctx, &mdb, editedUser.GetConnectionStringSecretName(mdb.Name)))
	deletedUser := mdb.Spec.Users[1]
	t.Run("Remove last user from MongoDB Resource", mongodbtests.RemoveLastUserFromMongoDBCommunity(ctx, &mdb))
	t.Run("MongoDB reaches Pending phase", mongodbtests.MongoDBReachesPendingPhase(ctx, &mdb))
	t.Run("Removed users are added to automation config", mongodbtests.AuthUsersDeletedIsUpdated(ctx, &mdb, deletedUser))
	t.Run("MongoDB reaches Running phase", mongodbtests.MongoDBReachesRunningPhase(ctx, &mdb))
	t.Run("Connection string secrets are cleaned up", mongodbtests.ConnectionStringSecretIsCleanedUp(ctx, &mdb, deletedUser.GetConnectionStringSecretName(mdb.Name)))
	t.Run("Delete MongoDB Resource", mongodbtests.DeleteMongoDBResource(&mdb, testCtx))
}
