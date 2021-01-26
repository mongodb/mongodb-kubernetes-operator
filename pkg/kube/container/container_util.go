package container

import corev1 "k8s.io/api/core/v1"

func GetByName(name string, containers []corev1.Container) *corev1.Container {
	for i, c := range containers {
		if c.Name == name {
			return &containers[i]
		}
	}
	return nil
}
