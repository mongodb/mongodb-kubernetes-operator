package scram

import (
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type secretGetter struct {
	secret corev1.Secret
}

func (c secretGetter) GetSecret(objectKey client.ObjectKey) (corev1.Secret, error) {
	if c.secret.Name == objectKey.Name && c.secret.Namespace == objectKey.Namespace {
		return c.secret, nil
	}
	return corev1.Secret{}, notFoundError()
}

func newGetter(s corev1.Secret) secret.Getter {
	return secretGetter{
		secret: s,
	}
}
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func TestReadExistingCredentials(t *testing.T) {
	mdbObjectKey := types.NamespacedName{Name: "mdb-0", Namespace: "default"}
	username := "mdbuser-0"
	t.Run("credentials are successfully generated when all fields are present", func(t *testing.T) {
		scramCredsSecret := validSecret(mdbObjectKey, username)

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
		scramCredsSecret := validSecret(mdbObjectKey, username)
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

func validSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder(). // valid secret
					SetName(scramCredentialsSecretName(objectKey.Name, username)).
					SetNamespace(objectKey.Namespace).
					SetField(sha1SaltKey, "ZRc7UfzR9P_-4qsp8PE").
					SetField(sha1StoredKey, "Bs4sePK0cdMy6n").
					SetField(sha1ServerKey, "eP6_p76ql_h8iiH").
					SetField(sha256SaltKey, "nyAbfVeXCWsLKoxOl").
					SetField(sha256StoredKey, "sMEAesMtaSYyaD7").
					SetField(sha256ServerKey, "IgXDX8uTN2JzN510NFlq").
					Build()
}

func invalidSecret(objectKey types.NamespacedName, username string) corev1.Secret {
	return secret.Builder().
		SetName(scramCredentialsSecretName(objectKey.Name, username)).
		SetNamespace(objectKey.Namespace).
		SetField(sha1SaltKey, "ZRc7UfzR9P_-4qsp8PE").
		SetField(sha1StoredKey, "Bs4sePK0cdMy6n").
		SetField(sha1ServerKey, "eP6_p76ql_h8iiH").
		Build()
}
