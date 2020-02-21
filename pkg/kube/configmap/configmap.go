package configmap

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient(client k8sclient.Client) Client {
	return Client{
		client: client,
	}
}

type Client struct {
	client k8sclient.Client
}

// Get provides a thin wrapper and client.client to access corev1.ConfigMap types
func (c Client) Get(key k8sclient.ObjectKey) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	if err := c.client.Get(context.TODO(), key, &cm); err != nil {
		return corev1.ConfigMap{}, err
	}
	return cm, nil
}

// Update provides a thin wrapper and client.Client to update corev1.ConfigMap types
func (c Client) Update(cm corev1.ConfigMap) error {
	if err := c.client.Update(context.TODO(), &cm); err != nil {
		return err
	}
	return nil
}

// Create provides a thin wrapper and client.Client to create corev1.ConfigMap types
func (c Client) Create(cm corev1.ConfigMap) error {
	if err := c.client.Create(context.TODO(), &cm); err != nil {
		return err
	}
	return nil
}

// Delete provides a thin wrapper and client.Client to delete corev1.ConfigMap types
func (c Client) Delete(cm corev1.ConfigMap) error {
	if err := c.client.Delete(context.TODO(), &cm); err != nil {
		return err
	}
	return nil
}

// GetData extracts the contents of the Data field in a given config map
func (c Client) GetData(key k8sclient.ObjectKey) (map[string]string, error) {
	cm, err := c.Get(key)
	if err != nil {
		return nil, err
	}
	return cm.Data, nil
}

// GetData extracts the contents of the Data field in a given config map
func (c Client) ReadKey(key string, objectKey k8sclient.ObjectKey) (string, error) {
	data, err := c.GetData(objectKey)
	if err != nil {
		return "", err
	}
	if val, ok := data[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("key \"%s\" not present in ConfigMap %s/%s", key, objectKey.Namespace, objectKey.Name)
}
