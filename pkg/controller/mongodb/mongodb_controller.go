package mongodb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/persistentvolumeclaim"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller/predicates"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	mdbClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	agentImageEnv                = "AGENT_IMAGE"
	preStopHookImageEnv          = "PRE_STOP_HOOK_IMAGE"
	agentHealthStatusFilePathEnv = "AGENT_STATUS_FILEPATH"
	preStopHookLogFilePathEnv    = "PRE_STOP_HOOK_LOG_PATH"

	AutomationConfigKey            = "automation-config"
	agentName                      = "mongodb-agent"
	mongodbName                    = "mongod"
	preStopHookName                = "mongod-prehook"
	dataVolumeName                 = "data-volume"
	versionManifestFilePath        = "/usr/local/version_manifest.json"
	readinessProbePath             = "/var/lib/mongodb-mms-automation/probes/readinessprobe"
	clusterFilePath                = "/var/lib/automation/config/automation-config"
	operatorServiceAccountName     = "mongodb-kubernetes-operator"
	agentHealthStatusFilePathValue = "/var/log/mongodb-mms-automation/healthstatus/agent-health-status.json"

	tlsCAMountPath     = "/var/lib/tls/ca/"
	tlsCACertName      = "ca.crt"
	tlsSecretMountPath = "/var/lib/tls/secret/"
	tlsSecretCertName  = "tls.crt"
	tlsSecretKeyName   = "tls.key"
	tlsServerMountPath = "/var/lib/tls/server/"
	tlsServerFileName  = "server.pem"
)

// Add creates a new MongoDB Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, readVersionManifestFromDisk))
}

