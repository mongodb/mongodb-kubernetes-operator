package pod

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Getter interface {
	GetPod(objectKey client.ObjectKey) (corev1.Pod, error)
}
