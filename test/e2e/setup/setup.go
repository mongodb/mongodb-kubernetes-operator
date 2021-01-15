package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	performCleanup = "PERFORM_CLEANUP"
	deployDir      = "deploy"
)

func InitTest(t *testing.T) (*f.Context, bool) {

	ctx := f.NewContext(t)
	if err := registerTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	if err := deployOperator(f.Global.Client); err != nil {
		t.Fatal(err)
	}

	clean := os.Getenv(performCleanup)

	return ctx, clean == "True"
}

func registerTypesWithFramework(newTypes ...runtime.Object) error {

	for _, newType := range newTypes {
		if err := f.AddToFrameworkScheme(apis.AddToScheme, newType); err != nil {
			return errors.Errorf("failed to add custom resource type %s to framework scheme: %s", newType.GetObjectKind(), err)
		}
	}
	return nil
}

// CreateTLSResources will setup the CA ConfigMap and cert-key Secret necessary for TLS
// The certificates and keys are stored in testdata/tls
func CreateTLSResources(namespace string, ctx *f.TestCtx) error { //nolint
	tlsConfig := e2eutil.NewTestTLSConfig(false)

	// Create CA ConfigMap
	ca, err := ioutil.ReadFile("testdata/tls/ca.crt")
	if err != nil {
		return nil
	}

	caConfigMap := configmap.Builder().
		SetName(tlsConfig.CaConfigMap.Name).
		SetNamespace(namespace).
		SetField("ca.crt", string(ca)).
		Build()

	err = f.Global.Client.Create(context.TODO(), &caConfigMap, &f.CleanupOptions{TestContext: ctx})
	if err != nil {
		return err
	}

	// Create server key and certificate secret
	cert, err := ioutil.ReadFile("testdata/tls/server.crt")
	if err != nil {
		return err
	}
	key, err := ioutil.ReadFile("testdata/tls/server.key")
	if err != nil {
		return err
	}

	certKeySecret := secret.Builder().
		SetName(tlsConfig.CertificateKeySecret.Name).
		SetNamespace(namespace).
		SetField("tls.crt", string(cert)).
		SetField("tls.key", string(key)).
		Build()

	return f.Global.Client.Create(context.TODO(), &certKeySecret, &f.CleanupOptions{TestContext: ctx})
}

// GeneratePasswordForUser will create a secret with a password for the given user
func GeneratePasswordForUser(mdbu mdbv1.MongoDBUser, ctx *f.Context, namespace string) (string, error) {
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
		nsp = f.Global.OperatorNamespace
	}

	passwordSecret := secret.Builder().
		SetName(mdbu.PasswordSecretRef.Name).
		SetNamespace(nsp).
		SetField(passwordKey, password).
		Build()

	return password, f.Global.Client.Create(context.TODO(), &passwordSecret, &f.CleanupOptions{TestContext: ctx})
}

func deployOperator(c f.FrameworkClient) error {
	testConfig := loadTestConfigFromEnv()

	if err := buildKubernetesResourceFromYamlFile(c, path.Join(deployDir, "role.yaml"), &rbacv1.Role{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role: %s", err)
	}
	fmt.Println("Successfully created the operator Role")

	if err := buildKubernetesResourceFromYamlFile(c, path.Join(deployDir, "service_account.yaml"), &corev1.ServiceAccount{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator service account: %s", err)
	}
	fmt.Println("Successfully created the operator Service Account")

	if err := buildKubernetesResourceFromYamlFile(c, path.Join(deployDir, "role_binding.yaml"), &rbacv1.RoleBinding{}, withNamespace(testConfig.namespace)); err != nil {
		return errors.Errorf("error building operator role binding: %s", err)
	}
	fmt.Println("Successfully created the operator Role Binding")
	watchNamespace := testConfig.namespace
	if testConfig.clusterWide {
		watchNamespace = "*"
	}
	fmt.Println("Successfully created the operator Role Binding")
	if err := buildKubernetesResourceFromYamlFile(c, path.Join(deployDir, "operator.yaml"),
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
func buildKubernetesResourceFromYamlFile(c f.FrameworkClient, yamlFilePath string, obj runtime.Object, options ...func(obj runtime.Object)) error {
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

	return createOrUpdate(c, obj)
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

func createOrUpdate(c f.FrameworkClient, obj runtime.Object) error {
	if err := c.Create(context.TODO(), obj, &f.CleanupOptions{}); err != nil {
		if apiErrors.IsAlreadyExists(err) {
			return c.Update(context.TODO(), obj)
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
