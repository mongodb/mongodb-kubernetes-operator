package construct

import (
	"fmt"
	"github.com/stretchr/objx"
	"os"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/persistentvolumeclaim"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

const (
	AgentName   = "mongodb-agent"
	MongodbName = "mongod"

	versionUpgradeHookName            = "mongod-posthook"
	ReadinessProbeContainerName       = "mongodb-agent-readinessprobe"
	readinessProbePath                = "/opt/scripts/readinessprobe"
	agentHealthStatusFilePathEnv      = "AGENT_STATUS_FILEPATH"
	clusterFilePath                   = "/var/lib/automation/config/cluster-config.json"
	mongodbDatabaseServiceAccountName = "mongodb-database"
	agentHealthStatusFilePathValue    = "/var/log/mongodb-mms-automation/healthstatus/agent-health-status.json"

	MongodbRepoUrl = "MONGODB_REPO_URL"

	headlessAgentEnv           = "HEADLESS_AGENT"
	podNamespaceEnv            = "POD_NAMESPACE"
	automationConfigEnv        = "AUTOMATION_CONFIG_MAP"
	AgentImageEnv              = "AGENT_IMAGE"
	MongodbImageEnv            = "MONGODB_IMAGE"
	VersionUpgradeHookImageEnv = "VERSION_UPGRADE_HOOK_IMAGE"
	ReadinessProbeImageEnv     = "READINESS_PROBE_IMAGE"
	ManagedSecurityContextEnv  = "MANAGED_SECURITY_CONTEXT"

	defaultDataDir               = "/data"
	automationMongodConfFileName = "automation-mongod.conf"
	keyfileFilePath              = "/var/lib/mongodb-mms-automation/authentication/keyfile"

	automationAgentOptions = " -skipMongoStart -noDaemonize -useLocalMongoDbTools"

	MongodbUserCommand = `current_uid=$(id -u)
declare -r current_uid
if ! grep -q "${current_uid}" /etc/passwd ; then
sed -e "s/^mongodb:/builder:/" /etc/passwd > /tmp/passwd
echo "mongodb:x:$(id -u):$(id -g):,,,:/:/bin/bash" >> /tmp/passwd
export NSS_WRAPPER_PASSWD=/tmp/passwd
export LD_PRELOAD=libnss_wrapper.so
export NSS_WRAPPER_GROUP=/etc/group
fi
`
)

// MongoDBStatefulSetOwner is an interface which any resource which generates a MongoDB StatefulSet should implement.
type MongoDBStatefulSetOwner interface {
	// ServiceName returns the name of the K8S service the operator will create.
	ServiceName() string
	// GetName returns the name of the resource.
	GetName() string
	// GetNamespace returns the namespace the resource is defined in.
	GetNamespace() string
	// GetMongoDBVersion returns the version of MongoDB to be used for this resource
	GetMongoDBVersion() string
	// AutomationConfigSecretName returns the name of the secret which will contain the automation config.
	AutomationConfigSecretName() string
	// GetUpdateStrategyType returns the UpdateStrategyType of the statefulset.
	GetUpdateStrategyType() appsv1.StatefulSetUpdateStrategyType
	// HasSeparateDataAndLogsVolumes returns whether or not the volumes for data and logs would need to be different.
	HasSeparateDataAndLogsVolumes() bool
	// GetAgentScramKeyfileSecretNamespacedName returns the NamespacedName of the secret which stores the keyfile for the agent.
	GetAgentKeyfileSecretNamespacedName() types.NamespacedName
	// DataVolumeName returns the name that the data volume should have
	DataVolumeName() string
	// LogsVolumeName returns the name that the data volume should have
	LogsVolumeName() string

	// GetMongodConfiguration returns the MongoDB configuration for each member.
	GetMongodConfiguration() map[string]interface{}
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

	keyFileNsName := mdb.GetAgentKeyfileSecretNamespacedName()
	keyFileVolume := statefulset.CreateVolumeFromEmptyDir(keyFileNsName.Name)
	keyFileVolumeVolumeMount := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))
	keyFileVolumeVolumeMountMongod := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))

	mongodbAgentVolumeMounts := []corev1.VolumeMount{agentHealthStatusVolumeMount, automationConfigVolumeMount, scriptsVolumeMount, keyFileVolumeVolumeMount}
	mongodVolumeMounts := []corev1.VolumeMount{mongodHealthStatusVolumeMount, hooksVolumeMount, keyFileVolumeVolumeMountMongod}
	dataVolumeClaim := statefulset.NOOP()
	logVolumeClaim := statefulset.NOOP()
	singleModeVolumeClaim := func(s *appsv1.StatefulSet) {}
	if mdb.HasSeparateDataAndLogsVolumes() {
		logVolumeMount := statefulset.CreateVolumeMount(mdb.LogsVolumeName(), automationconfig.DefaultAgentLogPath)
		dataVolumeMount := statefulset.CreateVolumeMount(mdb.DataVolumeName(), GetDBDataDir(mdb.GetMongodConfiguration()))
		dataVolumeClaim = statefulset.WithVolumeClaim(mdb.DataVolumeName(), dataPvc(mdb.DataVolumeName()))
		logVolumeClaim = statefulset.WithVolumeClaim(mdb.LogsVolumeName(), logsPvc(mdb.LogsVolumeName()))
		mongodbAgentVolumeMounts = append(mongodbAgentVolumeMounts, dataVolumeMount, logVolumeMount)
		mongodVolumeMounts = append(mongodVolumeMounts, dataVolumeMount, logVolumeMount)
	} else {
		mounts := []corev1.VolumeMount{
			statefulset.CreateVolumeMount(mdb.DataVolumeName(), GetDBDataDir(mdb.GetMongodConfiguration()), statefulset.WithSubPath("data")),
			statefulset.CreateVolumeMount(mdb.DataVolumeName(), automationconfig.DefaultAgentLogPath, statefulset.WithSubPath("logs")),
		}
		mongodbAgentVolumeMounts = append(mongodbAgentVolumeMounts, mounts...)
		mongodVolumeMounts = append(mongodVolumeMounts, mounts...)
		singleModeVolumeClaim = statefulset.WithVolumeClaim(mdb.DataVolumeName(), dataPvc(mdb.DataVolumeName()))
	}

	podSecurityContext := podtemplatespec.NOOP()
	managedSecurityContext := envvar.ReadBool(ManagedSecurityContextEnv)
	if !managedSecurityContext {
		podSecurityContext = podtemplatespec.WithSecurityContext(podtemplatespec.DefaultPodSecurityContext())
	}

	return statefulset.Apply(
		statefulset.WithName(mdb.GetName()),
		statefulset.WithNamespace(mdb.GetNamespace()),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithLabels(labels),
		statefulset.WithMatchLabels(labels),
		statefulset.WithReplicas(scale.ReplicasThisReconciliation(scaler)),
		statefulset.WithUpdateStrategyType(mdb.GetUpdateStrategyType()),
		dataVolumeClaim,
		logVolumeClaim,
		singleModeVolumeClaim,
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				podSecurityContext,
				podtemplatespec.WithPodLabels(labels),
				podtemplatespec.WithVolume(healthStatusVolume),
				podtemplatespec.WithVolume(hooksVolume),
				podtemplatespec.WithVolume(automationConfigVolume),
				podtemplatespec.WithVolume(scriptsVolume),
				podtemplatespec.WithVolume(keyFileVolume),
				podtemplatespec.WithServiceAccount(mongodbDatabaseServiceAccountName),
				podtemplatespec.WithContainer(AgentName, mongodbAgentContainer(mdb.AutomationConfigSecretName(), mongodbAgentVolumeMounts)),
				podtemplatespec.WithContainer(MongodbName, mongodbContainer(mdb.GetMongoDBVersion(), mongodVolumeMounts, mdb.GetMongodConfiguration())),
				podtemplatespec.WithInitContainer(versionUpgradeHookName, versionUpgradeHookInit([]corev1.VolumeMount{hooksVolumeMount})),
				podtemplatespec.WithInitContainer(ReadinessProbeContainerName, readinessProbeInit([]corev1.VolumeMount{scriptsVolumeMount})),
			),
		))
}

