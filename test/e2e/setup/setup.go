package setup

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pkg/errors"

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
func GeneratePasswordForUser(mdbu mdbv1.MongoDBUser, ctx *f.Context) (string, error) {
	passwordKey := mdbu.PasswordSecretRef.Key
	if passwordKey == "" {
		passwordKey = "password"
	}

	password, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return "", err
	}

	passwordSecret := secret.Builder().
		SetName(mdbu.PasswordSecretRef.Name).
		SetNamespace(f.Global.OperatorNamespace).
		SetField(passwordKey, password).
		Build()

	return password, f.Global.Client.Create(context.TODO(), &passwordSecret, &f.CleanupOptions{TestContext: ctx})
}
