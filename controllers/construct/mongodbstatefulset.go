package construct

import (
	"fmt"
	"os"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/persistentvolumeclaim"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

const (
	AgentName   = "mongodb-agent"
	MongodbName = "mongod"

	AgentImageEnv                  = "AGENT_IMAGE"
	versionUpgradeHookName         = "mongod-posthook"
	readinessProbeContainerName    = "mongodb-agent-readinessprobe"
	dataVolumeName                 = "data-volume"
	logVolumeName                  = "logs-volume"
	readinessProbePath             = "/opt/scripts/readinessprobe"
	agentHealthStatusFilePathEnv   = "AGENT_STATUS_FILEPATH"
	clusterFilePath                = "/var/lib/automation/config/cluster-config.json"
	operatorServiceAccountName     = "mongodb-kubernetes-operator"
	agentHealthStatusFilePathValue = "/var/log/mongodb-mms-automation/healthstatus/agent-health-status.json"

	readinessProbeImageEnv = "READINESS_PROBE_IMAGE"

	MongodbImageEnv = "MONGODB_IMAGE"
	MongodbRepoUrl  = "MONGODB_REPO_URL"

	versionUpgradeHookImageEnv = "VERSION_UPGRADE_HOOK_IMAGE"
	headlessAgentEnv           = "HEADLESS_AGENT"
	podNamespaceEnv            = "POD_NAMESPACE"
	automationConfigEnv        = "AUTOMATION_CONFIG_MAP"
)

// MongoDBStatefulSetOwner is an interface which any resource which generates a MongoDB StatefulSet should implement.
type MongoDBStatefulSetOwner interface {
	ServiceName() string
	GetName() string
	GetNamespace() string
	GetMongoDBVersion() string
	AutomationConfigSecretName() string
	GetAgentKeyfileSecretNamespacedName() types.NamespacedName
}

// BuildMongoDBReplicaSetStatefulSetModificationFunction builds the parts of the replica set that are common between every resource that implements
// MongoDBStatefulSetOwner.
// It doesn't configure TLS or additional containers/env vars that the statefulset might need.
func BuildMongoDBReplicaSetStatefulSetModificationFunction(mdb MongoDBStatefulSetOwner, scaler scale.ReplicaSetScaler) statefulset.Modification {
	labels := map[string]string{
		"app": mdb.ServiceName(),
	}

	// the health status volume is required in both agent and mongod pods.
	// the mongod requires it to determine if an upgrade is happening and needs to kill the pod
	// to prevent agent deadlock
	healthStatusVolume := statefulset.CreateVolumeFromEmptyDir("healthstatus")
	agentHealthStatusVolumeMount := statefulset.CreateVolumeMount(healthStatusVolume.Name, "/var/log/mongodb-mms-automation/healthstatus")
	mongodHealthStatusVolumeMount := statefulset.CreateVolumeMount(healthStatusVolume.Name, "/healthstatus")

	// hooks volume is only required on the mongod pod.
	hooksVolume := statefulset.CreateVolumeFromEmptyDir("hooks")
	hooksVolumeMount := statefulset.CreateVolumeMount(hooksVolume.Name, "/hooks", statefulset.WithReadOnly(false))

	// scripts volume is only required on the mongodb-agent pod.
	scriptsVolume := statefulset.CreateVolumeFromEmptyDir("agent-scripts")
	scriptsVolumeMount := statefulset.CreateVolumeMount(scriptsVolume.Name, "/opt/scripts", statefulset.WithReadOnly(false))

	automationConfigVolume := statefulset.CreateVolumeFromSecret("automation-config", mdb.AutomationConfigSecretName())
	automationConfigVolumeMount := statefulset.CreateVolumeMount(automationConfigVolume.Name, "/var/lib/automation/config", statefulset.WithReadOnly(true))

	dataVolumeMount := statefulset.CreateVolumeMount(dataVolumeName, "/data")
	logVolumeMount := statefulset.CreateVolumeMount(logVolumeName, automationconfig.DefaultAgentLogPath)

	mode := int32(0600)
	keyFileNsName := mdb.GetAgentKeyfileSecretNamespacedName()
	keyFileVolume := statefulset.CreateVolumeFromSecret(keyFileNsName.Name, keyFileNsName.Name, statefulset.WithSecretDefaultMode(&mode))
	keyFileVolumeVolumeMount := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))
	keyFileVolumeVolumeMountMongod := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))

	return statefulset.Apply(
		statefulset.WithName(mdb.GetName()),
		statefulset.WithNamespace(mdb.GetNamespace()),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithLabels(labels),
		statefulset.WithMatchLabels(labels),
		statefulset.WithReplicas(scale.ReplicasThisReconciliation(scaler)),
		statefulset.WithVolumeClaim(dataVolumeName, dataPvc()),
		statefulset.WithVolumeClaim(logVolumeName, logsPvc()),
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				podtemplatespec.WithPodLabels(labels),
				podtemplatespec.WithVolume(healthStatusVolume),
				podtemplatespec.WithVolume(hooksVolume),
				podtemplatespec.WithVolume(automationConfigVolume),
				podtemplatespec.WithVolume(scriptsVolume),
				podtemplatespec.WithVolume(keyFileVolume),
				podtemplatespec.WithServiceAccount(operatorServiceAccountName),
				podtemplatespec.WithContainer(AgentName, mongodbAgentContainer(mdb.AutomationConfigSecretName(), []corev1.VolumeMount{agentHealthStatusVolumeMount, automationConfigVolumeMount, dataVolumeMount, scriptsVolumeMount, logVolumeMount, keyFileVolumeVolumeMount})),
				podtemplatespec.WithContainer(MongodbName, mongodbContainer(mdb.GetMongoDBVersion(), []corev1.VolumeMount{mongodHealthStatusVolumeMount, dataVolumeMount, hooksVolumeMount, logVolumeMount, keyFileVolumeVolumeMountMongod})),
				podtemplatespec.WithInitContainer(versionUpgradeHookName, versionUpgradeHookInit([]corev1.VolumeMount{hooksVolumeMount})),
				podtemplatespec.WithInitContainer(readinessProbeContainerName, readinessProbeInit([]corev1.VolumeMount{scriptsVolumeMount})),
			),
		))
}

