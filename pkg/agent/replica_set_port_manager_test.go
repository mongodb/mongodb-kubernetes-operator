package agent

import (
	"fmt"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

func TestReplicaSetPortManagerCalculateExpectedPorts(t *testing.T) {
	type input struct {
		currentPodStates []PodState
		expectedPort     int
		currentAC        automationconfig.AutomationConfig
	}

	type output struct {
		portMap            map[string]int
		portChangeRequired bool
		oldPort            int
	}

	type testCase struct {
		in             input
		expectedOutput output
	}

	name := "mdb"
	podName := func(i int) types.NamespacedName {
		return types.NamespacedName{Namespace: "mongodb", Name: fmt.Sprintf("%s-%d", name, i)}
	}
	arbiterPodName := func(i int) types.NamespacedName {
		return types.NamespacedName{Namespace: "mongodb", Name: fmt.Sprintf("%s-arb-%d", name, i)}
	}

	generateConfig := func(ports []int, arbiterPorts []int) automationconfig.AutomationConfig {
		builder := automationconfig.NewBuilder()
		builder.SetMembers(len(ports))
		builder.SetArbiters(len(arbiterPorts))
		builder.SetName(name)
		builder.AddProcessModification(func(i int, process *automationconfig.Process) {
			if i < len(ports) {
				process.SetPort(ports[i])
			}
		})
		ac, err := builder.Build()
		require.NoError(t, err)
		return ac
	}

	generatePortMap := func(ports []int, arbiterPorts []int) map[string]int {
		portMap := map[string]int{}
		for i, port := range ports {
			portMap[podName(i).Name] = port
		}
		for i, port := range arbiterPorts {
			portMap[arbiterPodName(i+len(ports)).Name] = port
		}
		return portMap
	}

	testCases := map[string]testCase{
		"No ports are changed if there is existing config and pods are not ready": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: false, ReachedGoalState: false},
					{PodName: podName(1), Found: false, ReachedGoalState: false},
					{PodName: podName(2), Found: false, ReachedGoalState: false},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{1000, 1000, 1000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{1000, 1000, 1000}, nil),
				portChangeRequired: true,
				oldPort:            1000,
			},
		},
		"No ports are changed when not all pods reached goal state": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: false},
					{PodName: podName(2), Found: true, ReachedGoalState: true},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{1000, 1000, 1000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{1000, 1000, 1000}, nil),
				portChangeRequired: true,
				oldPort:            1000,
			},
		},
		"All ports set to expected when there are no processes in config yet": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: false},
					{PodName: podName(1), Found: true, ReachedGoalState: false},
					{PodName: podName(2), Found: true, ReachedGoalState: false},
				},
				expectedPort: 2000,
				currentAC:    generateConfig(nil, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 2000, 2000}, nil),
				portChangeRequired: false,
				oldPort:            2000,
			},
		},
		"Only one port changed when all pods are ready": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: true},
					{PodName: podName(2), Found: true, ReachedGoalState: true},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{1000, 1000, 1000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 1000, 1000}, nil),
				portChangeRequired: true,
				oldPort:            1000,
			},
		},
		"No port changes required when all ports changed but not all pods reached goal state": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: true},
					{PodName: podName(2), Found: true, ReachedGoalState: false},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{2000, 2000, 2000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 2000, 2000}, nil),
				portChangeRequired: false,
				oldPort:            2000,
			},
		},
		"No port changes required when all ports changed but only arbiter is not in a goal state": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: true},
					{PodName: podName(2), Found: true, ReachedGoalState: true},
					{PodName: arbiterPodName(3), Found: true, ReachedGoalState: true},
					{PodName: arbiterPodName(4), Found: true, ReachedGoalState: false},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{2000, 2000, 2000}, []int{2000, 2000}),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 2000, 2000}, []int{2000, 2000}),
				portChangeRequired: false,
				oldPort:            2000,
			},
		},
		"No port changes when scaling down and there are more processes in config than current pod states": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: true},
					{PodName: podName(2), Found: true, ReachedGoalState: true},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{2000, 2000, 2000, 2000, 2000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 2000, 2000}, nil),
				portChangeRequired: false,
				oldPort:            2000,
			},
		},
		"No port changes when scaling up and there are less processes in config than current pod states": {
			in: input{
				currentPodStates: []PodState{
					{PodName: podName(0), Found: true, ReachedGoalState: true},
					{PodName: podName(1), Found: true, ReachedGoalState: true},
					{PodName: podName(2), Found: true, ReachedGoalState: true},
					{PodName: podName(3), Found: false, ReachedGoalState: false},
				},
				expectedPort: 2000,
				currentAC:    generateConfig([]int{2000, 2000, 2000}, nil),
			},
			expectedOutput: output{
				portMap:            generatePortMap([]int{2000, 2000, 2000, 2000}, nil),
				portChangeRequired: false,
				oldPort:            2000,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			portManager := NewReplicaSetPortManager(zap.S(), tc.in.expectedPort, tc.in.currentPodStates, tc.in.currentAC.Processes)
			portMap, portChangeRequired, oldPort := portManager.calculateExpectedPorts()
			actualOutput := output{
				portMap:            portMap,
				portChangeRequired: portChangeRequired,
				oldPort:            oldPort,
			}
			assert.Equal(t, tc.expectedOutput, actualOutput)
		})
	}

}
