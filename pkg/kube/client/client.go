package client

import (
	"context"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/pod"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	KubernetesSecretClient
	// TODO: remove this function, add mongodb package which has GetAndUpdate function
	GetAndUpdate(ctx context.Context, nsName types.NamespacedName, obj k8sClient.Object, updateFunc func()) error
	configmap.GetUpdateCreateDeleter
	service.GetUpdateCreateDeleter
	statefulset.GetUpdateCreateDeleter
	pod.Getter
}

type KubernetesSecretClient interface {
	secret.GetUpdateCreateDeleter
}

type client struct {
	k8sClient.Client
}

// GetAndUpdate fetches the most recent version of the runtime.Object with the provided
// nsName and applies the update function. The update function should update "obj" from
// an outer scope
func (c client) GetAndUpdate(ctx context.Context, nsName types.NamespacedName, obj k8sClient.Object, updateFunc func()) error {
	err := c.Get(ctx, nsName, obj)
	if err != nil {
		return err
	}
	// apply the function on the most recent version of the resource
	updateFunc()
	return c.Update(ctx, obj)
}

// GetConfigMap provides a thin wrapper and client.client to access corev1.ConfigMap types
func (c client) GetConfigMap(ctx context.Context, objectKey k8sClient.ObjectKey) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	if err := c.Get(ctx, objectKey, &cm); err != nil {
		return corev1.ConfigMap{}, err
	}
	return cm, nil
}

// UpdateConfigMap provides a thin wrapper and client.Client to update corev1.ConfigMap types
func (c client) UpdateConfigMap(ctx context.Context, cm corev1.ConfigMap) error {
	return c.Update(ctx, &cm)
}

// CreateConfigMap provides a thin wrapper and client.Client to create corev1.ConfigMap types
func (c client) CreateConfigMap(ctx context.Context, cm corev1.ConfigMap) error {
	return c.Create(ctx, &cm)
}

// DeleteConfigMap deletes the configmap of the given object key
func (c client) DeleteConfigMap(ctx context.Context, key k8sClient.ObjectKey) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	return c.Delete(ctx, &cm)
}

// GetPod provides a thin wrapper and client.client to access corev1.Pod types.
func (c client) GetPod(ctx context.Context, objectKey k8sClient.ObjectKey) (corev1.Pod, error) {
	p := corev1.Pod{}
	if err := c.Get(ctx, objectKey, &p); err != nil {
		return corev1.Pod{}, err
	}
	return p, nil
}

// GetSecret provides a thin wrapper and client.Client to access corev1.Secret types
func (c client) GetSecret(ctx context.Context, objectKey k8sClient.ObjectKey) (corev1.Secret, error) {
	s := corev1.Secret{}
	if err := c.Get(ctx, objectKey, &s); err != nil {
		return corev1.Secret{}, err
	}
	return s, nil
}

// UpdateSecret provides a thin wrapper and client.Client to update corev1.Secret types
func (c client) UpdateSecret(ctx context.Context, secret corev1.Secret) error {
	return c.Update(ctx, &secret)
}

// CreateSecret provides a thin wrapper and client.Client to create corev1.Secret types
func (c client) CreateSecret(ctx context.Context, secret corev1.Secret) error {
	return c.Create(ctx, &secret)
}

// DeleteSecret provides a thin wrapper and client.Client to delete corev1.Secret types
func (c client) DeleteSecret(ctx context.Context, key k8sClient.ObjectKey) error {
	s := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
	}
	return c.Delete(ctx, &s)
}

// GetService provides a thin wrapper and client.Client to access corev1.Service types
func (c client) GetService(ctx context.Context, objectKey k8sClient.ObjectKey) (corev1.Service, error) {
	s := corev1.Service{}
	if err := c.Get(ctx, objectKey, &s); err != nil {
		return corev1.Service{}, err
	}
	return s, nil
}

// UpdateService provides a thin wrapper and client.Client to update corev1.Service types
func (c client) UpdateService(ctx context.Context, service corev1.Service) error {
	return c.Update(ctx, &service)
}

// CreateService provides a thin wrapper and client.Client to create corev1.Service types
func (c client) CreateService(ctx context.Context, service corev1.Service) error {
	return c.Create(ctx, &service)
}

// DeleteService provides a thin wrapper around client.Client to delete corev1.Service types
func (c client) DeleteService(ctx context.Context, objectKey k8sClient.ObjectKey) error {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectKey.Name,
			Namespace: objectKey.Namespace,
		},
	}
	return c.Delete(ctx, &svc)
}

// GetStatefulSet provides a thin wrapper and client.Client to access appsv1.StatefulSet types
func (c client) GetStatefulSet(ctx context.Context, objectKey k8sClient.ObjectKey) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	if err := c.Get(ctx, objectKey, &sts); err != nil {
		return appsv1.StatefulSet{}, err
	}
	return sts, nil
}

// UpdateStatefulSet provides a thin wrapper and client.Client to update appsv1.StatefulSet types
// the updated StatefulSet is returned
func (c client) UpdateStatefulSet(ctx context.Context, sts appsv1.StatefulSet) (appsv1.StatefulSet, error) {
	stsToUpdate := &sts
	err := c.Update(ctx, stsToUpdate)
	return *stsToUpdate, err
}

// CreateStatefulSet provides a thin wrapper and client.Client to create appsv1.StatefulSet types
func (c client) CreateStatefulSet(ctx context.Context, sts appsv1.StatefulSet) error {
	return c.Create(ctx, &sts)
}

// DeleteStatefulSet provides a thin wrapper and client.Client to delete appsv1.StatefulSet types
func (c client) DeleteStatefulSet(ctx context.Context, objectKey k8sClient.ObjectKey) error {
	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectKey.Name,
			Namespace: objectKey.Namespace,
		},
	}
	return c.Delete(ctx, &sts)
}
