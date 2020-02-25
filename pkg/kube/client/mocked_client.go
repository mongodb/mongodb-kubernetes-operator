package client

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// mockedClient dynamically creates maps to store instances of runtime.Object
type mockedClient struct {
	backingMap map[reflect.Type]map[k8sClient.ObjectKey]runtime.Object
}

// notFoundError returns an error which returns true for "errors.IsNotFound"
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func alreadyExistsError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonAlreadyExists}}
}

func NewMockedClient() k8sClient.Client {
	return &mockedClient{backingMap: map[reflect.Type]map[k8sClient.ObjectKey]runtime.Object{}}
}

func getObjectKey(obj runtime.Object) k8sClient.ObjectKey {
	ns := reflect.ValueOf(obj).Elem().FieldByName("Namespace").String()
	name := reflect.ValueOf(obj).Elem().FieldByName("Name").String()
	return types.NamespacedName{Name: name, Namespace: ns}
}

func (m *mockedClient) ensureMapFor(obj runtime.Object) map[k8sClient.ObjectKey]runtime.Object {
	t := reflect.TypeOf(obj)
	if _, ok := m.backingMap[t]; !ok {
		m.backingMap[t] = map[k8sClient.ObjectKey]runtime.Object{}
	}
	return m.backingMap[t]
}

func (m *mockedClient) Get(_ context.Context, key k8sClient.ObjectKey, obj runtime.Object) error {
	relevantMap := m.ensureMapFor(obj)
	if val, ok := relevantMap[key]; ok {
		v := reflect.ValueOf(obj).Elem()
		v.Set(reflect.ValueOf(val).Elem())
		return nil
	}
	return notFoundError()
}

func (m *mockedClient) Create(_ context.Context, obj runtime.Object, _ ...k8sClient.CreateOption) error {
	relevantMap := m.ensureMapFor(obj)
	if _, ok := relevantMap[getObjectKey(obj)]; ok {
		return alreadyExistsError()
	}
	relevantMap[getObjectKey(obj)] = obj
	return nil
}

func (m *mockedClient) List(_ context.Context, _ runtime.Object, _ ...k8sClient.ListOption) error {
	return nil
}

func (m *mockedClient) Delete(_ context.Context, _ runtime.Object, _ ...k8sClient.DeleteOption) error {
	return nil
}

func (m *mockedClient) Update(_ context.Context, obj runtime.Object, _ ...k8sClient.UpdateOption) error {
	relevantMap := m.ensureMapFor(obj)
	relevantMap[getObjectKey(obj)] = obj
	return nil
}

func (m *mockedClient) Patch(_ context.Context, _ runtime.Object, _ k8sClient.Patch, _ ...k8sClient.PatchOption) error {
	return nil
}

func (m *mockedClient) DeleteAllOf(_ context.Context, _ runtime.Object, _ ...k8sClient.DeleteAllOfOption) error {
	return nil
}

func (m *mockedClient) Status() k8sClient.StatusWriter {
	return nil
}
