package agent

import (
	"context"
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/pod"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// podAnnotationAgentVersion is the Pod Annotation key which contains the current version of the Automation Config
	// the Agent on the Pod is on now.
	podAnnotationAgentVersion = "agent.mongodb.com/version"
)

type PodState struct {
	PodName          types.NamespacedName
	Found            bool
	ReachedGoalState bool
	IsArbiter        bool
}

// AllReachedGoalState returns whether the agents associated with a given StatefulSet have reached goal state.
// it achieves this by reading the Pod annotations and checking to see if they have reached the expected config versions.
func AllReachedGoalState(ctx context.Context, sts appsv1.StatefulSet, podGetter pod.Getter, desiredMemberCount, targetConfigVersion int, log *zap.SugaredLogger) (bool, error) {
	// AllReachedGoalState does not use desiredArbitersCount for backwards compatibility
	podStates, err := GetAllDesiredMembersAndArbitersPodState(ctx, types.NamespacedName{
		Namespace: sts.Namespace,
		Name:      sts.Name,
	}, podGetter, desiredMemberCount, 0, targetConfigVersion, log)
	if err != nil {
		return false, err
	}

	var podsNotFound []string
	for _, podState := range podStates {
		if !podState.Found {
			podsNotFound = append(podsNotFound, podState.PodName.Name)
		} else if !podState.ReachedGoalState {
			return false, nil
		}
	}

	if len(podsNotFound) == desiredMemberCount {
		// no pods existing means that the StatefulSet hasn't been created yet - will be done during the next step
		return true, nil
	}

	if len(podsNotFound) > 0 {
		log.Infof("The following Pods don't exist: %v. Assuming they will be rescheduled by Kubernetes soon", podsNotFound)
		return false, nil
	}

	log.Infof("All %d Agents have reached Goal state", desiredMemberCount)
	return true, nil
}

// GetAllDesiredMembersAndArbitersPodState returns states of all desired pods in a replica set.
// Pod names to search for are calculated using desiredMemberCount and desiredArbitersCount. Each pod is then checked if it exists
// or if it reached goal state vs targetConfigVersion.
func GetAllDesiredMembersAndArbitersPodState(ctx context.Context, namespacedName types.NamespacedName, podGetter pod.Getter, desiredMembersCount, desiredArbitersCount, targetConfigVersion int, log *zap.SugaredLogger) ([]PodState, error) {
	podStates := make([]PodState, desiredMembersCount+desiredArbitersCount)

	membersPodNames := statefulSetPodNames(namespacedName.Name, desiredMembersCount)
	arbitersPodNames := arbitersStatefulSetPodNames(namespacedName.Name, desiredArbitersCount)

	for i, podName := range append(membersPodNames, arbitersPodNames...) {
		podNamespacedName := types.NamespacedName{Name: podName, Namespace: namespacedName.Namespace}
		podState := PodState{
			PodName:          podNamespacedName,
			Found:            true,
			ReachedGoalState: false,
			IsArbiter:        i >= len(membersPodNames),
		}

		p, err := podGetter.GetPod(ctx, podNamespacedName)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				// we can skip below iteration and check for our goal state since the pod is not available yet
				podState.Found = false
				podState.ReachedGoalState = false
				podStates[i] = podState
				continue
			} else {
				return nil, err
			}
		}

		podState.ReachedGoalState = ReachedGoalState(p, targetConfigVersion, log)
		podStates[i] = podState
	}

	return podStates, nil
}

// ReachedGoalState checks if a single Agent has reached the goal state. To do this it reads the Pod annotation
// to find out the current version the Agent is on.
func ReachedGoalState(pod corev1.Pod, targetConfigVersion int, log *zap.SugaredLogger) bool {
	currentAgentVersion, ok := pod.Annotations[podAnnotationAgentVersion]
	if !ok {
		log.Debugf("The Pod '%s' doesn't have annotation '%s' yet", pod.Name, podAnnotationAgentVersion)
		return false
	}
	if cast.ToInt(currentAgentVersion) != targetConfigVersion {
		log.Debugf("The Agent in the Pod '%s' hasn't reached the goal state yet (goal: %d, agent: %s)", pod.Name, targetConfigVersion, currentAgentVersion)
		return false
	}
	return true
}

// statefulSetPodNames returns a slice of names for a subset of the StatefulSet pods.
// we need a subset in the case of scaling up/down.
func statefulSetPodNames(name string, currentMembersCount int) []string {
	names := make([]string, currentMembersCount)
	for i := 0; i < currentMembersCount; i++ {
		names[i] = fmt.Sprintf("%s-%d", name, i)
	}
	return names
}

func arbitersStatefulSetPodNames(name string, currentArbitersCount int) []string {
	names := make([]string, currentArbitersCount)
	for i := 0; i < currentArbitersCount; i++ {
		names[i] = fmt.Sprintf("%s-arb-%d", name, i)
	}
	return names
}
