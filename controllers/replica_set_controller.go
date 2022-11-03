package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/imdario/mergo"
	"github.com/stretchr/objx"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/predicates"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/functions"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/merge"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/validation"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/service"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	clusterDomain = "CLUSTER_DOMAIN"

	lastSuccessfulConfiguration = "mongodb.com/v1.lastSuccessfulConfiguration"
	lastAppliedMongoDBVersion   = "mongodb.com/v1.lastAppliedMongoDBVersion"
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
	configMapWatcher := watch.New()
	return &ReplicaSetReconciler{
		client:           kubernetesClient.NewClient(mgrClient),
		scheme:           mgr.GetScheme(),
		log:              zap.S(),
		secretWatcher:    &secretWatcher,
		configMapWatcher: &configMapWatcher,
	}
}

// SetupWithManager sets up the controller with the Manager and configures the necessary watches.
func (r *ReplicaSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		For(&mdbv1.MongoDBCommunity{}, builder.WithPredicates(predicates.OnlyOnSpecChange())).
		Watches(&source.Kind{Type: &corev1.Secret{}}, r.secretWatcher).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, r.configMapWatcher).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

// ReplicaSetReconciler reconciles a MongoDB ReplicaSet
type ReplicaSetReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client           kubernetesClient.Client
	scheme           *runtime.Scheme
	log              *zap.SugaredLogger
	secretWatcher    *watch.ResourceWatcher
	configMapWatcher *watch.ResourceWatcher
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
	r.log.Infof("Reconciling MongoDB")

	r.log.Debug("Validating MongoDB.Spec")
	if err := r.validateSpec(mdb); err != nil {
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
				withMessage(Error, fmt.Sprintf("Error ensuring the service (members) exists: %s", err)).
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

	if err := r.ensureTLSResources(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error ensuring TLS resources: %s", err)).
				withFailedPhase(),
		)
	}

	if err := r.ensurePrometheusTLSResources(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error ensuring TLS resources: %s", err)).
				withFailedPhase(),
		)
	}

	if err := r.ensureUserResources(mdb); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error ensuring User config: %s", err)).
				withFailedPhase(),
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
	if err := statefulset.ResetUpdateStrategy(&mdb, r.client); err != nil {
		return status.Update(r.client.Status(), &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
				withFailedPhase(),
		)
	}

	if mdb.IsStillScaling() {
		return status.Update(r.client.Status(), &mdb, statusOptions().
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
				mdb.CurrentReplicas(), mdb.DesiredReplicas())).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
			withStatefulSetArbiters(mdb.StatefulSetArbitersThisReconciliation()).
			withMongoDBArbiters(mdb.AutomationConfigArbitersThisReconciliation()).
			withPendingPhase(10),
		)
	}

	res, err := status.Update(r.client.Status(), &mdb,
		statusOptions().
			withMongoURI(mdb.MongoURI(os.Getenv(clusterDomain))).
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
			withStatefulSetArbiters(mdb.StatefulSetArbitersThisReconciliation()).
			withMongoDBArbiters(mdb.AutomationConfigArbitersThisReconciliation()).
			withMessage(None, "").
			withRunningPhase().
			withVersion(mdb.GetMongoDBVersion()),
	)
	if err != nil {
		r.log.Errorf("Error updating the status of the MongoDB resource: %s", err)
		return res, err
	}

	if err := r.updateConnectionStringSecrets(mdb, os.Getenv(clusterDomain)); err != nil {
		r.log.Errorf("Could not update connection string secrets: %s", err)
	}

	if err := r.updateLastSuccessfulConfiguration(mdb); err != nil {
		r.log.Errorf("Could not save current spec as an annotation: %s", err)
	}

	if res.RequeueAfter > 0 || res.Requeue {
		r.log.Info("Requeuing reconciliation")
		return res, nil
	}

	r.log.Infof("Successfully finished reconciliation, MongoDB.Spec: %+v, MongoDB.Status: %+v", mdb.Spec, mdb.Status)
	return res, err
}

