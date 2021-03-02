package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/functions"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"

	"github.com/pkg/errors"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/imdario/mergo"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/stretchr/objx"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/validation"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/persistentvolumeclaim"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/probes"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	agentImageEnv                = "AGENT_IMAGE"
	clusterDNSName               = "CLUSTER_DNS_NAME"
	versionUpgradeHookImageEnv   = "VERSION_UPGRADE_HOOK_IMAGE"
	readinessProbeImageEnv       = "READINESS_PROBE_IMAGE"
	agentHealthStatusFilePathEnv = "AGENT_STATUS_FILEPATH"
	mongodbImageEnv              = "MONGODB_IMAGE"
	mongodbRepoUrl               = "MONGODB_REPO_URL"
	headlessAgentEnv             = "HEADLESS_AGENT"
	podNamespaceEnv              = "POD_NAMESPACE"
	automationConfigEnv          = "AUTOMATION_CONFIG_MAP"

	agentName                      = "mongodb-agent"
	mongodbName                    = "mongod"
	versionUpgradeHookName         = "mongod-posthook"
	readinessProbeContainerName    = "mongodb-agent-readinessprobe"
	dataVolumeName                 = "data-volume"
	logVolumeName                  = "logs-volume"
	readinessProbePath             = "/opt/scripts/readinessprobe"
	clusterFilePath                = "/var/lib/automation/config/cluster-config.json"
	operatorServiceAccountName     = "mongodb-kubernetes-operator"
	agentHealthStatusFilePathValue = "/var/log/mongodb-mms-automation/healthstatus/agent-health-status.json"

	// lastVersionAnnotationKey should indicate which version of MongoDB was last
	// configured
	lastVersionAnnotationKey    = "mongodb.com/v1.lastVersion"
	lastSuccessfulConfiguration = "mongodb.com/v1.lastSuccessfulConfiguration"
)

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
}

func NewReconciler(mgr manager.Manager) *ReplicaSetReconciler {
	mgrClient := mgr.GetClient()
	secretWatcher := watch.New()

	return &ReplicaSetReconciler{
		client:        kubernetesClient.NewClient(mgrClient),
		scheme:        mgr.GetScheme(),
		log:           zap.S(),
		secretWatcher: &secretWatcher,
	}
}

// SetupWithManager sets up the controller with the Manager and configures the necessary watches.
func (r *ReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mdbv1.MongoDBCommunity{}).
		Complete(r)
}

// ReplicaSetReconciler reconciles a MongoDB ReplicaSet
type ReplicaSetReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        kubernetesClient.Client
	scheme        *runtime.Scheme
	log           *zap.SugaredLogger
	secretWatcher *watch.ResourceWatcher
}

// +kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=mongodbcommunity,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=mongodbcommunity/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=mongodbcommunity/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list

// Reconcile reads that state of the cluster for a MongoDB object and makes changes based on the state read
// and what is in the MongoDB.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r ReplicaSetReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {

	// TODO: generalize preparation for resource
	// Fetch the MongoDB instance
	mdb := mdbv1.MongoDBCommunity{}
	err := r.client.Get(context.TODO(), request.NamespacedName, &mdb)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return result.OK()
		}
		r.log.Errorf("Error reconciling MongoDB resource: %s", err)
		// Error reading the object - requeue the request.
		return result.Failed()
	}

	r.log = zap.S().With("ReplicaSet", request.NamespacedName)
	r.log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)

	r.log.Debug("Validating MongoDB.Spec")
	if err := r.validateUpdate(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("error validating new Spec: %s", err)).
				withFailedPhase(),
		)
	}

	r.log.Debug("Ensuring the service exists")
	if err := r.ensureService(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error ensuring the service exists: %s", err)).
				withFailedPhase(),
		)
	}

	isTLSValid, err := r.validateTLSConfig(mdb)
	if err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error validating TLS config: %s", err)).
				withFailedPhase(),
		)
	}

	if !isTLSValid {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Info, "TLS config is not yet valid, retrying in 10 seconds").
				withPendingPhase(10),
		)
	}

	ready, err := r.deployMongoDBReplicaSet(mdb)
	if err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error deploying MongoDB ReplicaSet: %s", err)).
				withFailedPhase(),
		)
	}

	if !ready {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Info, "ReplicaSet is not yet ready, retrying in 10 seconds").
				withPendingPhase(10),
		)
	}

	r.log.Debug("Resetting StatefulSet UpdateStrategy to RollingUpdate")
	if err := r.resetStatefulSetUpdateStrategy(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
				withFailedPhase(),
		)
	}

	if scale.IsStillScaling(mdb) {
		return status.Update(r.client.Status(), &mdb, statusOptions().
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
				mdb.CurrentReplicas(), mdb.DesiredReplicas())).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
			withPendingPhase(10),
		)
	}

	res, err := status.Update(r.client.Status(), &mdb,
		statusOptions().
			withMongoURI(mdb.MongoURI()).
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
			withMessage(None, "").
			withRunningPhase(),
	)
	if err != nil {
		r.log.Errorf("Error updating the status of the MongoDB resource: %s", err)
		return res, err
	}

	if err := r.updateCurrentSpecAnnotation(mdb); err != nil {
		r.log.Errorf("Could not save current state as an annotation: %s", err)
	}

	if res.RequeueAfter > 0 || res.Requeue {
		r.log.Infow("Requeuing reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status:", mdb.Status)
		return res, nil
	}

	r.log.Infow("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status:", mdb.Status)
	return res, err
}

