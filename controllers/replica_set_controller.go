package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	agentHealthStatusFilePathEnv = "AGENT_STATUS_FILEPATH"
	mongodbImageEnv              = "MONGODB_IMAGE"
	mongodbRepoUrl               = "MONGODB_REPO_URL"
	headlessAgentEnv             = "HEADLESS_AGENT"
	podNamespaceEnv              = "POD_NAMESPACE"
	automationConfigEnv          = "AUTOMATION_CONFIG_MAP"

	AutomationConfigKey            = "cluster-config.json"
	agentName                      = "mongodb-agent"
	mongodbName                    = "mongod"
	versionUpgradeHookName         = "mongod-posthook"
	dataVolumeName                 = "data-volume"
	versionManifestFilePath        = "/usr/local/version_manifest.json"
	readinessProbePath             = "/var/lib/mongodb-mms-automation/probes/readinessprobe"
	clusterFilePath                = "/var/lib/automation/config/cluster-config.json"
	operatorServiceAccountName     = "mongodb-kubernetes-operator"
	agentHealthStatusFilePathValue = "/var/log/mongodb-mms-automation/healthstatus/agent-health-status.json"

	// lastVersionAnnotationKey should indicate which version of MongoDB was last
	// configured
	lastVersionAnnotationKey    = "mongodb.com/v1.lastVersion"
	lastSuccessfulConfiguration = "mongodb.com/v1.lastSuccessfulConfiguration"
	// tlsRolledOutAnnotationKey indicates if TLS has been fully rolled out
	tlsRolledOutAnnotationKey      = "mongodb.com/v1.tlsRolledOut"
	hasLeftReadyStateAnnotationKey = "mongodb.com/v1.hasLeftReadyStateAnnotationKey"

	trueAnnotation = "true"
)

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
}

// ManifestProvider is a function which returns the VersionManifest which
// contains the list of all available MongoDB versions
type ManifestProvider func() (automationconfig.VersionManifest, error)

func NewReconciler(mgr manager.Manager, manifestProvider ManifestProvider) *ReplicaSetReconciler {
	mgrClient := mgr.GetClient()
	secretWatcher := watch.New()

	mp := manifestProvider
	if mp == nil {
		mp = readVersionManifestFromDisk
	}

	return &ReplicaSetReconciler{
		client:           kubernetesClient.NewClient(mgrClient),
		scheme:           mgr.GetScheme(),
		manifestProvider: mp,
		log:              zap.S(),
		secretWatcher:    &secretWatcher,
	}
}

// SetupWithManager sets up the controller with the Manager and configures the necessary watches.
func (r *ReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mdbv1.MongoDBCommunity{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

// ReplicaSetReconciler reconciles a MongoDB ReplicaSet
type ReplicaSetReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client           kubernetesClient.Client
	scheme           *runtime.Scheme
	manifestProvider func() (automationconfig.VersionManifest, error)
	log              *zap.SugaredLogger
	secretWatcher    *watch.ResourceWatcher
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
func (r ReplicaSetReconciler) Reconcile(_ context.Context, request reconcile.Request) (reconcile.Result, error) {

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

	r.log.Debug("Ensuring Automation Config for deployment")
	if err := r.ensureAutomationConfig(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("error creating automation config secret: %s", err)).
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

	// if we're scaling down, we need to wait until the StatefulSet is at the
	// desired number of replicas. Scaling down has to happen one member at a time
	if scale.IsScalingDown(mdb) {
		res, err := checkIfStatefulSetMembersHaveBeenRemovedFromTheAutomationConfig(r.client, r.client.Status(), mdb)

		if err != nil {
			r.log.Errorf("Error checking if StatefulSet members have been removed from the automation config: %s", err)
			return result.Failed()
		}

		if result.ShouldRequeue(res, err) {
			r.log.Debugf("The expected number of Stateful Set members for scale down are not yet ready, requeuing reconciliation")
			return result.Retry(10)
		}
	}

	// at this stage we know we have successfully updated the automation config with the correct number of
	// members and the stateful set has the expected number of ready replicas. We can update our status
	// so we calculate these fields correctly going forward
	if err := updateScalingStatus(r.client.Status(), mdb); err != nil {
		r.log.Errorf("Failed updating the status of the MongoDB resource: %s", err)
		return result.Failed()
	}

	r.log.Debug("Validating TLS Config")
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

	r.log.Debug("Creating/Updating StatefulSet")
	if err := r.createOrUpdateStatefulSet(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error creating/updating StatefulSet: %s", err)).
				withFailedPhase(),
		)
	}

	currentSts, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Info, fmt.Sprintf("Error getting StatefulSet: %s", err)).
				withPendingPhase(0),
		)
	}

	r.log.Debugf("Ensuring StatefulSet is ready, with type: %s", getUpdateStrategyType(mdb))
	ready, err := r.isStatefulSetReady(mdb, &currentSts)
	if err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error checking StatefulSet status: %s", err)).
				withFailedPhase(),
		)
	}

	if !ready {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				// need to update the current replicas as they get ready so eventually the desired number becomes
				// ready one at a time
				withStatefulSetReplicas(int(currentSts.Status.ReadyReplicas)).
				withMessage(Info, fmt.Sprintf("StatefulSet %s/%s is not yet ready, retrying in 10 seconds", mdb.Namespace, mdb.Name)).
				withPendingPhase(10),
		)
	}

	r.log.Debug("Resetting StatefulSet UpdateStrategy")
	if err := r.resetStatefulSetUpdateStrategy(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
				withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
				withFailedPhase(),
		)
	}

	r.log.Debug("Setting MongoDB Annotations")
	annotations := map[string]string{
		lastVersionAnnotationKey:       mdb.Spec.Version,
		hasLeftReadyStateAnnotationKey: "false",
	}
	if err := r.setAnnotations(mdb.NamespacedName(), annotations); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
				withMessage(Error, fmt.Sprintf("Error setting annotations: %s", err)).
				withFailedPhase(),
		)
	}

	if err := r.completeTLSRollout(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
				withMessage(Error, fmt.Sprintf("Error completing TLS rollout: %s", err)).
				withFailedPhase(),
		)
	}

	if scale.IsStillScaling(mdb) {
		return status.Update(r.client.Status(), &mdb, statusOptions().
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
				mdb.CurrentReplicas(), mdb.DesiredReplicas())).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
			withPendingPhase(0),
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

