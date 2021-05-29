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
	ready, err := isPodReady(testConfig("testdata/health-status-deadlocked.json"))
	assert.True(t, ready)
	assert.NoError(t, err)
}

// TestNoDeadlock verifies that if the agent has started (but not finished) "WaitRsInit" and then there is another
// started phase ("WaitFeatureCompatibilityVersionCorrect") then no deadlock is found as the latter is considered to
// be the "current" step
func TestNoDeadlock(t *testing.T) {
	health, err := parseHealthStatus(testConfig("testdata/health-status-no-deadlock.json").HealthStatusReader)
	assert.NoError(t, err)
	stepStatus := findCurrentStep(health.ProcessPlans)

	assert.Equal(t, "WaitFeatureCompatibilityVersionCorrect", stepStatus.Step)

	ready, err := isPodReady(testConfig("testdata/health-status-no-deadlock.json"))
	assert.False(t, ready)
	assert.NoError(t, err)
}

// TestDeadlockDetection verifies that if the agent is in "WaitAllRsMembersUp" phase but started < 15 seconds ago
// then the function returns "not ready". To achieve this "started" is put into some long future.
// Note, that the status file is artificial: it has two plans (the first one is complete and has no moves) to make sure
// the readiness logic takes only the last plan for consideration
func TestNotReadyWaitingForRsReady(t *testing.T) {
	ready, err := isPodReady(testConfig("testdata/health-status-pending.json"))
	assert.False(t, ready)
	assert.NoError(t, err)
}

// TestNotReadyHealthFileHasNoPlans verifies that the readiness script doesn't panic if the health file has unexpected
// data (there are no plans at all)
func TestNotReadyHealthFileHasNoPlans(t *testing.T) {
	ready, err := isPodReady(testConfig("testdata/health-status-no-plans.json"))
	assert.False(t, ready)
	assert.NoError(t, err)
}

// TestNotReadyHealthFileHasNoProcesses verifies that the readiness script doesn't panic if the health file has unexpected
// data (there are no processes at all)
func TestNotReadyHealthFileHasNoProcesses(t *testing.T) {
	ready, err := isPodReady(testConfig("testdata/health-status-no-processes.json"))
	assert.False(t, ready)
	assert.NoError(t, err)
}

func TestNotReadyMongodIsDown(t *testing.T) {
	t.Run("Mongod is down for 90 seconds", func(t *testing.T) {
		ready, err := isPodReady(testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*90))
		assert.False(t, ready)
		assert.NoError(t, err)
	})
	t.Run("Mongod is down for 1 hour", func(t *testing.T) {
		ready, err := isPodReady(testConfigWithMongoUp("testdata/health-status-ok.json", time.Hour*1))
		assert.False(t, ready)
		assert.NoError(t, err)
	})
	t.Run("Mongod is down for 2 days", func(t *testing.T) {
		ready, err := isPodReady(testConfigWithMongoUp("testdata/health-status-ok.json", time.Hour*48))
		assert.False(t, ready)
		assert.NoError(t, err)
	})
}

func TestReadyMongodIsUp(t *testing.T) {
	t.Run("Mongod is down for 30 seconds", func(t *testing.T) {
		ready, err := isPodReady(testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*30))
		assert.True(t, ready)
		assert.NoError(t, err)
	})
	t.Run("Mongod is down for 1 second", func(t *testing.T) {
		ready, err := isPodReady(testConfigWithMongoUp("testdata/health-status-ok.json", time.Second*1))
		assert.True(t, ready)
		assert.NoError(t, err)
	})
}

// TestReady verifies that the probe reports "ready" despite "WaitRsInit" stage reporting as not reached
// (this is some bug in Automation Agent which we can work with)
func TestReady(t *testing.T) {
	ready, err := isPodReady(testConfig("testdata/health-status-ok.json"))
	assert.True(t, ready)
	assert.NoError(t, err)
}

