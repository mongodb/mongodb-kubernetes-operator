package authentication

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/mocks"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestEnable(t *testing.T) {
	t.Run("SCRAM only", func(t *testing.T) {
		auth := automationconfig.Auth{}
		user := mocks.BuildScramMongoDBUser("my-user")
		mdb := buildConfigurable("mdb", []string{constants.Sha256}, constants.Sha256, user)
		passwordSecret := secret.Builder().
			SetName(user.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(user.PasswordSecretKey, "TDg_DESiScDrJV6").
			Build()
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(passwordSecret)

		err := Enable(&auth, secrets, mdb)
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.Sha256, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.Sha256}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha256}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 1)
		assert.Equal(t, "my-user", auth.Users[0].Username)
		assert.Equal(t, "mms-automation", auth.AutoUser)
	})
	t.Run("SCRAM-SHA-256 and SCRAM-SHA-1", func(t *testing.T) {
		auth := automationconfig.Auth{}
		user := mocks.BuildScramMongoDBUser("my-user")
		mdb := buildConfigurable("mdb", []string{constants.Sha256, constants.Sha1}, constants.Sha256, user)
		passwordSecret := secret.Builder().
			SetName(user.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(user.PasswordSecretKey, "TDg_DESiScDrJV6").
			Build()
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(passwordSecret)

		err := Enable(&auth, secrets, mdb)
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.Sha256, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.Sha256, constants.Sha1}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha256, constants.Sha1}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 1)
		assert.Equal(t, "my-user", auth.Users[0].Username)
		assert.Equal(t, "mms-automation", auth.AutoUser)
	})
	t.Run("X509 only", func(t *testing.T) {
		auth := automationconfig.Auth{}
		user := mocks.BuildX509MongoDBUser("my-user")
		mdb := buildConfigurable("mdb", []string{constants.X509}, constants.X509, user)
		agentSecret := x509.CreateAgentCertificateSecret("tls.crt", mdb, false)
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret)

		err := Enable(&auth, secrets, mdb)
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 1)
		assert.Equal(t, "CN=my-user,OU=organizationalunit,O=organization", auth.Users[0].Username)
		assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB", auth.AutoUser)
	})
	t.Run("SCRAM and X509 with SCRAM agent", func(t *testing.T) {
		auth := automationconfig.Auth{}
		userScram := mocks.BuildScramMongoDBUser("my-user")
		userX509 := mocks.BuildX509MongoDBUser("my-user")
		mdb := buildConfigurable("mdb", []string{constants.Sha256, constants.X509}, constants.Sha256, userScram, userX509)
		passwordSecret := secret.Builder().
			SetName(userScram.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(userScram.PasswordSecretKey, "TDg_DESiScDrJV6").
			Build()
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(passwordSecret)

		err := Enable(&auth, secrets, mdb)
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.Sha256, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.Sha256, constants.X509}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha256}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 2)
		assert.Equal(t, "my-user", auth.Users[0].Username)
		assert.Equal(t, "CN=my-user,OU=organizationalunit,O=organization", auth.Users[1].Username)
		assert.Equal(t, "mms-automation", auth.AutoUser)
	})
	t.Run("SCRAM and X509 with X509 agent", func(t *testing.T) {
		auth := automationconfig.Auth{}
		userScram := mocks.BuildScramMongoDBUser("my-user")
		userX509 := mocks.BuildX509MongoDBUser("my-user")
		mdb := buildConfigurable("mdb", []string{constants.Sha256, constants.X509}, constants.X509, userScram, userX509)
		passwordSecret := secret.Builder().
			SetName(userScram.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(userScram.PasswordSecretKey, "TDg_DESiScDrJV6").
			Build()
		agentSecret := x509.CreateAgentCertificateSecret("tls.crt", mdb, false)
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(passwordSecret, agentSecret)

		err := Enable(&auth, secrets, mdb)
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.Sha256, constants.X509}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 2)
		assert.Equal(t, "my-user", auth.Users[0].Username)
		assert.Equal(t, "CN=my-user,OU=organizationalunit,O=organization", auth.Users[1].Username)
		assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB", auth.AutoUser)
	})

}

func buildConfigurable(name string, auth []string, agent string, users ...authtypes.User) authtypes.Configurable {
	return mocks.NewMockConfigurable(
		authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           "/path/to/keyfile",
			AuthMechanisms:    auth,
			AgentName:         constants.AgentName,
			AutoAuthMechanism: agent,
		},
		users,
		types.NamespacedName{
			Name:      name,
			Namespace: "default",
		},
		[]metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "mdbc",
			Name:       "my-ref",
		}},
	)
}
