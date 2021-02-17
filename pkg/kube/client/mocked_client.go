package client

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// mockedClient dynamically creates maps to store instances of k8sClient.Object
type mockedClient struct {
	backingMap map[reflect.Type]map[k8sClient.ObjectKey]k8sClient.Object
}

// notFoundError returns an error which returns true for "errors.IsNotFound"
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func alreadyExistsError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonAlreadyExists}}
}

func NewMockedClient() k8sClient.Client {
	return &mockedClient{backingMap: map[reflect.Type]map[k8sClient.ObjectKey]k8sClient.Object{}}
}

func (m *mockedClient) ensureMapFor(obj k8sClient.Object) map[k8sClient.ObjectKey]k8sClient.Object {
	t := reflect.TypeOf(obj)
	if _, ok := m.backingMap[t]; !ok {
		m.backingMap[t] = map[k8sClient.ObjectKey]k8sClient.Object{}
	}
	return m.backingMap[t]
}

func (m *mockedClient) Get(_ context.Context, key k8sClient.ObjectKey, obj k8sClient.Object) error {
	relevantMap := m.ensureMapFor(obj)
	if val, ok := relevantMap[key]; ok {
		if currSts, ok := val.(*appsv1.StatefulSet); ok {
			// TODO: this currently doesn't work with additional mongodb config
			// just doing it for StatefulSets for now
			objCopy := currSts.DeepCopyObject()
			v := reflect.ValueOf(obj).Elem()
			v.Set(reflect.ValueOf(objCopy).Elem())
		} else {
			v := reflect.ValueOf(obj).Elem()
			v.Set(reflect.ValueOf(val).Elem())
		}
		return nil
	}
	return notFoundError()
}

func (m *mockedClient) Create(_ context.Context, obj k8sClient.Object, _ ...k8sClient.CreateOption) error {
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	if _, ok := relevantMap[objKey]; ok {
		return alreadyExistsError()
	}

	switch v := obj.(type) {
	case *appsv1.StatefulSet:
		makeStatefulSetReady(v)
	}

	relevantMap[objKey] = obj
	return nil
}

// makeStatefulSetReady configures the statefulset to be in the running state.
func makeStatefulSetReady(set *appsv1.StatefulSet) {
	set.Status.UpdatedReplicas = *set.Spec.Replicas
	set.Status.ReadyReplicas = *set.Spec.Replicas
}

func (m *mockedClient) List(_ context.Context, _ k8sClient.ObjectList, _ ...k8sClient.ListOption) error {
	return nil
}

func (m *mockedClient) Delete(_ context.Context, obj k8sClient.Object, _ ...k8sClient.DeleteOption) error {
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	delete(relevantMap, objKey)
	return nil
}

func (m *mockedClient) Update(_ context.Context, obj k8sClient.Object, _ ...k8sClient.UpdateOption) error {
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	relevantMap[objKey] = obj
	return nil
}

func (m *mockedClient) Patch(_ context.Context, _ k8sClient.Object, _ k8sClient.Patch, _ ...k8sClient.PatchOption) error {
	return nil
}

func (m *mockedClient) DeleteAllOf(_ context.Context, _ k8sClient.Object, _ ...k8sClient.DeleteAllOfOption) error {
	return nil
}

func (m *mockedClient) Status() k8sClient.StatusWriter {
	return m
}

func (m *mockedClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m *mockedClient) Scheme() *runtime.Scheme {
	return nil
}
