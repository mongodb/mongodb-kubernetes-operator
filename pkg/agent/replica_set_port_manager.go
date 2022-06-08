package agent

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// ReplicaSetPortManager is used to determine which ports should be set in pods (mongod processes) and in the service.
// When the replica set is initially configured (no pods/processes are created yet), it works by simply setting desired port to all processes.
// When the replica set is created and running, it orchestrates port changes as mongodb-agent does not allow changing ports in more than one process at a time.
// For the running cluster, it changes ports only when all pods reached goal state and changes the ports one by one.
// For the whole process of port change, the service has both port exposed: old and new. After it finishes, only the new port is in the service.
type ReplicaSetPortManager struct {
	log                *zap.SugaredLogger
	expectedPort       int
	currentPodStates   []PodState
	currentACProcesses []automationconfig.Process
}

func NewReplicaSetPortManager(log *zap.SugaredLogger, expectedPort int, currentPodStates []PodState, currentACProcesses []automationconfig.Process) *ReplicaSetPortManager {
	return &ReplicaSetPortManager{log: log, expectedPort: expectedPort, currentPodStates: currentPodStates, currentACProcesses: currentACProcesses}
}

// GetPortsModification returns automation config modification function to be used in config builder.
// It calculates which ports are needed to be set in current reconcile process.
// For the pods, which are not created yet, it sets desired port immediately.
// For the pods, which are created and with its goal reached, it changes only one port at a time to allow
// agent to change port in one process at a time.
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

// GetServicePorts returns an array of corev1.ServicePort to be set in the service.
// If there is no port change in progress, it returns expectedPort named "mongodb".
// If there is port change in progress, then it returns both ports: old named "mongodb" and new named "mongodb-new".
// When the port change is finished, it falls back to the first case (no port change in progress) and "mongodb-new" will be renamed to "mongodb".
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

func (r *ReplicaSetPortManager) getProcessByName(name string) *automationconfig.Process {
	for i := 0; i < len(r.currentACProcesses); i++ {
		if r.currentACProcesses[i].Name == name {
			return &r.currentACProcesses[i]
		}
	}

	return nil
}

func (r *ReplicaSetPortManager) calculateExpectedPorts() (map[string]int, bool, int) {
	portMap := map[string]int{}

	// populate portMap with ports
	for _, podState := range r.currentPodStates {
		process := r.getProcessByName(podState.PodName.Name)
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
