package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	mdbClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/resourcerequirements"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	automationConfigKey   = "automation-config"
	agentName             = "mongodb-agent"
	mongodbName           = "mongod"
	agentImageEnvVariable = "AGENT_IMAGE"
)

// Add creates a new MongoDB Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	mgrClient := mgr.GetClient()
	return &ReplicaSetReconciler{client: mdbClient.NewClient(mgrClient), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("replicaset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MongoDB
	err = c.Watch(&source.Kind{Type: &mdbv1.MongoDB{}}, &handler.EnqueueRequestForObject{})
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
	client mdbClient.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MongoDB object and makes changes based on the state read
// and what is in the MongoDB.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReplicaSetReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := zap.S().With("ReplicaSet", request.NamespacedName)
	log.Info("Reconciling MongoDB")

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
		log.Errorf("error reconciling MongoDB resource: %s", err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: Read current automation config version from config map

	if err := r.ensureAutomationConfig(mdb); err != nil {
		log.Warnf("failed creating config map: %s", err)
		return reconcile.Result{}, err
	}

	// TODO: Create the service for the MDB resource

	sts, err := buildStatefulSet(mdb)
	if err != nil {
		log.Warnf("error building StatefulSet: %s", err)
		return reconcile.Result{}, nil
	}

	if err = r.client.CreateOrUpdate(&sts); err != nil {
		log.Warnf("error creating/updating StatefulSet: %s", err)
		return reconcile.Result{}, err
	}

	log.Info("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status", mdb.Status)
	return reconcile.Result{}, nil
}

func (r ReplicaSetReconciler) ensureAutomationConfig(mdb mdbv1.MongoDB) error {
	cm, err := buildAutomationConfigConfigMap(mdb)
	if err != nil {
		return err
	}
	if err := r.client.CreateOrUpdate(&cm); err != nil {
		return err
	}
	return nil
}

func buildAutomationConfig(mdb mdbv1.MongoDB) automationconfig.AutomationConfig {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, mdb.Name)
	return automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetMembers(mdb.Spec.Members).
		SetMongoDBVersion(mdb.Spec.Version).
		SetAutomationConfigVersion(1). // TODO: Correctly set the version
		Build()
}

func buildAutomationConfigConfigMap(mdb mdbv1.MongoDB) (corev1.ConfigMap, error) {
	ac := buildAutomationConfig(mdb)
	acBytes, err := json.Marshal(ac)
	if err != nil {
		return corev1.ConfigMap{}, err
	}

	return configmap.Builder().
		SetName(mdb.ConfigMapName()).
		SetNamespace(mdb.Namespace).
		SetField(automationConfigKey, string(acBytes)).
		Build(), nil
}

func buildContainers(mdb mdbv1.MongoDB) ([]corev1.Container, error) {
	agentCommand := []string{
		"agent/mongodb-agent",
		"-cluster=/var/lib/automation/config/automation-config",
		"-skipMongoStart",
	}
	agentContainer := corev1.Container{
		Name:      agentName,
		Image:     os.Getenv(agentImageEnvVariable),
		Resources: resourcerequirements.Defaults(),
		Command:   agentCommand,
	}

	mongoDbCommand := []string{
		"/bin/sh",
		"-c",
		`while [ ! -f /data/mongodb-automation.conf ]; do echo "[$(date)] waiting" ; sleep 10; done; mongod -f /data/mongodb-automation.conf`,
	}
	mongodbContainer := corev1.Container{
		Name:      mongodbName,
		Image:     fmt.Sprintf("mongo:%s", mdb.Spec.Version),
		Command:   mongoDbCommand,
		Resources: resourcerequirements.Defaults(),
	}
	return []corev1.Container{agentContainer, mongodbContainer}, nil
}

// buildStatefulSet takes a MongoDB resource and converts it into
// the corresponding stateful set
func buildStatefulSet(mdb mdbv1.MongoDB) (appsv1.StatefulSet, error) {
	labels := map[string]string{
		"dummy": "label",
	}

	containers, err := buildContainers(mdb)
	if err != nil {
		return appsv1.StatefulSet{}, fmt.Errorf("error creating containers for %s/%s: %s", mdb.Namespace, mdb.Name, err)
	}

	podSpecTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: containers,
		},
	}

	v := statefulset.CreateVolumeFromConfigMap("automation-config", "example-mongodb-config")
	vm := statefulset.CreateVolumeMount("automation-config", "/var/lib/automation/config", "")

	return statefulset.NewBuilder().
		SetPodTemplateSpec(podSpecTemplate).
		SetNamespace(mdb.Namespace).
		SetName(mdb.Name).
		SetReplicas(mdb.Spec.Members).
		SetLabels(labels).
		SetMatchLabels(labels).
		AddVolumeMount(agentName, vm).
		AddVolumeMount(mongodbName, vm).
		AddVolume(v).
		Build()
}

func getDomain(service, namespace, clusterName string) string {
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, namespace, clusterName)
}