// updateLastSuccessfulConfiguration annotates the MongoDBCommunity resource with the latest configuration
func (r *ReplicaSetReconciler) updateLastSuccessfulConfiguration(mdb mdbv1.MongoDBCommunity) error {
	currentSpec, err := json.Marshal(mdb.Spec)
	if err != nil {
		return err
	}

	specAnnotations := map[string]string{
		lastSuccessfulConfiguration: string(currentSpec),
		// the last version will be duplicated in two annotations.
		// This is needed to reuse the update strategy logic in enterprise
		lastAppliedMongoDBVersion: mdb.Spec.Version,
	}
	return annotations.SetAnnotations(&mdb, specAnnotations, r.client)
}

// ensureTLSResources creates any required TLS resources that the MongoDBCommunity
// requires for TLS configuration.
func (r *ReplicaSetReconciler) ensureTLSResources(mdb mdbv1.MongoDBCommunity) error {
	if !mdb.Spec.Security.TLS.Enabled {
		return nil
	}
	// the TLS secret needs to be created beforehand, as both the StatefulSet and AutomationConfig
	// require the contents.
	if mdb.Spec.Security.TLS.Enabled {
		r.log.Infof("TLS is enabled, creating/updating CA secret")
		if err := ensureCASecret(r.client, r.client, r.client, mdb); err != nil {
			return fmt.Errorf("could not ensure CA secret: %s", err)
		}
		r.log.Infof("TLS is enabled, creating/updating TLS secret")
		if err := ensureTLSSecret(r.client, mdb); err != nil {
			return fmt.Errorf("could not ensure TLS secret: %s", err)
		}
	}
	return nil
}

// ensurePrometheusTLSResources creates any required TLS resources that the MongoDBCommunity
// requires for TLS configuration.
func (r *ReplicaSetReconciler) ensurePrometheusTLSResources(mdb mdbv1.MongoDBCommunity) error {
	if mdb.Spec.Prometheus == nil || mdb.Spec.Prometheus.TLSSecretRef.Name == "" {
		return nil
	}

	// the TLS secret needs to be created beforehand, as both the StatefulSet and AutomationConfig
	// require the contents.
	r.log.Infof("Prometheus TLS is enabled, creating/updating TLS secret")
	if err := ensurePrometheusTLSSecret(r.client, mdb); err != nil {
		return fmt.Errorf("could not ensure TLS secret: %s", err)
	}

	return nil
}