// deployStatefulSet deploys the backing StatefulSet of the MongoDBCommunity resource.
// The returned boolean indicates that the StatefulSet is ready.
func (r *ReplicaSetReconciler) deployStatefulSet(mdb mdbv1.MongoDBCommunity) (bool, error) {
	r.log.Info("Creating/Updating StatefulSet")
	if err := r.createOrUpdateStatefulSet(mdb); err != nil {
		return false, errors.Errorf("error creating/updating StatefulSet: %s", err)
	}

	currentSts, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if err != nil {
		return false, errors.Errorf("error getting StatefulSet: %s", err)
	}

	r.log.Debugf("Ensuring StatefulSet is ready, with type: %s", getUpdateStrategyType(mdb))

	isReady := statefulset.IsReady(currentSts, mdb.StatefulSetReplicasThisReconciliation())

	if isReady {
		r.log.Infow("StatefulSet is ready",
			"replicas", currentSts.Spec.Replicas,
			"generation", currentSts.Generation,
			"observedGeneration", currentSts.Status.ObservedGeneration,
			"updateStrategy", currentSts.Spec.UpdateStrategy.Type,
		)
	}

	return isReady || currentSts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType, nil
}

// deployAutomationConfig deploys the AutomationConfig for the MongoDBCommunity resource.
// The returned boolean indicates whether or not that Agents have all reached goal state.
func (r *ReplicaSetReconciler) deployAutomationConfig(mdb mdbv1.MongoDBCommunity) (bool, error) {
	r.log.Infof("Creating/Updating AutomationConfig")

	sts, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if err != nil && !apiErrors.IsNotFound(err) {
		return false, fmt.Errorf("failed to get StatefulSet: %s", err)
	}

	ac, err := r.ensureAutomationConfig(mdb)
	if err != nil {
		return false, fmt.Errorf("failed to ensure AutomationConfig: %s", err)
	}

	// the StatefulSet has not yet been created, so the next stage of reconciliation will be
	// creating the StatefulSet and ensuring it reaches the Running phase.
	if apiErrors.IsNotFound(err) {
		return true, nil
	}

	r.log.Debugf("Waiting for agents to reach version %d", ac.Version)
	// Note: we pass in the expected number of replicas this reconciliation as we scale members one at a time. If we were
	// to pass in the final member count, we would be waiting for agents that do not exist yet to be ready.
	ready, err := agent.AllReachedGoalState(sts, r.client, mdb.StatefulSetReplicasThisReconciliation(), ac.Version, r.log)
	if err != nil {
		return false, fmt.Errorf("failed to ensure agents have reached goal state: %s", err)
	}

	return ready, nil
}

// shouldRunInOrder returns true if the order of execution of the AutomationConfig & StatefulSet
// functions should be sequential or not. A value of false indicates they will run in reversed order.
func (r *ReplicaSetReconciler) shouldRunInOrder(mdb mdbv1.MongoDBCommunity) bool {
	// The only case when we push the StatefulSet first is when we are ensuring TLS for the already existing ReplicaSet
	_, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if err == nil && mdb.Spec.Security.TLS.Enabled {
		r.log.Debug("Enabling TLS on an existing deployment, the StatefulSet must be updated first")
		return false
	}

	// if we are scaling up, we need to make sure the StatefulSet is scaled up first.
	if scale.IsScalingUp(mdb) {
		r.log.Debug("Scaling up the ReplicaSet, the StatefulSet must be updated first")
		return false
	}

	if scale.IsScalingDown(mdb) {
		r.log.Debug("Scaling down the ReplicaSet, the Automation Config must be updated first")
		return true
	}

	// when we change version, we need the StatefulSet images to be updated first, then the agent can get to goal
	// state on the new version.
	if isChangingVersion(mdb) {
		r.log.Debug("Version change in progress, the StatefulSet must be updated first")
		return false
	}

	return true
}

