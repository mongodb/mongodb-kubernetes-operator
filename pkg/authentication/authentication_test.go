package authentication

import (
	"context"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/mocks"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/stretchr/testify/assert"
)

func TestEnable(t *testing.T) {
	ctx := context.Background()
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

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
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

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
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
		agentSecret := x509.CreateAgentCertificateSecret("tls.crt", false, mdb.AgentCertificateSecretNamespacedName())
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret)

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 1)
		assert.Equal(t, "CN=my-user,OU=organizationalunit,O=organization", auth.Users[0].Username)
		assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB,C=US", auth.AutoUser)
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

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
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
		agentSecret := x509.CreateAgentCertificateSecret("tls.crt", false, mdb.AgentCertificateSecretNamespacedName())
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(passwordSecret, agentSecret)

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
		assert.NoError(t, err)

		assert.Equal(t, false, auth.Disabled)
		assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
		assert.Equal(t, []string{constants.Sha256, constants.X509}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
		assert.Len(t, auth.Users, 2)
		assert.Equal(t, "my-user", auth.Users[0].Username)
		assert.Equal(t, "CN=my-user,OU=organizationalunit,O=organization", auth.Users[1].Username)
		assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB,C=US", auth.AutoUser)
	})

}

func TestGetDeletedUsers(t *testing.T) {
	lastAppliedSpec := mdbv1.MongoDBCommunitySpec{
		Members:  3,
		Type:     "ReplicaSet",
		Version:  "7.0.2",
		Arbiters: 0,
		Security: mdbv1.Security{
			Authentication: mdbv1.Authentication{
				Modes: []mdbv1.AuthMode{"SCRAM"},
			},
		},
		Users: []mdbv1.MongoDBUser{
			{
				Name: "testUser",
				PasswordSecretRef: mdbv1.SecretKeyReference{
					Name: "password-secret-name",
				},
				ConnectionStringSecretName: "connection-string-secret",
				DB:                         "admin",
			},
		},
	}

	t.Run("no change same resource", func(t *testing.T) {
		actual := getRemovedUsersFromSpec(lastAppliedSpec, &lastAppliedSpec)

		var expected []automationconfig.DeletedUser
		assert.Equal(t, expected, actual)
	})

	t.Run("new user", func(t *testing.T) {
		current := mdbv1.MongoDBCommunitySpec{
			Members:  3,
			Type:     "ReplicaSet",
			Version:  "7.0.2",
			Arbiters: 0,
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			Users: []mdbv1.MongoDBUser{
				{
					Name: "testUser",
					PasswordSecretRef: mdbv1.SecretKeyReference{
						Name: "password-secret-name",
					},
					ConnectionStringSecretName: "connection-string-secret",
					DB:                         "admin",
				},
				{
					Name: "newUser",
					PasswordSecretRef: mdbv1.SecretKeyReference{
						Name: "new-password-secret-name",
					},
					ConnectionStringSecretName: "new-connection-string-secret",
					DB:                         "admin",
				},
			},
		}

		var expected []automationconfig.DeletedUser
		actual := getRemovedUsersFromSpec(current, &lastAppliedSpec)

		assert.Equal(t, expected, actual)
	})

	t.Run("removed one user", func(t *testing.T) {
		current := mdbv1.MongoDBCommunitySpec{
			Members:  3,
			Type:     "ReplicaSet",
			Version:  "7.0.2",
			Arbiters: 0,
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			Users: []mdbv1.MongoDBUser{},
		}

		expected := []automationconfig.DeletedUser{
			{
				User: "testUser",
				Dbs:  []string{"admin"},
			},
		}
		actual := getRemovedUsersFromSpec(current, &lastAppliedSpec)

		assert.Equal(t, expected, actual)
	})
}

func buildConfigurable(name string, auth []string, agent string, users ...authtypes.User) mocks.MockConfigurable {
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
