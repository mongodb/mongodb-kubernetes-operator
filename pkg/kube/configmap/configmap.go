package configmap

import (
	"fmt"

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

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
}

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
	return "", fmt.Errorf("key \"%s\" not present in ConfigMap %s/%s", key, objectKey.Namespace, objectKey.Name)
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
	cMap, err := getUpdateCreator.GetConfigMap(types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateConfigMap(cm)
		}
		return err
	}
	return getUpdateCreator.UpdateConfigMap(cMap)
}
