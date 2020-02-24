package mock

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mockedClient dynamically creates maps to store instances of runtime.Object
type mockedClient struct {
	backingMap map[reflect.Type]map[client.ObjectKey]runtime.Object
}

// notFoundError returns an error which returns true for "errors.IsNotFound"
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func NewClient() client.Client {
	return &mockedClient{backingMap: map[reflect.Type]map[client.ObjectKey]runtime.Object{}}
}

func getObjectKey(obj runtime.Object) client.ObjectKey {
	ns := reflect.ValueOf(obj).Elem().FieldByName("Namespace").String()
	name := reflect.ValueOf(obj).Elem().FieldByName("Name").String()
	return types.NamespacedName{Name: name, Namespace: ns}
}

func (m *mockedClient) ensureMapFor(obj runtime.Object) map[client.ObjectKey]runtime.Object {
	t := reflect.TypeOf(obj)
	if _, ok := m.backingMap[t]; !ok {
		m.backingMap[t] = map[client.ObjectKey]runtime.Object{}
	}
	return m.backingMap[t]
}

func (m *mockedClient) Get(_ context.Context, key client.ObjectKey, obj runtime.Object) error {
	relevantMap := m.ensureMapFor(obj)
	if val, ok := relevantMap[key]; ok {
		v := reflect.ValueOf(obj).Elem()
		v.Set(reflect.ValueOf(val).Elem())
		return nil
	}
	return notFoundError()
}

func (m *mockedClient) Create(_ context.Context, obj runtime.Object, _ ...client.CreateOption) error {
	relevantMap := m.ensureMapFor(obj)
	relevantMap[getObjectKey(obj)] = obj
	return nil
}

func (m *mockedClient) List(_ context.Context, _ runtime.Object, _ ...client.ListOption) error {
	return nil
}

func (m *mockedClient) Delete(_ context.Context, _ runtime.Object, _ ...client.DeleteOption) error {
	return nil
}

func (m *mockedClient) Update(_ context.Context, _ runtime.Object, _ ...client.UpdateOption) error {
	return nil
}

func (m *mockedClient) Patch(_ context.Context, _ runtime.Object, _ client.Patch, _ ...client.PatchOption) error {
	return nil
}

func (m *mockedClient) DeleteAllOf(_ context.Context, _ runtime.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}

func (m *mockedClient) Status() client.StatusWriter {
	return nil
}
