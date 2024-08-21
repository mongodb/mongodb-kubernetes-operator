package agent

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// ReplicaSetPortManager is used to determine which ports should be set in pods (mongod processes) and in the service.
// It is used for the two use cases:
//
// * Determining the port values for the initial automation config and service when the cluster is not created yet.
//
// * Determining the next port to be changed (next stage in the process of the port change) when there is a need of changing the port of running cluster.
//
// It does not depend on K8S API, and it is deterministic for the given parameters in the NewReplicaSetPortManager constructor.
//
// When the replica set is initially configured (no pods/processes are created yet), it works by simply setting desired port to all processes.
// When the replica set is created and running, it orchestrates port changes as mongodb-agent does not allow changing ports in more than one process at a time.
// For the running cluster, it changes ports only when all pods reached goal state and changes the ports one by one.
// For the whole process of port change, the service has both ports exposed: old and new. After it finishes, only the new port is in the service.
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

// calculateExpectedPorts is a helper function to calculate what should be the current ports set in all replica set pods.
// It's working deterministically using currentACProcesses from automation config and currentPodStates.
func (r *ReplicaSetPortManager) calculateExpectedPorts() (processPortMap map[string]int, portChangeRequired bool, oldPort int) {
	processPortMap = map[string]int{}

	// populate processPortMap with current ports
	// it also populates entries for not existing pods yet
	for _, podState := range r.currentPodStates {
		process := r.getProcessByName(podState.PodName.Name)
		if process == nil || process.GetPort() == 0 {
			// new processes are configured with correct port from the start
			processPortMap[podState.PodName.Name] = r.expectedPort
		} else {
			processPortMap[podState.PodName.Name] = process.GetPort()
		}
	}

	// check if there is a need to perform port change
	portChangeRequired = false
	// This is the only place we could get the old port value
	// As soon as the port is changed on the MongoDB resource we lose the old value.
	oldPort = r.expectedPort
	for _, port := range processPortMap {
		if port != r.expectedPort {
			portChangeRequired = true
			oldPort = port
			break
		}
	}

	// If there are no port changes we just return initial config.
	// This way this ReplicaSetPortManager is used also for setting the initial port values for all processes.
	if !portChangeRequired {
		r.log.Debugf("No port change required")
		return processPortMap, false, oldPort
	}

	// We only perform port change if all pods reached goal states.
	// That will guarantee, that we will not change more than one process' port at a time.
	for _, podState := range r.currentPodStates {
		if !podState.ReachedGoalState {
			r.log.Debugf("Port change required but not all pods reached goal state, abandoning port change")
			return processPortMap, true, oldPort
		}
	}

	// change the port only in the first eligible process as the agent cannot handle simultaneous port changes in multiple processes
	// We have guaranteed here that all pods are created and have reached the goal state.
	for _, podState := range r.currentPodStates {
		podName := podState.PodName.Name
		if processPortMap[podName] != r.expectedPort {
			r.log.Debugf("Changing port in process %s from %d to %d", podName, processPortMap[podName], r.expectedPort)
			processPortMap[podName] = r.expectedPort
			break
		}
	}

	return processPortMap, true, oldPort
}
