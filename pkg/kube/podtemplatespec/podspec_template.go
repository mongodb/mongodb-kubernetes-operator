package podtemplatespec

import (
	corev1 "k8s.io/api/core/v1"
)

type PodTemplateFunc func(*corev1.PodTemplateSpec)

const (
	notFound = -1
)

func findIndexByName(name string, containers []corev1.Container) int {
	for idx, c := range containers {
		if c.Name == name {
			return idx
		}
	}
	return notFound
}

func WithContainer(name string, containerfunc func(*corev1.Container)) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		idx := findIndexByName(name, podTemplateSpec.Spec.Containers)
		if idx == notFound {
			podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, corev1.Container{})
			idx = len(podTemplateSpec.Spec.Containers) - 1
		}
		c := &podTemplateSpec.Spec.Containers[idx]
		containerfunc(c)
	}
}

func WithInitContainer(name string, containerfunc func(*corev1.Container)) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		// if we are attempting to modify a container that does not exist, we will add a new one
		idx := findIndexByName(name, podTemplateSpec.Spec.InitContainers)
		if idx == notFound {
			podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, corev1.Container{})
			idx = len(podTemplateSpec.Spec.InitContainers) - 1
		}
		c := &podTemplateSpec.Spec.InitContainers[idx]
		containerfunc(c)
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
		for _, v := range template.Spec.Volumes {
			if v.Name == volume.Name {
				return
			}
		}
		template.Spec.Volumes = append(template.Spec.Volumes, volume)
	}
}

func Apply(templateMods ...PodTemplateFunc) PodTemplateFunc {
	return func(template *corev1.PodTemplateSpec) {
		for _, f := range templateMods {
			f(template)
		}
	}
}
