package client

import (
	"context"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestChangingName_CreatesNewObject(t *testing.T) {
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := configmap.CreateOrUpdate(client, cm)
	assert.NoError(t, err)

	newCm := corev1.ConfigMap{}
	objectKey := k8sClient.ObjectKeyFromObject(&cm)
	assert.NoError(t, err)

	err = client.Get(context.TODO(), objectKey, &newCm)
	assert.NoError(t, err)

	assert.Equal(t, newCm.Name, "some-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")

	newCm.Name = "new-name"

	objectKey = k8sClient.ObjectKeyFromObject(&newCm)
	_ = configmap.CreateOrUpdate(client, newCm)

	_ = client.Get(context.TODO(), objectKey, &newCm)

	assert.Equal(t, newCm.Name, "new-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")
}

func TestAddingDataField_ModifiesExistingObject(t *testing.T) {
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := configmap.CreateOrUpdate(client, cm)
	assert.NoError(t, err)

	cm.Data["new-field"] = "value"
	_ = configmap.CreateOrUpdate(client, cm)

	newCm := corev1.ConfigMap{}
	objectKey := k8sClient.ObjectKeyFromObject(&newCm)
	assert.NoError(t, err)
	_ = client.Get(context.TODO(), objectKey, &newCm)

	assert.Contains(t, cm.Data, "new-field")
	assert.Equal(t, cm.Data["new-field"], "value")
}

func TestDeleteConfigMap(t *testing.T) {
	cm := configmap.Builder().
		SetName("config-map").
		SetNamespace("default").
		Build()

	client := NewClient(NewMockedClient())
	err := client.CreateConfigMap(cm)
	assert.NoError(t, err)

	err = client.DeleteConfigMap(types.NamespacedName{Name: "config-map", Namespace: "default"})
	assert.NoError(t, err)

	_, err = client.GetConfigMap(types.NamespacedName{Name: "config-map", Namespace: "default"})
	assert.Equal(t, err, notFoundError())
}

// TestSetAnnotationsDoesNotChangeSuppliedObject verifies that the supplied object for annotations.SetAnnotations is not overridden due being a shallow copy.
// the function lies here, otherwise it will lead to import cycles.
func TestSetAnnotationsDoesNotChangeSuppliedObject(t *testing.T) {
	c := NewClient(NewMockedClient())
	backedService := service.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		SetAnnotations(map[string]string{"one": "annotation"}).
		SetClusterIP("123").
		Build()
	err := service.CreateOrUpdateService(c, backedService)
	assert.NoError(t, err)

	serviceWithoutAnnotation := service.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	// make sure this method only changes the annotations locally and in kube
	err = annotations.SetAnnotations(&serviceWithoutAnnotation, map[string]string{"new": "something"}, c)
	assert.NoError(t, err)
	assert.Len(t, serviceWithoutAnnotation.Annotations, 2)
	assert.Equal(t, "", serviceWithoutAnnotation.Spec.ClusterIP)

	err = c.Get(context.TODO(), types.NamespacedName{Name: serviceWithoutAnnotation.GetName(), Namespace: serviceWithoutAnnotation.GetNamespace()}, &serviceWithoutAnnotation)
	assert.NoError(t, err)
	assert.Len(t, serviceWithoutAnnotation.Annotations, 2)
	assert.Equal(t, "123", serviceWithoutAnnotation.Spec.ClusterIP)
}
