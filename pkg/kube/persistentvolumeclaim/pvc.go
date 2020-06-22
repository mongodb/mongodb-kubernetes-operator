package persistentvolumeclaim

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Modification func(claim *corev1.PersistentVolumeClaim)

func Apply(funcs ...Modification) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		for _, f := range funcs {
			f(claim)
		}
	}
}

func NOOP() Modification {
	return func(claim *corev1.PersistentVolumeClaim) {}
}

func WithName(name string) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Name = name
	}
}

func WithAccessModes(accessMode corev1.PersistentVolumeAccessMode) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{accessMode}
	}
}

func WithResourceRequests(requests corev1.ResourceList) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.Resources.Requests = requests
	}
}

func WithLabelSelector(selector *metav1.LabelSelector) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.Selector = selector
	}
}

func WithStorageClassName(storageClassName string) Modification {
	return func(claim *corev1.PersistentVolumeClaim) {
		claim.Spec.StorageClassName = &storageClassName
	}
}
