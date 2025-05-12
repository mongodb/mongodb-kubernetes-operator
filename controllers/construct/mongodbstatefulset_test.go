package construct

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCollectEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		envSetup    map[string]string
		expectedEnv []corev1.EnvVar
	}{
		{
			name: "Basic env vars set",
			envSetup: map[string]string{
				config.ReadinessProbeLoggerBackups: "3",
				config.ReadinessProbeLoggerMaxSize: "10M",
				config.ReadinessProbeLoggerMaxAge:  "7",
				config.WithAgentFileLogging:        "enabled",
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  config.AgentHealthStatusFilePathEnv,
					Value: "/healthstatus/agent-health-status.json",
				},
				{
					Name:  config.ReadinessProbeLoggerBackups,
					Value: "3",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxSize,
					Value: "10M",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxAge,
					Value: "7",
				},
				{
					Name:  config.WithAgentFileLogging,
					Value: "enabled",
				},
			},
		},
		{
			name: "Additional env var set",
			envSetup: map[string]string{
				config.ReadinessProbeLoggerBackups:  "3",
				config.ReadinessProbeLoggerMaxSize:  "10M",
				config.ReadinessProbeLoggerMaxAge:   "7",
				config.ReadinessProbeLoggerCompress: "true",
				config.WithAgentFileLogging:         "enabled",
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  config.AgentHealthStatusFilePathEnv,
					Value: "/healthstatus/agent-health-status.json",
				},
				{
					Name:  config.ReadinessProbeLoggerBackups,
					Value: "3",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxSize,
					Value: "10M",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxAge,
					Value: "7",
				},
				{
					Name:  config.ReadinessProbeLoggerCompress,
					Value: "true",
				},
				{
					Name:  config.WithAgentFileLogging,
					Value: "enabled",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			for key, value := range tt.envSetup {
				t.Setenv(key, value)
			}

			actualEnvVars := collectEnvVars()

			assert.EqualValues(t, tt.expectedEnv, actualEnvVars)
		})
	}
}

func TestGetContainerResources(t *testing.T) {
	// Test with a nil mdb
	t.Run("Nil mdb returns default resources", func(t *testing.T) {
		result := getContainerResources(nil, MongodbName)
		assert.Equal(t, resourcerequirements.Defaults(), result)
	})

	// Test with a mdb that doesn't have Resources specified
	t.Run("Mdb without Resources returns default resources", func(t *testing.T) {
		mdb := &mdbv1.MongoDBCommunity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-mongodb",
				Namespace: "my-namespace",
			},
			Spec: mdbv1.MongoDBCommunitySpec{
				Members: 3,
				Version: "4.4.0",
			},
		}

		result := getContainerResources(mdb, MongodbName)
		assert.Equal(t, resourcerequirements.Defaults(), result)
	})

	// Create custom resource requirements for testing
	customMongodRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	customAgentRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("500Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("250m"),
			corev1.ResourceMemory: resource.MustParse("250Mi"),
		},
	}

	// Test with a mdb that has Resources specified
	t.Run("Mdb with Resources specified returns custom resources", func(t *testing.T) {
		mdb := &mdbv1.MongoDBCommunity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-mongodb",
				Namespace: "my-namespace",
			},
			Spec: mdbv1.MongoDBCommunitySpec{
				Members: 3,
				Version: "4.4.0",
				Resources: mdbv1.ResourcesSpec{
					Mongod: &customMongodRes,
					Agent:  &customAgentRes,
				},
			},
		}

		// Test for mongod container
		mongodResult := getContainerResources(mdb, MongodbName)
		assert.Equal(t, customMongodRes, mongodResult)

		// Test for agent container
		agentResult := getContainerResources(mdb, AgentName)
		assert.Equal(t, customAgentRes, agentResult)

		// Test for a container that doesn't have custom resources specified
		readinessResult := getContainerResources(mdb, ReadinessProbeContainerName)
		assert.Equal(t, resourcerequirements.Defaults(), readinessResult)
	})

	// Test with resources for all container types
	t.Run("Mdb with all container resources specified", func(t *testing.T) {
		readinessRes := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		}

		hookRes := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("100Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("50Mi"),
			},
		}

		mdb := &mdbv1.MongoDBCommunity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-mongodb",
				Namespace: "my-namespace",
			},
			Spec: mdbv1.MongoDBCommunitySpec{
				Members: 3,
				Version: "4.4.0",
				Resources: mdbv1.ResourcesSpec{
					Mongod:             &customMongodRes,
					Agent:              &customAgentRes,
					ReadinessProbe:     &readinessRes,
					VersionUpgradeHook: &hookRes,
				},
			},
		}

		// Test all container types
		assert.Equal(t, customMongodRes, getContainerResources(mdb, MongodbName))
		assert.Equal(t, customAgentRes, getContainerResources(mdb, AgentName))
		assert.Equal(t, readinessRes, getContainerResources(mdb, ReadinessProbeContainerName))
		assert.Equal(t, hookRes, getContainerResources(mdb, versionUpgradeHookName))

		// Test for a non-existent container type
		assert.Equal(t, resourcerequirements.Defaults(), getContainerResources(mdb, "non-existent-container"))
	})
}

