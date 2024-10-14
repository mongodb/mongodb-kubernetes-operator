package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/headless"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.uber.org/zap"
)

const (
	headlessAgent                 = "HEADLESS_AGENT"
	mongodNotReadyIntervalMinutes = time.Minute * 1
)

var logger *zap.SugaredLogger

func init() {
	// By default, we log to the output (convenient for tests)
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
// Additionally if the previous check hasn't returned 'true' an additional check for wait steps is being performed
func isPodReady(ctx context.Context, conf config.Config) (bool, error) {
	healthStatus, err := parseHealthStatus(conf.HealthStatusReader)
	if err != nil {
		logger.Errorf("There was problem parsing health status file: %s", err)
		return false, nil
	}

	// The 'statuses' file can be empty only for OM Agents
	if len(healthStatus.Statuses) == 0 && !isHeadlessMode() {
		logger.Debug("'statuses' is empty. We assume there is no automation config for the agent yet. Returning ready.")
		return true, nil
	}

	// If the agent has reached the goal state
	inGoalState, err := isInGoalState(ctx, healthStatus, conf)
	if err != nil {
		logger.Errorf("There was problem checking the health status: %s", err)
		return false, err
	}

	inReadyState := isInReadyState(healthStatus)
	if !inReadyState {
		logger.Info("Mongod is not ready")
	}

	if inGoalState && inReadyState {
		logger.Info("The Agent has reached goal state. Returning ready.")
		return true, nil
	}

	// Fallback logic: the agent is not in goal state and got stuck in some steps
	if !inGoalState && isOnWaitingStep(healthStatus) {
		logger.Info("The Agent is on wait Step. Returning ready.")
		return true, nil
	}

	logger.Info("Reached the end of the check. Returning not ready.")
	return false, nil
}

// isOnWaitingStep returns true if the agent is stuck on waiting for the other Agents or something else to happen.
func isOnWaitingStep(health health.Status) bool {
	currentStep := findCurrentStep(health.MmsStatus)
	if currentStep != nil {
		return isWaitStep(currentStep)
	}
	return false
}

// findCurrentStep returns the step which the Agent is working now.
// The algorithm (described in https://github.com/10gen/ops-manager-kubernetes/pull/401#discussion_r333071555):
//   - Obtain the latest plan (the last one in the plans array)
//   - Find the last step, which has Started not nil and Completed nil. The Steps are processed as a tree in a BFS fashion.
//     The last element is very likely to be the Step the Agent is performing at the moment. There are some chances that
//     this is a waiting step, use isWaitStep to verify this.
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
	for processName, processStatus := range processStatuses {
		if len(processStatus.Plans) == 0 {
			logger.Errorf("The process %s doesn't contain any plans!", processName)
			return nil
		}
		currentPlan = processStatus.Plans[len(processStatus.Plans)-1]
	}

	if currentPlan.Completed != nil {
		logger.Debugf("The Agent hasn't reported working on the new config yet, the last plan finished at %s",
			currentPlan.Completed.Format(time.RFC3339))
		return nil
	}

	var lastStartedStep *health.StepStatus
	for _, m := range currentPlan.Moves {
		for _, s := range m.Steps {
			if s.Started != nil && s.Completed == nil {
				lastStartedStep = s
			}
		}
	}

	return lastStartedStep
}

// isWaitStep returns true is the Agent is currently waiting for something to happen.
//
// Most of the time, the Agent waits for an initialization by other member of the cluster. In such case,
// holding the rollout does not improve the overall system state. Even if the probe returns true too quickly
// the worst thing that can happen is a short service interruption, which is still better than full service outage.
//
// The 15 seconds explanation:
//   - The status file is written every 10s but the Agent processes steps independently of it
//   - In order to avoid reacting on a newly added wait Step (as they can naturally go away), we're giving the Agent
//     at least 15 sends to spend on that Step.
//   - This hopefully prevents the Probe from flipping False to True too quickly.
func isWaitStep(status *health.StepStatus) bool {
	// Some logic behind 15 seconds: the health status file is dumped each 10 seconds, so we are sure that if the agent
	// has been in the step for 10 seconds - this means it is waiting for the other hosts, and they are not available
	fifteenSecondsAgo := time.Now().Add(time.Duration(-15) * time.Second)
	if status.IsWaitStep && status.Completed == nil && status.Started.Before(fifteenSecondsAgo) {
		logger.Debugf("Indicated a wait Step, status: %s, started at %s but hasn't finished "+
			"yet. Marking the probe as ready", status.Step, status.Started.Format(time.RFC3339))
		return true
	}
	return false
}

func isInGoalState(ctx context.Context, health health.Status, conf config.Config) (bool, error) {
	if isHeadlessMode() {
		return headless.PerformCheckHeadlessMode(ctx, health, conf)
	}
	return performCheckOMMode(health), nil
}

// performCheckOMMode does a general check if the Agent has reached the goal state - must be called when Agent is in
// "OM mode"
func performCheckOMMode(health health.Status) bool {
	for _, v := range health.Statuses {
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

func parseHealthStatus(reader io.Reader) (health.Status, error) {
	var health health.Status
	data, err := io.ReadAll(reader)
	if err != nil {
		return health, err
	}

	err = json.Unmarshal(data, &health)
	return health, err
}

func initLogger(l *lumberjack.Logger) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	consoleCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.DebugLevel)

	cores := []zapcore.Core{consoleCore}
	if config.ReadBoolWitDefault(config.WithAgentFileLogging, "true") {
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(l),
			zap.DebugLevel)
		cores = append(cores, fileCore)
	}

	core := zapcore.NewTee(cores...)
	log := zap.New(core, zap.Development())
	logger = log.Sugar()

	logger.Infof("logging configuration: %+v", l)
}

func main() {
	ctx := context.Background()
	clientSet, err := kubernetesClientset()
	if err != nil {
		panic(err)
	}

	initLogger(config.GetLogger())

	healthStatusFilePath := config.GetEnvOrDefault(config.AgentHealthStatusFilePathEnv, config.DefaultAgentHealthStatusFilePath)
	file, err := os.Open(healthStatusFilePath)
	// The agent might be slow in creating the health status file.
	// In that case, we don't want to panic to show the message
	// in the kubernetes description. That would be a red herring, since that will solve itself with enough time.
	if err != nil {
		logger.Errorf("health status file not avaible yet: %s ", err)
		os.Exit(1)
	}

	cfg, err := config.BuildFromEnvVariables(clientSet, isHeadlessMode(), file)
	if err != nil {
		panic(err)
	}

	ready, err := isPodReady(ctx, cfg)
	if err != nil {
		panic(err)
	}
	if !ready {
		os.Exit(1)
	}
}

// isInReadyState checks the MongoDB Server state. It returns true if the mongod process is up and its state
// is PRIMARY or SECONDARY.
func isInReadyState(health health.Status) bool {
	if len(health.Statuses) == 0 {
		return true
	}
	for _, processHealth := range health.Statuses {
		// We know this loop should run only once, in Kubernetes there's
		// only 1 server managed per host.
		if !processHealth.ExpectedToBeUp {
			// Process may be down intentionally (if the process is marked as disabled in the automation config)
			return true
		}

		timeMongoUp := time.Unix(processHealth.LastMongoUpTime, 0)
		mongoUpThreshold := time.Now().Add(-mongodNotReadyIntervalMinutes)
		mongoIsHealthy := timeMongoUp.After(mongoUpThreshold)
		// The case in which the agent is too old to publish replication status is handled inside "IsReadyState"
		return mongoIsHealthy && processHealth.IsReadyState()
	}
	return false
}
