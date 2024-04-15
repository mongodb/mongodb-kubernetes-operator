package client

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestChangingName_CreatesNewObject(t *testing.T) {
	ctx := context.Background()
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := configmap.CreateOrUpdate(ctx, client, cm)
	assert.NoError(t, err)

	newCm := corev1.ConfigMap{}
	objectKey := k8sClient.ObjectKeyFromObject(&cm)
	assert.NoError(t, err)

	err = client.Get(ctx, objectKey, &newCm)
	assert.NoError(t, err)

	assert.Equal(t, newCm.Name, "some-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")

	newCm.Name = "new-name"

	objectKey = k8sClient.ObjectKeyFromObject(&newCm)
	_ = configmap.CreateOrUpdate(ctx, client, newCm)

	_ = client.Get(ctx, objectKey, &newCm)

	assert.Equal(t, newCm.Name, "new-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")
}

func TestAddingDataField_ModifiesExistingObject(t *testing.T) {
	ctx := context.Background()
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := configmap.CreateOrUpdate(ctx, client, cm)
	assert.NoError(t, err)

	cm.Data["new-field"] = "value"
	_ = configmap.CreateOrUpdate(ctx, client, cm)

	newCm := corev1.ConfigMap{}
	objectKey := k8sClient.ObjectKeyFromObject(&newCm)
	assert.NoError(t, err)
	_ = client.Get(ctx, objectKey, &newCm)

	assert.Contains(t, cm.Data, "new-field")
	assert.Equal(t, cm.Data["new-field"], "value")
}

func TestDeleteConfigMap(t *testing.T) {
	ctx := context.Background()
	cm := configmap.Builder().
		SetName("config-map").
		SetNamespace("default").
		Build()

	client := NewClient(NewMockedClient())
	err := client.CreateConfigMap(ctx, cm)
	assert.NoError(t, err)

	err = client.DeleteConfigMap(ctx, types.NamespacedName{Name: "config-map", Namespace: "default"})
	assert.NoError(t, err)

	_, err = client.GetConfigMap(ctx, types.NamespacedName{Name: "config-map", Namespace: "default"})
	assert.Equal(t, err, notFoundError())
}