func TestMongodbContainerWithCustomResources(t *testing.T) {
	// Create custom resource requirements for mongod
	customMongodRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	// Create a MongoDBCommunity with custom resources
	mdb := &mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mongodb",
			Namespace: "my-namespace",
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.4.0",
			Resources: mdbv1.ResourcesSpec{
				Mongod: &customMongodRes,
			},
		},
	}

	// Create a mongod configuration
	mongodConfig := mdbv1.NewMongodConfiguration()

	// Create volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data-volume",
			MountPath: "/data",
		},
	}

	// Test the mongodbContainer function
	t.Run("mongodbContainer should use custom resources", func(t *testing.T) {
		// Create the c
		containerMod := mongodbContainer("mongo:4.4.0", volumeMounts, mongodConfig, mdb)

		// Apply the modification to a c
		c := &corev1.Container{}
		containerMod(c)

		// Verify that the c has the custom resources
		assert.Equal(t, customMongodRes, c.Resources)
	})
}

func TestMongodbAgentContainerWithCustomResources(t *testing.T) {
	// Create custom resource requirements for agent
	customAgentRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("500Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("250m"),
			corev1.ResourceMemory: resource.MustParse("250Mi"),
		},
	}

	// Create a MongoDBCommunity with custom resources
	mdb := &mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mongodb",
			Namespace: "my-namespace",
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.4.0",
			Resources: mdbv1.ResourcesSpec{
				Agent: &customAgentRes,
			},
		},
	}

	// Create volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "data-volume",
			MountPath: "/data",
		},
	}

	// Test the mongodbAgentContainer function
	t.Run("mongodbAgentContainer should use custom resources", func(t *testing.T) {
		// Create the c
		containerMod := mongodbAgentContainer("test-secret", volumeMounts, mdbv1.LogLevelInfo, "/var/log/mongodb-agent.log", 24, "mongo-agent:1.0.0", mdb)

		// Apply the modification to a c
		c := &corev1.Container{}
		containerMod(c)

		// Verify that the c has the custom resources
		assert.Equal(t, customAgentRes, c.Resources)
	})
}

func TestReadinessProbeInitWithCustomResources(t *testing.T) {
	// Create custom resource requirements for readiness probe
	readinessRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("50Mi"),
		},
	}

	// Create a MongoDBCommunity with custom resources
	mdb := &mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mongodb",
			Namespace: "my-namespace",
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.4.0",
			Resources: mdbv1.ResourcesSpec{
				ReadinessProbe: &readinessRes,
			},
		},
	}

	// Create volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "scripts-volume",
			MountPath: "/opt/scripts",
		},
	}

	// Test the readinessProbeInit function
	t.Run("readinessProbeInit should use custom resources", func(t *testing.T) {
		// Create the c
		containerMod := readinessProbeInit(volumeMounts, "readiness-probe:1.0.0", mdb)

		// Apply the modification to a c
		c := &corev1.Container{}
		containerMod(c)

		// Verify that the c has the custom resources
		assert.Equal(t, readinessRes, c.Resources)
	})
}

func TestVersionUpgradeHookInitWithCustomResources(t *testing.T) {
	// Create custom resource requirements for version upgrade hook
	hookRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("100Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("50Mi"),
		},
	}

	// Create a MongoDBCommunity with custom resources
	mdb := &mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mongodb",
			Namespace: "my-namespace",
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.4.0",
			Resources: mdbv1.ResourcesSpec{
				VersionUpgradeHook: &hookRes,
			},
		},
	}

	// Create volume mounts
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "hooks-volume",
			MountPath: "/hooks",
		},
	}

	// Test the versionUpgradeHookInit function
	t.Run("versionUpgradeHookInit should use custom resources", func(t *testing.T) {
		// Create the c
		containerMod := versionUpgradeHookInit(volumeMounts, "version-upgrade-hook:1.0.0", mdb)

		// Apply the modification to a c
		c := &corev1.Container{}
		containerMod(c)

		// Verify that the c has the custom resources
		assert.Equal(t, hookRes, c.Resources)
	})
}

func TestResourcesArePassedToContainers(t *testing.T) {
	// Create custom resource requirements for mongod
	customMongodRes := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	// Create a MongoDBCommunity with custom resources
	mdb := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-mongodb",
			Namespace: "my-namespace",
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: 3,
			Version: "4.4.0",
			Resources: mdbv1.ResourcesSpec{
				Mongod: &customMongodRes,
			},
		},
	}

	// Test the mongodbContainer function
	t.Run("mongodbContainer uses custom resources when provided", func(t *testing.T) {
		c := container.New(mongodbContainer("mongo:4.4.0", []corev1.VolumeMount{}, mdbv1.NewMongodConfiguration(), &mdb))
		assert.Equal(t, customMongodRes, c.Resources)
	})

	// Test the mongodbAgentContainer function
	t.Run("getContainerResources returns custom resources when provided", func(t *testing.T) {
		res := getContainerResources(&mdb, MongodbName)
		assert.Equal(t, customMongodRes, res)
	})
}
