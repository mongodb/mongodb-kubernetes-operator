package secret

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
func (c Client) GetSecret(objectKey k8sclient.ObjectKey) (corev1.Secret, error) {
	secret := corev1.Secret{}
	if err := c.client.Get(context.TODO(), objectKey, &secret); err != nil {
		return corev1.Secret{}, err
	}
	return secret, nil
}

// Update provides a thin wrapper and client.Client to update corev1.ConfigMap types
func (c Client) UpdateSecret(secret corev1.Secret) error {
	if err := c.client.Update(context.TODO(), &secret); err != nil {
		return err
	}
	return nil
}

// Create provides a thin wrapper and client.Client to create corev1.ConfigMap types
func (c Client) CreateSecret(secret corev1.Secret) error {
	if err := c.client.Create(context.TODO(), &secret); err != nil {
		return err
	}
	return nil
}
