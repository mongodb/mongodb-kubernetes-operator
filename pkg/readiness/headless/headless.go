package headless

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/pod"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/secret"
	"go.uber.org/zap"
)

const (
	acVersionPath string = "/var/lib/automation/config/acVersion/version"
)

// PerformCheckHeadlessMode validates if the Agent has reached the correct goal state
// The state is fetched from K8s automation config Secret directly to avoid flakiness of mounting process
// Dev note: there is an alternative way to get current namespace: to read from
// /var/run/secrets/kubernetes.io/serviceaccount/namespace file (see
// https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod)
// though passing the namespace as an environment variable makes the code simpler for testing and saves an IO operation
func PerformCheckHeadlessMode(ctx context.Context, health health.Status, conf config.Config) (bool, error) {
	var targetVersion int64
	var err error

	targetVersion, err = secret.ReadAutomationConfigVersionFromSecret(ctx, conf.Namespace, conf.ClientSet, conf.AutomationConfigSecretName)
	if err != nil {
		// this file is expected to be present in case of AppDB, there is no point trying to access it in
		// community, it masks the underlying error
		if _, pathErr := os.Stat(acVersionPath); !os.IsNotExist(pathErr) {
			file, err := os.Open(acVersionPath)
			if err != nil {
				return false, err
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				return false, err
			}

			targetVersion, err = strconv.ParseInt(string(data), 10, 64)
			if err != nil {
				return false, err
			}
		} else {
			return false, fmt.Errorf("failed to fetch automation-config secret name: %s, err: %s", conf.AutomationConfigSecretName, err)
		}
	}

	currentAgentVersion := readCurrentAgentInfo(health, targetVersion)

	if err = pod.PatchPodAnnotation(ctx, conf.Namespace, currentAgentVersion, conf.Hostname, conf.ClientSet); err != nil {
		return false, err
	}

	return targetVersion == currentAgentVersion, nil
}

// readCurrentAgentInfo returns the version the Agent has reached and the rs member name
func readCurrentAgentInfo(health health.Status, targetVersion int64) int64 {
	for _, v := range health.MmsStatus {
		zap.S().Debugf("Automation Config version: %d, Agent last version: %d", targetVersion, v.LastGoalStateClusterConfigVersion)
		return v.LastGoalStateClusterConfigVersion
	}

	// If there are no plans, we always return target version.
	// Previously we relied on IsInGoalState, but the agent started sometimes returning IsInGoalState=false when scaling down members.
	// No plans will occur if the agent is just starting or if the current process is not in the process list in automation config.
	// Either way this is not a blocker for the operator to perform necessary statefulset changes on it.

	return targetVersion
}
