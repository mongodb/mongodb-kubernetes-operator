package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/mongodb/mongodb-kubernetes-operator/scripts/ci/e2e/crds"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type flags struct {
	kubeconfig    string
	deployDir     string
	namespace     string
	operatorImage string
}

func parseFlags() flags {
	var kubeconfig, namespace, deployDir, operatorImage *string
	if home := os.Getenv("HOME"); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = flag.String("namespace", "default", "the namespace the operator and tests should be deployed in")
	deployDir = flag.String("deployDir", "deploy/", "the path to the directory which contains the yaml deployment files")
	operatorImage = flag.String("operatorImage", "quay.io/mongodb/community-operator-dev:latest", "the image which should be used for the operator deployment")
	flag.Parse()

	return flags{
		kubeconfig:    *kubeconfig,
		deployDir:     *deployDir,
		namespace:     *namespace,
		operatorImage: *operatorImage,
	}
}

func main() {
	if err := runCmd(parseFlags()); err != nil {
		panic(err)
	}
}

func runCmd(f flags) error {
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", f.kubeconfig)
	if err != nil {
		return fmt.Errorf("error retreiving kubernetes config: %v", err)
	}

	c, err := client.New(config, client.Options{})

	if err != nil {
		return fmt.Errorf("error creating kubernetes client %v", err)
	}

	if err := crds.EnsureCreation(config, f.deployDir); err != nil {
		return fmt.Errorf("error ensuring CRDs: %v", err)
	}

	fmt.Println("Ensured CRDs")

	if err := ensureNamespace(f.namespace, c); err != nil {
		return fmt.Errorf("error ensuring namespace: %v", err)
	}
	fmt.Printf("Ensured namespace: %s\n", f.namespace)

	if err := deployOperator(f, c); err != nil {
		return fmt.Errorf("error deploying operator: %v", err)
	}
	fmt.Println("Successfully deployed the operator")
	return nil
}

func ensureNamespace(ns string, client client.Client) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: ns}, &corev1.Namespace{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error creating namespace: %v", err)
	} else if err == nil {
		fmt.Printf("Namespace %s already exists!\n", ns)
		return nil
	}

	newNamespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	if err := client.Create(context.TODO(), &newNamespace); err != nil {
		return fmt.Errorf("error creating namespace: %s", err)
	}
	return nil
}

func deployOperator(f flags, c client.Client) error {
	if err := buildOperatorRole(path.Join(f.deployDir, "role.yaml"), f.namespace, c); err != nil {
		return fmt.Errorf("error building operator role: %v", err)
	}
	fmt.Println("Successfully deployed the operator Role")
	if err := buildOperatorServiceAccount(path.Join(f.deployDir, "service_account.yaml"), f.namespace, c); err != nil {
		return fmt.Errorf("error building operator service account: %v", err)
	}
	fmt.Println("Successfully deployed the operator Service Account")
	if err := buildOperatorRoleBinding(path.Join(f.deployDir, "role_binding.yaml"), f.namespace, c); err != nil {
		return fmt.Errorf("error building operator role binding: %v", err)
	}
	fmt.Println("Successfully deployed the operator Role Binding")
	if err := buildOperatorDeployment(path.Join(f.deployDir, "operator.yaml"), f, c); err != nil {
		return fmt.Errorf("error building operator deployment: %v", err)
	}
	fmt.Println("Successfully deployed the operator Deployment")
	return nil
}

func buildOperatorRoleBinding(yamlFilePath, namespace string, c client.Client) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	roleBinding := rbacv1.RoleBinding{}
	if err := marshalRuntimeObjectFromYAMLBytes(data, &roleBinding); err != nil {
		return fmt.Errorf("error converting yaml bytes to role binding: %v", err)
	}
	roleBinding.Namespace = namespace

	return createOrUpdate(c, &roleBinding)
}

func buildOperatorServiceAccount(yamlFilePath, namespace string, c client.Client) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	serviceAccount := corev1.ServiceAccount{}
	if err := marshalRuntimeObjectFromYAMLBytes(data, &serviceAccount); err != nil {
		return fmt.Errorf("error converting yaml bytes to service account: %v", err)
	}

	serviceAccount.Namespace = namespace

	return createOrUpdate(c, &serviceAccount)
}

func marshalRuntimeObjectFromYAMLBytes(bytes []byte, obj runtime.Object) error {
	jsonBytes, err := yaml.YAMLToJSON(bytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, &obj)
}

func buildOperatorRole(yamlFilePath, namespace string, c client.Client) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	role := rbacv1.Role{}
	if err := marshalRuntimeObjectFromYAMLBytes(data, &role); err != nil {
		return fmt.Errorf("error converting yaml bytes to role: %v", err)
	}
	role.Namespace = namespace

	return createOrUpdate(c, &role)
}

func buildOperatorDeployment(yamlFilePath string, f flags, c client.Client) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	dep := appsv1.Deployment{}
	if err := marshalRuntimeObjectFromYAMLBytes(data, &dep); err != nil {
		return fmt.Errorf("error converting yaml bytes to deployment: %v", err)
	}
	dep.Namespace = f.namespace
	dep.Spec.Template.Spec.Containers[0].Image = f.operatorImage

	return createOrUpdate(c, &dep)
}

func createOrUpdate(c client.Client, obj runtime.Object) error {
	if err := c.Create(context.TODO(), obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return c.Update(context.TODO(), obj)
		}
		return fmt.Errorf("error creating %s in kubernetes: %v", obj.GetObjectKind(), err)
	}
	return nil
}