// deployMongoDBReplicaSet will ensure that both the AutomationConfig secret and backing StatefulSet
// have been successfully created. A boolean is returned indicating if the process is complete
// and an error if there was one.
func (r *ReplicaSetReconciler) deployMongoDBReplicaSet(mdb mdbv1.MongoDBCommunity) (bool, error) {
	return functions.RunSequentially(r.shouldRunInOrder(mdb),
		func() (bool, error) {
			return r.deployAutomationConfig(mdb)
		},
		func() (bool, error) {
			return r.deployStatefulSet(mdb)
		})
}

// resetStatefulSetUpdateStrategy ensures the stateful set is configured back to using RollingUpdateStatefulSetStrategyType
// and does not keep using OnDelete
func (r *ReplicaSetReconciler) resetStatefulSetUpdateStrategy(mdb mdbv1.MongoDBCommunity) error {
	if !isChangingVersion(mdb) {
		return nil
	}
	// if we changed the version, we need to reset the UpdatePolicy back to OnUpdate
	_, err := statefulset.GetAndUpdate(r.client, mdb.NamespacedName(), func(sts *appsv1.StatefulSet) {
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	})
	return err
}

func (r *ReplicaSetReconciler) ensureService(mdb mdbv1.MongoDBCommunity) error {
	svc := buildService(mdb)
	err := r.client.Create(context.TODO(), &svc)
	if err != nil && apiErrors.IsAlreadyExists(err) {
		r.log.Infof("The service already exists... moving forward: %s", err)
		return nil
	}
	return err
}

func (r *ReplicaSetReconciler) createOrUpdateStatefulSet(mdb mdbv1.MongoDBCommunity) error {
	set := appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), mdb.NamespacedName(), &set)
	err = k8sClient.IgnoreNotFound(err)
	if err != nil {
		return errors.Errorf("error getting StatefulSet: %s", err)
	}
	buildStatefulSetModificationFunction(mdb)(&set)
	if _, err = statefulset.CreateOrUpdate(r.client, set); err != nil {
		return errors.Errorf("error creating/updating StatefulSet: %s", err)
	}
	return nil
}

// setAnnotations updates the mongodb resource annotations by applying the provided annotations
// on top of the existing ones
func (r ReplicaSetReconciler) setAnnotations(nsName types.NamespacedName, annotations map[string]string) error {
	mdb := mdbv1.MongoDBCommunity{}
	return r.client.GetAndUpdate(nsName, &mdb, func() {
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		for key, val := range annotations {
			mdb.Annotations[key] = val
		}
	})
}

// ensureAutomationConfig makes sure the AutomationConfig secret has been successfully created. The automation config
// that was updated/created is returned.
func (r ReplicaSetReconciler) ensureAutomationConfig(mdb mdbv1.MongoDBCommunity) (automationconfig.AutomationConfig, error) {
	ac, err := r.buildAutomationConfig(mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, errors.Errorf("could not build automation config: %s", err)
	}

	return automationconfig.EnsureSecret(
		r.client,
		types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace},
		[]metav1.OwnerReference{getOwnerReference(mdb)},
		ac,
	)

}

func buildAutomationConfig(mdb mdbv1.MongoDBCommunity, auth automationconfig.Auth, currentAc automationconfig.AutomationConfig, modifications ...automationconfig.Modification) (automationconfig.AutomationConfig, error) {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, os.Getenv(clusterDNSName))
	zap.S().Debugw("AutomationConfigMembersThisReconciliation", "mdb.AutomationConfigMembersThisReconciliation()", mdb.AutomationConfigMembersThisReconciliation())

	return automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetMembers(mdb.AutomationConfigMembersThisReconciliation()).
		SetReplicaSetHorizons(mdb.Spec.ReplicaSetHorizons).
		SetPreviousAutomationConfig(currentAc).
		SetMongoDBVersion(mdb.Spec.Version).
		SetFCV(mdb.GetFCV()).
		SetAuth(auth).
		AddModifications(getMongodConfigModification(mdb)).
		AddModifications(modifications...).
		Build()
}

