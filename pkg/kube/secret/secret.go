package secret

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
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
	return "", fmt.Errorf("key \"%s\" not present in the Secret %s/%s", key, objectKey.Namespace, objectKey.Name)
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

	stringData := make(map[string]string)
	for k, v := range secret.Data {
		stringData[k] = string(v)
	}
	return stringData, nil
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
