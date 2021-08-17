package setup

import (
	"context"
	"fmt"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/helm"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
	"github.com/pkg/errors"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

type tlsSecretType string

const (
	performCleanupEnv               = "PERFORM_CLEANUP"
	helmChartPathEnv                = "HELM_CHART_PATH"
	CertKeyPair       tlsSecretType = "CERTKEYPAIR"
	Pem               tlsSecretType = "PEM"
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

// extractRegistryNameAndVersion splits a full image string and returns the individual components.
func extractRegistryNameAndVersion(fullStr string) (string, string, string) {
	splitString := strings.Split(fullStr, "/")
	registry := strings.Join(splitString[:len(splitString)-1], "/")

	splitString = strings.Split(splitString[len(splitString)-1], ":")
	version := "latest"
	if len(splitString) > 1 {
		version = splitString[len(splitString)-1]
	}
	name := splitString[0]
	return registry, name, version
}

// getHelmArgs returns a map of helm arguments that are required to install the operator.
func getHelmArgs(testConfig testConfig, watchNamespace string) map[string]string {
	agentRegistry, agentName, agentVersion := extractRegistryNameAndVersion(testConfig.agentImage)
	versionUpgradeHookRegistry, versionUpgradeHookName, versionUpgradeHookVersion := extractRegistryNameAndVersion(testConfig.versionUpgradeHookImage)
	readinessProbeRegistry, readinessProbeName, readinessProbeVersion := extractRegistryNameAndVersion(testConfig.readinessProbeImage)
	operatorRegistry, operatorName, operatorVersion := extractRegistryNameAndVersion(testConfig.operatorImage)

	helmArgs := make(map[string]string)

	helmArgs["namespace"] = testConfig.namespace

	helmArgs["operator.watchNamespace"] = watchNamespace
	helmArgs["operator.operator_image_name"] = operatorName
	helmArgs["operator.version"] = operatorVersion

	helmArgs["version_upgrade_hook.name"] = versionUpgradeHookName
	helmArgs["version_upgrade_hook.version"] = versionUpgradeHookVersion

	helmArgs["readiness_probe.name"] = readinessProbeName
	helmArgs["readiness_probe.version"] = readinessProbeVersion

	helmArgs["agent.version"] = agentVersion
	helmArgs["agent.name"] = agentName

	helmArgs["registry.version_upgrade_hook"] = versionUpgradeHookRegistry
	helmArgs["registry.operator"] = operatorRegistry
	helmArgs["registry.agent"] = agentRegistry
	helmArgs["registry.readiness_probe"] = readinessProbeRegistry

	return helmArgs
}

// deployOperator installs all resources required by the operator using helm.
func deployOperator() error {
	testConfig := loadTestConfigFromEnv()
	e2eutil.OperatorNamespace = testConfig.namespace
	fmt.Printf("Setting operator namespace to %s\n", e2eutil.OperatorNamespace)
	watchNamespace := testConfig.namespace
	if testConfig.clusterWide {
		watchNamespace = "*"
	}
	fmt.Printf("Setting namespace to watch to %s\n", watchNamespace)

	chartName := "mongodb-kubernetes-operator"
	if err := helm.Uninstall(chartName); err != nil {
		return err
	}

	helmChartPath := envvar.GetEnvOrDefault(helmChartPathEnv, "/workspace/helm-chart")
	helmArgs := getHelmArgs(testConfig, watchNamespace)
	if err := helm.Install(helmChartPath, chartName, helmArgs); err != nil {
		return err
	}

	dep := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongodb-kubernetes-operator",
			Namespace: e2eutil.OperatorNamespace,
		},
	}

	if err := wait.PollImmediate(time.Second, 30*time.Second, hasDeploymentRequiredReplicas(&dep)); err != nil {
		return errors.New("error building operator deployment: the deployment does not have the required replicas")
	}

	fmt.Println("Successfully created the operator deployment")
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
