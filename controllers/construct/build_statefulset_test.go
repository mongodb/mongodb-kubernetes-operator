package construct

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

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

func TestMongod_ContainerWithDigests(t *testing.T) {
	_ = os.Setenv(fmt.Sprintf("RELATED_IMAGE_%s_4_3", MongodbImageEnv), "quay.io/mongodb/mongodb-kubernetes-operator@sha256:1234af56296c10c9bd02cc85bb542a849e9a66aff0697d6359b449540696b1fd")

	c := container.New(mongodbContainer("4.3", []corev1.VolumeMount{}, mdbv1.NewMongodConfiguration()))

	t.Run("Image is from RELATED_IMAGE env var", func(t *testing.T) {
		assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator@sha256:1234af56296c10c9bd02cc85bb542a849e9a66aff0697d6359b449540696b1fd", c.Image)
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

func TestReplaceImageTagOrDigest(t *testing.T) {
	assert.Equal(t, "quay.io/mongodb/mongodb-agent:9876-54321", replaceImageTagOrDigest("quay.io/mongodb/mongodb-agent:1234-567", "9876-54321"))
	assert.Equal(t, "quay.io/mongodb/mongodb-agent:9876-54321", replaceImageTagOrDigest("quay.io/mongodb/mongodb-agent@sha256:6a82abae27c1ba1133f3eefaad71ea318f8fa87cc57fe9355d6b5b817ff97f1a", "9876-54321"))
	assert.Equal(t, "quay.io/mongodb/mongodb-enterprise-database:some-tag", replaceImageTagOrDigest("quay.io/mongodb/mongodb-enterprise-database:45678", "some-tag"))
}

func TestContainerImage(t *testing.T) {
	_ = os.Setenv(MongodbImageEnv, "quay.io/mongodb/mongodb-kubernetes-operator")
	_ = os.Setenv(fmt.Sprintf("RELATED_IMAGE_%s_1_0_0", MongodbImageEnv), "quay.io/mongodb/mongodb-kubernetes-operator@sha256:608daf56296c10c9bd02cc85bb542a849e9a66aff0697d6359b449540696b1fd")
	_ = os.Setenv(fmt.Sprintf("RELATED_IMAGE_%s_12_0_4_7554_1", MongodbImageEnv), "quay.io/mongodb/mongodb-kubernetes-operator@sha256:b631ee886bb49ba8d7b90bb003fe66051dadecbc2ac126ac7351221f4a7c377c")
	_ = os.Setenv(fmt.Sprintf("RELATED_IMAGE_%s_2_0_0_b20220912000000", MongodbImageEnv), "quay.io/mongodb/mongodb-kubernetes-operator@sha256:f1a7f49cd6533d8ca9425f25cdc290d46bb883997f07fac83b66cc799313adad")

	// there is no related image for 0.0.1
	assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator:0.0.1", ContainerImage(MongodbImageEnv, "0.0.1"))
	// for 10.2.25.6008-1 there is no RELATED_IMAGE variable set, so we use version instead of digest
	assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator:10.2.25.6008-1", ContainerImage(MongodbImageEnv, "10.2.25.6008-1"))
	// for following versions we set RELATED_IMAGE_MONGODB_IMAGE_* env variables to sha256 digest
	assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator@sha256:608daf56296c10c9bd02cc85bb542a849e9a66aff0697d6359b449540696b1fd", ContainerImage(MongodbImageEnv, "1.0.0"))
	assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator@sha256:b631ee886bb49ba8d7b90bb003fe66051dadecbc2ac126ac7351221f4a7c377c", ContainerImage(MongodbImageEnv, "12.0.4.7554-1"))
	assert.Equal(t, "quay.io/mongodb/mongodb-kubernetes-operator@sha256:f1a7f49cd6533d8ca9425f25cdc290d46bb883997f07fac83b66cc799313adad", ContainerImage(MongodbImageEnv, "2.0.0-b20220912000000"))

	// env var has version already, so it is replaced
	_ = os.Setenv(AgentImageEnv, "quay.io/mongodb/mongodb-agent:12.0.4.7554-1")
	assert.Equal(t, "quay.io/mongodb/mongodb-agent:10.2.25.6008-1", ContainerImage(AgentImageEnv, "10.2.25.6008-1"))

	// env var has version already, but there is related image with this version
	_ = os.Setenv(fmt.Sprintf("RELATED_IMAGE_%s_12_0_4_7554_1", AgentImageEnv), "quay.io/mongodb/mongodb-agent@sha256:a48829ce36bf479dc25a4de79234c5621b67beee62ca98a099d0a56fdb04791c")
	assert.Equal(t, "quay.io/mongodb/mongodb-agent@sha256:a48829ce36bf479dc25a4de79234c5621b67beee62ca98a099d0a56fdb04791c", ContainerImage(AgentImageEnv, "12.0.4.7554-1"))

	_ = os.Setenv(AgentImageEnv, "quay.io/mongodb/mongodb-agent@sha256:608daf56296c10c9bd02cc85bb542a849e9a66aff0697d6359b449540696b1fd")
	// env var has version already as digest, but there is related image with this version
	assert.Equal(t, "quay.io/mongodb/mongodb-agent@sha256:a48829ce36bf479dc25a4de79234c5621b67beee62ca98a099d0a56fdb04791c", ContainerImage(AgentImageEnv, "12.0.4.7554-1"))
	// env var has version already as digest, there is no related image with this version, so we use version instead of digest
	assert.Equal(t, "quay.io/mongodb/mongodb-agent:1.2.3", ContainerImage(AgentImageEnv, "1.2.3"))

}
