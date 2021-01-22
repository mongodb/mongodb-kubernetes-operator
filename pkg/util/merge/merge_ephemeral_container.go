package merge

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// EphemeralContainers merges two slices of EphemeralContainers merging each item by container name.
func EphemeralContainers(defaultContainers, overrideContainers []corev1.EphemeralContainer) []corev1.EphemeralContainer {
	mergedContainerMap := map[string]corev1.EphemeralContainer{}

	originalMap := createEphemeralContainerMap(defaultContainers)
	overrideMap := createEphemeralContainerMap(overrideContainers)

	for k, v := range originalMap {
		mergedContainerMap[k] = v
	}

	for k, v := range overrideMap {
		if orig, ok := originalMap[k]; ok {
			mergedContainerMap[k] = EphemeralContainer(orig, v)
		} else {
			mergedContainerMap[k] = v
		}
	}

	var mergedContainers []corev1.EphemeralContainer
	for _, v := range mergedContainerMap {
		mergedContainers = append(mergedContainers, v)
	}

	sort.SliceStable(mergedContainers, func(i, j int) bool {
		return mergedContainers[i].Name < mergedContainers[j].Name
	})
	return mergedContainers

}

// EphemeralContainer merges two EphemeralContainers together.
func EphemeralContainer(defaultContainer, overrideContainer corev1.EphemeralContainer) corev1.EphemeralContainer {
	merged := defaultContainer

	if overrideContainer.Name != "" {
		merged.Name = overrideContainer.Name
	}

	if overrideContainer.Image != "" {
		merged.Image = overrideContainer.Image
	}

	merged.Command = StringSlices(defaultContainer.Command, overrideContainer.Command)
	merged.Args = StringSlices(defaultContainer.Args, overrideContainer.Args)

	if overrideContainer.WorkingDir != "" {
		merged.WorkingDir = overrideContainer.WorkingDir
	}

	merged.Ports = ContainerPortSlicesByName(defaultContainer.Ports, overrideContainer.Ports)
	merged.Env = Envs(defaultContainer.Env, overrideContainer.Env)
	merged.Resources = ResourceRequirements(defaultContainer.Resources, overrideContainer.Resources)
	merged.VolumeMounts = VolumeMounts(defaultContainer.VolumeMounts, overrideContainer.VolumeMounts)
	merged.VolumeDevices = VolumeDevices(defaultContainer.VolumeDevices, overrideContainer.VolumeDevices)
	merged.LivenessProbe = Probe(defaultContainer.LivenessProbe, overrideContainer.LivenessProbe)
	merged.ReadinessProbe = Probe(defaultContainer.ReadinessProbe, overrideContainer.ReadinessProbe)
	merged.StartupProbe = Probe(defaultContainer.StartupProbe, overrideContainer.StartupProbe)
	merged.Lifecycle = LifeCycle(defaultContainer.Lifecycle, overrideContainer.Lifecycle)

	if overrideContainer.TerminationMessagePath != "" {
		merged.TerminationMessagePath = overrideContainer.TerminationMessagePath
	}

	if overrideContainer.TerminationMessagePolicy != "" {
		merged.TerminationMessagePolicy = overrideContainer.TerminationMessagePolicy
	}

	if overrideContainer.ImagePullPolicy != "" {
		merged.ImagePullPolicy = overrideContainer.ImagePullPolicy
	}

	merged.SecurityContext = SecurityContext(defaultContainer.SecurityContext, overrideContainer.SecurityContext)

	if overrideContainer.Stdin {
		merged.Stdin = overrideContainer.Stdin
	}

	if overrideContainer.StdinOnce {
		merged.StdinOnce = overrideContainer.StdinOnce
	}

	if overrideContainer.TTY {
		merged.TTY = overrideContainer.TTY
	}

	// EphemeralContainer only fields
	if overrideContainer.TargetContainerName != "" {
		merged.TargetContainerName = overrideContainer.TargetContainerName
	}

	return merged
}

func createEphemeralContainerMap(containers []corev1.EphemeralContainer) map[string]corev1.EphemeralContainer {
	m := make(map[string]corev1.EphemeralContainer)
	for _, v := range containers {
		m[v.Name] = v
	}
	return m
}
