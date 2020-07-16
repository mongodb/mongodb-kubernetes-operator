package scram

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
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

type secretGetter struct {
	secrets map[client.ObjectKey]corev1.Secret
}

func (c secretGetter) DeleteSecret(objectKey client.ObjectKey) error {
	delete(c.secrets, objectKey)
	return nil
}

func (c secretGetter) UpdateSecret(s corev1.Secret) error {
	c.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	return nil
}

func (c secretGetter) CreateSecret(secret corev1.Secret) error {
	return c.UpdateSecret(secret)
}

func (c secretGetter) GetSecret(objectKey client.ObjectKey) (corev1.Secret, error) {
	if s, ok := c.secrets[objectKey]; !ok {
		return corev1.Secret{}, notFoundError()
	} else {
		return s, nil
	}
}

func newMockedSecretGetUpdateCreateDeleter(secrets ...corev1.Secret) secret.GetUpdateCreateDeleter {
	g := secretGetter{}
	g.secrets = make(map[client.ObjectKey]corev1.Secret)
	for _, s := range secrets {
		g.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	}
	return g
}
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func TestReadExistingCredentials(t *testing.T) {
	mdbObjectKey := types.NamespacedName{Name: "mdb-0", Namespace: "default"}
	username := "mdbuser-0"
	t.Run("credentials are successfully generated when all fields are present", func(t *testing.T) {
		scramCredsSecret := validScramCredentialsSecret(mdbObjectKey, username)

		scram1Creds, scram256Creds, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, username)
		assert.NoError(t, err)
		assertScramCredsCredentialsValidity(t, scram1Creds, scram256Creds)
	})

	t.Run("credentials are not generated if a field is missing", func(t *testing.T) {
		scramCredsSecret := invalidSecret(mdbObjectKey, username)
		_, _, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, username)
		assert.Error(t, err)
	})

	t.Run("credentials are not generated if the secret does not exist", func(t *testing.T) {
		scramCredsSecret := validScramCredentialsSecret(mdbObjectKey, username)
		_, _, err := readExistingCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredsSecret), mdbObjectKey, "different-username")
		assert.Error(t, err)
	})

}

func TestComputeScramCredentials_ComputesSameStoredAndServerKey_WithSameSalt(t *testing.T) {
	sha1Salt, sha256SaltKey, err := generateSalts()
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
	mdb, user := buildMongoDBAndUser("mdb-0")
	t.Run("Fails when there is no password secret, and no credentials secret", func(t *testing.T) {
		_, _, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(), user, mdb)
		assert.Error(t, err)
	})
	t.Run("Existing credentials are used when password does not exist, but credentials secret has been created", func(t *testing.T) {
		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.Name)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredentialsSecret), user, mdb)
		assert.NoError(t, err)
		assertScramCredsCredentialsValidity(t, scram1Creds, scram256Creds)
	})
	t.Run("Changing password results in different credentials being returned", func(t *testing.T) {
		newPassword, err := generate.RandomFixedLengthStringOfSize(20)
		assert.NoError(t, err)

		differentPasswordSecret := secret.Builder().
			SetName(user.PasswordSecretRef.Name).
			SetNamespace(mdb.Namespace).
			SetField(user.PasswordSecretRef.Key, newPassword).
			Build()

		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.Name)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredentialsSecret, differentPasswordSecret), user, mdb)
		assert.NoError(t, err)
		assert.NotEqual(t, "nxBSYyZZIBZxStyt", scram1Creds.Salt)
		assert.NotEmpty(t, scram1Creds.Salt)
		assert.NotEqual(t, "MyZ6M67oK1EmwXH0wrlMm2oGF/A=", scram1Creds.StoredKey)
		assert.NotEmpty(t, scram1Creds.StoredKey)
		assert.NotEqual(t, "0o75SaMBt7uMGkjXddfhMEMbfbs=", scram1Creds.ServerKey)
		assert.NotEmpty(t, scram1Creds.ServerKey)
		assert.Equal(t, 10000, scram1Creds.IterationCount)

		assert.NotEqual(t, "lE--XcnhjEWM5ZjeVK1CoAN-F7_s", scram256Creds.Salt)
		assert.NotEmpty(t, scram256Creds.Salt)
		assert.NotEqual(t, "bsRzKAsJ8/tCdAN5VNWQmBtuMo/EKwsVqXfPkADgc/k=", scram256Creds.StoredKey)
		assert.NotEmpty(t, scram256Creds.StoredKey)
		assert.NotEqual(t, "HcvqR84CnQIM5JafNtAG9XJMHIB1I5DZs1Y63Pd84Tk=", scram256Creds.ServerKey)
		assert.NotEmpty(t, scram256Creds.ServerKey)
		assert.Equal(t, 15000, scram256Creds.IterationCount)
	})

	t.Run("Changing password results in different credentials being returned", func(t *testing.T) {
		differentPasswordSecret := secret.Builder().
			SetName(user.PasswordSecretRef.Name).
			SetNamespace(mdb.Namespace).
			SetField(user.PasswordSecretRef.Key, "TDg_DESiScDrJV6").
			Build()
		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.Name)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newMockedSecretGetUpdateCreateDeleter(scramCredentialsSecret, differentPasswordSecret), user, mdb)
		assert.NoError(t, err)
		assert.NotEqual(t, "UzNjdWsyUm51L01sYmV3enhybW1WQT09Cg==", scram1Creds.Salt)
		assert.NotEqual(t, "Bs4sePK0cdMy6n", scram1Creds.StoredKey)
		assert.NotEqual(t, "eP6_p76ql_h8iiH", scram1Creds.ServerKey)
		assert.Equal(t, 10000, scram1Creds.IterationCount)

		assert.NotEqual(t, "lE--XcnhjEWM5ZjeVK1CoAN-F7_s", scram256Creds.Salt)
		assert.NotEqual(t, "sMEAesMtaSYyaD7", scram256Creds.StoredKey)
		assert.NotEqual(t, "IgXDX8uTN2JzN510NFlq", scram256Creds.ServerKey)
		assert.Equal(t, 15000, scram256Creds.IterationCount)
	})
}

