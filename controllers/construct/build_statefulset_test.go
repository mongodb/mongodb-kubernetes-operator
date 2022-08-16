package construct

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"

	corev1 "k8s.io/api/core/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	os.Setenv(VersionUpgradeHookImageEnv, "version-upgrade-hook-image")
}

func newTestReplicaSet() mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "my-rs",
			Namespace:   "my-ns",
			Annotations: map[string]string{},
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.2.2",
		},
	}
}

func TestMultipleCalls_DoNotCauseSideEffects(t *testing.T) {
	_ = os.Setenv(MongodbRepoUrl, "repo")
	_ = os.Setenv(MongodbImageEnv, "mongo")
	_ = os.Setenv(AgentImageEnv, "agent-image")

	mdb := newTestReplicaSet()
	stsFunc := BuildMongoDBReplicaSetStatefulSetModificationFunction(&mdb, mdb)
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

func TestManagedSecurityContext(t *testing.T) {
	_ = os.Setenv(MongodbRepoUrl, "repo")
	_ = os.Setenv(MongodbImageEnv, "mongo")
	_ = os.Setenv(AgentImageEnv, "agent-image")
	_ = os.Setenv(podtemplatespec.ManagedSecurityContextEnv, "true")

	mdb := newTestReplicaSet()
	stsFunc := BuildMongoDBReplicaSetStatefulSetModificationFunction(&mdb, mdb)

	sts := &appsv1.StatefulSet{}
	stsFunc(sts)

	assertStatefulSetIsBuiltCorrectly(t, mdb, sts)
}

func TestMongod_Container(t *testing.T) {
	c := container.New(mongodbContainer("4.2", []corev1.VolumeMount{}, mdbv1.NewMongodConfiguration()))

	t.Run("Has correct Env vars", func(t *testing.T) {
		assert.Len(t, c.Env, 1)
		assert.Equal(t, agentHealthStatusFilePathEnv, c.Env[0].Name)
		assert.Equal(t, "/healthstatus/agent-health-status.json", c.Env[0].Value)
	})

	t.Run("Image is correct", func(t *testing.T) {
		assert.Equal(t, getMongoDBImage("4.2"), c.Image)
	})

	t.Run("Resource requirements are correct", func(t *testing.T) {
		assert.Equal(t, resourcerequirements.Defaults(), c.Resources)
	})
}

func assertStatefulSetIsBuiltCorrectly(t *testing.T, mdb mdbv1.MongoDBCommunity, sts *appsv1.StatefulSet) {
	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)
	assert.Equal(t, mdb.ServiceName(), sts.Spec.ServiceName)
	assert.Equal(t, mdb.Name, sts.Name)
	assert.Equal(t, mdb.Namespace, sts.Namespace)
	assert.Equal(t, mongodbDatabaseServiceAccountName, sts.Spec.Template.Spec.ServiceAccountName)
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].Env, 4)
	assert.Len(t, sts.Spec.Template.Spec.Containers[1].Env, 1)

	managedSecurityContext := envvar.ReadBool(podtemplatespec.ManagedSecurityContextEnv)
	if !managedSecurityContext {
		assert.NotNil(t, sts.Spec.Template.Spec.SecurityContext)
		assert.Equal(t, podtemplatespec.DefaultPodSecurityContext(), *sts.Spec.Template.Spec.SecurityContext)
	} else {
		assert.Nil(t, sts.Spec.Template.Spec.SecurityContext)
	}

	agentContainer := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "agent-image", agentContainer.Image)
	probe := agentContainer.ReadinessProbe
	assert.True(t, reflect.DeepEqual(probes.New(DefaultReadiness()), *probe))
	assert.Equal(t, probes.New(DefaultReadiness()).FailureThreshold, probe.FailureThreshold)
	assert.Len(t, agentContainer.VolumeMounts, 7)
	assert.NotNil(t, agentContainer.ReadinessProbe)
	if !managedSecurityContext {
		assert.NotNil(t, sts.Spec.Template.Spec.Containers[0].SecurityContext)
		assert.Equal(t, container.DefaultSecurityContext(), *sts.Spec.Template.Spec.Containers[0].SecurityContext)
	} else {
		assert.Nil(t, agentContainer.SecurityContext)
	}

	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "agent-scripts")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "automation-config")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "data-volume")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "healthstatus")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "logs-volume")
	assertContainsVolumeMountWithName(t, agentContainer.VolumeMounts, "my-rs-keyfile")

	mongodContainer := sts.Spec.Template.Spec.Containers[1]
	assert.Equal(t, "repo/mongo:4.2.2", mongodContainer.Image)
	assert.Len(t, mongodContainer.VolumeMounts, 6)
	if !managedSecurityContext {
		assert.NotNil(t, sts.Spec.Template.Spec.Containers[1].SecurityContext)
		assert.Equal(t, container.DefaultSecurityContext(), *sts.Spec.Template.Spec.Containers[1].SecurityContext)
	} else {
		assert.Nil(t, agentContainer.SecurityContext)
	}

	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "data-volume")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "healthstatus")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "hooks")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "logs-volume")
	assertContainsVolumeMountWithName(t, mongodContainer.VolumeMounts, "my-rs-keyfile")

	initContainer := sts.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, versionUpgradeHookName, initContainer.Name)
	assert.Equal(t, "version-upgrade-hook-image", initContainer.Image)
	assert.Len(t, initContainer.VolumeMounts, 1)
	if !managedSecurityContext {
		assert.NotNil(t, sts.Spec.Template.Spec.InitContainers[0].SecurityContext)
		assert.Equal(t, container.DefaultSecurityContext(), *sts.Spec.Template.Spec.InitContainers[0].SecurityContext)
	} else {
		assert.Nil(t, agentContainer.SecurityContext)
	}
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