// buildService creates a Service that will be used for the Replica Set StatefulSet
// that allows all the members of the STS to see each other.
// TODO: Make sure this Service is as minimal as possible, to not interfere with
// future implementations and Service Discovery mechanisms we might implement.
func buildService(mdb mdbv1.MongoDBCommunity) corev1.Service {
	label := make(map[string]string)
	label["app"] = mdb.ServiceName()
	return service.Builder().
		SetName(mdb.ServiceName()).
		SetNamespace(mdb.Namespace).
		SetSelector(label).
		SetServiceType(corev1.ServiceTypeClusterIP).
		SetClusterIP("None").
		SetPort(27017).
		SetPublishNotReadyAddresses(true).
		Build()
}

func getCurrentAutomationConfig(getUpdater secret.GetUpdater, mdb mdbv1.MongoDBCommunity) (automationconfig.AutomationConfig, error) {
	currentSecret, err := getUpdater.GetSecret(types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	if err != nil {
		// If the AC was not found we don't surface it as an error
		return automationconfig.AutomationConfig{}, k8sClient.IgnoreNotFound(err)
	}

	currentAc := automationconfig.AutomationConfig{}
	if err := json.Unmarshal(currentSecret.Data[automationconfig.ConfigKey], &currentAc); err != nil {
		return automationconfig.AutomationConfig{}, err
	}

	return currentAc, nil
}

// validateUpdate validates that the new Spec, corresponding to the existing one
// is still valid. If there is no a previous Spec, then the function assumes this is
// the first version of the MongoDB resource and skips.
func (r ReplicaSetReconciler) validateUpdate(mdb mdbv1.MongoDBCommunity) error {
	lastSuccessfulConfigurationSaved, ok := mdb.Annotations[lastSuccessfulConfiguration]
	if !ok {
		// First version of Spec, no need to validate
		return nil
	}

	prevSpec := mdbv1.MongoDBCommunitySpec{}
	err := json.Unmarshal([]byte(lastSuccessfulConfigurationSaved), &prevSpec)
	if err != nil {
		return err
	}

	return validation.Validate(prevSpec, mdb.Spec)
}

func getCustomRolesModification(mdb mdbv1.MongoDBCommunity) (automationconfig.Modification, error) {
	roles := mdb.Spec.Security.Roles
	if roles == nil {
		return automationconfig.NOOP(), nil
	}

	return func(config *automationconfig.AutomationConfig) {
		config.Roles = mdbv1.ConvertCustomRolesToAutomationConfigCustomRole(roles)
	}, nil
}

func (r ReplicaSetReconciler) buildAutomationConfig(mdb mdbv1.MongoDBCommunity) (automationconfig.AutomationConfig, error) {
	tlsModification, err := getTLSConfigModification(r.client, mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, errors.Errorf("could not configure TLS modification: %s", err)
	}

	customRolesModification, err := getCustomRolesModification(mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, errors.Errorf("could not configure custom roles: %s", err)
	}

	currentAC, err := getCurrentAutomationConfig(r.client, mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, errors.Errorf("could not read existing automation config: %s", err)
	}

	auth := automationconfig.Auth{}
	if err := scram.Enable(&auth, r.client, mdb); err != nil {
		return automationconfig.AutomationConfig{}, errors.Errorf("could not configure scram authentication: %s", err)
	}

	return buildAutomationConfig(
		mdb,
		auth,
		currentAC,
		tlsModification,
		customRolesModification,
	)
}

func (r ReplicaSetReconciler) updateCurrentSpecAnnotation(mdb mdbv1.MongoDBCommunity) error {
	currentSpec, err := json.Marshal(mdb.Spec)
	if err != nil {
		return err
	}

	annotations := map[string]string{
		lastSuccessfulConfiguration: string(currentSpec),
	}
	return r.setAnnotations(mdb.NamespacedName(), annotations)
}

// getPreviousSpec returns the last spec of the resource that was successfully achieved.
func getPreviousSpec(mdb mdbv1.MongoDBCommunity) mdbv1.MongoDBCommunitySpec {

	previousSpec, ok := mdb.Annotations[lastSuccessfulConfiguration]
	if !ok {
		return mdbv1.MongoDBCommunitySpec{}
	}

	spec := mdbv1.MongoDBCommunitySpec{}
	if err := json.Unmarshal([]byte(previousSpec), &spec); err != nil {
		return mdbv1.MongoDBCommunitySpec{}
	}
	return spec
}

// getMongodConfigModification will merge the additional configuration in the CRD
// into the configuration set up by the operator.
func getMongodConfigModification(mdb mdbv1.MongoDBCommunity) automationconfig.Modification {
	return func(ac *automationconfig.AutomationConfig) {
		for i := range ac.Processes {
			// Mergo requires both objects to have the same type
			// TODO: handle this error gracefully, we may need to add an error as second argument for all modification functions
			_ = mergo.Merge(&ac.Processes[i].Args26, objx.New(mdb.Spec.AdditionalMongodConfig.Object), mergo.WithOverride)
		}
	}
}

// getUpdateStrategyType returns the type of RollingUpgradeStrategy that the StatefulSet
// should be configured with
func getUpdateStrategyType(mdb mdbv1.MongoDBCommunity) appsv1.StatefulSetUpdateStrategyType {
	if !isChangingVersion(mdb) {
		return appsv1.RollingUpdateStatefulSetStrategyType
	}
	return appsv1.OnDeleteStatefulSetStrategyType
}

// buildStatefulSet takes a MongoDB resource and converts it into
// the corresponding stateful set
func buildStatefulSet(mdb mdbv1.MongoDBCommunity) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	buildStatefulSetModificationFunction(mdb)(&sts)
	return sts, nil
}

