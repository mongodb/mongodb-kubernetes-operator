package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	agentStatusFilePathEnv = "AGENT_STATUS_FILEPATH"

	defaultNamespace = "default"

	pollingInterval time.Duration = 1 * time.Second
	pollingDuration time.Duration = 60 * time.Second
)

func main() {
	ctx := context.Background()
	logger := setupLogger()

	logger.Info("Running version change post-start hook")

	if statusPath := os.Getenv(agentStatusFilePathEnv); statusPath == "" {
		logger.Fatalf(`Required environment variable "%s" not set`, agentStatusFilePathEnv)
		return
	}

	logger.Info("Waiting for agent health status...")
	health, err := waitForAgentHealthStatus()
	if err != nil {
		// If the pod has just restarted then the status file will not exist.
		// In that case we return and let mongod start again.
		if os.IsNotExist(err) {
			logger.Info("Agent health status file not found, mongod will start")
		} else {
			logger.Errorf("Error getting the agent health file: %s", err)
		}

		return
	}

	shouldDelete, err := shouldDeletePod(health)
	if err != nil {
		logger.Errorf("Error checking if pod should be deleted: %s", err)
	}

	if shouldDelete {
		logger.Infof("Pod should be deleted")
		if err := deletePod(ctx); err != nil {
			// We should not raise an error if the Pod could not be deleted. It can have even
			// worse consequences: Pod being restarted with the same version, and the agent
			// killing it immediately after.
			logger.Errorf("Could not manually trigger restart of this Pod because of: %s", err)
			logger.Errorf("Make sure the Pod is restarted in order for the upgrade process to continue")
		}

		// If the Pod needs to be killed, we'll wait until the Pod
		// is killed by Kubernetes, bringing the new container image
		// into play.
		var quit = make(chan struct{})
		logger.Info("Pod killed itself, waiting...")
		<-quit
	} else {
		logger.Info("Pod should not be deleted, mongod started")
	}
}

func setupLogger() *zap.SugaredLogger {
	log, err := zap.NewDevelopment()
	if err != nil {
		zap.S().Errorf("Error building logger config: %s", err)
		os.Exit(1)
	}

	return log.Sugar()
}

// waitForAgentHealthStatus will poll the health status file and wait for it to be updated.
// The agent doesn't write the plan to the file right away and hence we need to wait for the
// latest plan to be written.
func waitForAgentHealthStatus() (agent.Health, error) {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	totalTime := time.Duration(0)
	for range ticker.C {
		if totalTime > pollingDuration {
			break
		}
		totalTime += pollingInterval

		health, err := getAgentHealthStatus()
		if err != nil {
			return agent.Health{}, err
		}

		status, ok := health.Healthiness[getHostname()]
		if !ok {
			return agent.Health{}, fmt.Errorf("couldn't find status for hostname %s", getHostname())
		}

		// We determine if the file has been updated by checking if the process is not in goal state.
		// As the agent is currently executing a plan, the process should not be in goal state.
		if !status.IsInGoalState {
			return health, nil
		}
	}
	return agent.Health{}, fmt.Errorf("agent health status not ready after waiting %s", pollingDuration.String())

}

// getAgentHealthStatus returns an instance of agent.Health read
// from the health file on disk
func getAgentHealthStatus() (agent.Health, error) {
	f, err := os.Open(os.Getenv(agentStatusFilePathEnv))
	if err != nil {
		return agent.Health{}, err
	}
	defer f.Close()

	h, err := readAgentHealthStatus(f)
	if err != nil {
		return agent.Health{}, fmt.Errorf("could not read health status file: %s", err)
	}
	return h, err
}

// readAgentHealthStatus reads an instance of health.Health from the provided
// io.Reader
func readAgentHealthStatus(reader io.Reader) (agent.Health, error) {
	var h agent.Health
	data, err := io.ReadAll(reader)
	if err != nil {
		return h, err
	}
	err = json.Unmarshal(data, &h)
	return h, err
}

func getHostname() string {
	return os.Getenv("HOSTNAME")
}

// shouldDeletePod returns a boolean value indicating if this pod should be deleted
// this would be the case if the agent is currently trying to upgrade the version
// of mongodb.
func shouldDeletePod(health agent.Health) (bool, error) {
	status, ok := health.ProcessPlans[getHostname()]
	if !ok {
		return false, fmt.Errorf("hostname %s was not in the process plans", getHostname())
	}
	return isWaitingToBeDeleted(status), nil
}

// isWaitingToBeDeleted determines if the agent is currently waiting
// on the mongod pod to be restarted. In order to do this, we need to check the agent
// status file and determine if the mongod has been stopped and if we are in the process
// of a version change.
func isWaitingToBeDeleted(healthStatus agent.MmsDirectorStatus) bool {
	if len(healthStatus.Plans) == 0 {
		return false
	}
	lastPlan := healthStatus.Plans[len(healthStatus.Plans)-1]
	for _, m := range lastPlan.Moves {
		// When changing version the plan will contain a "ChangeVersion" step
		if m.Move == "ChangeVersion" {
			return true
		}
	}
	return false
}

// deletePod attempts to delete the pod this mongod is running in
func deletePod(ctx context.Context) error {
	thisPod, err := getThisPod()
	if err != nil {
		return fmt.Errorf("could not get pod: %s", err)
	}
	k8sClient, err := inClusterClient()
	if err != nil {
		return fmt.Errorf("could not get client: %s", err)
	}

	if err := k8sClient.Delete(ctx, &thisPod); err != nil {
		return fmt.Errorf("could not delete pod: %s", err)
	}
	return nil
}

// getThisPod returns an instance of corev1.Pod that points to the current pod
func getThisPod() (corev1.Pod, error) {
	podName := getHostname()
	if podName == "" {
		return corev1.Pod{}, fmt.Errorf("environment variable HOSTNAME was not present")
	}

	ns, err := getNamespace()
	if err != nil {
		return corev1.Pod{}, fmt.Errorf("could not read namespace: %s", err)
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
	}, nil
}

func inClusterClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get cluster config: %s", err)
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("could not create client: %s", err)
	}
	return k8sClient, nil
}

func getNamespace() (string, error) {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
		return ns, nil
	}
	return defaultNamespace, nil
}
