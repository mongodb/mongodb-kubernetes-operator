package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	AutomationConfigKey       = "automation-config"
	agentName                 = "mongodb-agent"
	mongodbName               = "mongod"
	agentImageEnvVariable     = "AGENT_IMAGE"
	versionManifestFilePath   = "/usr/local/version_manifest.json"
	readinessProbePath        = "/var/lib/mongodb-mms-automation/probes/readinessprobe"
	agentHealthStatusFilePath = "/var/log/mongodb-mms-automation/agent-health-status.json"
	clusterFilePath           = "/var/lib/automation/config/automation-config"
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
	client           mdbClient.Client
	scheme           *runtime.Scheme
	manifestProvider func() (automationconfig.VersionManifest, error)
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
		log.Warnf("error creating automation config config map: %s", err)
		return reconcile.Result{}, err
	}

	svc := buildService(mdb)
	if err = r.client.CreateOrUpdate(&svc); err != nil {
		log.Warnf("The service already exists... moving forward: %s", err)
	}

	sts, err := buildStatefulSet(mdb)
	if err != nil {
		log.Infof("Error building StatefulSet: %s", err)
		return reconcile.Result{}, nil
	}

	if err = r.client.CreateOrUpdate(&sts); err != nil {
		log.Infof("Error creating/updating StatefulSet: %s", err)
		return reconcile.Result{}, err
	} else {
		log.Infof("StatefulSet successfully Created/Updated")
	}

	log.Debugf("waiting for StatefulSet %s/%s to reach ready state", mdb.Namespace, mdb.Name)
	set := appsv1.StatefulSet{}
	timedOut, err := r.client.WaitForCondition(types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, time.Second*3, time.Second*30, &set, func() bool {
		return statefulset.IsReady(set)
	})

	if timedOut {
		log.Infof("Stateful Set has not yet reached the ready state, requeuing reconciliation")
		return reconcile.Result{Requeue: true}, nil
	}

	if err != nil {
		log.Errorf("error polling for statefulset: %+v", err)
		return reconcile.Result{}, err
	}

	log.Infof("Stateful Set reached ready state!")

	if err := r.updateStatusSuccess(&mdb); err != nil {
		log.Infof("Error updating the status of the MongoDB resource: %+v", err)
		return reconcile.Result{}, nil
	}

	log.Info("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status", mdb.Status)
	return reconcile.Result{}, nil
}

func (r ReplicaSetReconciler) updateStatusSuccess(mdb *mdbv1.MongoDB) error {
	mdb.UpdateSuccess()
	if err := r.client.Status().Update(context.TODO(), mdb); err != nil {
		return fmt.Errorf("error updating status: %+v", err)
	}
	return nil
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

func buildAutomationConfig(mdb mdbv1.MongoDB, mdbVersionConfig automationconfig.MongoDbVersionConfig) automationconfig.AutomationConfig {
	domain := getDomain(mdb.ServiceName(), mdb.Namespace, "")
	return automationconfig.NewBuilder().
		SetTopology(automationconfig.ReplicaSetTopology).
		SetName(mdb.Name).
		SetDomain(domain).
		SetMembers(mdb.Spec.Members).
		SetMongoDBVersion(mdb.Spec.Version).
		SetAutomationConfigVersion(1). // TODO: Correctly set the version
		SetFCV(mdb.GetFCV()).
		AddVersion(mdbVersionConfig).
		Build()
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

func (r ReplicaSetReconciler) buildAutomationConfigConfigMap(mdb mdbv1.MongoDB) (corev1.ConfigMap, error) {
	manifest, err := r.manifestProvider()
	if err != nil {
		return corev1.ConfigMap{}, fmt.Errorf("error reading version manifest from disk: %+v", err)
	}
	ac := buildAutomationConfig(mdb, manifest.BuildsForVersion(mdb.Spec.Version))
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

// buildContainers constructs the mongodb-agent container as well as the
// mongod container.
func buildContainers(mdb mdbv1.MongoDB) []corev1.Container {
	agentCommand := []string{
		"agent/mongodb-agent",
		"-cluster=" + clusterFilePath,
		"-skipMongoStart",
		"-noDaemonize",
		"-healthCheckFilePath=" + agentHealthStatusFilePath,
		"-serveStatusPort=5000",
	}

	readinessProbe := defaultReadinessProbe()
	agentContainer := corev1.Container{
		Name:            agentName,
		Image:           os.Getenv(agentImageEnvVariable),
		ImagePullPolicy: corev1.PullAlways,
		Resources:       resourcerequirements.Defaults(),
		Command:         agentCommand,
		ReadinessProbe:  &readinessProbe,
	}

	mongoDbCommand := []string{
		"/bin/sh",
		"-c",
		`while [ ! -f /data/automation-mongod.conf ]; do sleep 3 ; done ; sleep 2;  mongod -f /data/automation-mongod.conf`,
	}
	mongodbContainer := corev1.Container{
		Name:      mongodbName,
		Image:     fmt.Sprintf("mongo:%s", mdb.Spec.Version),
		Command:   mongoDbCommand,
		Resources: resourcerequirements.Defaults(),
	}
	return []corev1.Container{agentContainer, mongodbContainer}
}

func defaultReadinessProbe() corev1.Probe {
	return corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{readinessProbePath}},
		},
		// Setting the failure threshold to quite big value as the agent may spend some time to reach the goal
		FailureThreshold: 240,
		// The agent may be not on time to write the status file right after the container is created - we need to wait
		// for some time
		InitialDelaySeconds: 5,
	}
}

