package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/cmd/readiness/testdata"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/health"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
)

// TestDeadlockDetection verifies that if the agent is stuck in "WaitAllRsMembersUp" phase (started > 15 seconds ago)
// then the function returns "ready"
func TestDeadlockDetection(t *testing.T) {
	ctx := context.Background()
	type TestConfig struct {
		conf            config.Config
		isErrorExpected bool
		isReadyExpected bool
	}
	tests := map[string]TestConfig{
		"Ready but deadlocked on WaitAllRsMembersUp": {
			conf:            testConfig("testdata/health-status-deadlocked.json"),
			isReadyExpected: true,
		},
		"Ready but deadlocked on WaitCanUpdate while changing the versions with multiple plans": {
			conf:            testConfig("testdata/health-status-deadlocked-with-prev-config.json"),
			isReadyExpected: true,
		},
		"Ready but deadlocked on WaitHasCorrectAutomationCredentials (HELP-39937, HELP-39966)": {
			conf:            testConfig("testdata/health-status-deadlocked-waiting-for-correct-automation-credentials.json"),
			isReadyExpected: true,
		},
		"Ready and no deadlock detected": {
			conf:            testConfig("testdata/health-status-no-deadlock.json"),
			isReadyExpected: true,
		},
		"Ready and positive scenario": {
			conf:            testConfig("testdata/health-status-ok.json"),
			isReadyExpected: true,
		},
		"Ready and Pod readiness is correctly checked when no ReplicationStatus is present on the file": {
			conf:            testConfig("testdata/health-status-no-replication.json"),
			isReadyExpected: true,
		},
		"Ready and MongoDB replication state is reported by agents": {
			conf:            testConfig("testdata/health-status-ok-no-replica-status.json"),
			isReadyExpected: true,
		},
		"Not Ready If replication state is not PRIMARY or SECONDARY, Pod is not ready": {
			conf:            testConfig("testdata/health-status-not-readable-state.json"),
			isReadyExpected: false,
		},
		"Not Ready because of less than 15 seconds passed by after the health file update": {
			conf:            testConfig("testdata/health-status-pending.json"),
			isReadyExpected: false,
		},
		"Not Ready because there are no plans": {
			conf:            testConfig("testdata/health-status-no-plans.json"),
			isReadyExpected: false,
		},
		"Not Ready because there are no statuses": {
			conf:            testConfig("testdata/health-status-no-plans.json"),
			isReadyExpected: false,
		},
		"Not Ready because there are no processes": {
			conf:            testConfig("testdata/health-status-no-processes.json"),
			isReadyExpected: false,
		},
		"Not Ready because mongod is down for 90 seconds": {
			conf:            testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*90),
			isReadyExpected: false,
		},
		"Not Ready because mongod is down for 1 hour": {
			conf:            testConfigWithMongoUp("testdata/health-status-ok.json", time.Hour*1),
			isReadyExpected: false,
		},
		"Not Ready because mongod is down for 2 days": {
			conf:            testConfigWithMongoUp("testdata/health-status-ok.json", time.Hour*48),
			isReadyExpected: false,
		},
		"Ready and mongod is up for 30 seconds": {
			conf:            testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*30),
			isReadyExpected: true,
		},
		"Ready and mongod is up for 1 second": {
			conf:            testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*30),
			isReadyExpected: true,
		},
		"Not Ready because of mongod bootstrap errors": {
			conf:            testConfigWithMongoUp("testdata/health-status-error-tls.json", time.Second*30),
			isReadyExpected: false,
		},
		"Not Ready because of waiting on an upgrade start in a recomputed plan (a real scenario for an interrupted start in EA)": {
			conf:            testConfigWithMongoUp("testdata/health-status-enterprise-upgrade-interrupted.json", time.Second*30),
			isReadyExpected: false,
		},
	}
	for testName := range tests {
		testConfig := tests[testName]
		t.Run(testName, func(t *testing.T) {
			ready, err := isPodReady(ctx, testConfig.conf)
			if testConfig.isErrorExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testConfig.isReadyExpected, ready)
		})
	}
}

