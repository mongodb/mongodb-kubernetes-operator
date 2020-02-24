package mock

import (
	"context"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func TestMockedClient(t *testing.T) {
	mockedClient := NewClient()

	cm := configmap.Builder().
		SetName("cm-name").
		SetNamespace("cm-namespace").
		SetField("field-1", "value-1").
		Build()

	err := mockedClient.Create(context.TODO(), &cm)
	assert.NoError(t, err)

	newCm := corev1.ConfigMap{}
	err = mockedClient.Get(context.TODO(), types.NamespacedName{Name: "cm-name", Namespace: "cm-namespace"}, &newCm)
	assert.NoError(t, err)
	assert.Equal(t, "cm-namespace", newCm.Namespace)
	assert.Equal(t, "cm-name", newCm.Name)

	svc := service.Builder().
		SetName("svc-name").
		SetNamespace("svc-namespace").
		SetServiceType("service-type").
		Build()

	err = mockedClient.Create(context.TODO(), &svc)
	assert.NoError(t, err)

	newSvc := corev1.Service{}
	err = mockedClient.Get(context.TODO(), types.NamespacedName{Name: "svc-name", Namespace: "svc-namespace"}, &newSvc)
	assert.NoError(t, err)
	assert.Equal(t, "svc-namespace", newSvc.Namespace)
	assert.Equal(t, "svc-name", newSvc.Name)
}
