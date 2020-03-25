package client

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

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
	WaitForCondition(nsName types.NamespacedName, interval, duration time.Duration, obj runtime.Object, condition func() bool) (bool, error)
	UpdateLatest(nsName types.NamespacedName, obj runtime.Object, updateFunc func()) error
}

type client struct {
	k8sClient.Client
}

// WaitForCondition periodically fetches the runtime.Object with the given namespaced name and polls until
// the condition function returns true or it times out. The provided object "obj" is mutated. So it can
// be used in the condition function from an outer scope.
func (c client) WaitForCondition(nsName types.NamespacedName, interval, duration time.Duration, obj runtime.Object, condition func() bool) (bool, error) {
	err := wait.Poll(interval, duration, func() (done bool, err error) {
		if err := c.Get(context.TODO(), nsName, obj); err != nil {
			return false, err
		}
		return condition(), nil
	})
	return err == wait.ErrWaitTimeout, err
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

// UpdateLatest fetches the resource with the given NamespacedName, and applies the given
// updateFunc to the most recent version of the resource.
// the updateFunc is intended to accept an anonymous function which captures "obj" in the outer scope.
func (c client) UpdateLatest(nsName types.NamespacedName, obj runtime.Object, updateFunc func()) error {
	for i := 0; i < 3; i++ {
		err := c.Get(context.TODO(), nsName, obj)
		if err != nil {
			return err
		}

		// apply the function on the most recent version of the resource
		updateFunc()

		err = c.Update(context.TODO(), obj)
		if err == nil {
			return nil
		}
		if errors.IsConflict(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("the resource is experiencing some intensive concurrent modifications")
}

func namespacedNameFromObject(obj runtime.Object) types.NamespacedName {
	ns := reflect.ValueOf(obj).Elem().FieldByName("Namespace").String()
	name := reflect.ValueOf(obj).Elem().FieldByName("Name").String()
	return types.NamespacedName{
		Name:      name,
		Namespace: ns,
	}
}
