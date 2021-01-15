package replica_set_custom_role

import (
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	setup "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetCustomRole(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("mdb0", "")
	mdb.Spec.Roles = []mdbv1.AutomationConfigRole{{
		Role: "testRole",
		DB:   "admin",
		Privileges: []mdbv1.Privilege{
			{
				Resource: mdbv1.Resource{DB: "test", Collection: ""},
				Actions:  []string{"collStats", "createCollection", "dbStats", "find", "viewRole"},
			},
		},
		Roles: []mdbv1.Role{},
	}}

	password, err := setup.GeneratePasswordForUser(user, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("Keyfile authentication is configured", tester.HasKeyfileAuth(3))
	t.Run("Test Basic Connectivity", tester.ConnectivitySucceeds())
	t.Run("Ensure Authentication", tester.EnsureAuthenticationIsConfigured(3))
	t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

	roles := []automationconfig.CustomRole{
		automationconfig.NewCustomRoleBuilder().
			WithRole("testRole").
			WithDB("admin").
			AddPrivilege("test", "", false, false, []string{
				"collStats", "createCollection", "dbStats", "find", "viewRole",
			}).
			Build(),
	}
	t.Run("AutomationConfig has the correct custom role", mongodbtests.AutomationConfigHasTheExpectedCustomRoles(&mdb, roles))
	t.Run("Custom Role was created ", mongodbtests.ConnectAndVerifyExpectedCustomRoles(&mdb, "admin", user.Name, password, roles))

}
