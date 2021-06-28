package replica_set_tls

import (
	"fmt"
	"os"
	"testing"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/mongotester"
	"go.mongodb.org/mongo-driver/bson/primitive"

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

func TestReplicaSetAuthentication(t *testing.T) {
	ctx := setup.Setup(t)
	defer ctx.Teardown()

	mdb, user := e2eutil.NewTestMongoDB(ctx, "mdb0", "")
	pw, err := setup.GeneratePasswordForUser(ctx, user, "")
	if err != nil {
		t.Logf("ERROR password generation")
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))

	// Run all the possible configuration using sha256 or sha1
	t.Run(fmt.Sprintf("auth sha256: %t (label %t), sha1: %t", true, true, false), testConfigAuthentication(true, false, true, mdb, user, pw, ctx))
	t.Run(fmt.Sprintf("auth sha256: %t (label %t), sha1: %t", true, false, false), testConfigAuthentication(true, false, false, mdb, user, pw, ctx))
	t.Run(fmt.Sprintf("auth sha256: %t (label %t), sha1: %t", true, false, true), testConfigAuthentication(true, true, false, mdb, user, pw, ctx))
	t.Run(fmt.Sprintf("auth sha256: %t (label %t), sha1: %t", true, true, true), testConfigAuthentication(true, true, true, mdb, user, pw, ctx))
	t.Run(fmt.Sprintf("auth sha256: %t (label %t), sha1: %t", false, false, true), testConfigAuthentication(false, true, false, mdb, user, pw, ctx))
}

func testConfigAuthentication(acceptSHA256 bool, acceptSHA1 bool, useLabelForSha256 bool, mdb v1.MongoDBCommunity, user v1.MongoDBUser, pw string, ctx *e2eutil.Context) func(t *testing.T) {
	return func(t *testing.T) {
		enabledMechanisms := primitive.A{"SCRAM-SHA-256"}
		var acceptedModes []v1.AuthMode
		if acceptSHA256 {
			if useLabelForSha256 {
				acceptedModes = append(acceptedModes, "SCRAM")
			} else {
				acceptedModes = append(acceptedModes, "SCRAM-SHA-256")
			}
		}
		if acceptSHA1 {
			acceptedModes = append(acceptedModes, "SCRAM-SHA-1")
			if acceptSHA256 {
				enabledMechanisms = primitive.A{"SCRAM-SHA-256", "SCRAM-SHA-1"}
			} else {
				enabledMechanisms = primitive.A{"SCRAM-SHA-1"}
			}
		}

		t.Logf("Changing authentication mode")
		err := e2eutil.UpdateMongoDBResource(&mdb, func(db *v1.MongoDBCommunity) {
			db.Spec.Security.Authentication.Modes = acceptedModes
		})
		if err != nil {
			t.Fatal(err)
		}

		tester, err := mongotester.FromResource(t, mdb)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
		if acceptSHA256 {
			t.Run("Test Basic Connectivity with accepted auth", tester.ConnectivitySucceeds(WithScramSha(user.Name, pw, "SCRAM-SHA-256")))
		} else {
			t.Run("Test Basic Connectivity with unaccepted auth", tester.ConnectivityFails(WithScramSha(user.Name, pw, "SCRAM-SHA-256")))
		}
		if acceptSHA1 {
			t.Run("Test Basic Connectivity with accepted auth", tester.ConnectivitySucceeds(WithScramSha(user.Name, pw, "SCRAM-SHA-1")))
		} else {
			t.Run("Test Basic Connectivity with unaccepted auth", tester.ConnectivityFails(WithScramSha(user.Name, pw, "SCRAM-SHA-1")))
		}

		// Check with Cian version 2 meaning
		//t.Run("AutomationConfig has the correct version", mongodbtests.AutomationConfigVersionHasTheExpectedVersion(&mdb, 1))

		if acceptSHA256 {
			t.Run("Ensure Authentication", tester.EnsureAuthenticationSHAIsConfigured(3, enabledMechanisms, WithScramSha(user.Name, pw, "SCRAM-SHA-256")))
		}
		if acceptSHA1 {
			t.Run("Ensure Authentication", tester.EnsureAuthenticationSHAIsConfigured(3, enabledMechanisms, WithScramSha(user.Name, pw, "SCRAM-SHA-1")))
		}
	}
}
