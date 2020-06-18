package mongodb

import (
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func TestMultipleCalls_DoNotCauseSideEffects(t *testing.T) {
	mdb := newTestReplicaSet()
	stsFunc := buildStatefulSetModificationFunction(mdb)
	sts := &appsv1.StatefulSet{}

	t.Run("1st Call", func(t *testing.T) {
		stsFunc(sts)
		assertStatefulSetIsBuiltCorrectly(t, mdb, sts)
	})
	t.Run("2nd Call", func(t *testing.T) {
		stsFunc(sts)
		assertStatefulSetIsBuiltCorrectly(t, mdb, sts)
	})
	t.Run("3rd Call", func(t *testing.T) {
		stsFunc(sts)
		assertStatefulSetIsBuiltCorrectly(t, mdb, sts)
	})
}

func assertStatefulSetIsBuiltCorrectly(t *testing.T, mdb mdbv1.MongoDB, sts *appsv1.StatefulSet) {
	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, mdb.ServiceName(), sts.Spec.ServiceName)
	assert.Equal(t, mdb.Name, sts.Name)
	assert.Equal(t, mdb.Namespace, sts.Namespace)
	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	assert.Equal(t, operatorServiceAccountName, sts.Spec.Template.Spec.ServiceAccountName)
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].Env, 1)
	assert.Len(t, sts.Spec.Template.Spec.Containers[1].Env, 2)
	assert.Equal(t, "agent-image", sts.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "mongo:4.2.2", sts.Spec.Template.Spec.Containers[1].Image)
	assert.NotNil(t, sts.Spec.Template.Spec.Containers[0].ReadinessProbe)

	probe := sts.Spec.Template.Spec.Containers[0].ReadinessProbe
	assert.Equal(t, int32(240), probe.FailureThreshold)
	assert.Equal(t, int32(5), probe.InitialDelaySeconds)
}
