package replica_set_custom_role

import (
	"fmt"
	"os"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"

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

func TestReplicaSetCustomRole(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	someDB := "test"
	someCollection := "foo"
	anyDB := ""
	anyCollection := ""

	mdb, user := e2eutil.NewTestMongoDB("mdb0", "")
	mdb.Spec.Security.Roles = []mdbv1.CustomRole{
		{
			Role: "testRole",
			DB:   "admin",
			Privileges: []mdbv1.Privilege{
				{
					Resource: mdbv1.Resource{DB: &anyDB, Collection: &someCollection},
					Actions:  []string{"collStats", "find"},
				},
				{
					Resource: mdbv1.Resource{DB: &someDB, Collection: &anyCollection},
					Actions:  []string{"dbStats"},
				},
				{
					Resource: mdbv1.Resource{DB: &someDB, Collection: &someCollection},
					Actions:  []string{"collStats", "createCollection", "dbStats", "find"},
				},
			},
			Roles: []mdbv1.Role{},
		},
		{
			Role: "testClusterRole",
			DB:   "admin",
			Privileges: []mdbv1.Privilege{{
				Resource: mdbv1.Resource{Cluster: true},
				Actions:  []string{"dbStats", "find"},
			}},
			Roles: []mdbv1.Role{},
		},
		{
			Role: "testAnyResourceRole",
			DB:   "admin",
			Privileges: []mdbv1.Privilege{{
				Resource: mdbv1.Resource{AnyResource: true},
				Actions:  []string{"anyAction"},
			}},
			Roles: []mdbv1.Role{},
		},
	}

	_, err := setup.GeneratePasswordForUser(user, ctx, "")
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

	// Verify automation config roles and roles created in admin database.
	roles := mdbv1.ConvertCustomRolesToAutomationConfigCustomRole(mdb.Spec.Security.Roles)
	t.Run("AutomationConfig has the correct custom role", mongodbtests.AutomationConfigHasTheExpectedCustomRoles(&mdb, roles))
	t.Run("Custom Role was created ", tester.VerifyRoles(roles, 1))

}