// deployStatefulSet deploys the backing StatefulSet of the MongoDBCommunity resource.
//
// When `Spec.Arbiters` > 0, a second StatefulSet will be created, with the amount
// of Pods corresponding to the amount of expected arbiters.
//
// The returned boolean indicates that the StatefulSet is ready.
func (r *ReplicaSetReconciler) deployStatefulSet(mdb mdbv1.MongoDBCommunity) (bool, error) {
	r.log.Info("Creating/Updating StatefulSet")
	if err := r.createOrUpdateStatefulSet(mdb, false); err != nil {
		return false, fmt.Errorf("error creating/updating StatefulSet: %s", err)
	}

	r.log.Info("Creating/Updating StatefulSet for Arbiters")
	if err := r.createOrUpdateStatefulSet(mdb, true); err != nil {
		return false, fmt.Errorf("error creating/updating StatefulSet: %s", err)
	}

	currentSts, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if err != nil {
		return false, fmt.Errorf("error getting StatefulSet: %s", err)
	}

	r.log.Debugf("Ensuring StatefulSet is ready, with type: %s", mdb.GetUpdateStrategyType())

	isReady := statefulset.IsReady(currentSts, mdb.StatefulSetReplicasThisReconciliation())

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

	if isPreReadinessInitContainerStatefulSet(sts) {
		r.log.Debugf("The existing StatefulSet did not have the readiness probe init container, skipping pod annotation check.")
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
	sts, err := r.client.GetStatefulSet(mdb.NamespacedName())
	if !statefulset.IsReady(sts, mdb.StatefulSetReplicasThisReconciliation()) && mdb.Spec.Security.TLS.Enabled {
		r.log.Debug("Enabling TLS on a deployment with a StatefulSet that is not Ready, the Automation Config must be updated first")
		return true
	}
	if err == nil && mdb.Spec.Security.TLS.Enabled {
		r.log.Debug("Enabling TLS on an existing deployment, the StatefulSet must be updated first")
		return false
	}

	// if we are scaling up, we need to make sure the StatefulSet is scaled up first.
	if scale.IsScalingUp(mdb) || mdb.CurrentArbiters() < mdb.DesiredArbiters() {
		if scale.HasZeroReplicas(mdb) {
			r.log.Debug("Scaling up the ReplicaSet when there is no replicas, the Automation Config must be updated first")
			return true
		}
		r.log.Debug("Scaling up the ReplicaSet, the StatefulSet must be updated first")
		return false
	}

	if scale.IsScalingDown(mdb) {
		r.log.Debug("Scaling down the ReplicaSet, the Automation Config must be updated first")
		return true
	}

	// when we change version, we need the StatefulSet images to be updated first, then the agent can get to goal
	// state on the new version.
	if mdb.IsChangingVersion() {
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

// ensureService creates a Service unless it already exists.
//
// The Service definition is built from the `mdb` resource. If `isArbiter` is set to true, the Service
// will be created for the arbiters Statefulset.
func (r *ReplicaSetReconciler) ensureService(mdb mdbv1.MongoDBCommunity) error {
	processPortManager, err := r.createProcessPortManager(mdb)
	if err != nil {
		return err
	}

	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: mdb.ServiceName(), Namespace: mdb.Namespace}}
	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, svc, func() error {
		resourceVersion := svc.ResourceVersion // Save resourceVersion for later
		*svc = r.buildService(mdb, processPortManager)
		svc.ResourceVersion = resourceVersion
		return nil
	})
	if err != nil {
		r.log.Errorf("Could not create or patch the service: %s", err)
		return nil
	}

	r.log.Infow("Create/Update operation succeeded", "operation", op)

	return err
}

// createProcessPortManager is a helper method for creating new ReplicaSetPortManager.
// ReplicaSetPortManager needs current automation config and current pod state and the code for getting them
// was extracted here as it is used in ensureService and buildAutomationConfig.
func (r *ReplicaSetReconciler) createProcessPortManager(mdb mdbv1.MongoDBCommunity) (*agent.ReplicaSetPortManager, error) {
	currentAC, err := automationconfig.ReadFromSecret(r.client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	if err != nil {
		return nil, fmt.Errorf("could not read existing automation config: %s", err)
	}

	currentPodStates, err := agent.GetAllDesiredMembersAndArbitersPodState(mdb.NamespacedName(), r.client, mdb.StatefulSetReplicasThisReconciliation(), mdb.StatefulSetArbitersThisReconciliation(), currentAC.Version, r.log)
	if err != nil {
		return nil, fmt.Errorf("cannot get all pods goal state: %w", err)
	}

	return agent.NewReplicaSetPortManager(r.log, mdb.Spec.AdditionalMongodConfig.GetDBPort(), currentPodStates, currentAC.Processes), nil
}

func (r *ReplicaSetReconciler) createOrUpdateStatefulSet(mdb mdbv1.MongoDBCommunity, isArbiter bool) error {
	set := appsv1.StatefulSet{}

	name := mdb.NamespacedName()
	if isArbiter {
		name = mdb.ArbiterNamespacedName()
	}

	err := r.client.Get(context.TODO(), name, &set)
	err = k8sClient.IgnoreNotFound(err)
	if err != nil {
		return fmt.Errorf("error getting StatefulSet: %s", err)
	}

	buildStatefulSetModificationFunction(mdb)(&set)
	if isArbiter {
		buildArbitersModificationFunction(mdb)(&set)
	}

	if _, err = statefulset.CreateOrUpdate(r.client, set); err != nil {
		return fmt.Errorf("error creating/updating StatefulSet: %s", err)
	}
	return nil
}

// ensureAutomationConfig makes sure the AutomationConfig secret has been successfully created. The automation config
// that was updated/created is returned.
func (r ReplicaSetReconciler) ensureAutomationConfig(mdb mdbv1.MongoDBCommunity) (automationconfig.AutomationConfig, error) {
	ac, err := r.buildAutomationConfig(mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not build automation config: %s", err)
	}

	return automationconfig.EnsureSecret(
		r.client,
		types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace},
		mdb.GetOwnerReferences(),
		ac,
	)
}

