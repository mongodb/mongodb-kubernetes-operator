package construct

import (
	"os"
	"reflect"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

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
			Version: "6.0.5",
		},
	}
}

func TestMultipleCalls_DoNotCauseSideEffects(t *testing.T) {
	t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
	t.Setenv(MongodbImageEnv, "mongodb-community-server")
	t.Setenv(AgentImageEnv, "agent-image")

	mdb := newTestReplicaSet()
	stsFunc := BuildMongoDBReplicaSetStatefulSetModificationFunction(&mdb, &mdb)
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
	t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
	t.Setenv(MongodbImageEnv, "mongodb-community-server")
	t.Setenv(AgentImageEnv, "agent-image")
	t.Setenv(podtemplatespec.ManagedSecurityContextEnv, "true")

	mdb := newTestReplicaSet()
	stsFunc := BuildMongoDBReplicaSetStatefulSetModificationFunction(&mdb, &mdb)

	sts := &appsv1.StatefulSet{}
	stsFunc(sts)

	assertStatefulSetIsBuiltCorrectly(t, mdb, sts)
}

func TestGetMongoDBImage(t *testing.T) {
	type testConfig struct {
		setArgs       func(t *testing.T)
		version       string
		expectedImage string
	}
	tests := map[string]testConfig{
		"Default UBI8 Community image": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
				t.Setenv(MongodbImageEnv, "mongodb-community-server")
			},
			version:       "6.0.5",
			expectedImage: "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8",
		},
		"Overridden UBI8 Enterprise image": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
				t.Setenv(MongodbImageEnv, "mongodb-enterprise-server")
			},
			version:       "6.0.5",
			expectedImage: "docker.io/mongodb/mongodb-enterprise-server:6.0.5-ubi8",
		},
		"Overridden UBI8 Enterprise image from Quay": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "quay.io/mongodb")
				t.Setenv(MongodbImageEnv, "mongodb-enterprise-server")
			},
			version:       "6.0.5",
			expectedImage: "quay.io/mongodb/mongodb-enterprise-server:6.0.5-ubi8",
		},
		"Overridden Ubuntu Community image": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
				t.Setenv(MongodbImageEnv, "mongodb-community-server")
				t.Setenv(MongoDBImageType, "ubuntu2204")
			},
			version:       "6.0.5",
			expectedImage: "docker.io/mongodb/mongodb-community-server:6.0.5-ubuntu2204",
		},
		"Overridden UBI Community image": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "docker.io/mongodb")
				t.Setenv(MongodbImageEnv, "mongodb-community-server")
				t.Setenv(MongoDBImageType, "ubi8")
			},
			version:       "6.0.5",
			expectedImage: "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8",
		},
		"Docker Inc images": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "docker.io")
				t.Setenv(MongodbImageEnv, "mongo")
			},
			version:       "6.0.5",
			expectedImage: "docker.io/mongo:6.0.5",
		},
		"Deprecated AppDB images defined the old way": {
			setArgs: func(t *testing.T) {
				t.Setenv(MongodbRepoUrl, "quay.io")
				t.Setenv(MongodbImageEnv, "mongodb/mongodb-enterprise-appdb-database-ubi")
				// In this example, we intentionally don't use the suffix from the env. variable and let users
				// define it in the version instead. There are some known customers who do this.
				// This is a backwards compatibility case.
				t.Setenv(MongoDBImageType, "will-be-ignored")
			},

			version:       "5.0.14-ent",
			expectedImage: "quay.io/mongodb/mongodb-enterprise-appdb-database-ubi:5.0.14-ent",
		},
	}
	for testName := range tests {
		t.Run(testName, func(t *testing.T) {
			testConfig := tests[testName]
			testConfig.setArgs(t)
			image := getMongoDBImage(testConfig.version)
			assert.Equal(t, testConfig.expectedImage, image)
		})
	}
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

func TestMongoDBAgentLogging_Container(t *testing.T) {
	c := container.New(mongodbAgentContainer("test-mongodb-automation-config", []corev1.VolumeMount{}, "INFO", "/var/log/mongodb-mms-automation/automation-agent.log", 24, false))

	t.Run("Has correct Env vars", func(t *testing.T) {
		assert.Len(t, c.Env, 7)
		assert.Equal(t, agentLogFileEnv, c.Env[0].Name)
		assert.Equal(t, "/var/log/mongodb-mms-automation/automation-agent.log", c.Env[0].Value)
		assert.Equal(t, agentLogLevelEnv, c.Env[1].Name)
		assert.Equal(t, "INFO", c.Env[1].Value)
		assert.Equal(t, agentMaxLogFileDurationHoursEnv, c.Env[2].Name)
		assert.Equal(t, "24", c.Env[2].Value)
	})
}

func assertStatefulSetIsBuiltCorrectly(t *testing.T, mdb mdbv1.MongoDBCommunity, sts *appsv1.StatefulSet) {
	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	assert.Len(t, sts.Spec.Template.Spec.InitContainers, 2)
	assert.Equal(t, mdb.ServiceName(), sts.Spec.ServiceName)
	assert.Equal(t, mdb.Name, sts.Name)
	assert.Equal(t, mdb.Namespace, sts.Namespace)
	assert.Equal(t, mongodbDatabaseServiceAccountName, sts.Spec.Template.Spec.ServiceAccountName)
	assert.Len(t, sts.Spec.Template.Spec.Containers[0].Env, 7)
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
	assert.Equal(t, "docker.io/mongodb/mongodb-community-server:6.0.5-ubi8", mongodContainer.Image)
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
