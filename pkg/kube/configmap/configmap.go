package configmap

import (
	"strings"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetConfigMap(objectKey client.ObjectKey) (corev1.ConfigMap, error)
}

type Updater interface {
	UpdateConfigMap(cm corev1.ConfigMap) error
}

type Creator interface {
	CreateConfigMap(cm corev1.ConfigMap) error
}

type Deleter interface {
	DeleteConfigMap(key client.ObjectKey) error
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

const (
	lineSeparator     = "\n"
	keyValueSeparator = "="
)

// ReadKey accepts a ConfigMap Getter, the object of the ConfigMap to get, and the key within
// the config map to read. It returns the string value, and an error if one occurred.
func ReadKey(getter Getter, key string, objectKey client.ObjectKey) (string, error) {
	data, err := ReadData(getter, objectKey)
	if err != nil {
		return "", err
	}
	if val, ok := data[key]; ok {
		return val, nil
	}
	return "", errors.Errorf("key \"%s\" not present in ConfigMap %s/%s", key, objectKey.Namespace, objectKey.Name)
}

// ReadData extracts the contents of the Data field in a given config map
func ReadData(getter Getter, key client.ObjectKey) (map[string]string, error) {
	cm, err := getter.GetConfigMap(key)
	if err != nil {
		return nil, err
	}
	return cm.Data, nil
}

// UpdateField updates the sets "key" to the given "value"
func UpdateField(getUpdater GetUpdater, objectKey client.ObjectKey, key, value string) error {
	cm, err := getUpdater.GetConfigMap(objectKey)
	if err != nil {
		return err
	}
	cm.Data[key] = value
	return getUpdater.UpdateConfigMap(cm)
}

// CreateOrUpdate creates the given ConfigMap if it doesn't exist,
// or updates it if it does.
func CreateOrUpdate(getUpdateCreator GetUpdateCreator, cm corev1.ConfigMap) error {
	_, err := getUpdateCreator.GetConfigMap(types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateConfigMap(cm)
		}
		return err
	}
	return getUpdateCreator.UpdateConfigMap(cm)
}

// filelikePropertiesToMap converts a file-like field in a ConfigMap to a map[string]string.
func filelikePropertiesToMap(s string) (map[string]string, error) {
	keyValPairs := map[string]string{}
	s = strings.TrimRight(s, lineSeparator)
	for _, keyValPair := range strings.Split(s, lineSeparator) {
		splittedPair := strings.Split(keyValPair, keyValueSeparator)
		if len(splittedPair) != 2 {
			return nil, errors.Errorf("%s is not a valid key-value pair", keyValPair)
		}
		keyValPairs[splittedPair[0]] = splittedPair[1]
	}
	return keyValPairs, nil
}

// ReadFileLikeField reads a ConfigMap with file-like properties and returns the value inside one of the fields.
func ReadFileLikeField(getter Getter, objectKey client.ObjectKey, externalKey string, internalKey string) (string, error) {
	cmData, err := ReadData(getter, objectKey)
	if err != nil {
		return "", err
	}
	mappingString, ok := cmData[externalKey]
	if !ok {
		return "", errors.Errorf("key %s is not present in ConfigMap %s", externalKey, objectKey)
	}
	mapping, err := filelikePropertiesToMap(mappingString)
	if err != nil {
		return "", err
	}
	value, ok := mapping[internalKey]
	if !ok {
		return "", errors.Errorf("key %s is not present in the %s field of ConfigMap %s", internalKey, externalKey, objectKey)
	}
	return value, nil
}

// Exists return whether a configmap with the given namespaced name exists
func Exists(cmGetter Getter, nsName types.NamespacedName) (bool, error) {
	_, err := cmGetter.GetConfigMap(nsName)

	if err != nil {
		if apiErrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
