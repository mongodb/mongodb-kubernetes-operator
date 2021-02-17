package e2eutil

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/deprecated/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

// TestClient is the global client used by e2e tests.
var TestClient *E2ETestClient

// OperatorNamespace tracks the namespace in which the operator is deployed.
var OperatorNamespace string

// CleanupOptions are a way to register cleanup functions on object creation using the test client.
type CleanupOptions struct {
	TestContext *Context
}

// ApplyToCreate is a required method for CleanupOptions passed to the Create api.
func (*CleanupOptions) ApplyToCreate(*client.CreateOptions) {}

// Context tracks cleanup functions to be called at the end of a test.
type Context struct {
	cleanupFuncs [](func() error)
	t            *testing.T
}

// NewContext creates a context.
func NewContext(t *testing.T) *Context {
	return &Context{t: t}
}

// Cleanup is called at the end of a test.
func (ctx *Context) Cleanup() {
	for _, fn := range ctx.cleanupFuncs {
		err := fn()
		if err != nil {
			fmt.Println(err)
		}
	}
}

// AddCleanupFunc adds a cleanup function to the context to be called at the end of a test.
func (ctx *Context) AddCleanupFunc(fn func() error) {
	ctx.cleanupFuncs = append(ctx.cleanupFuncs, fn)
}

// E2ETestClient is a wrapper on client.Client that provides cleanup functionality.
type E2ETestClient struct {
	Client client.Client
}

// NewE2ETestClient creates a new E2ETestClient.
func newE2ETestClient(config *rest.Config, scheme *runtime.Scheme) (*E2ETestClient, error) {
	cli, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return &E2ETestClient{Client: cli}, err
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

// RunTest is the main entry point function for an e2e test.
func RunTest(m *testing.M) (int, error) {
	var cfg *rest.Config
	var testEnv *envtest.Environment
	var err error

	useExistingCluster := true
	testEnv = &envtest.Environment{
		UseExistingCluster:       &useExistingCluster,
		AttachControlPlaneOutput: true,
		KubeAPIServerFlags:       []string{"--authorization-mode=RBAC"},
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
