package podtemplatespec

import (
	"encoding/json"
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

type PodTemplateFunc func(*corev1.PodTemplateSpec)

const (
	notFound = -1
)

func prettyPrint(i interface{}) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	zap.S().Infof(string(b))
}

func WithContainer(name string, container func(*corev1.Container)) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		idx := findIndexByName(name, podTemplateSpec.Spec.Containers)
		if idx == notFound {
			idx = 0
			zap.S().Infof("[%s] was not found", name)
			podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, corev1.Container{})
		}
		c := &podTemplateSpec.Spec.Containers[idx]
		zap.S().Info("PRINTING C")
		prettyPrint(c)
		container(c)
		prettyPrint(c)
	}
}

func WithInitContainer(name string, container func(*corev1.Container)) PodTemplateFunc {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		idx := findIndexByName(name, podTemplateSpec.Spec.InitContainers)
		if idx == notFound {
			idx = 0
			podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, corev1.Container{})
		}
		c := &podTemplateSpec.Spec.InitContainers[idx]
		zap.S().Info("PRINTING INIT C")
		prettyPrint(c)
		container(c)
		prettyPrint(c)
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

func findIndexByName(name string, containers []corev1.Container) int {
	for idx, c := range containers {
		if c.Name == name {
			return idx
		}
	}
	return notFound
}