// ManifestProvider is a function which returns the VersionManifest which
// contains the list of all available MongoDB versions
type ManifestProvider func() (automationconfig.VersionManifest, error)

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, manifestProvider ManifestProvider) reconcile.Reconciler {
	mgrClient := mgr.GetClient()
	return &ReplicaSetReconciler{
		client:           mdbClient.NewClient(mgrClient),
		scheme:           mgr.GetScheme(),
		manifestProvider: manifestProvider,
		log:              zap.S(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("replicaset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MongoDB
	err = c.Watch(&source.Kind{Type: &mdbv1.MongoDB{}}, &handler.EnqueueRequestForObject{}, predicates.OnlyOnSpecChange())
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReplicaSetReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReplicaSetReconciler{}

// ReplicaSetReconciler reconciles a MongoDB ReplicaSet
type ReplicaSetReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client           mdbClient.Client
	scheme           *runtime.Scheme
	manifestProvider func() (automationconfig.VersionManifest, error)
	log              *zap.SugaredLogger
}

// Reconcile reads that state of the cluster for a MongoDB object and makes changes based on the state read
// and what is in the MongoDB.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReplicaSetReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log = zap.S().With("ReplicaSet", request.NamespacedName)
	r.log.Info("Reconciling MongoDB")

	// TODO: generalize preparation for resource
	// Fetch the MongoDB instance
	mdb := mdbv1.MongoDB{}
	err := r.client.Get(context.TODO(), request.NamespacedName, &mdb)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		r.log.Errorf("error reconciling MongoDB resource: %s", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if err := r.ensureAutomationConfig(mdb); err != nil {
		r.log.Warnf("error creating automation config config map: %s", err)
		return reconcile.Result{}, err
	}

	r.log.Debug("Ensuring the service exists")
	if err := r.ensureService(mdb); err != nil {
		r.log.Warnf("Error ensuring the service exists: %s", err)
		return reconcile.Result{}, err
	}

	r.log.Debug("Creating/Updating StatefulSet")
	if err := r.createOrUpdateStatefulSet(mdb); err != nil {
		r.log.Warnf("Error creating/updating StatefulSet: %+v", err)
		return reconcile.Result{}, err
	}

	currentSts := appsv1.StatefulSet{}
	if err := r.client.Get(context.TODO(), mdb.NamespacedName(), &currentSts); err != nil {
		r.log.Warnf("Error getting StatefulSet: %s", err)
		return reconcile.Result{}, err
	}

	r.log.Debugf("Ensuring StatefulSet is ready, with type: %s", getUpdateStrategyType(mdb))
	ready, err := r.isStatefulSetReady(mdb, &currentSts)
	if err != nil {
		r.log.Warnf("error checking StatefulSet status: %+v", err)
		return reconcile.Result{}, err
	}

	if !ready {
		r.log.Infof("StatefulSet %s/%s is not yet ready, retrying in 10 seconds", mdb.Namespace, mdb.Name)
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}

	r.log.Debug("Resetting StatefulSet UpdateStrategy")
	if err := r.resetStatefulSetUpdateStrategy(mdb); err != nil {
		r.log.Warnf("error resetting StatefulSet UpdateStrategyType: %+v", err)
		return reconcile.Result{}, err
	}

	r.log.Debug("Setting MongoDB Annotations")
	if err := r.setAnnotation(types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, mdbv1.LastVersionAnnotationKey, mdb.Spec.Version); err != nil {
		r.log.Warnf("Error setting annotation: %+v", err)
		return reconcile.Result{}, err
	}

	if err := r.completeTLSRollout(mdb); err != nil {
		return reconcile.Result{}, err
	}

	r.log.Debug("Updating MongoDB Status")
	newStatus, err := r.updateAndReturnStatusSuccess(&mdb)
	if err != nil {
		r.log.Warnf("Error updating the status of the MongoDB resource: %+v", err)
		return reconcile.Result{}, err
	}

	r.log.Infow("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status", newStatus)
	return reconcile.Result{}, nil
}

// resetStatefulSetUpdateStrategy ensures the stateful set is configured back to using RollingUpdateStatefulSetStrategyType
// and does not keep using OnDelete
func (r *ReplicaSetReconciler) resetStatefulSetUpdateStrategy(mdb mdbv1.MongoDB) error {
	if !mdb.IsChangingVersion() {
		return nil
	}
	// if we changed the version, we need to reset the UpdatePolicy back to OnUpdate
	sts := &appsv1.StatefulSet{}
	return r.client.GetAndUpdate(types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, sts, func() {
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	})
}

// isStatefulSetReady checks to see if the stateful set corresponding to the given MongoDB resource
// is currently ready.
func (r *ReplicaSetReconciler) isStatefulSetReady(mdb mdbv1.MongoDB, existingStatefulSet *appsv1.StatefulSet) (bool, error) {
	stsFunc := buildStatefulSetModificationFunction(mdb)
	stsCopy := existingStatefulSet.DeepCopyObject()
	stsFunc(existingStatefulSet)
	stsCopyBytes, err := json.Marshal(stsCopy)
	if err != nil {
		return false, err
	}

	stsBytes, err := json.Marshal(existingStatefulSet)
	if err != nil {
		return false, err
	}

	// comparison is done with bytes instead of reflect.DeepEqual as there are
	// some issues with nil/empty maps not being compared correctly otherwise
	areEqual := bytes.Compare(stsCopyBytes, stsBytes) == 0
	isReady := statefulset.IsReady(*existingStatefulSet, mdb.Spec.Members)
	return areEqual && isReady, nil
}

func (r *ReplicaSetReconciler) ensureService(mdb mdbv1.MongoDB) error {
	svc := buildService(mdb)
	err := r.client.Create(context.TODO(), &svc)
	if err != nil && errors.IsAlreadyExists(err) {
		r.log.Infof("The service already exists... moving forward: %s", err)
		return nil
	}
	return err
}

func (r *ReplicaSetReconciler) createOrUpdateStatefulSet(mdb mdbv1.MongoDB) error {
	set := appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), mdb.NamespacedName(), &set)
	err = k8sClient.IgnoreNotFound(err)
	if err != nil {
		return fmt.Errorf("error getting StatefulSet: %s", err)
	}
	buildStatefulSetModificationFunction(mdb)(&set)
	if err = r.client.CreateOrUpdate(&set); err != nil {
		return fmt.Errorf("error creating/updating StatefulSet: %s", err)
	}
	return nil
}

// completeTLSRollout will update the automation config and set an annotation indicating that TLS has been rolled out.
// At this stage, TLS hasn't yet been enabled but the keys and certs have all been mounted.
// The automation config will be updated and the agents will continue work on gradually enabling TLS across the replica set.
func (r *ReplicaSetReconciler) completeTLSRollout(mdb mdbv1.MongoDB) error {
	if mdb.Spec.TLS.Enabled && !mdb.HasRolledOutTLS() {
		r.log.Debug("Completing TLS rollout")

		mdb.Annotations[mdbv1.TLSRolledOutKey] = "true"
		if err := r.ensureAutomationConfig(mdb); err != nil {
			r.log.Warnf("error updating automation config after TLS rollout: %s", err)
			return err
		}

		if err := r.setAnnotation(types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, mdbv1.TLSRolledOutKey, "true"); err != nil {
			r.log.Warnf("Error setting TLS annotation: %+v", err)
			return err
		}
	}

	return nil
}

// setAnnotation updates the monogdb resource with the given namespaced name and sets the annotation
// "key" with the provided value "val"
func (r ReplicaSetReconciler) setAnnotation(nsName types.NamespacedName, key, val string) error {
	mdb := mdbv1.MongoDB{}
	return r.client.GetAndUpdate(nsName, &mdb, func() {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[key] = val
	})
}

// updateAndReturnStatusSuccess should be called after a successful reconciliation
// the resource's status is updated to reflect to the state, and any other cleanup
// operators should be performed here
func (r ReplicaSetReconciler) updateAndReturnStatusSuccess(mdb *mdbv1.MongoDB) (mdbv1.MongoDBStatus, error) {
	newMdb := &mdbv1.MongoDB{}
	if err := r.client.Get(context.TODO(), mdb.NamespacedName(), newMdb); err != nil {
		return mdbv1.MongoDBStatus{}, fmt.Errorf("error getting resource: %+v", err)
	}
	newMdb.UpdateSuccess()
	if err := r.client.Status().Update(context.TODO(), newMdb); err != nil {
		return mdbv1.MongoDBStatus{}, fmt.Errorf("error updating status: %+v", err)
	}
	return newMdb.Status, nil
}

func (r ReplicaSetReconciler) ensureAutomationConfig(mdb mdbv1.MongoDB) error {
	cm, err := r.buildAutomationConfigConfigMap(mdb)
	if err != nil {
		return err
	}

	if err := r.client.CreateOrUpdate(&cm); err != nil {
		return err
	}
	return nil
}

func buildAutomationConfig(mdb mdbv1.MongoDB, mdbVersionConfig automationconfig.MongoDbVersionConfig, client mdbClient.Client) (automationconfig.AutomationConfig, error) {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, "")

	currentAc, err := getCurrentAutomationConfig(client, mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, err
	}
	builder := automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetMembers(mdb.Spec.Members).
		SetPreviousAutomationConfig(currentAc).
		SetMongoDBVersion(mdb.Spec.Version).
		SetFCV(mdb.GetFCV()).
		AddVersion(mdbVersionConfig)

	// Enable TLS in the automation config after the certs and keys have been rolled out to all pods.
	// The agent needs these to be in place before the config is updated.
	// The agents will handle the gradual introduction of TLS in accordance with: https://docs.mongodb.com/manual/tutorial/upgrade-cluster-to-ssl/
	if mdb.Spec.TLS.Enabled && mdb.HasRolledOutTLS() {
		mode := automationconfig.SSLModeRequired
		if mdb.Spec.TLS.Optional {
			// SSLModePreferred requires server-server connections to use TLS but makes it optional for clients.
			mode = automationconfig.SSLModePreferred
		}

		builder.SetTLS(
			tlsCAMountPath+tlsCACertName,
			tlsServerMountPath+tlsServerFileName,
			mode,
		)
	}

	newAc, err := builder.Build()
	if err != nil {
		return automationconfig.AutomationConfig{}, err
	}

	return newAc, nil
}

