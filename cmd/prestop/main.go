package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agenthealth"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var logger *zap.SugaredLogger

const (
	agentStatusFilePath = "AGENT_STATUS_FILEPATH"
)

func getNamespace() (string, error) {
	data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
		return ns, nil
	}
	return "default", nil
}

// deletePod attempts to delete the pod this mongod is running in
func deletePod() error {
	thisPod, err := getThisPod()
	if err != nil {
		return fmt.Errorf("error getting this pod: %v", err)
	}
	k8sClient, err := inClusterClient()
	if err != nil {
		return fmt.Errorf("error getting client: %v", err)
	}

	if err := k8sClient.Delete(context.TODO(), &thisPod); err != nil {
		return fmt.Errorf("error deleting pod: %v", err)
	}
	return nil
}

func inClusterClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster config: %+v", err)
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %+v", err)
	}
	return k8sClient, nil
}

func prettyPrint(i interface{}) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	zap.S().Info(string(b))
}

// shouldDeletePod returns a boolean  value indicating if this pod should be deleted
// this would be the case if the agent is currently trying to upgrade the version
// of mongodb.
func shouldDeletePod() (bool, error) {
	f, err := os.Open(os.Getenv(agentStatusFilePath))
	if err != nil {
		return false, fmt.Errorf("error opening file: %s", err)
	}
	defer f.Close()

	h, err := readAgentHealthStatus(f)
	prettyPrint(h)
	if err != nil {
		return false, fmt.Errorf("error reading agent health status: %s", err)
	}
	hostname := os.Getenv("HOSTNAME")
	status, ok := h.ProcessPlans[hostname]
	if !ok {
		return false, fmt.Errorf("hostname %s was not in the process plans", hostname)
	}

	return isWaitingToBeDeleted(status), nil
}

// agentRequiresPodDeletionToContinue determines if the agent is currently waiting
// on the mongod pod to be restarted. In order to do this, we need to check the agent
// status file and determine if the mongod has been stopped and if we are in the process
// of a version change.
func agentRequiresPodDeletionToContinue(h agenthealth.Health) bool {
	for _, plan := range h.ProcessPlans {
		if len(plan.Plans) == 0 {
			return false
		}
		lastPlan := plan.Plans[len(plan.Plans)-1]
		for _, m := range lastPlan.Moves {
			if changingVersionMove := m.Move == "ChangeVersion"; !changingVersionMove {
				continue
			}
			for _, s := range m.Steps {
				successfullyStoppedMongod := s.Step == "Stop" && s.Completed != nil && s.Result == "success"
				if successfullyStoppedMongod {
					return true
				}
			}
		}
		return false
	}
	return true
}

// getThisPod returns an instance of corev1.Pod that points to the current pod
func getThisPod() (corev1.Pod, error) {
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		return corev1.Pod{}, fmt.Errorf("environment variable HOSTNAME was not present")
	}

	ns, err := getNamespace()
	if err != nil {
		return corev1.Pod{}, fmt.Errorf("error reading namespace: %+v", err)
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
	}, nil
}

// readAgentHealthStatus reads an instance of health.Health from the provided
// io.Reader
func readAgentHealthStatus(reader io.Reader) (agenthealth.Health, error) {
	var h agenthealth.Health
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return h, err
	}
	err = json.Unmarshal(data, &h)
	return h, err
}

func main() {
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = []string{
		"/hooks/pre-hook.log",
	}
	log, err := cfg.Build()
	if err != nil {
		zap.S().Errorf("Error building logger config: %v", err)
		os.Exit(1)
	}
	logger = log.Sugar()

	if filePath := os.Getenv(agentStatusFilePath); filePath == "" {
		logger.Fatal("Environment variable: %s must be set!", agentStatusFilePath)
		os.Exit(1)
	}
	shouldDelete, err := shouldDeletePod()
	logger.Debugf("shouldDeletePod=%t", shouldDelete)
	if err != nil {
		logger.Errorf("Error in shouldDeletePod: %s", err)
		os.Exit(1)
	}

	if !shouldDelete {
		os.Exit(0)
	}

	if err := deletePod(); err != nil {
		logger.Errorf("Error deleting pod: %s", err)
		os.Exit(1)
	}
}

// agentRequiresPodDeletionToContinue determines if the agent is currently waiting
// on the mongod pod to be restarted. In order to do this, we need to check the agent
// status file and determine if the mongod has been stopped and if we are in the process
// of a version change.
func isWaitingToBeDeleted(healthStatus agenthealth.MmsDirectorStatus) bool {
	if len(healthStatus.Plans) == 0 {
		return false
	}
	lastPlan := healthStatus.Plans[len(healthStatus.Plans)-1]
	for _, m := range lastPlan.Moves {
		if changingVersionMove := m.Move == "ChangeVersion"; !changingVersionMove {
			continue
		}

		for _, s := range m.Steps {
			isStopStep := s.Step == "Stop"
			completedSuccessfully := s.Completed != nil && s.Result == "success"

			// if we have stopped successfully, this means we are waiting for the mongod to be terminated
			if isStopStep && completedSuccessfully {
				return true
			}
		}
	}
	return false
}