// isChangingVersion returns true if an attempted version change is occurring.
func isChangingVersion(mdb mdbv1.MongoDBCommunity) bool {
	prevSpec := getPreviousSpec(mdb)
	return prevSpec.Version != "" && prevSpec.Version != mdb.Spec.Version
}

func mongodbAgentContainer(automationConfigSecretName string, volumeMounts []corev1.VolumeMount) container.Modification {
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
	repoUrl := os.Getenv(mongodbRepoUrl)
	if strings.HasSuffix(repoUrl, "/") {
		repoUrl = strings.TrimRight(repoUrl, "/")
	}
	mongoImageName := os.Getenv(mongodbImageEnv)
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
		container.WithName(mongodbName),
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

func buildStatefulSetModificationFunction(mdb mdbv1.MongoDBCommunity) statefulset.Modification {
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

	return statefulset.Apply(
		statefulset.WithName(mdb.Name),
		statefulset.WithNamespace(mdb.Namespace),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithLabels(labels),
		statefulset.WithMatchLabels(labels),
		statefulset.WithOwnerReference([]metav1.OwnerReference{getOwnerReference(mdb)}),
		statefulset.WithReplicas(mdb.StatefulSetReplicasThisReconciliation()),
		statefulset.WithUpdateStrategyType(getUpdateStrategyType(mdb)),
		statefulset.WithVolumeClaim(dataVolumeName, dataPvc()),
		statefulset.WithVolumeClaim(logVolumeName, logsPvc()),
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				podtemplatespec.WithPodLabels(labels),
				podtemplatespec.WithVolume(healthStatusVolume),
				podtemplatespec.WithVolume(hooksVolume),
				podtemplatespec.WithVolume(automationConfigVolume),
				podtemplatespec.WithVolume(scriptsVolume),
				podtemplatespec.WithServiceAccount(operatorServiceAccountName),
				podtemplatespec.WithContainer(agentName, mongodbAgentContainer(mdb.AutomationConfigSecretName(), []corev1.VolumeMount{agentHealthStatusVolumeMount, automationConfigVolumeMount, dataVolumeMount, scriptsVolumeMount})),
				podtemplatespec.WithContainer(mongodbName, mongodbContainer(mdb.Spec.Version, []corev1.VolumeMount{mongodHealthStatusVolumeMount, dataVolumeMount, hooksVolumeMount, logVolumeMount})),
				podtemplatespec.WithInitContainer(versionUpgradeHookName, versionUpgradeHookInit([]corev1.VolumeMount{hooksVolumeMount})),
				podtemplatespec.WithInitContainer(readinessProbeContainerName, readinessProbeInit([]corev1.VolumeMount{scriptsVolumeMount})),
				buildTLSPodSpecModification(mdb),
				buildScramPodSpecModification(),
			),
		),
		statefulset.WithCustomSpecs(mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec),
	)
}

func getOwnerReference(mdb mdbv1.MongoDBCommunity) metav1.OwnerReference {
	return *metav1.NewControllerRef(&mdb, schema.GroupVersionKind{
		Group:   mdbv1.GroupVersion.Group,
		Version: mdbv1.GroupVersion.Version,
		Kind:    mdb.Kind,
	})
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