func TestConvertMongoDBUserToAutomationConfigUser(t *testing.T) {
	mdb, user := buildMongoDBAndUser("mdb-0")

	t.Run("When password exists, the user is created in the automation config", func(t *testing.T) {
		passwordSecret := secret.Builder().
			SetName(user.PasswordSecretRef.Name).
			SetNamespace(mdb.Namespace).
			SetField(user.PasswordSecretRef.Key, "TDg_DESiScDrJV6").
			Build()

		acUser, err := convertMongoDBUserToAutomationConfigUser(newMockedSecretGetUpdateCreateDeleter(passwordSecret), mdb, user)

		assert.NoError(t, err)
		assert.Equal(t, user.Name, acUser.Username)
		assert.Equal(t, user.DB, "admin")
		assert.Equal(t, len(user.Roles), len(acUser.Roles))
		assert.NotNil(t, acUser.ScramSha1Creds)
		assert.NotNil(t, acUser.ScramSha256Creds)
		for i, acRole := range acUser.Roles {
			assert.Equal(t, user.Roles[i].Name, acRole.Role)
			assert.Equal(t, user.Roles[i].DB, acRole.Database)
		}
	})

	t.Run("If there is no password secret, the creation fails", func(t *testing.T) {
		_, err := convertMongoDBUserToAutomationConfigUser(newMockedSecretGetUpdateCreateDeleter(), mdb, user)
		assert.Error(t, err)
	})
}

