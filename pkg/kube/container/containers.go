package container

import corev1 "k8s.io/api/core/v1"

type Modification func(*corev1.Container)

func WithName(name string) Modification {
	return func(container *corev1.Container) {
		container.Name = name
	}
}

func WithImage(image string) Modification {
	return func(container *corev1.Container) {
		container.Image = image
	}
}

func WithImagePullPolicy(pullPolicy corev1.PullPolicy) Modification {
	return func(container *corev1.Container) {
		container.ImagePullPolicy = pullPolicy
	}
}

func WithReadinessProbe(probe corev1.Probe) Modification {
	return func(container *corev1.Container) {
		container.ReadinessProbe = &probe
	}
}

func WithResourceRequirements(resources corev1.ResourceRequirements) Modification {
	return func(container *corev1.Container) {
		container.Resources = resources
	}
}

func WithCommand(cmd []string) Modification {
	return func(container *corev1.Container) {
		container.Command = cmd
	}
}

func WithEnv(envs []corev1.EnvVar) Modification {
	return func(container *corev1.Container) {
		container.Env = envs
	}
}

func WithVolumeMounts(volumeMounts []corev1.VolumeMount) Modification {
	volumesMountsCopy := make([]corev1.VolumeMount, len(volumeMounts))
	copy(volumesMountsCopy, volumeMounts)
	return func(container *corev1.Container) {
		container.VolumeMounts = volumesMountsCopy
	}
}

func Apply(modifications ...Modification) Modification {
	return func(container *corev1.Container) {
		for _, mod := range modifications {
			mod(container)
		}
	}
}
