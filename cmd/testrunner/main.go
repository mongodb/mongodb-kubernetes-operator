package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/pkg/errors"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/pod"
	"k8s.io/client-go/kubernetes"

	"github.com/mongodb/mongodb-kubernetes-operator/cmd/testrunner/crds"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type flags struct {
	deployDir               string
	namespace               string
	operatorImage           string
	versionUpgradeHookImage string
	testImage               string
	test                    string
	performCleanup          string
}

func parseFlags() flags {
	var namespace, deployDir, operatorImage, versionUpgradeHookImage, testImage, test, performCleanup *string
	namespace = flag.String("namespace", "default", "the namespace the operator and tests should be deployed in")
	deployDir = flag.String("deployDir", "deploy/", "the path to the directory which contains the yaml deployment files")
	operatorImage = flag.String("operatorImage", "quay.io/mongodb/community-operator-dev:latest", "the image which should be used for the operator deployment")
	versionUpgradeHookImage = flag.String("versionUpgradeHookImage", "quay.io/mongodb/community-operator-pre-stop-hook:latest", "the version upgrade post-start hook image")
	testImage = flag.String("testImage", "quay.io/mongodb/community-operator-e2e:latest", "the image which should be used for the operator e2e tests")
	test = flag.String("test", "", "test e2e test that should be run. (name of folder containing the test)")
	performCleanup = flag.String("performCleanup", "1", "specifies whether to performing a cleanup the context or not")
	flag.Parse()

	return flags{
		deployDir:               *deployDir,
		namespace:               *namespace,
		operatorImage:           *operatorImage,
		versionUpgradeHookImage: *versionUpgradeHookImage,
		testImage:               *testImage,
		test:                    *test,
		performCleanup:          *performCleanup,
	}
}

func main() {
	if err := runCmd(parseFlags()); err != nil {
		panic(err)
	}
}

func runCmd(f flags) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.Errorf("could not get kubernetes config: %s", err)
	}

	k8s, err := k8sClient.New(config, k8sClient.Options{})
	if err != nil {
		return errors.Errorf("could not create client: %s", err)
	}

	c := client.NewClient(k8s)

	if err := ensureNamespace(f.namespace, c); err != nil {
		return errors.Errorf("could not create namespace: %s", err)
	}

	fmt.Printf("Ensured namespace: %s\n", f.namespace)

	if err := crds.EnsureCreation(config, f.deployDir); err != nil {
		return errors.Errorf("could not ensure CRDs: %s", err)
	}

	fmt.Println("Ensured CRDs")
	if err := deployOperator(f, c); err != nil {
		return errors.Errorf("could not deploy operator: %s", err)
	}
	fmt.Println("Successfully deployed the operator")

	testToRun := "test/operator-sdk-test.yaml"
	if err := buildKubernetesResourceFromYamlFile(c, testToRun, &corev1.Pod{}, withNamespace(f.namespace), withTestImage(f.testImage), withTest(f.test), withEnvVar("PERFORM_CLEANUP", f.performCleanup)); err != nil {
		return errors.Errorf("error deploying test: %s", err)
	}

	nsName := types.NamespacedName{Name: "operator-sdk-test", Namespace: f.namespace}

	fmt.Println("Waiting for pod to be ready...")
	testPod, err := pod.WaitForPhase(c, nsName, time.Second*5, time.Minute*5, corev1.PodRunning)
	if err != nil {
		return errors.Errorf("error waiting for test pod to be created: %s", err)
	}

	fmt.Println("Tailing pod logs...")
	if err := tailPodLogs(config, testPod); err != nil {
		return errors.Errorf("could not tail pod logs: %s", err)
	}

	_, err = pod.WaitForPhase(c, nsName, time.Second*5, time.Minute, corev1.PodSucceeded)
	if err != nil {
		return errors.Errorf("error waiting for test to finish: %v", err)
	}

	fmt.Println("Test passed!")
	return nil
}

func tailPodLogs(config *rest.Config, testPod corev1.Pod) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return errors.Errorf("could not get clientset: %s", err)
	}

	if err := pod.GetLogs(os.Stdout, pod.CoreV1FollowStreamer(testPod, clientset.CoreV1())); err != nil {
		return errors.Errorf("could not tail pod logs: %s", err)
	}
	return nil
}

