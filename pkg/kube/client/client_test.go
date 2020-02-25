package client

import (
	"context"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestChangingName_CreatesNewObject(t *testing.T) {
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := client.CreateOrUpdate(&cm)
	assert.NoError(t, err)

	newCm := corev1.ConfigMap{}
	objectKey, err := k8sClient.ObjectKeyFromObject(&cm)
	assert.NoError(t, err)

	err = client.Get(context.TODO(), objectKey, &newCm)
	assert.NoError(t, err)

	assert.Equal(t, newCm.Name, "some-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")

	newCm.Name = "new-name"

	objectKey, err = k8sClient.ObjectKeyFromObject(&newCm)
	err = client.CreateOrUpdate(&newCm)

	err = client.Get(context.TODO(), objectKey, &newCm)

	assert.Equal(t, newCm.Name, "new-name")
	assert.Equal(t, newCm.Namespace, "some-namespace")
}

func TestAddingDataField_ModifiesExistingObject(t *testing.T) {
	cm := configmap.Builder().
		SetName("some-name").
		SetNamespace("some-namespace").
		Build()

	client := NewClient(NewMockedClient())
	err := client.CreateOrUpdate(&cm)
	assert.NoError(t, err)

	cm.Data["new-field"] = "value"
	err = client.CreateOrUpdate(&cm)

	newCm := corev1.ConfigMap{}
	objectKey, err := k8sClient.ObjectKeyFromObject(&newCm)
	assert.NoError(t, err)
	err = client.Get(context.TODO(), objectKey, &newCm)

	assert.Contains(t, cm.Data, "new-field")
	assert.Equal(t, cm.Data["new-field"], "value")
}