func readVersionManifestFromDisk() (automationconfig.VersionManifest, error) {
	bytes, err := ioutil.ReadFile(versionManifestFilePath)
	if err != nil {
		return automationconfig.VersionManifest{}, err
	}
	return versionManifestFromBytes(bytes)
}

func versionManifestFromBytes(bytes []byte) (automationconfig.VersionManifest, error) {
	versionManifest := automationconfig.VersionManifest{}
	if err := json.Unmarshal(bytes, &versionManifest); err != nil {
		return automationconfig.VersionManifest{}, err
	}
	return versionManifest, nil
}

// buildService creates a Service that will be used for the Replica Set StatefulSet
// that allows all the members of the STS to see each other.
// TODO: Make sure this Service is as minimal as posible, to not interfere with
// future implementations and Service Discovery mechanisms we might implement.
func buildService(mdb mdbv1.MongoDB) corev1.Service {
	label := make(map[string]string)
	label["app"] = mdb.ServiceName()
	return service.Builder().
		SetName(mdb.ServiceName()).
		SetNamespace(mdb.Namespace).
		SetSelector(label).
		SetServiceType(corev1.ServiceTypeClusterIP).
		SetClusterIP("None").
		SetPort(27017).
		Build()
}

func getCurrentAutomationConfig(client mdbClient.Client, mdb mdbv1.MongoDB) (automationconfig.AutomationConfig, error) {
	currentCm := corev1.ConfigMap{}
	currentAc := automationconfig.AutomationConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &currentCm); err != nil {
		// If the AC was not found we don't surface it as an error
		return automationconfig.AutomationConfig{}, k8sClient.IgnoreNotFound(err)

	}
	if err := json.Unmarshal([]byte(currentCm.Data[AutomationConfigKey]), &currentAc); err != nil {
		return automationconfig.AutomationConfig{}, err
	}
	return currentAc, nil
}

