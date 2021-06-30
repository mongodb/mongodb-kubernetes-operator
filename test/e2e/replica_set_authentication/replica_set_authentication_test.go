package replica_set_authentication

import (
	"fmt"
	"os"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
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
		t.Fatal(err)
	}
	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))

	// Run all the possible configuration using sha256 or sha1
	t.Run("Auth test with SHA-256", testConfigAuthentication(mdb, user, pw))
	t.Run("Auth test with SHA-256 and SHA-1", testConfigAuthentication(mdb, user, pw, withSha1()))
	t.Run("Auth test with SHA-256 (using label)", testConfigAuthentication(mdb, user, pw, withLabeledSha256()))
	t.Run("Auth test with SHA-256 (using label) and SHA-1", testConfigAuthentication(mdb, user, pw, withSha1(), withLabeledSha256()))
	t.Run("Auth test with SHA-1", testConfigAuthentication(mdb, user, pw, withSha1(), withoutSha256()))
}

type authOptions struct {
	sha256, sha1, useLabelForSha256 bool
}

func withoutSha256() func(*authOptions) {
	return func(opts *authOptions) {
		opts.sha256 = false
	}
}
func withLabeledSha256() func(*authOptions) {
	return func(opts *authOptions) {
		opts.sha256 = true
		opts.useLabelForSha256 = true
	}
}
func withSha1() func(*authOptions) {
	return func(opts *authOptions) {
		opts.sha1 = true
	}
}

// testConfigAuthentication run the tests using the autho ptions to update mdb and then checks that the resources are correctly configured
func testConfigAuthentication(mdb mdbv1.MongoDBCommunity, user mdbv1.MongoDBUser, pw string, allOptions ...func(*authOptions)) func(t *testing.T) {
	return func(t *testing.T) {

		pickedOpts := authOptions{
			sha256: true,
		}
		for _, opt := range allOptions {
			opt(&pickedOpts)
		}
		t.Logf("Config: use Sha256: %t (use label: %t), use Sha1: %t", pickedOpts.sha256, pickedOpts.useLabelForSha256, pickedOpts.sha1)

		enabledMechanisms := primitive.A{"SCRAM-SHA-256"}
		var acceptedModes []mdbv1.AuthMode
		if pickedOpts.sha256 {
			if pickedOpts.useLabelForSha256 {
				acceptedModes = append(acceptedModes, "SCRAM")
			} else {
				acceptedModes = append(acceptedModes, "SCRAM-SHA-256")
			}
		}
		if pickedOpts.sha1 {
			acceptedModes = append(acceptedModes, "SCRAM-SHA-1")
			if pickedOpts.sha256 {
				enabledMechanisms = primitive.A{"SCRAM-SHA-256", "SCRAM-SHA-1"}
			} else {
				enabledMechanisms = primitive.A{"SCRAM-SHA-1"}
			}
		}

		err := e2eutil.UpdateMongoDBResource(&mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Security.Authentication.Modes = acceptedModes
		})
		if err != nil {
			t.Fatal(err)
		}

		tester, err := FromResource(t, mdb)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
		if pickedOpts.sha256 {
			t.Run("Test Basic Connectivity with accepted auth", tester.ConnectivitySucceeds(WithScramWithAuth(user.Name, pw, "SCRAM-SHA-256")))
		} else {
			t.Run("Test Basic Connectivity with unaccepted auth", tester.ConnectivityFails(WithScramWithAuth(user.Name, pw, "SCRAM-SHA-256")))
		}
		if pickedOpts.sha1 {
			t.Run("Test Basic Connectivity with accepted auth", tester.ConnectivitySucceeds(WithScramWithAuth(user.Name, pw, "SCRAM-SHA-1")))
		} else {
			t.Run("Test Basic Connectivity with unaccepted auth", tester.ConnectivityFails(WithScramWithAuth(user.Name, pw, "SCRAM-SHA-1")))
		}

		if pickedOpts.sha256 {
			t.Run("Ensure Authentication", tester.EnsureAuthenticationWithAuthIsConfigured(3, enabledMechanisms, WithScramWithAuth(user.Name, pw, "SCRAM-SHA-256")))
		}
		if pickedOpts.sha1 {
			t.Run("Ensure Authentication", tester.EnsureAuthenticationWithAuthIsConfigured(3, enabledMechanisms, WithScramWithAuth(user.Name, pw, "SCRAM-SHA-1")))
		}
	}
}
