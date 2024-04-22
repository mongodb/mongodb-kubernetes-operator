package secret

import (
	"context"
	"testing"

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

func (c secretGetter) GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	if c.secret.Name == objectKey.Name && c.secret.Namespace == objectKey.Namespace {
		return c.secret, nil
	}
	return corev1.Secret{}, notFoundError()
}

func newGetter(s corev1.Secret) Getter {
	return secretGetter{
		secret: s,
	}
}

func TestReadKey(t *testing.T) {
	ctx := context.Background()
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetField("key1", "value1").
			SetField("key2", "value2").
			Build(),
	)

	value, err := ReadKey(ctx, getter, "key1", nsName("namespace", "name"))
	assert.Equal(t, "value1", value)
	assert.NoError(t, err)

	value, err = ReadKey(ctx, getter, "key2", nsName("namespace", "name"))
	assert.Equal(t, "value2", value)
	assert.NoError(t, err)

	_, err = ReadKey(ctx, getter, "key3", nsName("namespace", "name"))
	assert.Error(t, err)
}

func TestReadData(t *testing.T) {
	getter := newGetter(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetField("key1", "value1").
			SetField("key2", "value2").
			Build(),
	)
	t.Run("ReadStringData", func(t *testing.T) {
		ctx := context.Background()
		stringData, err := ReadStringData(ctx, getter, nsName("namespace", "name"))
		assert.NoError(t, err)

		assert.Contains(t, stringData, "key1")
		assert.Contains(t, stringData, "key2")

		assert.Equal(t, "value1", stringData["key1"])
		assert.Equal(t, "value2", stringData["key2"])
	})

	t.Run("ReadByteData", func(t *testing.T) {
		ctx := context.Background()
		data, err := ReadByteData(ctx, getter, nsName("namespace", "name"))
		assert.NoError(t, err)

		assert.Contains(t, data, "key1")
		assert.Contains(t, data, "key2")

		assert.Equal(t, []byte("value1"), data["key1"])
		assert.Equal(t, []byte("value2"), data["key2"])
	})

}

func nsName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: namespace}
}

func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

type secretGetUpdater struct {
	secret corev1.Secret
}

func (c secretGetUpdater) GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	if c.secret.Name == objectKey.Name && c.secret.Namespace == objectKey.Namespace {
		return c.secret, nil
	}
	return corev1.Secret{}, notFoundError()
}

func (c *secretGetUpdater) UpdateSecret(ctx context.Context, secret corev1.Secret) error {
	c.secret = secret
	return nil
}

func newGetUpdater(s corev1.Secret) GetUpdater {
	return &secretGetUpdater{
		secret: s,
	}
}

func TestUpdateField(t *testing.T) {
	ctx := context.Background()
	getUpdater := newGetUpdater(
		Builder().
			SetName("name").
			SetNamespace("namespace").
			SetField("field1", "value1").
			SetField("field2", "value2").
			Build(),
	)
	err := UpdateField(ctx, getUpdater, nsName("namespace", "name"), "field1", "newValue")
	assert.NoError(t, err)
	val, _ := ReadKey(ctx, getUpdater, "field1", nsName("namespace", "name"))
	assert.Equal(t, "newValue", val)
	val2, _ := ReadKey(ctx, getUpdater, "field2", nsName("namespace", "name"))
	assert.Equal(t, "value2", val2)
}

type mockSecretGetUpdateCreateDeleter struct {
	secrets  map[client.ObjectKey]corev1.Secret
	apiCalls int
}

func (c *mockSecretGetUpdateCreateDeleter) DeleteSecret(ctx context.Context, key client.ObjectKey) error {
	delete(c.secrets, key)
	c.apiCalls += 1
	return nil
}

func (c *mockSecretGetUpdateCreateDeleter) UpdateSecret(ctx context.Context, secret corev1.Secret) error {
	c.secrets[types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}] = secret
	c.apiCalls += 1
	return nil
}

func (c *mockSecretGetUpdateCreateDeleter) CreateSecret(ctx context.Context, secret corev1.Secret) error {
	return c.UpdateSecret(ctx, secret)
}

func (c *mockSecretGetUpdateCreateDeleter) GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	c.apiCalls += 1
	if s, ok := c.secrets[objectKey]; !ok {
		return corev1.Secret{}, notFoundError()
	} else {
		return s, nil
	}
}

func TestCreateOrUpdateIfNeededCreate(t *testing.T) {
	ctx := context.Background()
	mock := &mockSecretGetUpdateCreateDeleter{
		secrets:  map[client.ObjectKey]corev1.Secret{},
		apiCalls: 0,
	}

	secret := getDefaultSecret()

	// first time it does not exist, we create it
	err := CreateOrUpdateIfNeeded(ctx, mock, secret)
	assert.NoError(t, err)
	assert.Equal(t, 2, mock.apiCalls) // 2 calls -> get + creation
}

func TestCreateOrUpdateIfNeededUpdate(t *testing.T) {
	ctx := context.Background()
	mock := &mockSecretGetUpdateCreateDeleter{
		secrets:  map[client.ObjectKey]corev1.Secret{},
		apiCalls: 0,
	}
	secret := getDefaultSecret()

	{
		err := mock.CreateSecret(ctx, secret)
		assert.NoError(t, err)
		mock.apiCalls = 0
	}

	{
		secret.Data = map[string][]byte{"test": {1, 2, 3}}
		// secret differs -> we update
		err := CreateOrUpdateIfNeeded(ctx, mock, secret)
		assert.NoError(t, err)
		assert.Equal(t, 2, mock.apiCalls) // 2 calls -> get + update
	}
}

func TestCreateOrUpdateIfNeededEqual(t *testing.T) {
	ctx := context.Background()
	mock := &mockSecretGetUpdateCreateDeleter{
		secrets:  map[client.ObjectKey]corev1.Secret{},
		apiCalls: 0,
	}
	secret := getDefaultSecret()

	{
		err := mock.CreateSecret(ctx, secret)
		assert.NoError(t, err)
		mock.apiCalls = 0
	}

	{
		// the secret already exists, so we only call get
		err := CreateOrUpdateIfNeeded(ctx, mock, secret)
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.apiCalls) // 1 call -> get
	}
}

func getDefaultSecret() corev1.Secret {
	secret :=
		Builder().
			SetName("secret").
			SetNamespace("mdb.Namespace").
			SetStringMapToData(map[string]string{"password": "my-password"}).
			Build()
	return secret
}
