package replica_set_scram

import (
	"fmt"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongodbtests"
	. "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/mongotester"
	"github.com/mongodb/mongodb-kubernetes-operator/test/e2e/setup"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/types"
)

func TestMain(m *testing.M) {
	f.MainEntry(m)
}

func TestReplicaSetScram(t *testing.T) {
	ctx, shouldCleanup := setup.InitTest(t)

	if shouldCleanup {
		defer ctx.Cleanup()
	}

	mdb, user := e2eutil.NewTestMongoDB("scram-mdb")
	correctPassword, err := setup.GeneratePasswordForUser(user, ctx)
	if err != nil {
		t.Fatal(err)
	}

	wrongPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		t.Fatal(err)
	}

	tester, err := FromResource(t, mdb)
	if err != nil {
		t.Fatal(err)
	}

	userCredentialsSecretKey := types.NamespacedName{
		Namespace: mdb.Namespace,
		Name:      fmt.Sprintf("%s-%s-scram-credentials", mdb.Name, user.Name),
	}
	expectedKeysInCredentialsSecret := []string{
		"sha1-salt",
		"sha256-salt",
		"sha-1-server-key",
		"sha-256-server-key",
		"sha-1-stored-key",
		"sha-256-stored-key",
	}

	t.Run("Create MongoDB Resource", mongodbtests.CreateMongoDBResource(&mdb, ctx))
	t.Run("Basic tests", mongodbtests.BasicFunctionality(&mdb))
	t.Run("User cannot authenticate with different password", tester.ConnectivityFails(WithScram(user.Name, wrongPassword)))
	t.Run("User can authenticate with correct password", tester.ConnectivitySucceeds(WithScram(user.Name, correctPassword)))
	t.Run("Credentials secret was generated with all the required values", mongodbtests.SecretHasKeys(userCredentialsSecretKey, expectedKeysInCredentialsSecret...))

}
