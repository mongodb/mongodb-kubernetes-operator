package scram

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"go.uber.org/zap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
}

const (
	testSha1Salt      = "zEt5uDSnr/l9paFPsQzhAA=="
	testSha1ServerKey = "LEm/fv4gM0Y/XizbUoz/hULRnX0="
	testSha1StoredKey = "0HzXK7NtK40HXVn6zOqrNKVl+MY="

	testSha256Salt      = "qRr+7VgicfVcFjwZhu8u5JSE5ZeVBUP1A+lM4A=="
	testSha256ServerKey = "C9FIUhP6mqwe/2SJIheGBpOIqlxuq9Nh3fs+t+R/3zk="
	testSha256StoredKey = "7M7dUSY0sHTOXdNnoPSVbXg9Flon1b3t8MINGI8Tst0="
)

func newMockedSecretGetUpdateCreateDeleter(secrets ...corev1.Secret) secret.GetUpdateCreateDeleter {
	mockSecretGetUpdateCreateDeleter := mockSecretGetUpdateCreateDeleter{}
	mockSecretGetUpdateCreateDeleter.secrets = make(map[client.ObjectKey]corev1.Secret)
	for _, s := range secrets {
		mockSecretGetUpdateCreateDeleter.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	}
	return mockSecretGetUpdateCreateDeleter
}
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func TestReadExistingCredentials(t *testing.T) {
	mdbObjectKey := types.NamespacedName{Name: "mdb-0", Namespace: "default"}
	user := buildMongoDBUser("mdbuser-0")
	t.Run("credentials are successfully generated when all fields are present", func(t *testing.T) {
		scramCredsSecret := validScramCredentialsSecret(mdbObjectKey, user.ScramCredentialsSecretName)

		scram1Creds, scram256Creds, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, user.ScramCredentialsSecretName)
		assert.NoError(t, err)
		assertScramCredsCredentialsValidity(t, scram1Creds, scram256Creds)
	})

	t.Run("credentials are not generated if a field is missing", func(t *testing.T) {
		scramCredsSecret := invalidSecret(mdbObjectKey, user.ScramCredentialsSecretName)
		_, _, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, user.ScramCredentialsSecretName)
		assert.Error(t, err)
	})

	t.Run("credentials are not generated if the secret does not exist", func(t *testing.T) {
		scramCredsSecret := validScramCredentialsSecret(mdbObjectKey, user.ScramCredentialsSecretName)
		_, _, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, "different-username")
		assert.Error(t, err)
	})

}

func TestComputeScramCredentials_ComputesSameStoredAndServerKey_WithSameSalt(t *testing.T) {
	sha1Salt, sha256SaltKey, err := generate.Salts()
	assert.NoError(t, err)

	username := "user-1"
	password := "X6oSVAfD1la8fJwhfN" // nolint

	for i := 0; i < 10; i++ {
		sha1Creds0, sha256Creds0, err := computeScramShaCredentials(username, password, sha1Salt, sha256SaltKey)
		assert.NoError(t, err)
		sha1Creds1, sha256Creds1, err := computeScramShaCredentials(username, password, sha1Salt, sha256SaltKey)
		assert.NoError(t, err)

		assert.True(t, reflect.DeepEqual(sha1Creds0, sha1Creds1))
		assert.True(t, reflect.DeepEqual(sha256Creds0, sha256Creds1))
	}
}

func TestEnsureScramCredentials(t *testing.T) {
	mdb, user := buildConfigurableAndUser("mdb-0")
	t.Run("Fails when there is no password secret, and no credentials secret", func(t *testing.T) {
		_, _, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(), user, mdb.NamespacedName())
		assert.Error(t, err)
	})
	t.Run("Existing credentials are used when password does not exist, but credentials secret has been created", func(t *testing.T) {
		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.ScramCredentialsSecretName)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredentialsSecret), user, mdb.NamespacedName())
		assert.NoError(t, err)
		assertScramCredsCredentialsValidity(t, scram1Creds, scram256Creds)
	})
	t.Run("Changing password results in different credentials being returned", func(t *testing.T) {
		newPassword, err := generate.RandomFixedLengthStringOfSize(20)
		assert.NoError(t, err)

		differentPasswordSecret := secret.Builder().
			SetName(user.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(user.PasswordSecretKey, newPassword).
			Build()

		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.ScramCredentialsSecretName)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredentialsSecret, differentPasswordSecret), user, mdb.NamespacedName())
		assert.NoError(t, err)
		assert.NotEqual(t, testSha1Salt, scram1Creds.Salt)
		assert.NotEmpty(t, scram1Creds.Salt)
		assert.NotEqual(t, testSha1StoredKey, scram1Creds.StoredKey)
		assert.NotEmpty(t, scram1Creds.StoredKey)
		assert.NotEqual(t, testSha1StoredKey, scram1Creds.ServerKey)
		assert.NotEmpty(t, scram1Creds.ServerKey)
		assert.Equal(t, 10000, scram1Creds.IterationCount)

		assert.NotEqual(t, testSha256Salt, scram256Creds.Salt)
		assert.NotEmpty(t, scram256Creds.Salt)
		assert.NotEqual(t, testSha256StoredKey, scram256Creds.StoredKey)
		assert.NotEmpty(t, scram256Creds.StoredKey)
		assert.NotEqual(t, testSha256ServerKey, scram256Creds.ServerKey)
		assert.NotEmpty(t, scram256Creds.ServerKey)
		assert.Equal(t, 15000, scram256Creds.IterationCount)
	})

}

