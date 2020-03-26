package client

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	objCopy := obj.DeepCopyObject()
	err := c.Get(context.TODO(), namespacedNameFromObject(obj), objCopy)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.TODO(), obj)
		}
		return err
	}
	return c.Update(context.TODO(), obj)
}

func namespacedNameFromObject(obj runtime.Object) types.NamespacedName {
	ns := reflect.ValueOf(obj).Elem().FieldByName("Namespace").String()
	name := reflect.ValueOf(obj).Elem().FieldByName("Name").String()
	return types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
}
