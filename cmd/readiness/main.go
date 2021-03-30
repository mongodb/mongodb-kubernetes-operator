package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/headless"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.uber.org/zap"
)

const (
	headlessAgent = "HEADLESS_AGENT"
)

var riskySteps []string
var logger *zap.SugaredLogger

func init() {
	riskySteps = []string{"WaitAllRsMembersUp", "WaitRsInit"}

	// By default we log to the output (convenient for tests)
	cfg := zap.NewDevelopmentConfig()
	log, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	logger = log.Sugar()
}

// isPodReady main function which makes decision if the pod is ready or not. The decision is based on the information
// from the AA health status file.
// The logic depends on if the pod is a standard MongoDB or an AppDB one.
// - If MongoDB: then just the 'statuses[0].IsInGoalState` field is used to learn if the Agent has reached the goal
// - if AppDB: the 'mmsStatus[0].lastGoalVersionAchieved' field is compared with the one from mounted automation config
// Additionally if the previous check hasn't returned 'true' the "deadlock" case is checked to make sure the Agent is
// not waiting for the other members.
func isPodReady(conf config.Config) bool {
	fd, err := os.Open(conf.HealthStatusFilePath)
	if err != nil {
		logger.Warn("No health status file exists, assuming the Automation agent is old")
		return true
	}
	defer fd.Close()

	health, err := readAgentHealthStatus(fd)
	if err != nil {
		logger.Errorf("Failed to read agent health status file: %s", err)
		// panicking allows to see the problem in the events for the pod (kubectl describe pod ..)
		panic("Failed to read agent health status file: %s")
	}

	// The 'statuses' file can be empty only for OM Agents
	if len(health.Healthiness) == 0 && !isHeadlessMode() {
		logger.Info("'statuses' is empty. We assume there is no automation config for the agent yet.")
		return true
	}

	// If the agent has reached the goal state - returning true
	inGoalState, err := isInGoalState(health, conf)
	if err != nil {
		logger.Errorf("There was problem checking the health status: %s", err)
		panic(err)
	}

	inReadyState := isInReadyState(health)
	if inGoalState && inReadyState {
		logger.Info("Agent has reached goal state")
		return true
	}

	// Failback logic: the agent is not in goal state and got stuck in some steps
	if hasDeadlockedSteps(health) {
		return true
	}

	return false
}

func readAgentHealthStatus(file *os.File) (health.Status, error) {
	var health health.Status

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return health, err
	}

	err = json.Unmarshal(data, &health)
	return health, err
}

// hasDeadlockedSteps returns true if the agent is stuck on waiting for the other agents
func hasDeadlockedSteps(health health.Status) bool {
	currentStep := findCurrentStep(health.ProcessPlans)
	if currentStep != nil {
		return isDeadlocked(currentStep)
	}
	return false
}

// findCurrentStep returns the step which seems to be run by the Agent now. The step is always in the last plan
// (see https://github.com/10gen/ops-manager-kubernetes/pull/401#discussion_r333071555) so we iterate over all the steps
// there and find the last step which has "Started" non nil
// (indeed this is not the perfect logic as sometimes the agent doesn't update the 'Started' as well - see
// 'health-status-ok.json', but seems it works for finding deadlocks still
//noinspection GoNilness
func findCurrentStep(processStatuses map[string]health.MmsDirectorStatus) *health.StepStatus {
	var currentPlan *health.PlanStatus
	if len(processStatuses) == 0 {
		// Seems shouldn't happen but let's check anyway - may be needs to be changed to Info if this happens
		logger.Warnf("There is no information about Agent process plans")
		return nil
	}
	if len(processStatuses) > 1 {
		logger.Errorf("Only one process status is expected but got %d!", len(processStatuses))
		return nil
	}
	// There is always only one process managed by the Agent - so there will be only one loop
	for k, v := range processStatuses {
		if len(v.Plans) == 0 {
			logger.Errorf("The process %s doesn't contain any plans!", k)
			return nil
		}
		currentPlan = v.Plans[len(v.Plans)-1]
	}

	if currentPlan.Completed != nil {
		logger.Debugf("The Agent hasn't reported working on the new config yet, the last plan finished at %s",
			currentPlan.Completed.Format(time.RFC3339))
		return nil
	}

	var lastStartedStep *health.StepStatus
	for _, m := range currentPlan.Moves {
		for _, s := range m.Steps {
			if s.Started != nil {
				lastStartedStep = s
			}
		}
	}

	return lastStartedStep
}

