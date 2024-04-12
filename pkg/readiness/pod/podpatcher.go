package pod

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type patchValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

type Patcher struct {
	clientset kubernetes.Interface
}

func NewKubernetesPodPatcher(clientSet kubernetes.Interface) Patcher {
	return Patcher{clientset: clientSet}
}

func (p Patcher) patchPod(ctx context.Context, namespace, podName string, payload []patchValue) (*v1.Pod, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return p.clientset.CoreV1().Pods(namespace).Patch(ctx, podName, types.JSONPatchType, data, metav1.PatchOptions{})
}