func (r ReplicaSetReconciler) buildAutomationConfigConfigMap(mdb mdbv1.MongoDB) (corev1.ConfigMap, error) {
	manifest, err := r.manifestProvider()
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("error reading version manifest from disk: %+v", err)
	}

	ac, err := buildAutomationConfig(mdb, manifest.BuildsForVersion(mdb.Spec.Version), r.client)
	if err != nil {
		return corev1.ConfigMap{}, err
	}
	acBytes, err := json.Marshal(ac)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	return configmap.Builder().
		SetName(mdb.ConfigMapName()).
		SetNamespace(mdb.Namespace).
		SetField(AutomationConfigKey, string(acBytes)).
		Build(), nil
}

// getUpdateStrategyType returns the type of RollingUpgradeStrategy that the StatefulSet
// should be configured with
func getUpdateStrategyType(mdb mdbv1.MongoDB) appsv1.StatefulSetUpdateStrategyType {
	if !mdb.IsChangingVersion() {
		return appsv1.RollingUpdateStatefulSetStrategyType
	}
	return appsv1.OnDeleteStatefulSetStrategyType
}

// buildStatefulSet takes a MongoDB resource and converts it into
// the corresponding stateful set
func buildStatefulSet(mdb mdbv1.MongoDB) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	buildStatefulSetModificationFunction(mdb)(&sts)
	return sts, nil
}