func buildAutomationConfig(mdb mdbv1.MongoDBCommunity, auth automationconfig.Auth, currentAc automationconfig.AutomationConfig, modifications ...automationconfig.Modification) (automationconfig.AutomationConfig, error) {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, os.Getenv(clusterDomain))
	arbiterDomain := getDomain(mdb.ServiceName(), mdb.Namespace, os.Getenv(clusterDomain))

	zap.S().Debugw("AutomationConfigMembersThisReconciliation", "mdb.AutomationConfigMembersThisReconciliation()", mdb.AutomationConfigMembersThisReconciliation())

	arbitersCount := mdb.AutomationConfigArbitersThisReconciliation()
	if mdb.AutomationConfigMembersThisReconciliation() < mdb.Spec.Members {
		// Have not reached desired amount of members yet, should not scale arbiters
		arbitersCount = mdb.Status.CurrentMongoDBArbiters
	}

	return automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetArbiterDomain(arbiterDomain).
		SetMembers(mdb.AutomationConfigMembersThisReconciliation()).
		SetArbiters(arbitersCount).
		SetReplicaSetHorizons(mdb.Spec.ReplicaSetHorizons).
		SetPreviousAutomationConfig(currentAc).
		SetMongoDBVersion(mdb.Spec.Version).
		SetFCV(mdb.Spec.FeatureCompatibilityVersion).
		SetOptions(automationconfig.Options{DownloadBase: "/var/lib/mongodb-mms-automation"}).
		SetAuth(auth).
		SetDataDir(mdb.GetMongodConfiguration().GetDBDataDir()).
		AddModifications(getMongodConfigModification(mdb)).
		AddModifications(modifications...).
		Build()
}

// buildService creates a Service that will be used for the Replica Set StatefulSet
// that allows all the members of the STS to see each other.
func (r *ReplicaSetReconciler) buildService(mdb mdbv1.MongoDBCommunity, portManager *agent.ReplicaSetPortManager) corev1.Service {
	label := make(map[string]string)
	name := mdb.ServiceName()

	label["app"] = name

	serviceBuilder := service.Builder().
		SetName(name).
		SetNamespace(mdb.Namespace).
		SetSelector(label).
		SetLabels(label).
		SetServiceType(corev1.ServiceTypeClusterIP).
		SetClusterIP("None").
		SetPublishNotReadyAddresses(true).
		SetOwnerReferences(mdb.GetOwnerReferences())

	for _, servicePort := range portManager.GetServicePorts() {
		tmpServicePort := servicePort
		serviceBuilder.AddPort(&tmpServicePort)
	}

	serviceBuilder.AddPort(prometheusPort(mdb))

	return serviceBuilder.Build()
}