func mongodbAgentContainer(automationConfigSecretName string, volumeMounts []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(AgentName),
		container.WithImage(os.Getenv(AgentImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithReadinessProbe(DefaultReadiness()),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithVolumeMounts(volumeMounts),
		container.WithCommand([]string{
			"agent/mongodb-agent",
			"-cluster=" + clusterFilePath,
			"-skipMongoStart",
			"-noDaemonize",
			"-healthCheckFilePath=" + agentHealthStatusFilePathValue,
			"-serveStatusPort=5000",
			"-useLocalMongoDbTools",
		},
		),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  headlessAgentEnv,
				Value: "true",
			},
			corev1.EnvVar{
				Name: podNamespaceEnv,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			corev1.EnvVar{
				Name:  automationConfigEnv,
				Value: automationConfigSecretName,
			},
			corev1.EnvVar{
				Name:  agentHealthStatusFilePathEnv,
				Value: agentHealthStatusFilePathValue,
			},
		),
	)
}

func versionUpgradeHookInit(volumeMount []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(versionUpgradeHookName),
		container.WithCommand([]string{"cp", "version-upgrade-hook", "/hooks/version-upgrade"}),
		container.WithImage(os.Getenv(versionUpgradeHookImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithVolumeMounts(volumeMount),
	)
}

func DefaultReadiness() probes.Modification {
	return probes.Apply(
		probes.WithExecCommand([]string{readinessProbePath}),
		probes.WithFailureThreshold(60), // TODO: this value needs further consideration
		probes.WithInitialDelaySeconds(5),
	)
}

func dataPvc() persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(dataVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildDefaultStorageRequirements()),
	)
}

func logsPvc() persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(logVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildStorageRequirements("2G")),
	)
}

// readinessProbeInit returns a modification function which will add the readiness probe container.
// this container will copy the readiness probe binary into the /opt/scripts directory.
func readinessProbeInit(volumeMount []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(readinessProbeContainerName),
		container.WithCommand([]string{"cp", "/probes/readinessprobe", "/opt/scripts/readinessprobe"}),
		container.WithImage(os.Getenv(readinessProbeImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithVolumeMounts(volumeMount),
	)
}

func getMongoDBImage(version string) string {
	repoUrl := os.Getenv(MongodbRepoUrl)
	if strings.HasSuffix(repoUrl, "/") {
		repoUrl = strings.TrimRight(repoUrl, "/")
	}
	mongoImageName := os.Getenv(MongodbImageEnv)
	return fmt.Sprintf("%s/%s:%s", repoUrl, mongoImageName, version)
}

func mongodbContainer(version string, volumeMounts []corev1.VolumeMount) container.Modification {
	mongoDbCommand := []string{
		"/bin/sh",
		"-c",
		`
# run post-start hook to handle version changes
/hooks/version-upgrade

# wait for config to be created by the agent
while [ ! -f /data/automation-mongod.conf ]; do sleep 3 ; done ; sleep 2 ;

# start mongod with this configuration
exec mongod -f /data/automation-mongod.conf ;
`,
	}

	return container.Apply(
		container.WithName(MongodbName),
		container.WithImage(getMongoDBImage(version)),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithCommand(mongoDbCommand),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  agentHealthStatusFilePathEnv,
				Value: "/healthstatus/agent-health-status.json",
			},
		),
		container.WithVolumeMounts(volumeMounts),
	)
}