func mongodbAgentContainer(volumeMounts []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(agentName),
		container.WithImage(os.Getenv(agentImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithReadinessProbe(defaultReadiness()),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithVolumeMounts(volumeMounts),
		container.WithCommand([]string{
			"agent/mongodb-agent",
			"-cluster=" + clusterFilePath,
			"-skipMongoStart",
			"-noDaemonize",
			"-healthCheckFilePath=" + agentHealthStatusFilePathValue,
			"-serveStatusPort=5000",
		},
		),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  agentHealthStatusFilePathEnv,
				Value: agentHealthStatusFilePathValue,
			},
		),
	)
}

func preStopHookInit(volumeMount []corev1.VolumeMount) container.Modification {
	return container.Apply(
		container.WithName(preStopHookName),
		container.WithCommand([]string{"cp", "pre-stop-hook", "/hooks/pre-stop-hook"}),
		container.WithImage(os.Getenv(preStopHookImageEnv)),
		container.WithImagePullPolicy(corev1.PullAlways),
		container.WithVolumeMounts(volumeMount),
	)
}

func mongodbContainer(version string, volumeMounts []corev1.VolumeMount) container.Modification {
	mongoDbCommand := []string{
		"/bin/sh",
		"-c",
		// we execute the pre-stop hook once the mongod has been gracefully shut down by the agent.
		`while [ ! -f /data/automation-mongod.conf ]; do sleep 3 ; done ; sleep 2 ;
# start mongod with this configuration
mongod -f /data/automation-mongod.conf ;

# start the pre-stop-hook to restart the Pod when needed
# If the Pod does not require to be restarted, the pre-stop-hook will
# exit(0) for Kubernetes to restart the container.
/hooks/pre-stop-hook ;
`,
	}

	return container.Apply(
		container.WithName(mongodbName),
		container.WithImage(fmt.Sprintf("mongo:%s", version)),
		container.WithResourceRequirements(resourcerequirements.Defaults()),
		container.WithCommand(mongoDbCommand),
		container.WithEnvs(
			corev1.EnvVar{
				Name:  agentHealthStatusFilePathEnv,
				Value: "/healthstatus/agent-health-status.json",
			},
			corev1.EnvVar{
				Name:  preStopHookLogFilePathEnv,
				Value: "/hooks/pre-stop-hook.log",
			},
		),
		container.WithVolumeMounts(volumeMounts),
	)
}

