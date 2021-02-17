package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

const (
	performCleanup = "PERFORM_CLEANUP"
	deployDir      = "/workspace/config/manager"
	roleDir        = "/workspace/config/rbac"
)

func InitTest(t *testing.T) (*e2eutil.Context, bool) {
	ctx := e2eutil.NewContext(t)

	if err := deployOperator(); err != nil {
		t.Fatal(err)
	}

	clean := os.Getenv(performCleanup)

	return ctx, clean == "True"
}

// CreateTLSResources will setup the CA ConfigMap and cert-key Secret necessary for TLS
// The certificates and keys are stored in testdata/tls
func CreateTLSResources(namespace string, ctx *e2eutil.Context) error { //nolint
	tlsConfig := e2eutil.NewTestTLSConfig(false)

	// Create CA ConfigMap
	ca, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "ca.crt"))
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
	cert, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "server.crt"))
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "server.key"))
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
func GeneratePasswordForUser(mdbu mdbv1.MongoDBUser, ctx *e2eutil.Context, namespace string) (string, error) {
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

func deployOperator() error {
	testConfig := loadTestConfigFromEnv()

	e2eutil.OperatorNamespace = testConfig.namespace
	fmt.Printf("Setting operator namespace to %s\n", e2eutil.OperatorNamespace)
	watchNamespace := testConfig.namespace
	if testConfig.clusterWide {
		watchNamespace = "*"
	}
	fmt.Printf("Setting namespace to watch to %s\n", watchNamespace)

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir, "role.yaml"), &rbacv1.Role{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role: %s", err)
	}
	fmt.Println("Successfully created the operator Role")

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir, "service_account.yaml"), &corev1.ServiceAccount{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator service account: %s", err)
	}
	fmt.Println("Successfully created the operator Service Account")

	if err := buildKubernetesResourceFromYamlFile(path.Join(roleDir, "role_binding.yaml"), &rbacv1.RoleBinding{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role binding: %s", err)
	}
	fmt.Println("Successfully created the operator Role Binding")
	if err := buildKubernetesResourceFromYamlFile(path.Join(deployDir, "manager.yaml"),
		&appsv1.Deployment{},
		withNamespace(testConfig.namespace),
		withOperatorImage(testConfig.operatorImage),
		withVersionUpgradeHookImage(testConfig.versionUpgradeHookImage),
		withEnvVar("WATCH_NAMESPACE", watchNamespace),
	); err != nil {
		return errors.Errorf("error building operator deployment: %s", err)
	}
	fmt.Println("Successfully created the operator Deployment")
	return nil
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

// withVersionUpgradeHookImage sets the value of the VERSION_UPGRADE_HOOK_IMAGE
// EnvVar from first container to `image`. The EnvVar is updated
// if it exists. Or appended if there is no EnvVar with this `Name`.
func withVersionUpgradeHookImage(image string) func(runtime.Object) {
	return func(obj runtime.Object) {
		if dep, ok := obj.(*appsv1.Deployment); ok {
			versionUpgradeHookEnv := corev1.EnvVar{
				Name:  "VERSION_UPGRADE_HOOK_IMAGE",
				Value: image,
			}
			found := false
			for idx := range dep.Spec.Template.Spec.Containers[0].Env {
				if dep.Spec.Template.Spec.Containers[0].Env[idx].Name == versionUpgradeHookEnv.Name {
					dep.Spec.Template.Spec.Containers[0].Env[idx].Value = versionUpgradeHookEnv.Value
					found = true
				}
			}
			if !found {
				dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, versionUpgradeHookEnv)
			}
		}
	}
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
