package client

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient(c k8sClient.Client) Client {
	return client{
		Client: c,
	}
}

type Client interface {
	k8sClient.Client
	CreateOrUpdate(obj runtime.Object) error
}

type client struct {
	k8sClient.Client
}

// CreateOrUpdate will either Create the runtime.Object if it doesn't exist, or Update it
// if it does
func (c client) CreateOrUpdate(obj runtime.Object) error {
	err := c.Create(context.TODO(), obj)
	if errors.IsAlreadyExists(err) {
		return c.Update(context.TODO(), obj)
	}
	if err != nil {
		return err
	}
	return c.Update(context.TODO(), obj)
}