// isStatefulSetReady checks to see if the stateful set corresponding to the given MongoDB resource
// is currently ready.
func (r *ReplicaSetReconciler) isStatefulSetReady(mdb mdbv1.MongoDBCommunity, existingStatefulSet *appsv1.StatefulSet) (bool, error) {
	stsFunc := buildStatefulSetModificationFunction(mdb)
	stsCopy := existingStatefulSet.DeepCopyObject()
	stsFunc(existingStatefulSet)

	stsCopyBytes, err := json.Marshal(stsCopy)
	if err != nil {
		return false, errors.Errorf("unable to marshal StatefulSet copy: %s", err)
	}

	stsBytes, err := json.Marshal(existingStatefulSet)
	if err != nil {
		return false, errors.Errorf("unable to marshal existing StatefulSet: %s", err)
	}

	//comparison is done with bytes instead of reflect.DeepEqual as there are
	//some issues with nil/empty maps not being compared correctly otherwise
	areEqual := bytes.Equal(stsCopyBytes, stsBytes)

	isReady := statefulset.IsReady(*existingStatefulSet, mdb.StatefulSetReplicasThisReconciliation())

	if existingStatefulSet.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType && !isReady {
		r.log.Info("StatefulSet has left ready state, version upgrade in progress")
		annotations := map[string]string{
			hasLeftReadyStateAnnotationKey: trueAnnotation,
		}
		if err := r.setAnnotations(mdb.NamespacedName(), annotations); err != nil {
			return false, errors.Errorf("could not set %s annotation to true: %s", hasLeftReadyStateAnnotationKey, err)
		}
	}

	hasPerformedUpgrade := mdb.Annotations[hasLeftReadyStateAnnotationKey] == trueAnnotation
	r.log.Infow("StatefulSet Readiness", "isReady", isReady, "hasPerformedUpgrade", hasPerformedUpgrade, "areEqual", areEqual)

	if existingStatefulSet.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return areEqual && isReady && hasPerformedUpgrade, nil
	}

	return areEqual && isReady, nil
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

func (r ReplicaSetReconciler) ensureAutomationConfig(mdb mdbv1.MongoDBCommunity) error {
	s, err := r.buildAutomationConfigSecret(mdb)
	if err != nil {
		return err
	}
	return secret.CreateOrUpdate(r.client, s)
}

func buildAutomationConfig(mdb mdbv1.MongoDBCommunity, mdbVersionConfig automationconfig.MongoDbVersionConfig, currentAc automationconfig.AutomationConfig, modifications ...automationconfig.Modification) (automationconfig.AutomationConfig, error) {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, os.Getenv(clusterDNSName))
	zap.S().Debugw("AutomationConfigMembersThisReconciliation", "mdb.AutomationConfigMembersThisReconciliation()", mdb.AutomationConfigMembersThisReconciliation())

	builder := automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetMembers(mdb.AutomationConfigMembersThisReconciliation()).
		SetReplicaSetHorizons(mdb.Spec.ReplicaSetHorizons).
		SetPreviousAutomationConfig(currentAc).
		SetMongoDBVersion(mdb.Spec.Version).
		SetFCV(mdb.GetFCV()).
		AddVersion(mdbVersionConfig).
		AddModifications(getMongodConfigModification(mdb)).
		AddModifications(modifications...)
	newAc, err := builder.Build()
	if err != nil {
		return automationconfig.AutomationConfig{}, err
	}

	return newAc, nil
}

