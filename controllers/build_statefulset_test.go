package controllers

import (
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func init() {
	os.Setenv(versionUpgradeHookImageEnv, "version-upgrade-hook-image")
}

func TestMultipleCalls_DoNotCauseSideEffects(t *testing.T) {
	_ = os.Setenv(mongodbRepoUrl, "repo")
	_ = os.Setenv(mongodbImageEnv, "mongo")

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

func assertStatefulSetIsBuiltCorrectly(t *testing.T, mdb mdbv1.MongoDBCommunity, sts *appsv1.StatefulSet) {
	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)
	assert.Equal(t, mdb.ServiceName(), sts.Spec.ServiceName)
	assert.Equal(t, mdb.Name, sts.Name)
	assert.Equal(t, mdb.Namespace, sts.Namespace)
	assert.Equal(t, appsv1.RollingUpdateStatefulSetStrategyType, sts.Spec.UpdateStrategy.Type)
	assert.Equal(t, operatorServiceAccountName, sts.Spec.Template.Spec.ServiceAccountName)
	assert.Len(t, sts.Spec.Template.Spec.Containers[1].Env, 4)
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].Env, 1)

	agentContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, "agent-image", agentContainer.Image)
	probe := agentContainer.ReadinessProbe
	assert.True(t, reflect.DeepEqual(probes.New(defaultReadiness()), *probe))
	assert.Equal(t, probes.New(defaultReadiness()).FailureThreshold, probe.FailureThreshold)
	assert.Equal(t, int32(5), probe.InitialDelaySeconds)
	assert.Len(t, agentContainer.VolumeMounts, 6)

	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "agent-scripts")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "automation-config")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "data-volume")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "healthstatus")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "my-rs-keyfile")

	mongodContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "repo/mongo:4.2.2", mongodContainer.Image)
	assert.NotNil(t, sts.Spec.Template.Spec.Containers[1].ReadinessProbe)
	assert.Len(t, mongodContainer.VolumeMounts, 5)

	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "data-volume")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "healthstatus")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "hooks")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "logs-volume")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "my-rs-keyfile")

	initContainer := sts.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, versionUpgradeHookName, initContainer.Name)
	assert.Equal(t, "version-upgrade-hook-image", initContainer.Image)
	assert.Len(t, initContainer.VolumeMounts, 1)
}

func assertContainsVolumeMountWithName(t *testing.T, mounts []corev1.VolumeMount, name string) {
	found := false
	for _, m := range mounts {
		if m.Name == name {
			found = true
			break
		}
	}
	assert.True(t, found, "Mounts should have contained a mount with name %s, but didn't. Actual mounts: %v", name, mounts)
}
