package pod

import (
	"go.uber.org/zap"
	"strconv"
	"strings"

	"k8s.io/client-go/kubernetes"
)

const mongodbAgentVersionAnnotation = "agent.mongodb.com/version"

func PatchPodAnnotation(podNamespace string, lastVersionAchieved int64, memberName string, clientSet kubernetes.Interface) error {
	patcher := NewKubernetesPodPatcher(clientSet)
	mdbAgentVersion := strconv.FormatInt(lastVersionAchieved, 10)
	return patchPod(patcher, podNamespace, mdbAgentVersion, memberName)
}

func patchPod(patcher Patcher, podNamespace string, mdbAgentVersion string, memberName string) error {
	payload := []patchValue{{
		Op:    "add",
		Path:  "/metadata/annotations/" + strings.Replace(mongodbAgentVersionAnnotation, "/", "~1", -1),
		Value: mdbAgentVersion,
	}}

	pod, err := patcher.patchPod(podNamespace, memberName, payload)
	if pod != nil {
		zap.S().Debugf("Updated Pod annotation: %v (%s)", pod.Annotations, memberName)
	}
	return err
}