func isDeadlocked(status *health.StepStatus) bool {
	// Some logic behind 15 seconds: the health status file is dumped each 10 seconds so we are sure that if the agent
	// has been in the the step for 10 seconds - this means it is waiting for the other hosts and they are not available
	fifteenSecondsAgo := time.Now().Add(time.Duration(-15) * time.Second)
	if contains.String(riskySteps, status.Step) && status.Completed == nil && status.Started.Before(fifteenSecondsAgo) {
		logger.Infof("Indicated a possible deadlock, status: %s, started at %s but hasn't finished "+
			"yet. Marking the probe as ready", status.Step, status.Started.Format(time.RFC3339))
		return true
	}
	return false
}

func isInGoalState(health health.Status, conf config.Config) (bool, error) {
	if isHeadlessMode() {
		return headless.PerformCheckHeadlessMode(health, conf)
	}
	return performCheckOMMode(health), nil
}

// performCheckOMMode does a general check if the Agent has reached the goal state - must be called when Agent is in
// "OM mode"
func performCheckOMMode(health health.Status) bool {
	for _, v := range health.Healthiness {
		logger.Debug(v)
		if v.IsInGoalState {
			return true
		}
	}
	return false
}

func isHeadlessMode() bool {
	return os.Getenv(headlessAgent) == "true"
}

func kubernetesClientset() (kubernetes.Interface, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in cluster config: %s", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %s", err)
	}
	return clientset, nil
}

func main() {
	clientSet, err := kubernetesClientset()
	if err != nil {
		panic(err)
	}

	config, err := config.BuildFromEnvVariables(clientSet, isHeadlessMode())
	if err != nil {
		panic(err)
	}
	cfg := zap.NewDevelopmentConfig()
	// In production we log to the file
	cfg.OutputPaths = []string{
		config.LogFilePath,
	}
	log, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	logger = log.Sugar()
	if !isPodReady(config) {
		os.Exit(1)
	}
}

// isInReadyState checks the MongoDB Server state. It returns true if the state
// is PRIMARY or SECONDARY.
// This function will always return true if the agent doesn't publish this state.
func isInReadyState(health health.Status) bool {
	if len(health.Healthiness) == 0 {
		return true
	}
	for _, processHealth := range health.Healthiness {
		// We know this loop should run only once, in Kubernetes there's
		// only 1 server managed per host.
		if processHealth.ReplicaStatus == nil {
			// We always return true if the Agent does not publish mongodb
			// server state
			return true
		}

		if mongoDbServerHasStarted(health) {
			// There should be only one entry reported for this Pod.
			return processHealth.IsReadyState()
		}
	}
	return false
}

// mongoDbServerHasStarted checks if the current plan includes a Move and a Step
// of type "StartFresh" with a Result of "success".
//
// This function will return true if the agent has been able to successfully
// start the MongoDB server.
func mongoDbServerHasStarted(health health.Status) bool {
	plan := findCurrentPlan(health.ProcessPlans)
	if plan == nil {
		return false
	}

	for _, move := range plan.Moves {
		for _, step := range move.Steps {
			if step.Step == "StartFresh" && step.Result == "success" {
				return true
			}
		}
	}

	return false
}

// findCurrentPlan returns the current plan as informed by the Agent.
//
// The current plan is the last plan from the `processStatuses` parameter, this
// is, the plan that's currently being processed by the agent.
func findCurrentPlan(processStatuses map[string]health.MmsDirectorStatus) *health.PlanStatus {
	var currentPlan *health.PlanStatus
	if len(processStatuses) == 0 {
		// Seems shouldn't happen but let's check anyway - may be needs to be
		// changed to Info if this happens.
		logger.Warnf("There is no information about Agent process plans")
		return nil
	}
	if len(processStatuses) > 1 {
		logger.Errorf("Only one process status is expected but got %d!", len(processStatuses))
		return nil
	}
	// There is only one process managed by the Agent - so will only check one
	// iteration.
	for k, v := range processStatuses {
		if len(v.Plans) == 0 {
			logger.Errorf("The process %s doesn't contain any plans!", k)
			return nil
		}
		currentPlan = v.Plans[len(v.Plans)-1]
	}

	return currentPlan
}
