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
	"k8s.io/apimachinery/pkg/api/resource"
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

type tlsSecretType string

const (
	performCleanupEnv = "PERFORM_CLEANUP"
	deployDirEnv      = "DEPLOY_DIR"
	roleDirEnv        = "ROLE_DIR"

	CertKeyPair tlsSecretType = "CERTKEYPAIR"
	Pem         tlsSecretType = "PEM"
)

func Setup(t *testing.T) *e2eutil.Context {
	ctx, err := e2eutil.NewContext(t, envvar.ReadBool(performCleanupEnv))

	if err != nil {
		t.Fatal(err)
	}

	if err := deployOperator(ctx); err != nil {
		t.Fatal(err)
	}

	return ctx
}

// CreateTLSResources will setup the CA ConfigMap and cert-key Secret necessary for TLS
// The certificates and keys are stored in testdata/tls
func CreateTLSResources(namespace string, ctx *e2eutil.Context, secretType tlsSecretType) error {
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
		SetDataField("ca.crt", string(ca)).
		SetLabels(e2eutil.TestLabels()).
		Build()

	err = e2eutil.TestClient.Create(context.TODO(), &caConfigMap, &e2eutil.CleanupOptions{TestContext: ctx})
	if err != nil {
		return err
	}

	certKeySecretBuilder := secret.Builder().
		SetName(tlsConfig.CertificateKeySecret.Name).
		SetLabels(e2eutil.TestLabels()).
		SetNamespace(namespace)

	if secretType == CertKeyPair {
		// Create server key and certificate secret
		cert, err := ioutil.ReadFile(path.Join(testDataDir, "server.crt"))
		if err != nil {
			return err
		}
		key, err := ioutil.ReadFile(path.Join(testDataDir, "server.key"))
		if err != nil {
			return err
		}
		certKeySecretBuilder.SetField("tls.crt", string(cert)).SetField("tls.key", string(key))
	}
	if secretType == Pem {
		pem, err := ioutil.ReadFile(path.Join(testDataDir, "server.pem"))
		if err != nil {
			return err
		}
		certKeySecretBuilder.SetField("tls.pem", string(pem))
	}

	certKeySecret := certKeySecretBuilder.Build()

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
		SetLabels(e2eutil.TestLabels()).
		Build()

	return password, e2eutil.TestClient.Create(context.TODO(), &passwordSecret, &e2eutil.CleanupOptions{TestContext: ctx})
}

func roleDir() string {
	return envvar.GetEnvOrDefault(roleDirEnv, "/workspace/config/rbac")
}

func deployDir() string {
	return envvar.GetEnvOrDefault(deployDirEnv, "/workspace/config/manager")
}

func deployOperator(ctx *e2eutil.Context) error {

	testConfig := loadTestConfigFromEnv()

	e2eutil.OperatorNamespace = testConfig.namespace
	fmt.Printf("Setting operator namespace to %s\n", e2eutil.OperatorNamespace)
	watchNamespace := testConfig.namespace
	if testConfig.clusterWide {
		watchNamespace = "*"
	}
	fmt.Printf("Setting namespace to watch to %s\n", watchNamespace)

	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "role.yaml"), &rbacv1.Role{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role: %s", err)
	}
	fmt.Println("Successfully created the operator Role")

	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "service_account.yaml"), &corev1.ServiceAccount{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator service account: %s", err)
	}
	fmt.Println("Successfully created the operator Service Account")

	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "role_binding.yaml"), &rbacv1.RoleBinding{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role binding: %s", err)
	}
	fmt.Println("Successfully created the operator Role Binding")

	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "role_database.yaml"), &rbacv1.Role{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building mongodb database role: %s", err)
	}
	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "service_account_database.yaml"), &corev1.ServiceAccount{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building mongodb database service account: %s", err)
	}
	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(roleDir(), "role_binding_database.yaml"), &rbacv1.RoleBinding{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building mongodb database role binding: %s", err)
	}
	fmt.Println("Successfully created the role, service account and role binding for the database")

	dep := &appsv1.Deployment{}

	if err := buildKubernetesResourceFromYamlFile(ctx, path.Join(deployDir(), "manager.yaml"),
		dep,
		withNamespace(testConfig.namespace),
		withOperatorImage(testConfig.operatorImage),
		withEnvVar("WATCH_NAMESPACE", watchNamespace),
		withEnvVar(construct.AgentImageEnv, testConfig.agentImage),
		withEnvVar(construct.ReadinessProbeImageEnv, testConfig.readinessProbeImage),
		withEnvVar(construct.VersionUpgradeHookImageEnv, testConfig.versionUpgradeHookImage),
		withCPURequest("50m"),
	); err != nil {
		return errors.Errorf("error building operator deployment: %s", err)
	}

	if err := wait.PollImmediate(time.Second, 30*time.Second, hasDeploymentRequiredReplicas(dep)); err != nil {
		return errors.New("error building operator deployment: the deployment does not have the required replicas")
	}

	fmt.Println("Successfully created the operator Deployment")
	return nil
}

