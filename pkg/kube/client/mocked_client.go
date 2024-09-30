package client

import (
	"context"
	"encoding/json"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var (
	_ k8sClient.Client       = mockedClient{}
	_ k8sClient.StatusWriter = mockedStatusWriter{}
)

type patchValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// mockedClient dynamically creates maps to store instances of k8sClient.Object
type mockedClient struct {
	backingMap map[reflect.Type]map[k8sClient.ObjectKey]k8sClient.Object
}

func (m mockedClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	panic("not implemented")
}

func (m mockedClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	panic("not implemented")
}

func (m mockedClient) Create(_ context.Context, obj k8sClient.Object, _ ...k8sClient.CreateOption) error {
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

func (m mockedClient) Update(_ context.Context, obj k8sClient.Object, _ ...k8sClient.UpdateOption) error {
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	if _, ok := relevantMap[objKey]; !ok {
		return errors.NewNotFound(schema.GroupResource{}, obj.GetName())
	}
	relevantMap[objKey] = obj
	return nil
}

func (m mockedClient) Patch(_ context.Context, obj k8sClient.Object, patch k8sClient.Patch, _ ...k8sClient.PatchOption) error {
	if patch.Type() != types.JSONPatchType {
		return fmt.Errorf("patch types different from JSONPatchType are not yet implemented")
	}
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	var patches []patchValue
	data, err := patch.Data(obj)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &patches)
	if err != nil {
		return err
	}
	objectAnnotations := obj.GetAnnotations()
	for _, patch := range patches {
		if patch.Op != "replace" {
			return fmt.Errorf("patch operations different from \"replace\" are not yet implemented")
		}
		if !strings.HasPrefix(patch.Path, "/metadata/annotations") {
			return fmt.Errorf("patch that modify something different from annotations are not yet implemented")
		}
		if patch.Path == "/metadata/annotations" {
			objectAnnotations = map[string]string{}
			continue
		}
		pathElements := strings.SplitAfterN(patch.Path, "/metadata/annotations/", 2)
		finalPatchPath := strings.Replace(pathElements[1], "~1", "/", 1)
		switch val := patch.Value.(type) {
		case string:
			objectAnnotations[finalPatchPath] = val
		default:
			return fmt.Errorf("patch operations with values that are not strings are not implemented yet: %+v", pathElements[1])
		}
	}
	obj.SetAnnotations(objectAnnotations)
	relevantMap[objKey] = obj
	return nil
}

type mockedStatusWriter struct {
	parent mockedClient
}

func (m mockedStatusWriter) Create(ctx context.Context, obj k8sClient.Object, _ k8sClient.Object, _ ...k8sClient.SubResourceCreateOption) error {
	return m.parent.Create(ctx, obj)
}

func (m mockedStatusWriter) Update(ctx context.Context, obj k8sClient.Object, _ ...k8sClient.SubResourceUpdateOption) error {
	return m.parent.Update(ctx, obj)
}

func (m mockedStatusWriter) Patch(ctx context.Context, obj k8sClient.Object, patch k8sClient.Patch, _ ...k8sClient.SubResourcePatchOption) error {
	return m.parent.Patch(ctx, obj, patch)
}

func (m mockedClient) SubResource(string) k8sClient.SubResourceClient {
	panic("implement me")
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

func (m mockedClient) ensureMapFor(obj k8sClient.Object) map[k8sClient.ObjectKey]k8sClient.Object {
	t := reflect.TypeOf(obj)
	if _, ok := m.backingMap[t]; !ok {
		m.backingMap[t] = map[k8sClient.ObjectKey]k8sClient.Object{}
	}
	return m.backingMap[t]
}

func (m mockedClient) Get(_ context.Context, key k8sClient.ObjectKey, obj k8sClient.Object, _ ...k8sClient.GetOption) error {
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

// makeStatefulSetReady configures the stateful to be in the running state.
func makeStatefulSetReady(set *appsv1.StatefulSet) {
	set.Status.UpdatedReplicas = *set.Spec.Replicas
	set.Status.ReadyReplicas = *set.Spec.Replicas
}

func (m mockedClient) List(_ context.Context, _ k8sClient.ObjectList, _ ...k8sClient.ListOption) error {
	return nil
}

func (m mockedClient) Delete(_ context.Context, obj k8sClient.Object, _ ...k8sClient.DeleteOption) error {
	relevantMap := m.ensureMapFor(obj)
	objKey := k8sClient.ObjectKeyFromObject(obj)
	delete(relevantMap, objKey)
	return nil
}

func (m mockedClient) DeleteAllOf(_ context.Context, _ k8sClient.Object, _ ...k8sClient.DeleteAllOfOption) error {
	return nil
}

func (m mockedClient) Status() k8sClient.StatusWriter {
	return mockedStatusWriter{parent: m}
}

func (m mockedClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m mockedClient) Scheme() *runtime.Scheme {
	return nil
}
