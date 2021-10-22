package setup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/helm"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
	waite2e "github.com/mongodb/mongodb-kubernetes-operator/test/e2e/util/wait"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

type tlsSecretType string

const (
	performCleanupEnv               = "PERFORM_CLEANUP"
	CertKeyPair       tlsSecretType = "CERTKEYPAIR"
	Pem               tlsSecretType = "PEM"
)

func Setup(t *testing.T) *e2eutil.Context {
	ctx, err := e2eutil.NewContext(t, envvar.ReadBool(performCleanupEnv))

	if err != nil {
		t.Fatal(err)
	}

	config := loadTestConfigFromEnv()
	if err := deployOperator(config, "mdb", false); err != nil {
		t.Fatal(err)
	}

	return ctx
}

func SetupWithTLS(t *testing.T, resourceName string) (*e2eutil.Context, TestConfig) {
	ctx, err := e2eutil.NewContext(t, envvar.ReadBool(performCleanupEnv))

	if err != nil {
		t.Fatal(err)
	}

	config := loadTestConfigFromEnv()
	if err := deployCertManager(config); err != nil {
		t.Fatal(err)
	}

	if err := deployOperator(config, resourceName, true); err != nil {
		t.Fatal(err)
	}

	return ctx, config
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
// this function expects the input to be in the form of some/registry/imagename:tag.
func extractRegistryNameAndVersion(fullImage string) (string, string, string) {
	splitString := strings.Split(fullImage, "/")
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
func getHelmArgs(testConfig TestConfig, watchNamespace string, resourceName string, withTLS bool) map[string]string {
	agentRegistry, agentName, agentVersion := extractRegistryNameAndVersion(testConfig.AgentImage)
	versionUpgradeHookRegistry, versionUpgradeHookName, versionUpgradeHookVersion := extractRegistryNameAndVersion(testConfig.VersionUpgradeHookImage)
	readinessProbeRegistry, readinessProbeName, readinessProbeVersion := extractRegistryNameAndVersion(testConfig.ReadinessProbeImage)
	operatorRegistry, operatorName, operatorVersion := extractRegistryNameAndVersion(testConfig.OperatorImage)

	helmArgs := make(map[string]string)

	helmArgs["namespace"] = testConfig.Namespace

	helmArgs["operator.watchNamespace"] = watchNamespace
	helmArgs["operator.operatorImageName"] = operatorName
	helmArgs["operator.version"] = operatorVersion

	helmArgs["versionUpgradeHook.name"] = versionUpgradeHookName
	helmArgs["versionUpgradeHook.version"] = versionUpgradeHookVersion

	helmArgs["readinessProbe.name"] = readinessProbeName
	helmArgs["readinessProbe.version"] = readinessProbeVersion

	helmArgs["agent.version"] = agentVersion
	helmArgs["agent.name"] = agentName

	helmArgs["registry.versionUpgradeHook"] = versionUpgradeHookRegistry
	helmArgs["registry.operator"] = operatorRegistry
	helmArgs["registry.agent"] = agentRegistry
	helmArgs["registry.readinessProbe"] = readinessProbeRegistry

	helmArgs["createResource"] = strconv.FormatBool(false)
	helmArgs["resource.name"] = resourceName
	helmArgs["resource.tls.enabled"] = strconv.FormatBool(withTLS)
	helmArgs["resource.tls.useCertManager"] = strconv.FormatBool(withTLS)

	return helmArgs
}

// deployOperator installs all resources required by the operator using helm.
func deployOperator(config TestConfig, resourceName string, withTLS bool) error {
	e2eutil.OperatorNamespace = config.Namespace
	fmt.Printf("Setting operator namespace to %s\n", e2eutil.OperatorNamespace)
	watchNamespace := config.Namespace
	if config.ClusterWide {
		watchNamespace = "*"
	}
	fmt.Printf("Setting namespace to watch to %s\n", watchNamespace)

	helmChartName := "mongodb-kubernetes-operator"
	if err := helm.Uninstall(helmChartName, config.Namespace); err != nil {
		return err
	}

	helmArgs := getHelmArgs(config, watchNamespace, resourceName, withTLS)
	helmFlags := map[string]string{
		"skip-crds":        "",
		"namespace":        config.Namespace,
		"create-namespace": "",
	}
	if err := helm.Install(config.HelmChartPath, helmChartName, helmFlags, helmArgs); err != nil {
		return err
	}

	dep, err := waite2e.ForDeploymentToExist("mongodb-kubernetes-operator", time.Second*10, time.Minute*1, e2eutil.OperatorNamespace)
	if err != nil {
		return err
	}

	quantityCPU, err := resource.ParseQuantity("50m")
	if err != nil {
		return err
	}

	for _, cont := range dep.Spec.Template.Spec.Containers {
		cont.Resources.Requests["cpu"] = quantityCPU
	}

	err = e2eutil.TestClient.Update(context.TODO(), &dep)
	if err != nil {
		return err
	}

	if err := wait.PollImmediate(time.Second, 30*time.Second, hasDeploymentRequiredReplicas(&dep)); err != nil {
		return errors.New("error building operator deployment: the deployment does not have the required replicas")
	}
	fmt.Println("Successfully installed the operator deployment")
	return nil
}

func deployCertManager(config TestConfig) error {
	const helmChartName = "cert-manager"
	if err := helm.Uninstall(helmChartName, config.CertManagerNamespace); err != nil {
		return errors.Errorf("failed to uninstall cert-manager Helm chart: %s", err)
	}

	charlUrl := fmt.Sprintf("https://charts.jetstack.io/charts/cert-manager-%s.tgz", config.CertManagerVersion)
	flags := map[string]string{
		"version":          config.CertManagerVersion,
		"namespace":        config.CertManagerNamespace,
		"create-namespace": "",
	}
	values := map[string]string{"installCRDs": "true"}
	if err := helm.Install(charlUrl, helmChartName, flags, values); err != nil {
		return errors.Errorf("failed to install cert-manager Helm chart: %s", err)
	}
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