func BaseAgentCommand() string {
	return "agent/mongodb-agent -cluster=" + clusterFilePath + " -healthCheckFilePath=" + agentHealthStatusFilePathValue + " -serveStatusPort=5000"
}

func AutomationAgentCommand() []string {
	return []string{"/bin/bash", "-c", MongodbUserCommand + BaseAgentCommand() + automationAgentOptions}
}

func mongodbAgentContainer(automationConfigSecretName string, volumeMounts []corev1.VolumeMount) container.Modification {
	securityContext := container.NOOP()
	managedSecurityContext := envvar.ReadBool(ManagedSecurityContextEnv)
	if !managedSecurityContext {
		securityContext = container.WithSecurityContext(container.DefaultSecurityContext())
	}
	return container.Apply(
		container.WithName(AgentName),
		container.WithImage(os.Getenv(AgentImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithReadinessProbe(DefaultReadiness()),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithVolumeMounts(volumeMounts),
		securityContext,
		container.WithCommand(AutomationAgentCommand()),
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
		container.WithImage(os.Getenv(VersionUpgradeHookImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithVolumeMounts(volumeMount),
	)
}

func DefaultReadiness() probes.Modification {
	return probes.Apply(
		probes.WithExecCommand([]string{readinessProbePath}),
		probes.WithFailureThreshold(40),
		probes.WithInitialDelaySeconds(5),
	)
}

func dataPvc(dataVolumeName string) persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(dataVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildDefaultStorageRequirements()),
	)
}

func logsPvc(logsVolumeName string) persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(logsVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildStorageRequirements("2G")),
	)
}

