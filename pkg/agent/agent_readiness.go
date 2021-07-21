package agent

import (
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

// AllReachedGoalState returns whether or not the agents associated with a given StatefulSet have reached goal state.
// it achieves this by reading the Pod annotations and checking to see if they have reached the expected config versions.
func AllReachedGoalState(sts appsv1.StatefulSet, podGetter pod.Getter, desiredMemberCount, targetConfigVersion int, log *zap.SugaredLogger) (bool, error) {
	var podsNotFound []string

	for _, podName := range statefulSetPodNames(sts, desiredMemberCount) {
		p, err := podGetter.GetPod(types.NamespacedName{Name: podName, Namespace: sts.Namespace})
		if err != nil {
			if apiErrors.IsNotFound(err) {
				podsNotFound = append(podsNotFound, podName)
				continue
			}
			return false, err
		}

		if reachedGoalState := ReachedGoalState(p, targetConfigVersion, log); !reachedGoalState {
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
func statefulSetPodNames(sts appsv1.StatefulSet, currentMembersCount int) []string {
	names := make([]string, currentMembersCount)
	for i := 0; i < currentMembersCount; i++ {
		names[i] = fmt.Sprintf("%s-%d", sts.Name, i)
	}
	return names
}