func TestEnsureEnabler(t *testing.T) {
	t.Run("Should fail if there is no password present for the user", func(t *testing.T) {
		mdb, _ := buildMongoDBAndUser("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()
		_, err := EnsureEnabler(s, mdb.ScramCredentialsNamespacedName(), mdb)
		assert.Error(t, err)
	})
	t.Run("Agent Credentials Secret should be created if there are no users", func(t *testing.T) {
		mdb := buildMongoDB("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()
		_, err := EnsureEnabler(s, mdb.ScramCredentialsNamespacedName(), mdb)
		assert.NoError(t, err)

		agentCredentialsSecret, err := s.GetSecret(mdb.ScramCredentialsNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(agentCredentialsSecret, AgentKeyfileKey, AgentPasswordKey))
		assert.NotEmpty(t, agentCredentialsSecret.Data[AgentPasswordKey])
		assert.NotEmpty(t, agentCredentialsSecret.Data[AgentKeyfileKey])
	})

	t.Run("Agent Secret is used if it exists", func(t *testing.T) {
		mdb := buildMongoDB("mdb-0")

		agentPasswordSecret := secret.Builder().
			SetName(mdb.ScramCredentialsNamespacedName().Name).
			SetNamespace(mdb.ScramCredentialsNamespacedName().Namespace).
			SetField(AgentPasswordKey, "A21Zv5agv3EKXFfM").
			SetField(AgentKeyfileKey, "RuPeMaIe2g0SNTTa").
			Build()

		s := newMockedSecretGetUpdateCreateDeleter(agentPasswordSecret)
		_, err := EnsureEnabler(s, mdb.ScramCredentialsNamespacedName(), mdb)
		assert.NoError(t, err)

		agentCredentialsSecret, err := s.GetSecret(mdb.ScramCredentialsNamespacedName())
		assert.NoError(t, err)
		assert.True(t, secret.HasAllKeys(agentCredentialsSecret, AgentKeyfileKey, AgentPasswordKey))
		assert.Equal(t, "A21Zv5agv3EKXFfM", string(agentCredentialsSecret.Data[AgentPasswordKey]))
		assert.Equal(t, "RuPeMaIe2g0SNTTa", string(agentCredentialsSecret.Data[AgentKeyfileKey]))

	})

	t.Run("Agent Credentials Secret should be created", func(t *testing.T) {
		mdb := buildMongoDB("mdb-0")
		s := newMockedSecretGetUpdateCreateDeleter()
		_, err := EnsureEnabler(s, mdb.NamespacedName(), mdb)
		assert.NoError(t, err)
	})
}

func buildMongoDB(name string) mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: mdbv1.MongoDBSpec{
			Members:                     3,
			Type:                        "ReplicaSet",
			Version:                     "4.0.6",
			FeatureCompatibilityVersion: "4.0",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			Users: []mdbv1.MongoDBUser{},
		},
	}
}

func buildMongoDBAndUser(name string) (mdbv1.MongoDB, mdbv1.MongoDBUser) {
	mdb := buildMongoDB(name)
	mdb.Spec.Users = []mdbv1.MongoDBUser{
		{
			Name: fmt.Sprintf("%s-user", name),
			DB:   "admin",
			PasswordSecretRef: mdbv1.SecretKeyReference{
				Key:  fmt.Sprintf("%s-password", name),
				Name: fmt.Sprintf("%s-password-secret", name),
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
			},
		},
	}

	return mdb, mdb.Spec.Users[0]
}

func assertScramCredsCredentialsValidity(t *testing.T, scram1Creds, scram256Creds scramcredentials.ScramCreds) {
	assert.Equal(t, "nxBSYyZZIBZxStyt", scram1Creds.Salt)
	assert.Equal(t, "MyZ6M67oK1EmwXH0wrlMm2oGF/A=", scram1Creds.StoredKey)
	assert.Equal(t, "0o75SaMBt7uMGkjXddfhMEMbfbs=", scram1Creds.ServerKey)
	assert.Equal(t, 10000, scram1Creds.IterationCount)

	assert.Equal(t, "lE--XcnhjEWM5ZjeVK1CoAN-F7_s", scram256Creds.Salt)
	assert.Equal(t, "bsRzKAsJ8/tCdAN5VNWQmBtuMo/EKwsVqXfPkADgc/k=", scram256Creds.StoredKey)
	assert.Equal(t, "HcvqR84CnQIM5JafNtAG9XJMHIB1I5DZs1Y63Pd84Tk=", scram256Creds.ServerKey)
	assert.Equal(t, 15000, scram256Creds.IterationCount)
}

func validScramCredentialsSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder(). // valid secret
					SetName(scramCredentialsSecretName(objectKey.Name, username)).
					SetNamespace(objectKey.Namespace).
					SetField(sha1SaltKey, "nxBSYyZZIBZxStyt").
					SetField(sha1StoredKeyKey, "MyZ6M67oK1EmwXH0wrlMm2oGF/A=").
					SetField(sha1ServerKeyKey, "0o75SaMBt7uMGkjXddfhMEMbfbs=").
					SetField(sha256SaltKey, "lE--XcnhjEWM5ZjeVK1CoAN-F7_s").
					SetField(sha256StoredKeyKey, "bsRzKAsJ8/tCdAN5VNWQmBtuMo/EKwsVqXfPkADgc/k=").
					SetField(sha256ServerKeyKey, "HcvqR84CnQIM5JafNtAG9XJMHIB1I5DZs1Y63Pd84Tk=").
					Build()
}

func invalidSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder().
		SetName(scramCredentialsSecretName(objectKey.Name, username)).
		SetNamespace(objectKey.Namespace).
		SetField(sha1SaltKey, "nxBSYyZZIBZxStyt").
		SetField(sha1StoredKeyKey, "Bs4sePK0cdMy6n").
		SetField(sha1ServerKeyKey, "eP6_p76ql_h8iiH").
		Build()
}
