package secret

import (
	"reflect"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetSecret(objectKey client.ObjectKey) (corev1.Secret, error)
}

type Updater interface {
	UpdateSecret(secret corev1.Secret) error
}

type Creator interface {
	CreateSecret(secret corev1.Secret) error
}

type Deleter interface {
	DeleteSecret(objectKey client.ObjectKey) error
}

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
}

type GetUpdateCreateDeleter interface {
	Getter
	Updater
	Creator
	Deleter
}

func ReadKey(getter Getter, key string, objectKey client.ObjectKey) (string, error) {
	data, err := ReadStringData(getter, objectKey)
	if err != nil {
		return "", err
	}
	if val, ok := data[key]; ok {
		return val, nil
	}
	return "", errors.Errorf(`key "%s" not present in the Secret %s/%s`, key, objectKey.Namespace, objectKey.Name)
}

// ReadByteData reads the Data field of the secret with the given objectKey
func ReadByteData(getter Getter, objectKey client.ObjectKey) (map[string][]byte, error) {
	secret, err := getter.GetSecret(objectKey)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

// ReadStringData reads the StringData field of the secret with the given objectKey
func ReadStringData(getter Getter, key client.ObjectKey) (map[string]string, error) {
	secret, err := getter.GetSecret(key)
	if err != nil {
		return nil, err
	}

	return dataToStringData(secret.Data), nil
}

func dataToStringData(data map[string][]byte) map[string]string {
	stringData := make(map[string]string)
	for k, v := range data {
		stringData[k] = string(v)
	}
	return stringData
}

// UpdateField updates a single field in the secret with the provided objectKey
func UpdateField(getUpdater GetUpdater, objectKey client.ObjectKey, key, value string) error {
	secret, err := getUpdater.GetSecret(objectKey)
	if err != nil {
		return err
	}
	secret.Data[key] = []byte(value)
	return getUpdater.UpdateSecret(secret)
}

// CreateOrUpdate creates the Secret if it doesn't exist, other wise it updates it
func CreateOrUpdate(getUpdateCreator GetUpdateCreator, secret corev1.Secret) error {
	_, err := getUpdateCreator.GetSecret(types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateSecret(secret)
		}
		return err
	}
	return getUpdateCreator.UpdateSecret(secret)
}

// HasAllKeys returns true if the provided secret contains an element for every
// key provided. False if a single element is absent
func HasAllKeys(secret corev1.Secret, keys ...string) bool {
	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return false
		}
	}
	return true
}

// EnsureSecretWithKey makes sure the Secret with the given name has a key with the given value if the key is not already present.
// if the key is present, it will return the existing value associated with this key.
func EnsureSecretWithKey(secretGetUpdateCreateDeleter GetUpdateCreateDeleter, nsName types.NamespacedName, ownerReferences []metav1.OwnerReference, key, value string) (string, error) {
	existingSecret, err0 := secretGetUpdateCreateDeleter.GetSecret(nsName)
	if err0 != nil {
		if apiErrors.IsNotFound(err0) {
			s := Builder().
				SetNamespace(nsName.Namespace).
				SetName(nsName.Name).
				SetField(key, value).
				SetOwnerReferences(ownerReferences).
				Build()

			if err1 := secretGetUpdateCreateDeleter.CreateSecret(s); err1 != nil {
				return "", err1
			}
			return value, nil
		}
		return "", err0
	}
	return string(existingSecret.Data[key]), nil
}

// CopySecret copies secret object(data) from one cluster client to another, the from and to cluster-client can belong to the same or different clusters
func CopySecret(fromClient Getter, toClient GetUpdateCreator, sourceSecretNsName, destNsName types.NamespacedName) error {
	s, err := fromClient.GetSecret(sourceSecretNsName)
	if err != nil {
		return err
	}

	secretCopy := Builder().
		SetName(destNsName.Name).
		SetNamespace(destNsName.Namespace).
		SetByteData(s.Data).
		SetDataType(s.Type).
		Build()

	return CreateOrUpdate(toClient, secretCopy)
}

// Exists return whether a secret with the given namespaced name exists
func Exists(secretGetter Getter, nsName types.NamespacedName) (bool, error) {
	_, err := secretGetter.GetSecret(nsName)

	if err != nil {
		if apiErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateOrUpdateIfNeeded creates a secret if it doesn't exists, or updates it if needed.
func CreateOrUpdateIfNeeded(getUpdateCreator GetUpdateCreator, secret corev1.Secret) error {
	// Check if the secret exists
	olsSecret, err := getUpdateCreator.GetSecret(types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	if err != nil {

		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateSecret(secret)
		}
		return err
	}

	if reflect.DeepEqual(secret.StringData, dataToStringData(olsSecret.Data)) {
		return nil
	}

	// They are different so we need to update it
	return getUpdateCreator.UpdateSecret(secret)
}