func buildStatefulSetModificationFunction(mdb mdbv1.MongoDB) statefulset.Modification {
	labels := map[string]string{
		"app": mdb.ServiceName(),
	}

	ownerReferences := []metav1.OwnerReference{
		*metav1.NewControllerRef(&mdb, schema.GroupVersionKind{
			Group:   mdbv1.SchemeGroupVersion.Group,
			Version: mdbv1.SchemeGroupVersion.Version,
			Kind:    mdb.Kind,
		}),
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

	automationConfigVolume := statefulset.CreateVolumeFromConfigMap("automation-config", mdb.ConfigMapName())
	automationConfigVolumeMount := statefulset.CreateVolumeMount(automationConfigVolume.Name, "/var/lib/automation/config", statefulset.WithReadOnly(true))

	dataVolume := statefulset.CreateVolumeMount(dataVolumeName, "/data")

	agentVolumeMounts := []corev1.VolumeMount{agentHealthStatusVolumeMount, automationConfigVolumeMount, dataVolume}
	mongodVolumeMounts := []corev1.VolumeMount{mongodHealthStatusVolumeMount, hooksVolumeMount, dataVolume}

	tlsPodSpec := podtemplatespec.Apply()
	if mdb.Spec.TLS.Enabled {
		// Configure an empty volume into which the TLS init container will write the certificate and key file
		tlsVolume := statefulset.CreateVolumeFromEmptyDir("tls")
		tlsVolumeMount := statefulset.CreateVolumeMount(tlsVolume.Name, tlsServerMountPath, statefulset.WithReadOnly(false))
		agentVolumeMounts = append(agentVolumeMounts, tlsVolumeMount)
		mongodVolumeMounts = append(mongodVolumeMounts, tlsVolumeMount)

		// Configure a volume which mounts the CA certificate from a ConfigMap
		// The certificate is used by both mongod and the agent
		caVolume := statefulset.CreateVolumeFromConfigMap("tls-ca", mdb.Spec.TLS.CAConfigMapRef)
		caVolumeMount := statefulset.CreateVolumeMount(caVolume.Name, tlsCAMountPath, statefulset.WithReadOnly(true))
		agentVolumeMounts = append(agentVolumeMounts, caVolumeMount)
		mongodVolumeMounts = append(mongodVolumeMounts, caVolumeMount)

		// Configure a volume which mounts the secret holding the server key and certificate
		// The same key-certificate pair is used for all servers
		tlsSecretVolume := statefulset.CreateVolumeFromSecret("tls-secret", mdb.Spec.TLS.SecretRef)
		tlsSecretVolumeMount := statefulset.CreateVolumeMount(tlsSecretVolume.Name, tlsSecretMountPath, statefulset.WithReadOnly(true))

		// MongoDB expects both key and certificate to be provided in a single PEM file
		// We are using a secret format where they are stored in separate fields, tls.crt and tls.key
		// Because of this we need to use an init container which reads the two files mounted from the secret and combines them into one
		tlsPodSpec = podtemplatespec.Apply(
			podtemplatespec.WithInitContainer("tls-init", tlsInit(tlsVolumeMount, tlsSecretVolumeMount)),
			podtemplatespec.WithVolume(tlsVolume),
			podtemplatespec.WithVolume(caVolume),
			podtemplatespec.WithVolume(tlsSecretVolume),
		)
	}

	return statefulset.Apply(
		statefulset.WithName(mdb.Name),
		statefulset.WithNamespace(mdb.Namespace),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithLabels(labels),
		statefulset.WithMatchLabels(labels),
		statefulset.WithOwnerReference(ownerReferences),
		statefulset.WithReplicas(mdb.Spec.Members),
		statefulset.WithUpdateStrategyType(getUpdateStrategyType(mdb)),
		statefulset.WithVolumeClaim(dataVolumeName, defaultPvc()),
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				podtemplatespec.WithPodLabels(labels),
				podtemplatespec.WithVolume(healthStatusVolume),
				podtemplatespec.WithVolume(hooksVolume),
				podtemplatespec.WithVolume(automationConfigVolume),
				podtemplatespec.WithServiceAccount(operatorServiceAccountName),
				podtemplatespec.WithContainer(agentName, mongodbAgentContainer(agentVolumeMounts)),
				podtemplatespec.WithContainer(mongodbName, mongodbContainer(mdb.Spec.Version, mongodVolumeMounts)),
				podtemplatespec.WithInitContainer(preStopHookName, preStopHookInit([]corev1.VolumeMount{hooksVolumeMount})),
				tlsPodSpec,
			),
		),
	)
}

// tlsInit creates an init container which combines the mounted tls.key and tls.crt into a single PEM file
func tlsInit(tlsMount, tlsSecretMount corev1.VolumeMount) container.Modification {
	command := fmt.Sprintf(
		"cat %s %s > %s",
		tlsSecretMountPath+tlsSecretCertName,
		tlsSecretMountPath+tlsSecretKeyName,
		tlsServerMountPath+tlsServerFileName)

	return container.Apply(
		container.WithName("tls-init"),
		container.WithImage("busybox"),
		container.WithCommand([]string{"sh", "-c", command}),
		container.WithVolumeMounts([]corev1.VolumeMount{tlsMount, tlsSecretMount}),
	)
}

func getDomain(service, namespace, clusterName string) string {
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, namespace, clusterName)
}

func defaultReadiness() probes.Modification {
	return probes.Apply(
		probes.WithExecCommand([]string{readinessProbePath}),
		probes.WithFailureThreshold(240),
		probes.WithInitialDelaySeconds(5),
	)
}

func defaultPvc() persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(dataVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildDefaultStorageRequirements()),
	)
}