// buildStatefulSet takes a MongoDB resource and converts it into
// the corresponding stateful set
func buildStatefulSet(mdb mdbv1.MongoDB) (appsv1.StatefulSet, error) {
	labels := map[string]string{
		"app": mdb.ServiceName(),
	}

	podSpecTemplate := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			Containers: buildContainers(mdb),
		},
	}

	builder := statefulset.NewBuilder().
		SetPodTemplateSpec(podSpecTemplate).
		SetNamespace(mdb.Namespace).
		SetName(mdb.Name).
		SetReplicas(mdb.Spec.Members).
		SetLabels(labels).
		SetMatchLabels(labels).
		SetServiceName(mdb.ServiceName())

	// TODO: Add this section to architecture document.
	// The design of the multi-container and the different volumes mounted to them is as follows:
	// There will be two volumes mounted:
	// 1. "data-volume": Access to /data for both agent and mongod. Shared data is required because
	//    agent writes automation-mongod.conf file in it and reads certain lock files from there.
	// 2. "automation-config": This is /var/lib/automation/config that holds the automation config
	//    mounted from a ConfigMap. This is only required in the Agent container.
	dataVolume, dataVolumeClaim := buildDataVolumeClaim()
	builder.
		AddVolumeMount(mongodbName, dataVolume).
		AddVolumeMount(agentName, dataVolume).
		AddVolumeClaimTemplates(dataVolumeClaim)
	// the automation config is only mounted, as read only, on the agent container
	automationConfigVolume := statefulset.CreateVolumeFromConfigMap("automation-config", "example-mongodb-config")
	automationConfigVolumeMount := statefulset.CreateVolumeMount("automation-config", "/var/lib/automation/config", statefulset.WithReadOnly(true))
	builder.
		AddVolume(automationConfigVolume).
		AddVolumeMount(agentName, automationConfigVolumeMount)

	return builder.Build()
}

func buildDataVolumeClaim() (corev1.VolumeMount, []corev1.PersistentVolumeClaim) {
	dataVolume := statefulset.CreateVolumeMount("data-volume", "/data")
	dataVolumeClaim := []corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-volume",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: resourcerequirements.BuildDefaultStorageRequirements(),
			},
		},
	}}

	return dataVolume, dataVolumeClaim
}

func getDomain(service, namespace, clusterName string) string {
	if clusterName == "" {
		clusterName = "cluster.local"
	}
	return fmt.Sprintf("%s.%s.svc.%s", service, namespace, clusterName)
}
