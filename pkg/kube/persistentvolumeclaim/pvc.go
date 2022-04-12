package persistentvolumeclaim

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Modification func(claim *corev1.PersistentVolumeClaim)

// Apply returns a function which applies a series of Modification functions to a *corev1.PersistentVolumeClaim
func Apply(funcs ...Modification) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		for _, f := range funcs {
			f(claim)
		}
	}
}

// NOOP is a valid Modification which applies no changes
func NOOP() Modification {
	return func(claim *corev1.PersistentVolumeClaim) {}
}

// WithName sets the PersistentVolumeClaim's name
func WithName(name string) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Name = name
	}
}

// WithAccessModes sets the PersistentVolumeClaim's AccessModes
func WithAccessModes(accessMode corev1.PersistentVolumeAccessMode) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{accessMode}
	}
}

// WithResourceRequests sets the PersistentVolumeClaim's Resource Requests
func WithResourceRequests(requests corev1.ResourceList) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.Resources.Requests = requests
	}
}

// WithLabelSelector sets the PersistentVolumeClaim's LevelSelector
func WithLabelSelector(selector *metav1.LabelSelector) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.Selector = selector
	}
}

// WithStorageClassName sets the PersistentVolumeClaim's storage class name
func WithStorageClassName(storageClassName string) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.StorageClassName = &storageClassName
	}
}

// WithLabels sets the PersistentVolumeClaim's labels
func WithLabels(labels map[string]string) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Labels = labels
	}
}
