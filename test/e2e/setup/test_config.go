package setup

import (
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
)

const (
	testNamespaceEnvName  = "TEST_NAMESPACE"
	operatorImageEnvName  = "OPERATOR_IMAGE"
	clusterWideEnvName    = "CLUSTER_WIDE"
	performCleanupEnvName = "PERFORM_CLEANUP"
)

type testConfig struct {
	namespace               string
	operatorImage           string
	versionUpgradeHookImage string
	clusterWide             bool
	performCleanup          bool
	agentImage              string
	readinessProbeImage     string
}

func loadTestConfigFromEnv() testConfig {
	return testConfig{
		namespace:               envvar.GetEnvOrDefault(testNamespaceEnvName, "mongodb"),
		operatorImage:           envvar.GetEnvOrDefault(operatorImageEnvName, "quay.io/mongodb/community-operator-dev:latest"),
		versionUpgradeHookImage: envvar.GetEnvOrDefault(construct.VersionUpgradeHookImageEnv, "quay.io/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.2"),
		agentImage:              envvar.GetEnvOrDefault(construct.AgentImageEnv, "quay.io/mongodb/mongodb-agent:10.29.0.6830-1"), // TODO: better way to decide default agent image.
		clusterWide:             envvar.ReadBool(clusterWideEnvName),
		performCleanup:          envvar.ReadBool(performCleanupEnvName),
		readinessProbeImage:     envvar.GetEnvOrDefault(construct.ReadinessProbeImageEnv, "quay.io/mongodb/mongodb-kubernetes-readinessprobe:1.0.3"),
	}
}