// readinessProbeInit returns a modification function which will add the readiness probe container.
// this container will copy the readiness probe binary into the /opt/scripts directory.
func readinessProbeInit(volumeMount []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(ReadinessProbeContainerName),
		container.WithCommand([]string{"cp", "/probes/readinessprobe", "/opt/scripts/readinessprobe"}),
		container.WithImage(os.Getenv(ReadinessProbeImageEnv)),
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

// GetDBDataDir returns the db path which should be used.
func GetDBDataDir(additionalMongoDBConfig objx.Map) string {
	if additionalMongoDBConfig == nil {
		return defaultDataDir
	}
	return additionalMongoDBConfig.Get("storage.dbPath").Str(defaultDataDir)
}

func mongodbContainer(version string, volumeMounts []corev1.VolumeMount, additionalMongoDBConfig map[string]interface{}) container.Modification {
	filePath := GetDBDataDir(additionalMongoDBConfig) + "/" + automationMongodConfFileName
	mongoDbCommand := fmt.Sprintf(`
#run post-start hook to handle version changes
/hooks/version-upgrade

# wait for config and keyfile to be created by the agent
 while ! [ -f %s -a -f %s ]; do sleep 3 ; done ; sleep 2 ;

# with mongod configured to append logs, we need to provide them to stdout as
# mongod does not write to stdout and a log file
tail -F /var/log/mongodb-mms-automation/mongodb.log > /dev/stdout &

# start mongod with this configuration
exec mongod -f %s;

`, filePath, keyfileFilePath, filePath)

	containerCommand := []string{
		"/bin/sh",
		"-c",
		mongoDbCommand,
	}

	securityContext := container.NOOP()
	managedSecurityContext := envvar.ReadBool(ManagedSecurityContextEnv)
	if !managedSecurityContext {
		securityContext = container.WithSecurityContext(container.DefaultSecurityContext())
	}

	return container.Apply(
		container.WithName(MongodbName),
		container.WithImage(getMongoDBImage(version)),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithCommand(containerCommand),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  agentHealthStatusFilePathEnv,
				Value: "/healthstatus/agent-health-status.json",
			},
		),
		container.WithVolumeMounts(volumeMounts),

		securityContext,
	)
}
