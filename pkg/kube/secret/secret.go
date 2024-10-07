package secret

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error)
}

type Updater interface {
	UpdateSecret(ctx context.Context, secret corev1.Secret) error
}

type Creator interface {
	CreateSecret(ctx context.Context, secret corev1.Secret) error
}

type Deleter interface {
	DeleteSecret(ctx context.Context, key client.ObjectKey) error
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

func ReadKey(ctx context.Context, getter Getter, key string, objectKey client.ObjectKey) (string, error) {
	data, err := ReadStringData(ctx, getter, objectKey)
	if err != nil {
		return "", err
	}
	if val, ok := data[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf(`key "%s" not present in the Secret %s/%s`, key, objectKey.Namespace, objectKey.Name)
}

// ReadByteData reads the Data field of the secret with the given objectKey
func ReadByteData(ctx context.Context, getter Getter, objectKey client.ObjectKey) (map[string][]byte, error) {
	secret, err := getter.GetSecret(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

// ReadStringData reads the StringData field of the secret with the given objectKey
func ReadStringData(ctx context.Context, getter Getter, key client.ObjectKey) (map[string]string, error) {
	secret, err := getter.GetSecret(ctx, key)
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
func UpdateField(ctx context.Context, getUpdater GetUpdater, objectKey client.ObjectKey, key, value string) error {
	secret, err := getUpdater.GetSecret(ctx, objectKey)
	if err != nil {
		return err
	}
	secret.Data[key] = []byte(value)
	return getUpdater.UpdateSecret(ctx, secret)
}

// CreateOrUpdate creates the Secret if it doesn't exist, other wise it updates it
func CreateOrUpdate(ctx context.Context, getUpdateCreator GetUpdateCreator, secret corev1.Secret) error {
	if err := getUpdateCreator.UpdateSecret(ctx, secret); err != nil {
		if SecretNotExist(err) {
			return getUpdateCreator.CreateSecret(ctx, secret)
		} else {
			return err
		}
	}
	return nil
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
func EnsureSecretWithKey(ctx context.Context, secretGetUpdateCreateDeleter GetUpdateCreateDeleter, nsName types.NamespacedName, ownerReferences []metav1.OwnerReference, key, value string) (string, error) {
	existingSecret, err0 := secretGetUpdateCreateDeleter.GetSecret(ctx, nsName)
	if err0 != nil {
		if SecretNotExist(err0) {
			s := Builder().
				SetNamespace(nsName.Namespace).
				SetName(nsName.Name).
				SetField(key, value).
				SetOwnerReferences(ownerReferences).
				Build()

			if err1 := secretGetUpdateCreateDeleter.CreateSecret(ctx, s); err1 != nil {
				return "", err1
			}
			return value, nil
		}
		return "", err0
	}
	return string(existingSecret.Data[key]), nil
}

// CopySecret copies secret object(data) from one cluster client to another, the from and to cluster-client can belong to the same or different clusters
func CopySecret(ctx context.Context, fromClient Getter, toClient GetUpdateCreator, sourceSecretNsName, destNsName types.NamespacedName) error {
	s, err := fromClient.GetSecret(ctx, sourceSecretNsName)
	if err != nil {
		return err
	}

	secretCopy := Builder().
		SetName(destNsName.Name).
		SetNamespace(destNsName.Namespace).
		SetByteData(s.Data).
		SetDataType(s.Type).
		Build()

	return CreateOrUpdate(ctx, toClient, secretCopy)
}

// Exists return whether a secret with the given namespaced name exists
func Exists(ctx context.Context, secretGetter Getter, nsName types.NamespacedName) (bool, error) {
	_, err := secretGetter.GetSecret(ctx, nsName)

	if err != nil {
		if apiErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// HasOwnerReferences checks whether an existing secret has a given set of owner references.
func HasOwnerReferences(secret corev1.Secret, ownerRefs []metav1.OwnerReference) bool {
	secretRefs := secret.GetOwnerReferences()
	for _, ref := range ownerRefs {
		if !contains.OwnerReferences(secretRefs, ref) {
			return false
		}
	}
	return true
}

// CreateOrUpdateIfNeeded creates a secret if it doesn't exist, or updates it if needed.
func CreateOrUpdateIfNeeded(ctx context.Context, getUpdateCreator GetUpdateCreator, secret corev1.Secret) error {
	// Check if the secret exists
	oldSecret, err := getUpdateCreator.GetSecret(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateSecret(ctx, secret)
		}
		return err
	}

	// Our secret builder never sets or uses secret.stringData, so we should only rely on secret.Data
	if reflect.DeepEqual(secret.Data, oldSecret.Data) {
		return nil
	}

	// They are different so we need to update it
	return getUpdateCreator.UpdateSecret(ctx, secret)
}

func SecretNotExist(err error) bool {
	if err == nil {
		return false
	}
	return apiErrors.IsNotFound(err) || strings.Contains(err.Error(), "secret not found")
}
