package setup

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
)

const (
	testNamespaceEnvName      = "TEST_NAMESPACE"
	operatorImageEnvName      = "OPERATOR_IMAGE"
	clusterWideEnvName        = "CLUSTER_WIDE"
	versionUpgradeHookEnvName = "VERSION_UPGRADE_HOOK_IMAGE"
	performCleanupEnvName     = "PERFORM_CLEANUP"
)

type testConfig struct {
	namespace               string
	operatorImage           string
	versionUpgradeHookImage string
	clusterWide             bool
	performCleanup          bool
}

func loadTestConfigFromEnv() testConfig {
	return testConfig{
		namespace:               envvar.GetEnvOrDefault(testNamespaceEnvName, "default"),
		operatorImage:           envvar.GetEnvOrDefault(operatorImageEnvName, "quay.io/mongodb/community-operator-dev:latest"),
		versionUpgradeHookImage: envvar.GetEnvOrDefault(versionUpgradeHookEnvName, "quay.io/mongodb/mongodb-kubernetes-operator-version-upgrade-post-start-hook:1.0.2"),
		clusterWide:             envvar.ReadBool(clusterWideEnvName),
		performCleanup:          envvar.ReadBool(performCleanupEnvName),
	}
}
