package configmap

import (
	"context"

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
func (c Client) GetConfigMap(objectKey k8sclient.ObjectKey) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	if err := c.client.Get(context.TODO(), objectKey, &cm); err != nil {
		return corev1.ConfigMap{}, err
	}
	return cm, nil
}

// Update provides a thin wrapper and client.Client to update corev1.ConfigMap types
func (c Client) UpdateConfigMap(cm corev1.ConfigMap) error {
	if err := c.client.Update(context.TODO(), &cm); err != nil {
		return err
	}
	return nil
}

// Create provides a thin wrapper and client.Client to create corev1.ConfigMap types
func (c Client) CreateConfigMap(cm corev1.ConfigMap) error {
	if err := c.client.Create(context.TODO(), &cm); err != nil {
		return err
	}
	return nil
}
