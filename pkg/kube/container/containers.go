package container

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/lifecycle"
	corev1 "k8s.io/api/core/v1"
)

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

func WithReadinessProbe(probeFunc func(*corev1.Probe)) Modification {
	return func(container *corev1.Container) {
		if container.ReadinessProbe == nil {
			container.ReadinessProbe = &corev1.Probe{}
		}
		probeFunc(container.ReadinessProbe)
	}
}

func WithLivenessProbe(readinessProbeFunc func(*corev1.Probe)) Modification {
	return func(container *corev1.Container) {
		if container.LivenessProbe == nil {
			container.LivenessProbe = &corev1.Probe{}
		}
		readinessProbeFunc(container.LivenessProbe)
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

func WithLifecycle(lifeCycleMod lifecycle.Modification) Modification {
	return func(container *corev1.Container) {
		if container.Lifecycle == nil {
			container.Lifecycle = &corev1.Lifecycle{}
		}
		lifeCycleMod(container.Lifecycle)
	}
}

func WithEnv(envs ...corev1.EnvVar) Modification {
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

func WithPorts(ports []corev1.ContainerPort) Modification {
	return func(container *corev1.Container) {
		container.Ports = ports
	}
}

func WithSecurityContext(context corev1.SecurityContext) Modification {
	return func(container *corev1.Container) {
		container.SecurityContext = &context
	}
}

func NOOP() Modification {
	return func(container *corev1.Container) {}
}

func Apply(modifications ...Modification) Modification {
	return func(container *corev1.Container) {
		for _, mod := range modifications {
			mod(container)
		}
	}
}