func ensureNamespace(ns string, client client.Client) error {
	err := client.Get(context.TODO(), types.NamespacedName{Name: ns}, &corev1.Namespace{})
	if err != nil && !apiErrors.IsNotFound(err) {
		return errors.Errorf("error creating namespace: %v", err)
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
		return errors.Errorf("error creating namespace: %s", err)
	}
	return nil
}

func deployOperator(f flags, c client.Client) error {
	if err := buildKubernetesResourceFromYamlFile(c, path.Join(f.deployDir, "role.yaml"), &rbacv1.Role{}, withNamespace(f.namespace)); err != nil {
		return errors.Errorf("error building operator role: %v", err)
	}
	fmt.Println("Successfully created the operator Role")

	if err := buildKubernetesResourceFromYamlFile(c, path.Join(f.deployDir, "service_account.yaml"), &corev1.ServiceAccount{}, withNamespace(f.namespace)); err != nil {
		return errors.Errorf("error building operator service account: %v", err)
	}
	fmt.Println("Successfully created the operator Service Account")

	if err := buildKubernetesResourceFromYamlFile(c, path.Join(f.deployDir, "role_binding.yaml"), &rbacv1.RoleBinding{}, withNamespace(f.namespace)); err != nil {
		return errors.Errorf("error building operator role binding: %v", err)
	}
	fmt.Println("Successfully created the operator Role Binding")
	if err := buildKubernetesResourceFromYamlFile(c, path.Join(f.deployDir, "operator.yaml"),
		&appsv1.Deployment{},
		withNamespace(f.namespace),
		withOperatorImage(f.operatorImage),
		withVersionUpgradeHookImage(f.versionUpgradeHookImage)); err != nil {
		return errors.Errorf("error building operator deployment: %v", err)
	}
	fmt.Println("Successfully created the operator Deployment")
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

// withTestImage assumes that the type being created is a corev1.Pod
// and will have no effect when used with other types
func withTestImage(image string) func(obj runtime.Object) {
	return func(obj runtime.Object) {
		if testPod, ok := obj.(*corev1.Pod); ok {
			testPod.Spec.Containers[0].Image = image
		}
	}
}

func withEnvVar(key, val string) func(obj runtime.Object) {
	return func(obj runtime.Object) {
		if testPod, ok := obj.(*corev1.Pod); ok {
			testPod.Spec.Containers[0].Env = append(testPod.Spec.Containers[0].Env, corev1.EnvVar{Name: key, Value: val})
		}
	}
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

// withTest configures the test Pod to launch with the correct
// command which will target the given test
func withTest(test string) func(obj runtime.Object) {
	return func(obj runtime.Object) {
		if testPod, ok := obj.(*corev1.Pod); ok {
			testPod.Spec.Containers[0].Command = []string{
				"/bin/operator-sdk",
				"test",
				"local",
				fmt.Sprintf("./test/e2e/%s", test),
				"--operator-namespace",
				testPod.Namespace,
				"--verbose",
				"--kubeconfig",
				"/etc/config/kubeconfig",
				"--go-test-flags",
				"-timeout=20m",
			}
		}
	}
}

// buildKubernetesResourceFromYamlFile will create the kubernetes resource defined in yamlFilePath. All of the functional options
// provided will be applied before creation.
func buildKubernetesResourceFromYamlFile(c client.Client, yamlFilePath string, obj runtime.Object, options ...func(obj runtime.Object)) error {
	data, err := ioutil.ReadFile(yamlFilePath)
	if err != nil {
		return errors.Errorf("error reading file: %v", err)
	}

	if err := marshalRuntimeObjectFromYAMLBytes(data, obj); err != nil {
		return errors.Errorf("error converting yaml bytes to service account: %v", err)
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

func createOrUpdate(c client.Client, obj runtime.Object) error {
	if err := c.Create(context.TODO(), obj); err != nil {
		if apiErrors.IsAlreadyExists(err) {
			return c.Update(context.TODO(), obj)
		}
		return errors.Errorf("error creating %s in kubernetes: %v", obj.GetObjectKind(), err)
	}
	return nil
}
