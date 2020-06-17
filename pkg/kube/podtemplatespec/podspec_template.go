package podtemplatespec

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	corev1 "k8s.io/api/core/v1"
)

type PodTemplateFunc func(*corev1.PodTemplateSpec)

func WithContainers(containers ...corev1.Container) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		for _, c := range containers {
			if !containsContainer(podTemplateSpec.Spec.Containers, c) {
				podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, c)
			}
		}
	}
}

func EditContainer(idx int, modFunc container.Modification) PodTemplateFunc {
	return func(template *corev1.PodTemplateSpec) {
		c := &template.Spec.Containers[idx]
		modFunc(c)
	}
}

func WithInitContainers(containers ...corev1.Container) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		for _, c := range containers {
			if !containsContainer(podTemplateSpec.Spec.InitContainers, c) {
				podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, containers...)
			}
		}
	}
}

func EditInitContainer(idx int, modFunc container.Modification) PodTemplateFunc {
	return func(template *corev1.PodTemplateSpec) {
		c := &template.Spec.InitContainers[idx]
		modFunc(c)
	}
}

func WithPodLabels(labels map[string]string) PodTemplateFunc {
	if labels == nil {
		labels = map[string]string{}
	}
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.ObjectMeta.Labels = labels
	}
}

func WithServiceAccount(serviceAccountName string) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.ServiceAccountName = serviceAccountName
	}
}

func WithVolume(volume corev1.Volume) PodTemplateFunc {
	return func(template *corev1.PodTemplateSpec) {
		template.Spec.Volumes = append(template.Spec.Volumes, volume)
	}
}

func Modify(templateMods ...PodTemplateFunc) PodTemplateFunc {
	return func(template *corev1.PodTemplateSpec) {
		for _, f := range templateMods {
			f(template)
		}
	}
}

func containsContainer(containers []corev1.Container, c corev1.Container) bool {
	for _, v := range containers {
		if v.Name == c.Name {
			return true
		}
	}
	return false
}
