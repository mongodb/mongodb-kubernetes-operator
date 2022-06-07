package controllers

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type ReplicaSetPortManager struct {
	log              *zap.SugaredLogger
	expectedPort     int
	currentPodStates []agent.PodState
	currentAC        automationconfig.AutomationConfig
}

func NewReplicaSetPortManager(log *zap.SugaredLogger, expectedPort int, currentPodStates []agent.PodState, currentAC automationconfig.AutomationConfig) *ReplicaSetPortManager {
	return &ReplicaSetPortManager{log: log, expectedPort: expectedPort, currentPodStates: currentPodStates, currentAC: currentAC}
}

func (r *ReplicaSetPortManager) GetPortsModification() automationconfig.Modification {
	portMap, _, _ := r.calculateExpectedPorts()
	r.log.Debugf("Calculated process port map: %+v", portMap)
	return func(config *automationconfig.AutomationConfig) {
		for i := range config.Processes {
			process := config.Processes[i]
			process.SetPort(portMap[process.Name])
		}
	}
}

// GetServicePorts returns array of corev1.ServicePort.
// If there is no port change in progress it returns expectedPort named "mongodb".
// If there is port change in progress then it returns both ports: old named "mongodb" and new named "mongodb-new".
// When port change is finished, it falls back to the first case (no port change) and "mongodb-new" will be renamed to "mongodb".
func (r *ReplicaSetPortManager) GetServicePorts() []corev1.ServicePort {
	_, portChangeRequired, oldPort := r.calculateExpectedPorts()

	if !portChangeRequired || oldPort == r.expectedPort {
		return []corev1.ServicePort{{
			Port: int32(r.expectedPort),
			Name: "mongodb",
		}}
	}

	servicePorts := []corev1.ServicePort{
		{
			Port: int32(r.expectedPort),
			Name: "mongodb-new",
		},
		{
			Port: int32(oldPort),
			Name: "mongodb",
		},
	}

	r.log.Debugf("Port change in progress, setting service ports: %+v", servicePorts)

	return servicePorts
}

func (r *ReplicaSetPortManager) calculateExpectedPorts() (map[string]int, bool, int) {
	portMap := map[string]int{}

	// populate portMap with ports
	for _, podState := range r.currentPodStates {
		process := r.currentAC.GetProcessByName(podState.PodName.Name)
		if process == nil || process.GetPort() == 0 {
			// new processes are configured with correct port from the start
			portMap[podState.PodName.Name] = r.expectedPort
		} else {
			portMap[podState.PodName.Name] = process.GetPort()
		}
	}

	portChangeRequired := false
	oldPort := r.expectedPort
	for _, port := range portMap {
		if port != r.expectedPort {
			portChangeRequired = true
			oldPort = port
			break
		}
	}

	if !portChangeRequired {
		r.log.Debugf("No port change required")
		return portMap, false, oldPort
	}

	for _, podState := range r.currentPodStates {
		if !podState.ReachedGoalState {
			r.log.Debugf("Port change required but not all pods reached goal state, abandoning port change")
			return portMap, portChangeRequired, oldPort
		}
	}

	// change port only in the first eligible process as the agent cannot handle simultaneous port changes in multiple processes
	for _, podState := range r.currentPodStates {
		podName := podState.PodName.Name
		if portMap[podName] != r.expectedPort {
			r.log.Debugf("Changing port in process %s from %d to %d", podName, portMap[podName], r.expectedPort)
			portMap[podName] = r.expectedPort
			break
		}
	}

	return portMap, portChangeRequired, oldPort
}
