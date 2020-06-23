package container

import (
	"sort"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/lifecycle"
	corev1 "k8s.io/api/core/v1"
)

type Modification func(*corev1.Container)

func Apply(modifications ...Modification) Modification {
	return func(container *corev1.Container) {
		for _, mod := range modifications {
			mod(container)
		}
	}
}

func New(mods ...Modification) corev1.Container {
	c := corev1.Container{}
	for _, mod := range mods {
		mod(&c)
	}
	return c
}

func NOOP() Modification {
	return func(container *corev1.Container) {}
}

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

func WithEnvs(envs ...corev1.EnvVar) Modification {
	return func(container *corev1.Container) {
		container.Env = mergeEnvs(container.Env, envs)
	}
}

func mergeEnvs(existing, desired []corev1.EnvVar) []corev1.EnvVar {
	envMap := make(map[string]corev1.EnvVar)
	for _, env := range existing {
		envMap[env.Name] = env
	}

	for _, env := range desired {
		envMap[env.Name] = env
	}

	var mergedEnv []corev1.EnvVar
	for _, env := range envMap {
		mergedEnv = append(mergedEnv, env)
	}

	sort.SliceStable(mergedEnv, func(i, j int) bool {
		return mergedEnv[i].Name < mergedEnv[j].Name
	})
	return mergedEnv
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
