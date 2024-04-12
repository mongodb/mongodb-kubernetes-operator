package client

import (
	"context"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMockedClient(t *testing.T) {
	ctx := context.Background()
	mockedClient := NewMockedClient()

	cm := configmap.Builder().
		SetName("cm-name").
		SetNamespace("cm-namespace").
		SetDataField("field-1", "value-1").
		SetData(map[string]string{"key-2": "field-2"}).
		Build()

	err := mockedClient.Create(ctx, &cm)
	assert.NoError(t, err)

	newCm := corev1.ConfigMap{}
	err = mockedClient.Get(ctx, types.NamespacedName{Name: "cm-name", Namespace: "cm-namespace"}, &newCm)
	assert.NoError(t, err)
	assert.Equal(t, "cm-namespace", newCm.Namespace)
	assert.Equal(t, "cm-name", newCm.Name)
	assert.Equal(t, newCm.Data, map[string]string{"field-1": "value-1", "key-2": "field-2"})

	svc := service.Builder().
		SetName("svc-name").
		SetNamespace("svc-namespace").
		SetServiceType("service-type").
		Build()

	err = mockedClient.Create(ctx, &svc)
	assert.NoError(t, err)

	newSvc := corev1.Service{}
	err = mockedClient.Get(ctx, types.NamespacedName{Name: "svc-name", Namespace: "svc-namespace"}, &newSvc)
	assert.NoError(t, err)
	assert.Equal(t, "svc-namespace", newSvc.Namespace)
	assert.Equal(t, "svc-name", newSvc.Name)
}
