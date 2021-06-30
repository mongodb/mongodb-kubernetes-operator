package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

const (
	performCleanupEnv = "PERFORM_CLEANUP"
	deployDirEnv      = "DEPLOY_DIR"
	roleDirEnv        = "ROLE_DIR"
)

func Setup(t *testing.T) *e2eutil.Context {
	ctx, err := e2eutil.NewContext(t, envvar.ReadBool(performCleanupEnv))

	if err != nil {
		t.Fatal(err)
	}

	if err := deployOperator(); err != nil {
		t.Fatal(err)
	}

	return ctx
}

// CreateTLSResources will setup the CA ConfigMap and cert-key Secret necessary for TLS
// The certificates and keys are stored in testdata/tls
func CreateTLSResources(namespace string, ctx *e2eutil.Context) error { //nolint
	tlsConfig := e2eutil.NewTestTLSConfig(false)

	// Create CA ConfigMap
	testDataDir := e2eutil.TlsTestDataDir()
	ca, err := ioutil.ReadFile(path.Join(testDataDir, "ca.crt"))
	if err != nil {
		return nil
	}

	caConfigMap := configmap.Builder().
		SetName(tlsConfig.CaConfigMap.Name).
		SetNamespace(namespace).
		SetField("ca.crt", string(ca)).
		Build()

	err = e2eutil.TestClient.Create(context.TODO(), &caConfigMap, &e2eutil.CleanupOptions{TestContext: ctx})
	if err != nil {
		return err
	}

	// Create server key and certificate secret
	cert, err := ioutil.ReadFile(path.Join(testDataDir, "server.crt"))
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile(path.Join(testDataDir, "server.key"))
	if err != nil {
		return err
	}

	certKeySecret := secret.Builder().
		SetName(tlsConfig.CertificateKeySecret.Name).
		SetNamespace(namespace).
		SetField("tls.crt", string(cert)).
		SetField("tls.key", string(key)).
		Build()

	return e2eutil.TestClient.Create(context.TODO(), &certKeySecret, &e2eutil.CleanupOptions{TestContext: ctx})
}

// GeneratePasswordForUser will create a secret with a password for the given user
func GeneratePasswordForUser(ctx *e2eutil.Context, mdbu mdbv1.MongoDBUser, namespace string) (string, error) {
	passwordKey := mdbu.PasswordSecretRef.Key
	if passwordKey == "" {
		passwordKey = "password"
	}

	password, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return "", err
	}

	nsp := namespace
	if nsp == "" {
		nsp = e2eutil.OperatorNamespace
	}

	passwordSecret := secret.Builder().
		SetName(mdbu.PasswordSecretRef.Name).
		SetNamespace(nsp).
		SetField(passwordKey, password).
		Build()

	return password, e2eutil.TestClient.Create(context.TODO(), &passwordSecret, &e2eutil.CleanupOptions{TestContext: ctx})
}

func roleDir() string {
	return envvar.GetEnvOrDefault(roleDirEnv, "/workspace/config/rbac")
}

func deployDir() string {
	return envvar.GetEnvOrDefault(deployDirEnv, "/workspace/config/manager")
}

func deployOperator() error {
	testConfig := loadTestConfigFromEnv()

	e2eutil.OperatorNamespace = testConfig.namespace
	fmt.Printf("Setting operator namespace to %s\n", e2eutil.OperatorNamespace)
	watchNamespace := testConfig.namespace
	if testConfig.clusterWide {
		watchNamespace = "*"
	}
	fmt.Printf("Setting namespace to watch to %s\n", watchNamespace)

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir(), "role.yaml"), &rbacv1.Role{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role: %s", err)
	}
	fmt.Println("Successfully created the operator Role")

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir(), "service_account.yaml"), &corev1.ServiceAccount{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator service account: %s", err)
	}
	fmt.Println("Successfully created the operator Service Account")

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir(), "role_binding.yaml"), &rbacv1.RoleBinding{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role binding: %s", err)
	}
	fmt.Println("Successfully created the operator Role Binding")

	if err := buildKubernetesResourceFromYamlFile(path.Join(deployDir(), "manager.yaml"),
		&appsv1.Deployment{},
		withNamespace(testConfig.namespace),
		withOperatorImage(testConfig.operatorImage),
		withEnvVar("WATCH_NAMESPACE", watchNamespace),
		withEnvVar(construct.AgentImageEnv, testConfig.agentImage),
		withEnvVar(construct.ReadinessProbeImageEnv, testConfig.readinessProbeImage),
		withEnvVar(construct.VersionUpgradeHookImageEnv, testConfig.versionUpgradeHookImage),
	); err != nil {
		return errors.Errorf("error building operator deployment: %s", err)
	}

	// TODO: check if we could get Name from service account

	// Note: PollImmediate tries a condition func until it returns true, an error, or the timeout is reached.
	// PollImmediate always checks 'condition' before waiting for the interval. 'condition' will always be invoked at least once.
	// Some intervals may be missed if the condition takes too long or the time window is too short.
	if err := wait.PollImmediate(time.Second, time.Duration(30)*time.Second, hasDeploymentRequiredReplicas(&appsv1.Deployment{})); err != nil {
		return errors.New("error building operator deployment: the deployment does not have the required replicas (possibly because the image is not correct)")
	}

	fmt.Println("Successfully created the operator Deployment")
	return nil
}