func readVersionManifestFromDisk() (automationconfig.VersionManifest, error) {
	versionManifestBytes, err := ioutil.ReadFile(versionManifestFilePath)
	if err != nil {
		return automationconfig.VersionManifest{}, err
	}
	return versionManifestFromBytes(versionManifestBytes)
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
	if err := json.Unmarshal(currentSecret.Data[AutomationConfigKey], &currentAc); err != nil {
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

func (r ReplicaSetReconciler) buildAutomationConfigSecret(mdb mdbv1.MongoDBCommunity) (corev1.Secret, error) {

	manifest, err := r.manifestProvider()
	if err != nil {
		return corev1.Secret{}, errors.Errorf("could not read version manifest from disk: %s", err)
	}

	authModification, err := scram.EnsureScram(r.client, mdb.ScramCredentialsNamespacedName(), mdb)
	if err != nil {
		return corev1.Secret{}, errors.Errorf("could not ensure scram credentials: %s", err)
	}

	tlsModification, err := getTLSConfigModification(r.client, mdb)
	if err != nil {
		return corev1.Secret{}, errors.Errorf("could not configure TLS modification: %s", err)
	}

	customRolesModification, err := getCustomRolesModification(mdb)
	if err != nil {
		return corev1.Secret{}, errors.Errorf("could not configure custom roles: %s", err)
	}

	currentAC, err := getCurrentAutomationConfig(r.client, mdb)
	if err != nil {
		return corev1.Secret{}, errors.Errorf("could not read existing automation config: %s", err)
	}

	ac, err := buildAutomationConfig(
		mdb,
		manifest.BuildsForVersion(mdb.Spec.Version),
		currentAC,
		authModification,
		tlsModification,
		customRolesModification,
	)
	if err != nil {
		return corev1.Secret{}, fmt.Errorf("could not build automation config: %s", err)
	}
	acBytes, err := json.Marshal(ac)
	if err != nil {
		return corev1.Secret{}, fmt.Errorf("could not marshal automation config: %s", err)
	}

	return secret.Builder().
		SetName(mdb.AutomationConfigSecretName()).
		SetNamespace(mdb.Namespace).
		SetField(AutomationConfigKey, string(acBytes)).
		Build(), nil
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

func isChangingVersion(mdb mdbv1.MongoDBCommunity) bool {
	if lastVersion, ok := mdb.Annotations[lastVersionAnnotationKey]; ok {
		return (mdb.Spec.Version != lastVersion) && lastVersion != ""
	}
	return false
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

	automationConfigVolume := statefulset.CreateVolumeFromSecret("automation-config", mdb.AutomationConfigSecretName())
	automationConfigVolumeMount := statefulset.CreateVolumeMount(automationConfigVolume.Name, "/var/lib/automation/config", statefulset.WithReadOnly(true))

	dataVolume := statefulset.CreateVolumeMount(dataVolumeName, "/data")

	return statefulset.Apply(
		statefulset.WithName(mdb.Name),
		statefulset.WithNamespace(mdb.Namespace),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithLabels(labels),
		statefulset.WithMatchLabels(labels),
		statefulset.WithOwnerReference([]metav1.OwnerReference{getOwnerReference(mdb)}),
		statefulset.WithReplicas(mdb.StatefulSetReplicasThisReconciliation()),
		statefulset.WithUpdateStrategyType(getUpdateStrategyType(mdb)),
		statefulset.WithVolumeClaim(dataVolumeName, defaultPvc()),
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				podtemplatespec.WithPodLabels(labels),
				podtemplatespec.WithVolume(healthStatusVolume),
				podtemplatespec.WithVolume(hooksVolume),
				podtemplatespec.WithVolume(automationConfigVolume),
				podtemplatespec.WithServiceAccount(operatorServiceAccountName),
				podtemplatespec.WithContainer(agentName, mongodbAgentContainer(mdb.AutomationConfigSecretName(), []corev1.VolumeMount{agentHealthStatusVolumeMount, automationConfigVolumeMount, dataVolume})),
				podtemplatespec.WithContainer(mongodbName, mongodbContainer(mdb.Spec.Version, []corev1.VolumeMount{mongodHealthStatusVolumeMount, dataVolume, hooksVolumeMount})),
				podtemplatespec.WithInitContainer(versionUpgradeHookName, versionUpgradeHookInit([]corev1.VolumeMount{hooksVolumeMount})),
				buildTLSPodSpecModification(mdb),
				buildScramPodSpecModification(mdb),
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

func defaultPvc() persistentvolumeclaim.Modification {
	return persistentvolumeclaim.Apply(
		persistentvolumeclaim.WithName(dataVolumeName),
		persistentvolumeclaim.WithAccessModes(corev1.ReadWriteOnce),
		persistentvolumeclaim.WithResourceRequests(resourcerequirements.BuildDefaultStorageRequirements()),
	)
}
