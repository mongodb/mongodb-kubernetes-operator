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
	LocalOperatorEnvName            = "MDB_LOCAL_OPERATOR"
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
	MongoDBImage            string
	MongoDBRepoUrl          string
	LocalOperator           bool
}

func LoadTestConfigFromEnv() TestConfig {
	return TestConfig{
		Namespace:               envvar.GetEnvOrDefault(testNamespaceEnvName, "mongodb"),                                                                                           // nolint:forbidigo
		CertManagerNamespace:    envvar.GetEnvOrDefault(testCertManagerNamespaceEnvName, "cert-manager"),                                                                           // nolint:forbidigo
		CertManagerVersion:      envvar.GetEnvOrDefault(testCertManagerVersionEnvName, "v1.5.3"),                                                                                   // nolint:forbidigo
		OperatorImage:           envvar.GetEnvOrDefault(operatorImageEnvName, "quay.io/mongodb/community-operator-dev:latest"),                                                     // nolint:forbidigo
		MongoDBImage:            envvar.GetEnvOrDefault(construct.MongodbImageEnv, "mongodb-community-server"),                                                                     // nolint:forbidigo
		MongoDBRepoUrl:          envvar.GetEnvOrDefault(construct.MongodbRepoUrlEnv, "quay.io/mongodb"),                                                                            // nolint:forbidigo
		VersionUpgradeHookImage: envvar.GetEnvOrDefault(construct.VersionUpgradeHookImageEnv, "quay.io/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.2"), // nolint:forbidigo
		// TODO: better way to decide default agent image.
		AgentImage:          envvar.GetEnvOrDefault(construct.AgentImageEnv, "quay.io/mongodb/mongodb-agent-ubi:10.29.0.6830-1"),                 // nolint:forbidigo
		ClusterWide:         envvar.ReadBool(clusterWideEnvName),                                                                                 // nolint:forbidigo
		PerformCleanup:      envvar.ReadBool(performCleanupEnvName),                                                                              // nolint:forbidigo
		ReadinessProbeImage: envvar.GetEnvOrDefault(construct.ReadinessProbeImageEnv, "quay.io/mongodb/mongodb-kubernetes-readinessprobe:1.0.3"), // nolint:forbidigo
		HelmChartPath:       envvar.GetEnvOrDefault(helmChartPathEnvName, "/workspace/helm-charts/charts/community-operator"),                    // nolint:forbidigo
		LocalOperator:       envvar.ReadBool(LocalOperatorEnvName),                                                                               // nolint:forbidigo
	}
}