// validateSpec checks if the MongoDB resource Spec is valid.
// If there has not yet been a successful configuration, the function runs the intial Spec validations. Otherwise
// it checks that the attempted Spec is valid in relation to the Spec that resulted from that last successful configuration.
func (r ReplicaSetReconciler) validateSpec(mdb mdbv1.MongoDBCommunity) error {
	lastSuccessfulConfigurationSaved, ok := mdb.Annotations[lastSuccessfulConfiguration]
	if !ok {
		// First version of Spec
		return validation.ValidateInitalSpec(mdb, r.log)
	}

	lastSpec := mdbv1.MongoDBCommunitySpec{}
	err := json.Unmarshal([]byte(lastSuccessfulConfigurationSaved), &lastSpec)
	if err != nil {
		return err
	}

	return validation.ValidateUpdate(mdb, lastSpec, r.log)
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
	tlsModification, err := getTLSConfigModification(r.client, r.client, mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not configure TLS modification: %s", err)
	}

	customRolesModification, err := getCustomRolesModification(mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not configure custom roles: %s", err)
	}

	currentAC, err := automationconfig.ReadFromSecret(r.client, types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace})
	if err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not read existing automation config: %s", err)
	}

	auth := automationconfig.Auth{}
	if err := scram.Enable(&auth, r.client, mdb); err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not configure scram authentication: %s", err)
	}

	prometheusModification := automationconfig.NOOP()
	if mdb.Spec.Prometheus != nil {
		secretNamespacedName := types.NamespacedName{Name: mdb.Spec.Prometheus.PasswordSecretRef.Name, Namespace: mdb.Namespace}
		r.secretWatcher.Watch(secretNamespacedName, mdb.NamespacedName())

		prometheusModification, err = getPrometheusModification(r.client, mdb)
		if err != nil {
			return automationconfig.AutomationConfig{}, fmt.Errorf("could not enable TLS on Prometheus endpoint: %s", err)
		}
	}

	processPortManager, err := r.createProcessPortManager(mdb)
	if err != nil {
		return automationconfig.AutomationConfig{}, err
	}

	automationConfig, err := buildAutomationConfig(
		mdb,
		auth,
		currentAC,
		tlsModification,
		customRolesModification,
		prometheusModification,
		processPortManager.GetPortsModification(),
	)

	if err != nil {
		return automationconfig.AutomationConfig{}, fmt.Errorf("could not create an automation config: %s", err)
	}

	if mdb.Spec.AutomationConfigOverride != nil {
		automationConfig = merge.AutomationConfigs(automationConfig, overrideToAutomationConfig(*mdb.Spec.AutomationConfigOverride))
	}

	return automationConfig, nil
}

// overrideToAutomationConfig turns an automation config override from the resource spec into an automation config
// which can be used to merge.
func overrideToAutomationConfig(override mdbv1.AutomationConfigOverride) automationconfig.AutomationConfig {
	var processes []automationconfig.Process
	for _, p := range override.Processes {
		processes = append(processes, automationconfig.Process{
			Name:     p.Name,
			Disabled: p.Disabled,
		})
	}

	// TODO: currently we are just merging processes. Other fields can be added here.
	return automationconfig.AutomationConfig{
		Processes: processes,
	}
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

// buildStatefulSet takes a MongoDB resource and converts it into
// the corresponding stateful set
func buildStatefulSet(mdb mdbv1.MongoDBCommunity) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	buildStatefulSetModificationFunction(mdb)(&sts)
	return sts, nil
}

func buildStatefulSetModificationFunction(mdb mdbv1.MongoDBCommunity) statefulset.Modification {
	commonModification := construct.BuildMongoDBReplicaSetStatefulSetModificationFunction(&mdb, mdb)
	return statefulset.Apply(
		commonModification,
		statefulset.WithOwnerReference(mdb.GetOwnerReferences()),
		statefulset.WithPodSpecTemplate(
			podtemplatespec.Apply(
				buildTLSPodSpecModification(mdb),
				buildTLSPrometheus(mdb),
			),
		),

		statefulset.WithCustomSpecs(mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec),
	)
}

func buildArbitersModificationFunction(mdb mdbv1.MongoDBCommunity) statefulset.Modification {
	return statefulset.Apply(
		statefulset.WithReplicas(mdb.StatefulSetArbitersThisReconciliation()),
		statefulset.WithServiceName(mdb.ServiceName()),
		statefulset.WithName(mdb.ArbiterNamespacedName().Name),
	)
}

func getDomain(service, namespace, clusterName string) string {
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, namespace, clusterName)
}

// isPreReadinessInitContainerStatefulSet determines if the existing StatefulSet has been configured with the readiness probe init container.
// if this is not the case, then we should ensure to skip past the annotation check otherwise the pods will remain in pending state forever.
func isPreReadinessInitContainerStatefulSet(sts appsv1.StatefulSet) bool {
	return container.GetByName(construct.ReadinessProbeContainerName, sts.Spec.Template.Spec.InitContainers) == nil
}
