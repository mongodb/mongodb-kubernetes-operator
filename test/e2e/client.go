package e2eutil

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"

	// Needed for running tests on GCP
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// TestClient is the global client used by e2e tests.
var TestClient *E2ETestClient

// OperatorNamespace tracks the namespace in which the operator is deployed.
var OperatorNamespace string

// CleanupOptions are a way to register cleanup functions on object creation using the test client.
type CleanupOptions struct {
	TestContext *TestContext
}

// ApplyToCreate is a required method for CleanupOptions passed to the Create api.
func (*CleanupOptions) ApplyToCreate(*client.CreateOptions) {}

// TestContext tracks cleanup functions to be called at the end of a test.
type TestContext struct {
	Ctx context.Context

	// shouldPerformCleanup indicates whether or not cleanup should happen after this test
	shouldPerformCleanup bool

	// ExecutionId is a unique identifier for this test run.
	ExecutionId string

	// cleanupFuncs is a list of functions which will clean up resources
	// after the test ends.
	cleanupFuncs []func() error

	// t is the testing.T which will be used for the duration of the test.
	t *testing.T
}

// NewContext creates a context.
func NewContext(ctx context.Context, t *testing.T, performCleanup bool) (*TestContext, error) {
	testId, err := generate.RandomValidDNS1123Label(10)
	if err != nil {
		return nil, err
	}

	return &TestContext{Ctx: ctx, t: t, ExecutionId: testId, shouldPerformCleanup: performCleanup}, nil
}

// Teardown is called at the end of a test.
func (ctx *TestContext) Teardown() {
	if !ctx.shouldPerformCleanup {
		return
	}
	for _, fn := range ctx.cleanupFuncs {
		err := fn()
		if err != nil {
			fmt.Println(err)
		}
	}
}

// AddCleanupFunc adds a cleanup function to the context to be called at the end of a test.
func (ctx *TestContext) AddCleanupFunc(fn func() error) {
	ctx.cleanupFuncs = append(ctx.cleanupFuncs, fn)
}

// E2ETestClient is a wrapper on client.Client that provides cleanup functionality.
type E2ETestClient struct {
	Client client.Client
	// We need the core API client for some operations that the controller-runtime client doesn't support
	// (e.g. exec into the container)
	CoreV1Client  corev1client.CoreV1Client
	DynamicClient dynamic.Interface
	restConfig    *rest.Config
}

// NewE2ETestClient creates a new E2ETestClient.
func newE2ETestClient(config *rest.Config, scheme *runtime.Scheme) (*E2ETestClient, error) {
	cli, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	coreClient, err := corev1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &E2ETestClient{Client: cli, CoreV1Client: *coreClient, DynamicClient: dynamicClient, restConfig: config}, err
}

// Create wraps client.Create to provide post-test cleanup functionality.
func (c *E2ETestClient) Create(ctx context.Context, obj client.Object, cleanupOptions *CleanupOptions) error {
	err := c.Client.Create(ctx, obj)
	if err != nil {
		return err
	}

	if cleanupOptions == nil || cleanupOptions.TestContext == nil {
		return nil
	}

	cleanupOptions.TestContext.AddCleanupFunc(func() error {
		err := TestClient.Delete(ctx, obj)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	})

	return nil
}

// Delete wraps client.Delete.
func (c *E2ETestClient) Delete(ctx context.Context, obj client.Object) error {
	return c.Client.Delete(ctx, obj)
}

// Update wraps client.Update.
func (c *E2ETestClient) Update(ctx context.Context, obj client.Object) error {
	return c.Client.Update(ctx, obj)
}

// Get wraps client.Get.
func (c *E2ETestClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object) error {
	return c.Client.Get(ctx, key, obj)
}

func (c *E2ETestClient) Execute(ctx context.Context, pod corev1.Pod, containerName, command string) (string, error) {
	req := c.CoreV1Client.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"/bin/sh", "-c", command},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return "", err
	}
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", fmt.Errorf(`failed executing command "%s" on %v/%v: %s ("%s")`, command, pod.Namespace, pod.Name, err, errBuf.String())
	}

	if errBuf.String() != "" {
		return buf.String(), fmt.Errorf("remote command %s on %v/%v raised an error: %s", command, pod.Namespace, pod.Name, errBuf.String())
	}
	return buf.String(), nil
}

// RunTest is the main entry point function for an e2e test.
func RunTest(m *testing.M) (int, error) {
	var cfg *rest.Config
	var testEnv *envtest.Environment
	var err error

	useExistingCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster:       &useExistingCluster,
		AttachControlPlaneOutput: true,
	}

	fmt.Println("Starting test environment")
	cfg, err = testEnv.Start()
	if err != nil {
		return 1, err
	}

	err = mdbv1.AddToScheme(scheme.Scheme)
	if err != nil {
		return 1, err
	}

	TestClient, err = newE2ETestClient(cfg, scheme.Scheme)
	if err != nil {
		return 1, err
	}

	fmt.Println("Starting test")
	code := m.Run()

	err = testEnv.Stop()
	if err != nil {
		return code, err
	}

	return code, nil
}