// TestNoDeadlockForDownloadProcess verifies that the steps not listed as "riskySteps" (like "download") are not
// considered as stuck
func TestNoDeadlockForDownloadProcess(t *testing.T) {
	before := time.Now().Add(time.Duration(-30) * time.Second)
	downloadStatus := &health.StepStatus{
		Step:      "Download",
		Started:   &before,
		Completed: nil,
		Result:    "",
	}

	assert.False(t, isDeadlocked(downloadStatus))
}

// TestNoDeadlockForImmediateWaitRs verifies the "WaitRsInit" step is not marked as deadlocked if
// it was started < 15 seconds ago
func TestNoDeadlockForImmediateWaitRs(t *testing.T) {
	before := time.Now().Add(time.Duration(-10) * time.Second)
	downloadStatus := &health.StepStatus{
		Step:      "WaitRsInit",
		Started:   &before,
		Completed: nil,
		Result:    "Wait",
	}

	assert.False(t, isDeadlocked(downloadStatus))
}

// TestHeadlessAgentHasntReachedGoal verifies that the probe reports "false" if the config version is higher than the
// last achieved version of the Agent
// Note that the edge case is checked here: the health-status-ok.json has the "WaitRsInit" phase stuck in the last plan
// (as Agent doesn't marks all the step statuses finished when it reaches the goal) but this doesn't affect the result
// as the whole plan is complete already
func TestHeadlessAgentHasntReachedGoal(t *testing.T) {
	_ = os.Setenv(headlessAgent, "true")
	c := testConfig("testdata/health-status-ok.json")
	c.ClientSet = fake.NewSimpleClientset(testdata.TestPod(c.Namespace, c.Hostname), testdata.TestSecret(c.Namespace, c.AutomationConfigSecretName, 6))
	ready, err := isPodReady(c)
	assert.False(t, ready)
	assert.NoError(t, err)
	thePod, _ := c.ClientSet.CoreV1().Pods(c.Namespace).Get(context.TODO(), c.Hostname, metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "5"}, thePod.Annotations)

	os.Unsetenv(headlessAgent)
}

// TestHeadlessAgentReachedGoal verifies that the probe reports "true" if the config version is equal to the
// last achieved version of the Agent
func TestHeadlessAgentReachedGoal(t *testing.T) {
	_ = os.Setenv(headlessAgent, "true")
	c := testConfig("testdata/health-status-ok.json")
	c.ClientSet = fake.NewSimpleClientset(testdata.TestPod(c.Namespace, c.Hostname), testdata.TestSecret(c.Namespace, c.AutomationConfigSecretName, 5))
	ready, err := isPodReady(c)
	assert.True(t, ready)
	assert.NoError(t, err)
	thePod, _ := c.ClientSet.CoreV1().Pods(c.Namespace).Get(context.TODO(), c.Hostname, metav1.GetOptions{})
	assert.Equal(t, map[string]string{"agent.mongodb.com/version": "5"}, thePod.Annotations)

	os.Unsetenv(headlessAgent)
}

func TestPodReadiness(t *testing.T) {
	t.Run("Pod readiness is correctly checked when no ReplicationStatus is present on the file ", func(t *testing.T) {
		ready, err := isPodReady(testConfig("testdata/health-status-no-replication.json"))
		assert.True(t, ready)
		assert.NoError(t, err)
	})

	t.Run("MongoDB replication state is reported by agents", func(t *testing.T) {
		ready, err := isPodReady(testConfig("testdata/health-status-ok-no-replica-status.json"))
		assert.True(t, ready)
		assert.NoError(t, err)
	})

	t.Run("If replication state is not PRIMARY or SECONDARY, Pod is not ready", func(t *testing.T) {
		ready, err := isPodReady(testConfig("testdata/health-status-not-readable-state.json"))
		assert.False(t, ready)
		assert.NoError(t, err)
	})
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

	for key, processHealth := range status.Healthiness {
		processHealth.LastMongoUpTime = time.Now().Add(-timeSinceMongoLastUp).Unix()
		// Need to reassign the object back to map as 'processHealth' is a copy of the struct
		status.Healthiness[key] = processHealth
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
