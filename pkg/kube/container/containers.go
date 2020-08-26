package container

import (
	"sort"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/lifecycle"
	corev1 "k8s.io/api/core/v1"
)

type Modification func(*corev1.Container)

// Apply returns a function which applies a series of Modification functions to a *corev1.Container
func Apply(modifications ...Modification) Modification {
	return func(container *corev1.Container) {
		for _, mod := range modifications {
			mod(container)
		}
	}
}

// New returns a concrete corev1.Container instance which has been modified based on the provided
// modifications
func New(mods ...Modification) corev1.Container {
	c := corev1.Container{}
	for _, mod := range mods {
		mod(&c)
	}
	return c
}

// NOOP is a valid Modification which applies no changes
func NOOP() Modification {
	return func(container *corev1.Container) {}
}

// WithName sets the container name
func WithName(name string) Modification {
	return func(container *corev1.Container) {
		container.Name = name
	}
}

// WithImage sets the container image
func WithImage(image string) Modification {
	return func(container *corev1.Container) {
		container.Image = image
	}
}

// WithImagePullPolicy sets the container pullPolicy
func WithImagePullPolicy(pullPolicy corev1.PullPolicy) Modification {
	return func(container *corev1.Container) {
		container.ImagePullPolicy = pullPolicy
	}
}

// WithReadinessProbe modifies the container's Readiness Probe
func WithReadinessProbe(probeFunc func(*corev1.Probe)) Modification {
	return func(container *corev1.Container) {
		if container.ReadinessProbe == nil {
			container.ReadinessProbe = &corev1.Probe{}
		}
		probeFunc(container.ReadinessProbe)
	}
}

// WithLivenessProbe modifies the container's Liveness Probe
func WithLivenessProbe(readinessProbeFunc func(*corev1.Probe)) Modification {
	return func(container *corev1.Container) {
		if container.LivenessProbe == nil {
			container.LivenessProbe = &corev1.Probe{}
		}
		readinessProbeFunc(container.LivenessProbe)
	}
}

// WithResourceRequirements sets the container's Resources
func WithResourceRequirements(resources corev1.ResourceRequirements) Modification {
	return func(container *corev1.Container) {
		container.Resources = resources
	}
}

// WithCommand sets the containers Command
func WithCommand(cmd []string) Modification {
	return func(container *corev1.Container) {
		container.Command = cmd
	}
}

// WithLifecycle applies the lifecycle Modification to this container's
// Lifecycle
func WithLifecycle(lifeCycleMod lifecycle.Modification) Modification {
	return func(container *corev1.Container) {
		if container.Lifecycle == nil {
			container.Lifecycle = &corev1.Lifecycle{}
		}
		lifeCycleMod(container.Lifecycle)
	}
}

// WithEnvs ensures all of the provided envs exist in the container
func WithEnvs(envs ...corev1.EnvVar) Modification {
	return func(container *corev1.Container) {
		container.Env = envvar.MergeWithOverride(container.Env, envs)
	}
}

// WithVolumeMounts sets the VolumeMounts
func WithVolumeMounts(volumeMounts []corev1.VolumeMount) Modification {
	volumesMountsCopy := make([]corev1.VolumeMount, len(volumeMounts))
	copy(volumesMountsCopy, volumeMounts)
	return func(container *corev1.Container) {
		merged := map[string]corev1.VolumeMount{}
		for _, ex := range container.VolumeMounts {
			merged[volumeMountToString(ex)] = ex
		}
		for _, des := range volumesMountsCopy {
			merged[volumeMountToString(des)] = des
		}

		var final []corev1.VolumeMount
		for _, v := range merged {
			final = append(final, v)
		}
		sort.SliceStable(final, func(i, j int) bool {
			a := final[i]
			b := final[j]
			return volumeMountToString(a) < volumeMountToString(b)
		})
		container.VolumeMounts = final
	}
}

func volumeMountToString(v corev1.VolumeMount) string {
	return strings.Join([]string{v.Name, v.MountPath, v.SubPath}, "-")
}

// WithPorts sets the container's Ports
func WithPorts(ports []corev1.ContainerPort) Modification {
	return func(container *corev1.Container) {
		container.Ports = ports
	}
}

// WithSecurityContext sets teh container's SecurityContext
func WithSecurityContext(context corev1.SecurityContext) Modification {
	return func(container *corev1.Container) {
		container.SecurityContext = &context
	}
}

// WithAgentFlags takes a slice of envVars corresponding to
// pairs of key-value agent startup flags and concatenates them
// into a single string that is then passed as env variable AGENT_FLAGS
func WithAgentFlags(envs ...corev1.EnvVar) Modification {
	return func(container *corev1.Container) {
		agentParams := ""
		for _, variable := range envs {
			agentParams += " -" + variable.Name + " " + variable.Value
		}
		container.Env = envvar.MergeWithOverride(container.Env, []corev1.EnvVar{{Name: "AGENT_FLAGS", Value: agentParams}})
	}
}
