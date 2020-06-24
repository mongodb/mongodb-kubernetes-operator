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

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
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

func ReadByteData(getter Getter, key client.ObjectKey) (map[string][]byte, error) {
	secret, err := getter.GetSecret(key)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func ReadStringData(getter Getter, key client.ObjectKey) (map[string]string, error) {
	secret, err := getter.GetSecret(key)
	if err != nil {
		return nil, err
	}
	return secret.StringData, nil
}

func UpdateField(getUpdater GetUpdater, objectKey client.ObjectKey, key, value string) error {
	secret, err := getUpdater.GetSecret(objectKey)
	if err != nil {
		return err
	}
	secret.Data[key] = []byte(value)
	return getUpdater.UpdateSecret(secret)
}

func CreateOrUpdate(getUpdateCreator GetUpdateCreator, secret corev1.Secret) error {
	secret, err := getUpdateCreator.GetSecret(types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateSecret(secret)
		}
		return err
	}
	return getUpdateCreator.UpdateSecret(secret)
}