func TestObtainingCurrentStep(t *testing.T) {
	noDeadlockHealthExample, _ := parseHealthStatus(testConfig("testdata/health-status-no-deadlock.json").HealthStatusReader)
	now := time.Now()
	tenMinutesAgo := time.Now().Add(-time.Minute * 10)

	type TestConfig struct {
		processStatuses map[string]health.MmsDirectorStatus
		expectedStep    string
	}
	tests := map[string]TestConfig{
		"No deadlock example should point to WaitFeatureCompatibilityVersionCorrect": {
			processStatuses: noDeadlockHealthExample.MmsStatus,
			expectedStep:    "WaitFeatureCompatibilityVersionCorrect",
		},
		"Find single Started Step": {
			processStatuses: map[string]health.MmsDirectorStatus{
				"ignore": {
					Plans: []*health.PlanStatus{
						{
							Moves: []*health.MoveStatus{
								{
									Steps: []*health.StepStatus{
										{
											Step:      "will be ignored as completed",
											Started:   &tenMinutesAgo,
											Completed: &now,
										},
										{
											Step:    "test",
											Started: &tenMinutesAgo,
										},
										{
											Step:      "will be ignored as completed",
											Started:   &tenMinutesAgo,
											Completed: &now,
										},
									},
								},
							},
							Started: &tenMinutesAgo,
						},
					},
				},
			},
			expectedStep: "test",
		},
		"Find no Step in completed plan": {
			processStatuses: map[string]health.MmsDirectorStatus{
				"ignore": {
					Plans: []*health.PlanStatus{
						{
							Moves: []*health.MoveStatus{
								{
									Steps: []*health.StepStatus{
										{
											Step:    "test",
											Started: &tenMinutesAgo,
										},
									},
								},
							},
							Started:   &tenMinutesAgo,
							Completed: &now,
						},
					},
				},
			},
			expectedStep: "",
		},
		"Find single Started step in the latest plan only": {
			processStatuses: map[string]health.MmsDirectorStatus{
				"ignore": {
					Plans: []*health.PlanStatus{
						{
							Moves: []*health.MoveStatus{
								{
									Steps: []*health.StepStatus{
										{
											Step:    "will be ignored as only the last plan is evaluated",
											Started: &tenMinutesAgo,
										},
									},
								},
							},
							Started: &tenMinutesAgo,
						},
						{
							Moves: []*health.MoveStatus{
								{
									Steps: []*health.StepStatus{
										{
											Step:    "test",
											Started: &tenMinutesAgo,
										},
									},
								},
							},
							Started: &tenMinutesAgo,
						},
					},
				},
			},
			expectedStep: "test",
		},
	}
	for testName := range tests {
		testConfig := tests[testName]
		t.Run(testName, func(t *testing.T) {
			step := findCurrentStep(testConfig.processStatuses)
			if len(testConfig.expectedStep) == 0 {
				assert.Nil(t, step)
			} else {
				assert.Equal(t, testConfig.expectedStep, step.Step)
			}
		})
	}
}

// TestReadyWithWaitForCorrectBinaries tests the Static Containers Architecture mode for the Agent.
// In this case, the Readiness Probe needs to return Ready and let the StatefulSet Controller to proceed
// with the Pod rollout.
func TestReadyWithWaitForCorrectBinaries(t *testing.T) {
	ctx := context.Background()
	c := testConfigWithMongoUp("testdata/health-status-ok-with-WaitForCorrectBinaries.json", time.Second*30)
	ready, err := isPodReady(ctx, c)

	assert.True(t, ready)
	assert.NoError(t, err)
}

// TestHeadlessAgentHasntReachedGoal verifies that the probe reports "false" if the config version is higher than the
// last achieved version of the Agent
// Note that the edge case is checked here: the health-status-ok.json has the "WaitRsInit" phase stuck in the last plan
// (as Agent doesn't marks all the step statuses finished when it reaches the goal) but this doesn't affect the result
// as the whole plan is complete already
func TestHeadlessAgentHasntReachedGoal(t *testing.T) {
	ctx := context.Background()
	t.Setenv(headlessAgent, "true")
	c := testConfig("testdata/health-status-ok.json")
	c.ClientSet = fake.NewSimpleClientset(testdata.TestPod(c.Namespace, c.Hostname), testdata.TestSecret(c.Namespace, c.AutomationConfigSecretName, 6))
	ready, err := isPodReady(ctx, c)
	assert.False(t, ready)
	assert.NoError(t, err)
	thePod, _ := c.ClientSet.CoreV1().Pods(c.Namespace).Get(ctx, c.Hostname, metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "5"}, thePod.Annotations)
}

// TestHeadlessAgentReachedGoal verifies that the probe reports "true" if the config version is equal to the
// last achieved version of the Agent
func TestHeadlessAgentReachedGoal(t *testing.T) {
	ctx := context.Background()
	t.Setenv(headlessAgent, "true")
	c := testConfig("testdata/health-status-ok.json")
	c.ClientSet = fake.NewSimpleClientset(testdata.TestPod(c.Namespace, c.Hostname), testdata.TestSecret(c.Namespace, c.AutomationConfigSecretName, 5))
	ready, err := isPodReady(ctx, c)
	assert.True(t, ready)
	assert.NoError(t, err)
	thePod, _ := c.ClientSet.CoreV1().Pods(c.Namespace).Get(ctx, c.Hostname, metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "5"}, thePod.Annotations)
}

func testConfig(healthFilePath string) config.Config {
	return testConfigWithMongoUp(healthFilePath, 15*time.Second)
}

func testConfigWithMongoUp(healthFilePath string, timeSinceMongoLastUp time.Duration) config.Config {
	file, err := os.Open(healthFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	status, err := parseHealthStatus(file)
	if err != nil {
		panic(err)
	}

	for key, processHealth := range status.Statuses {
		processHealth.LastMongoUpTime = time.Now().Add(-timeSinceMongoLastUp).Unix()
		// Need to reassign the object back to map as 'processHealth' is a copy of the struct
		status.Statuses[key] = processHealth
	}

	return config.Config{
		HealthStatusReader:         NewTestHealthStatusReader(status),
		Namespace:                  "test-ns",
		AutomationConfigSecretName: "test-mongodb-automation-config",
		Hostname:                   "test-mongodb-0",
	}
}

func NewTestHealthStatusReader(status health.Status) io.Reader {
	data, err := json.Marshal(status)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}
