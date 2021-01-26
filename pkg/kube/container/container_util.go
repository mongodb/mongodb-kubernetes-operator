package container

import corev1 "k8s.io/api/core/v1"

// GetByName returns a container with the given name from the slice of containers.
// nil is returned if the container does not exist.
func GetByName(name string, containers []corev1.Container) *corev1.Container {
	for i, c := range containers {
		if c.Name == name {
			return &containers[i]
		}
	}
	return nil
}
