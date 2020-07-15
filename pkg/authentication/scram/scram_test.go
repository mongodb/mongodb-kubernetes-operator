package scram

import (
	"fmt"
	"os"
	"reflect"
	"testing"

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

func newGetter(secrets ...corev1.Secret) secret.GetUpdateCreator {
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

		scram1Creds, scram256Creds, err := readExistingCredentials(newGetter(scramCredsSecret), mdbObjectKey, username)
		assert.NoError(t, err)

		assert.Equal(t, "ZRc7UfzR9P_-4qsp8PE", scram1Creds.Salt)
		assert.Equal(t, "Bs4sePK0cdMy6n", scram1Creds.StoredKey)
		assert.Equal(t, "eP6_p76ql_h8iiH", scram1Creds.ServerKey)
		assert.Equal(t, 10000, scram1Creds.IterationCount)

		assert.Equal(t, "nyAbfVeXCWsLKoxOl", scram256Creds.Salt)
		assert.Equal(t, "sMEAesMtaSYyaD7", scram256Creds.StoredKey)
		assert.Equal(t, "IgXDX8uTN2JzN510NFlq", scram256Creds.ServerKey)
		assert.Equal(t, 15000, scram256Creds.IterationCount)
	})

	t.Run("credentials are not generated if a field is missing", func(t *testing.T) {
		scramCredsSecret := invalidSecret(mdbObjectKey, username)
		_, _, err := readExistingCredentials(newGetter(scramCredsSecret), mdbObjectKey, username)
		assert.Error(t, err)
	})

	t.Run("credentials are not generated if the secret does not exist", func(t *testing.T) {
		scramCredsSecret := validScramCredentialsSecret(mdbObjectKey, username)
		_, _, err := readExistingCredentials(newGetter(scramCredsSecret), mdbObjectKey, "different-username")
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
		_, _, err := ensureScramCredentials(newGetter(), user, mdb)
		assert.Error(t, err)
	})
	t.Run("Credentials are fetched when password does not exist, but credential secret does", func(t *testing.T) {
		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.Name)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newGetter(scramCredentialsSecret), user, mdb)
		assert.NoError(t, err)
		assertScram256CredsCredentialsValidity(t, scram1Creds, scram256Creds)
	})
	t.Run("Changing password results in different credentials being returned", func(t *testing.T) {
		differentPasswordSecret := secret.Builder().
			SetName(user.PasswordSecretRef.Name).
			SetNamespace(mdb.Namespace).
			SetField(user.PasswordSecretRef.Key, "TDg_DESiScDrJV6").
			Build()
		scramCredentialsSecret := validScramCredentialsSecret(mdb.NamespacedName(), user.Name)
		scram1Creds, scram256Creds, err := ensureScramCredentials(newGetter(scramCredentialsSecret, differentPasswordSecret), user, mdb)
		assert.NoError(t, err)
		assert.NotEqual(t, "UzNjdWsyUm51L01sYmV3enhybW1WQT09Cg==", scram1Creds.Salt)
		assert.NotEqual(t, "Bs4sePK0cdMy6n", scram1Creds.StoredKey)
		assert.NotEqual(t, "eP6_p76ql_h8iiH", scram1Creds.ServerKey)
		assert.Equal(t, 10000, scram1Creds.IterationCount)

		assert.NotEqual(t, "Gy4ZNMr-SYEsEpAEZv", scram256Creds.Salt)
		assert.NotEqual(t, "sMEAesMtaSYyaD7", scram256Creds.StoredKey)
		assert.NotEqual(t, "IgXDX8uTN2JzN510NFlq", scram256Creds.ServerKey)
		assert.Equal(t, 15000, scram256Creds.IterationCount)

	})

}

func buildMongoDBAndUser(name string) (mdbv1.MongoDB, mdbv1.MongoDBUser) {
	mdb := mdbv1.MongoDB{
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
			Users: []mdbv1.MongoDBUser{
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
			},
		},
	}
	return mdb, mdb.Spec.Users[0]
}

func assertScram256CredsCredentialsValidity(t *testing.T, scram1Creds, scram256Creds scramcredentials.ScramCreds) {
	assert.Equal(t, "UzNjdWsyUm51L01sYmV3enhybW1WQT09Cg==", scram1Creds.Salt)
	assert.Equal(t, "Bs4sePK0cdMy6n", scram1Creds.StoredKey)
	assert.Equal(t, "eP6_p76ql_h8iiH", scram1Creds.ServerKey)
	assert.Equal(t, 10000, scram1Creds.IterationCount)

	assert.Equal(t, "ajdf1E1QTsNAQdBEodB4vzQOFuvcw9K6PmouVg==", scram256Creds.Salt)
	assert.Equal(t, "sMEAesMtaSYyaD7", scram256Creds.StoredKey)
	assert.Equal(t, "IgXDX8uTN2JzN510NFlq", scram256Creds.ServerKey)
	assert.Equal(t, 15000, scram256Creds.IterationCount)
}

func validScramCredentialsSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder(). // valid secret
					SetName(scramCredentialsSecretName(objectKey.Name, username)).
					SetNamespace(objectKey.Namespace).
					SetField(sha1SaltKey, "UzNjdWsyUm51L01sYmV3enhybW1WQT09Cg==").
					SetField(sha1StoredKeyKey, "Bs4sePK0cdMy6n").
					SetField(sha1ServerKeyKey, "eP6_p76ql_h8iiH").
					SetField(sha256SaltKey, "ajdf1E1QTsNAQdBEodB4vzQOFuvcw9K6PmouVg==").
					SetField(sha256StoredKeyKey, "sMEAesMtaSYyaD7").
					SetField(sha256ServerKeyKey, "IgXDX8uTN2JzN510NFlq").
					Build()
}

func invalidSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder().
		SetName(scramCredentialsSecretName(objectKey.Name, username)).
		SetNamespace(objectKey.Namespace).
		SetField(sha1SaltKey, "ZRc7UfzR9P_-4qsp8PE").
		SetField(sha1StoredKeyKey, "Bs4sePK0cdMy6n").
		SetField(sha1ServerKeyKey, "eP6_p76ql_h8iiH").
		Build()
}