func TestConvertMongoDBUserToAutomationConfigUser(t *testing.T) {
	mdb, user := buildConfigurableAndUser("mdb-0")

	t.Run("When password exists, the user is created in the automation config", func(t *testing.T) {
		passwordSecret := secret.Builder().
			SetName(user.PasswordSecretName).
			SetNamespace(mdb.NamespacedName().Namespace).
			SetField(user.PasswordSecretKey, "TDg_DESiScDrJV6").
			Build()

		acUser, err := convertMongoDBUserToAutomationConfigUser(newMockedSecretGetUpdateCreateDeleter(passwordSecret), mdb.NamespacedName(), user)

		assert.NoError(t, err)
		assert.Equal(t, user.Username, acUser.Username)
		assert.Equal(t, user.Database, "admin")
		assert.Equal(t, len(user.Roles), len(acUser.Roles))
		assert.NotNil(t, acUser.ScramSha1Creds)
		assert.NotNil(t, acUser.ScramSha256Creds)
		for i, acRole := range acUser.Roles {
			assert.Equal(t, user.Roles[i].Name, acRole.Role)
			assert.Equal(t, user.Roles[i].Database, acRole.Database)
		}
	})

	t.Run("If there is no password secret, the creation fails", func(t *testing.T) {
		_, err := convertMongoDBUserToAutomationConfigUser(newMockedSecretGetUpdateCreateDeleter(), mdb.NamespacedName(), user)
		assert.Error(t, err)
	})
}