// hasDeploymentRequiredReplicas returns a condition function that indicates whether the given deployment
// currently has the required amount of replicas in the ready state as specified in spec.replicas
func hasDeploymentRequiredReplicas(dep *appsv1.Deployment) wait.ConditionFunc {
	return func() (bool, error) {
		err := e2eutil.TestClient.Get(context.TODO(),
			types.NamespacedName{Name: dep.Name,
				Namespace: e2eutil.OperatorNamespace},
			dep)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				return false, nil
			}
			return false, errors.Errorf("error getting operator deployment: %s", err)
		}
		if dep.Status.ReadyReplicas == *dep.Spec.Replicas {
			return true, nil
		}
		return false, nil
	}
}

// buildKubernetesResourceFromYamlFile will create the kubernetes resource defined in yamlFilePath. All of the functional options
// provided will be applied before creation.
func buildKubernetesResourceFromYamlFile(ctx *e2eutil.Context, yamlFilePath string, obj client.Object, options ...func(obj runtime.Object) error) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return errors.Errorf("error reading file: %s", err)
	}

	if err := marshalRuntimeObjectFromYAMLBytes(data, obj); err != nil {
		return errors.Errorf("error converting yaml bytes to service account: %s", err)
	}

	for _, opt := range options {
		if err := opt(obj); err != nil {
			return err
		}
	}

	obj.SetLabels(e2eutil.TestLabels())
	return createOrUpdate(ctx, obj)
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

func createOrUpdate(ctx *e2eutil.Context, obj client.Object) error {
	if err := e2eutil.TestClient.Create(context.TODO(), obj, &e2eutil.CleanupOptions{TestContext: ctx}); err != nil {
		if apiErrors.IsAlreadyExists(err) {
			return e2eutil.TestClient.Update(context.TODO(), obj)
		}
		return errors.Errorf("error creating %s in kubernetes: %s", obj.GetObjectKind(), err)
	}
	return nil
}

// withCPURequest assumes that the underlying type is an appsv1.Deployment.
// it returns a function which will change the amount
// requested for the CPUresource. There will be
// no effect when used with a non-deployment type
func withCPURequest(cpuRequest string) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		dep, ok := obj.(*appsv1.Deployment)
		if !ok {
			return errors.Errorf("withCPURequest() called on a non-deployment object")
		}
		quantityCPU, okCPU := resource.ParseQuantity(cpuRequest)
		if okCPU != nil {
			return okCPU
		}
		for _, cont := range dep.Spec.Template.Spec.Containers {
			cont.Resources.Requests["cpu"] = quantityCPU
		}

		return nil
	}
}

// withNamespace returns a function which will assign the namespace
// of the underlying type to the value specified. We can
// add new types here as required.
func withNamespace(ns string) func(runtime.Object) error {
	return func(obj runtime.Object) error {
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
		default:
			return errors.Errorf("withNamespace() called on a non supported object")
		}

		return nil
	}
}

func withEnvVar(key, val string) func(obj runtime.Object) error {
	return func(obj runtime.Object) error {
		if testPod, ok := obj.(*corev1.Pod); ok {
			testPod.Spec.Containers[0].Env = updateEnvVarList(testPod.Spec.Containers[0].Env, key, val)
		}
		if testDeployment, ok := obj.(*appsv1.Deployment); ok {
			testDeployment.Spec.Template.Spec.Containers[0].Env = updateEnvVarList(testDeployment.Spec.Template.Spec.Containers[0].Env, key, val)
		}

		return nil
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
// an error return when used with a non-deployment type
func withOperatorImage(image string) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			dep.Spec.Template.Spec.Containers[0].Image = image
			return nil
		}

		return fmt.Errorf("withOperatorImage() called on a non-deployment object")
	}
}