// hasDeploymentRequiredReplicas returns a condition function that indicates whether the given deployment
// currently has the required amount of replicas
func hasDeploymentRequiredReplicas(dep *appsv1.Deployment) wait.ConditionFunc {
	// Note: ConditionFunc returns true if the condition is satisfied, or an error if the loop should be aborted.
	return func() (bool, error) {
		if err := getDeployment(dep); err != nil {
			fmt.Printf("returning error\n")
			return false, errors.Errorf("error getting operator deployment: %s\n", err)
		}
		fmt.Printf("actual replicas %d\n", dep.Status.ReadyReplicas)
		fmt.Printf("required replicas %d\n", *dep.Spec.Replicas)
		switch dep.Status.ReadyReplicas {
		case *dep.Spec.Replicas:
			fmt.Printf("returning true\n")
			return true, nil
		default:
			fmt.Printf("returning false\n")
			return false, nil
		}
	}
}

// getDeployment fills the empty deployment object with fields from the resource and returns an error if it cannot get it
func getDeployment(dep *appsv1.Deployment) error {
	return e2eutil.TestClient.Get(context.TODO(),
		types.NamespacedName{Name: "mongodb-kubernetes-operator",
			Namespace: e2eutil.OperatorNamespace},
		dep)
}

// buildKubernetesResourceFromYamlFile will create the kubernetes resource defined in yamlFilePath. All of the functional options
// provided will be applied before creation.
func buildKubernetesResourceFromYamlFile(yamlFilePath string, obj client.Object, options ...func(obj runtime.Object)) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return errors.Errorf("error reading file: %s", err)
	}

	if err := marshalRuntimeObjectFromYAMLBytes(data, obj); err != nil {
		return errors.Errorf("error converting yaml bytes to service account: %s", err)
	}

	for _, opt := range options {
		opt(obj)
	}

	return createOrUpdate(obj)
}

// marshalRuntimeObjectFromYAMLBytes accepts the bytes of a yaml resource
// and unmarshals them into the provided runtime Object
func marshalRuntimeObjectFromYAMLBytes(bytes []byte, obj runtime.Object) error {
	jsonBytes, err := yaml.YAMLToJSON(bytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, &obj)
}

func createOrUpdate(obj client.Object) error {
	if err := e2eutil.TestClient.Create(context.TODO(), obj, &e2eutil.CleanupOptions{}); err != nil {
		if apiErrors.IsAlreadyExists(err) {
			return e2eutil.TestClient.Update(context.TODO(), obj)
		}
		return errors.Errorf("error creating %s in kubernetes: %s", obj.GetObjectKind(), err)
	}
	return nil
}

// withNamespace returns a function which will assign the namespace
// of the underlying type to the value specified. We can
// add new types here as required.
func withNamespace(ns string) func(runtime.Object) {
	return func(obj runtime.Object) {
		switch v := obj.(type) {
		case *rbacv1.Role:
			v.Namespace = ns
		case *corev1.ServiceAccount:
			v.Namespace = ns
		case *rbacv1.RoleBinding:
			v.Namespace = ns
		case *corev1.Pod:
			v.Namespace = ns
		case *appsv1.Deployment:
			v.Namespace = ns
		}
	}
}

func withEnvVar(key, val string) func(obj runtime.Object) {
	return func(obj runtime.Object) {
		if testPod, ok := obj.(*corev1.Pod); ok {
			testPod.Spec.Containers[0].Env = updateEnvVarList(testPod.Spec.Containers[0].Env, key, val)
		}
		if testDeployment, ok := obj.(*appsv1.Deployment); ok {
			testDeployment.Spec.Template.Spec.Containers[0].Env = updateEnvVarList(testDeployment.Spec.Template.Spec.Containers[0].Env, key, val)
		}
	}
}

func updateEnvVarList(envVarList []corev1.EnvVar, key, val string) []corev1.EnvVar {
	for index, envVar := range envVarList {
		if envVar.Name == key {
			envVarList[index] = corev1.EnvVar{Name: key, Value: val}
			return envVarList
		}
	}
	return append(envVarList, corev1.EnvVar{Name: key, Value: val})
}

// withOperatorImage assumes that the underlying type is an appsv1.Deployment
// which has the operator container as the first container. There will be
// no effect when used with a non-deployment type
func withOperatorImage(image string) func(runtime.Object) {
	return func(obj runtime.Object) {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			dep.Spec.Template.Spec.Containers[0].Image = image
		}
	}
}