func TestConfigureScram(t *testing.T) {
	t.Run("Should fail if there is no password present for the user", func(t *testing.T) {
		mdb, _ := buildConfigurableAndUser("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()

		auth := automationconfig.Auth{}
		err := Enable(&auth, s, mdb)
		assert.Error(t, err)
	})
	t.Run("Agent Credentials Secret should be created if there are no users", func(t *testing.T) {
		mdb := buildConfigurable("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()
		auth := automationconfig.Auth{}
		err := Enable(&auth, s, mdb)
		assert.NoError(t, err)

		passwordSecret, err := s.GetSecret(mdb.GetAgentPasswordSecretNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(passwordSecret, AgentPasswordKey))
		assert.NotEmpty(t, passwordSecret.Data[AgentPasswordKey])

		keyfileSecret, err := s.GetSecret(mdb.GetAgentKeyfileSecretNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(keyfileSecret, AgentKeyfileKey))
		assert.NotEmpty(t, keyfileSecret.Data[AgentKeyfileKey])
	})

	t.Run("Agent Password Secret is used if it exists", func(t *testing.T) {
		mdb := buildConfigurable("mdb-0")

		agentPasswordSecret := secret.Builder().
			SetName(mdb.GetAgentPasswordSecretNamespacedName().Name).
			SetNamespace(mdb.GetAgentPasswordSecretNamespacedName().Namespace).
			SetField(AgentPasswordKey, "A21Zv5agv3EKXFfM").
			Build()

		s := newMockedSecretGetUpdateCreateDeleter(agentPasswordSecret)
		auth := automationconfig.Auth{}
		err := Enable(&auth, s, mdb)
		assert.NoError(t, err)

		ps, err := s.GetSecret(mdb.GetAgentPasswordSecretNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(ps, AgentPasswordKey))
		assert.NotEmpty(t, ps.Data[AgentPasswordKey])
		assert.Equal(t, "A21Zv5agv3EKXFfM", string(ps.Data[AgentPasswordKey]))

	})

	t.Run("Agent Keyfile Secret is used if present", func(t *testing.T) {
		mdb := buildConfigurable("mdb-0")

		keyfileSecret := secret.Builder().
			SetName(mdb.GetAgentKeyfileSecretNamespacedName().Name).
			SetNamespace(mdb.GetAgentKeyfileSecretNamespacedName().Namespace).
			SetField(AgentKeyfileKey, "RuPeMaIe2g0SNTTa").
			Build()

		s := newMockedSecretGetUpdateCreateDeleter(keyfileSecret)
		auth := automationconfig.Auth{}
		err := Enable(&auth, s, mdb)
		assert.NoError(t, err)

		ks, err := s.GetSecret(mdb.GetAgentKeyfileSecretNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(ks, AgentKeyfileKey))
		assert.Equal(t, "RuPeMaIe2g0SNTTa", string(ks.Data[AgentKeyfileKey]))

	})

	t.Run("Agent Credentials Secret should be created", func(t *testing.T) {
		mdb := buildConfigurable("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()
		auth := automationconfig.Auth{}
		err := Enable(&auth, s, mdb)
		assert.NoError(t, err)
	})
}

func buildConfigurable(name string, users ...User) Configurable {
	return mockConfigurable{
		opts: Options{
			AuthoritativeSet:   false,
			KeyFile:            "/path/to/keyfile",
			AutoAuthMechanisms: []string{Sha256},
			AgentName:          AgentName,
			AutoAuthMechanism:  Sha256,
		},
		users: users,
		nsName: types.NamespacedName{
			Name:      name,
			Namespace: "default",
		},
	}
}

func buildMongoDBUser(name string) User {
	return User{
		Username: fmt.Sprintf("%s-user", name),
		Database: "admin",
		Roles: []Role{
			{
				Database: "testing",
				Name:     "readWrite",
			},
			{
				Database: "testing",
				Name:     "clusterAdmin",
			},
			// admin roles for reading FCV
			{
				Database: "admin",
				Name:     "readWrite",
			},
			{
				Database: "admin",
				Name:     "clusterAdmin",
			},
		},
		PasswordSecretKey:  fmt.Sprintf("%s-password", name),
		PasswordSecretName: fmt.Sprintf("%s-password-secret", name),
	}

}

func buildConfigurableAndUser(name string) (Configurable, User) {
	mdb := buildConfigurable(name, User{
		Username: fmt.Sprintf("%s-user", name),
		Database: "admin",
		Roles: []Role{
			{
				Name:     "testing",
				Database: "readWrite",
			},
			{
				Database: "testing",
				Name:     "clusterAdmin",
			},
			// admin roles for reading FCV
			{
				Database: "admin",
				Name:     "readWrite",
			},
			{
				Database: "admin",
				Name:     "clusterAdmin",
			},
		},
		PasswordSecretKey:  fmt.Sprintf("%s-password", name),
		PasswordSecretName: fmt.Sprintf("%s-password-secret", name),
	})
	return mdb, mdb.GetScramUsers()[0]
}

func assertScramCredsCredentialsValidity(t *testing.T, scram1Creds, scram256Creds scramcredentials.ScramCreds) {
	assert.Equal(t, testSha1Salt, scram1Creds.Salt)
	assert.Equal(t, testSha1StoredKey, scram1Creds.StoredKey)
	assert.Equal(t, testSha1ServerKey, scram1Creds.ServerKey)
	assert.Equal(t, 10000, scram1Creds.IterationCount)

	assert.Equal(t, testSha256Salt, scram256Creds.Salt)
	assert.Equal(t, testSha256StoredKey, scram256Creds.StoredKey)
	assert.Equal(t, testSha256ServerKey, scram256Creds.ServerKey)
	assert.Equal(t, 15000, scram256Creds.IterationCount)
}

// validScramCredentialsSecret returns a secret that has all valid scram credentials
func validScramCredentialsSecret(objectKey types.NamespacedName, scramCredentialsSecretName string) corev1.Secret {
	return secret.Builder().
		SetName(scramCredentialsSecretName).
		SetNamespace(objectKey.Namespace).
		SetField(sha1SaltKey, testSha1Salt).
		SetField(sha1StoredKeyKey, testSha1StoredKey).
		SetField(sha1ServerKeyKey, testSha1ServerKey).
		SetField(sha256SaltKey, testSha256Salt).
		SetField(sha256StoredKeyKey, testSha256StoredKey).
		SetField(sha256ServerKeyKey, testSha256ServerKey).
		Build()
}

// invalidSecret returns a secret that is incomplete
func invalidSecret(objectKey types.NamespacedName, scramCredentialsSecretName string) corev1.Secret {
	return secret.Builder().
		SetName(scramCredentialsSecretName).
		SetNamespace(objectKey.Namespace).
		SetField(sha1SaltKey, "nxBSYyZZIBZxStyt").
		SetField(sha1StoredKeyKey, "Bs4sePK0cdMy6n").
		SetField(sha1ServerKeyKey, "eP6_p76ql_h8iiH").
		Build()
}
