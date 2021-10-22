package setup

import (
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
)

const (
	testNamespaceEnvName            = "TEST_NAMESPACE"
	testCertManagerNamespaceEnvName = "TEST_CERT_MANAGER_NAMESPACE"
	testCertManagerVersionEnvName   = "TEST_CERT_MANAGER_VERSION"
	operatorImageEnvName            = "OPERATOR_IMAGE"
	clusterWideEnvName              = "CLUSTER_WIDE"
	performCleanupEnvName           = "PERFORM_CLEANUP"
	helmChartPathEnvName            = "HELM_CHART_PATH"
)

type TestConfig struct {
	Namespace               string
	CertManagerNamespace    string
	CertManagerVersion      string
	OperatorImage           string
	VersionUpgradeHookImage string
	ClusterWide             bool
	PerformCleanup          bool
	AgentImage              string
	ReadinessProbeImage     string
	HelmChartPath           string
}

func loadTestConfigFromEnv() TestConfig {
	return TestConfig{
		Namespace:               envvar.GetEnvOrDefault(testNamespaceEnvName, "mongodb"),
		CertManagerNamespace:    envvar.GetEnvOrDefault(testCertManagerNamespaceEnvName, "cert-manager"),
		CertManagerVersion:      envvar.GetEnvOrDefault(testCertManagerVersionEnvName, "v1.5.3"),
		OperatorImage:           envvar.GetEnvOrDefault(operatorImageEnvName, "quay.io/mongodb/community-operator-dev:latest"),
		VersionUpgradeHookImage: envvar.GetEnvOrDefault(construct.VersionUpgradeHookImageEnv, "quay.io/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.2"),
		AgentImage:              envvar.GetEnvOrDefault(construct.AgentImageEnv, "quay.io/mongodb/mongodb-agent:10.29.0.6830-1"), // TODO: better way to decide default agent image.
		ClusterWide:             envvar.ReadBool(clusterWideEnvName),
		PerformCleanup:          envvar.ReadBool(performCleanupEnvName),
		ReadinessProbeImage:     envvar.GetEnvOrDefault(construct.ReadinessProbeImageEnv, "quay.io/mongodb/mongodb-kubernetes-readinessprobe:1.0.3"),
		HelmChartPath:           envvar.GetEnvOrDefault(helmChartPathEnvName, "/workspace/helm-charts/charts/community-operator"),
	}
}
