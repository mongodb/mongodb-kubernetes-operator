package setup

import (
	"context"
	"fmt"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	performCleanup = "PERFORM_CLEANUP"
)

func InitTest(t *testing.T) (*f.Context, bool) {
	ctx := f.NewContext(t)

	if err := registerTypesWithFramework(&mdbv1.MongoDB{}); err != nil {
		t.Fatal(err)
	}

	clean := os.Getenv(performCleanup)

	return ctx, clean == "True"
}

func registerTypesWithFramework(newTypes ...runtime.Object) error {

	for _, newType := range newTypes {
		if err := f.AddToFrameworkScheme(apis.AddToScheme, newType); err != nil {
			return fmt.Errorf("failed to add custom resource type %s to framework scheme: %v", newType.GetObjectKind(), err)
		}
	}
	return nil
}

// CreateTLSResources will setup the CA ConfigMap and cert-key Secret necessary for TLS
// The certificates and keys are stored in testdata/tls
func CreateTLSResources(namespace string, ctx *f.TestCtx) error {
		tlsConfig := e2eutil.NewTestTLSConfig(false)

		// Create CA ConfigMap
		ca, err := ioutil.ReadFile("testdata/tls/ca.crt")
		if err != nil {
			return nil
		}

		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tlsConfig.CAConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				"ca.crt": string(ca),
			},
		}
		err = f.Global.Client.Create(context.TODO(), &configMap, &f.CleanupOptions{TestContext: ctx})
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

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tlsConfig.ServerSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"tls.crt": cert,
				"tls.key": key,
			},
		}
		return f.Global.Client.Create(context.TODO(), &secret, &f.CleanupOptions{TestContext: ctx})
	}
}
