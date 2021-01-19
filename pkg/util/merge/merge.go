package merge

import (
	"sort"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
	corev1 "k8s.io/api/core/v1"
)

// StringSlices accepts two slices of strings, and returns a string slice
// containing the distinct elements present in both. The elements are returned sorted.
func StringSlices(slice1, slice2 []string) []string {
	mergedStrings := make([]string, 0)
	mergedStrings = append(mergedStrings, slice1...)
	for _, s := range slice2 {
		if !contains.String(mergedStrings, s) {
			mergedStrings = append(mergedStrings, s)
		}
	}
	sort.SliceStable(mergedStrings, func(i, j int) bool {
		return mergedStrings[i] < mergedStrings[j]
	})

	return mergedStrings
}

func Container(defaultContainer, overrideContainer corev1.Container) corev1.Container {
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

	// TODO: merge Env

	// TODO: merge Resources

	// TODO: merge VolumeMounts

	// TODO: merge VolumeDevices

	// TODO: merge LivenessProbe

	// TODO: merge ReadinessProve

	// TODO: merge StartupProbe

	// TODO: merge Lifecycle

	if overrideContainer.TerminationMessagePath != "" {
		merged.TerminationMessagePath = overrideContainer.TerminationMessagePath
	}

	if overrideContainer.TerminationMessagePolicy != "" {
		merged.TerminationMessagePolicy = overrideContainer.TerminationMessagePolicy
	}

	if overrideContainer.ImagePullPolicy != "" {
		merged.ImagePullPolicy = overrideContainer.ImagePullPolicy
	}

	// TODO: merge SecurityContext

	if overrideContainer.Stdin {
		merged.Stdin = overrideContainer.Stdin
	}

	if overrideContainer.StdinOnce {
		merged.StdinOnce = overrideContainer.StdinOnce
	}

	if overrideContainer.TTY {
		merged.TTY = overrideContainer.TTY
	}

	return merged
}

// ContainerPorts merges all of the fields of the overridePort on top of the defaultPort
// if the fields don't have a zero value. Thw new ContainerPort is returned.
func ContainerPorts(defaultPort, overridePort corev1.ContainerPort) corev1.ContainerPort {
	mergedPort := defaultPort
	if overridePort.Name != "" {
		mergedPort.Name = overridePort.Name
	}
	if overridePort.ContainerPort != 0 {
		mergedPort.ContainerPort = overridePort.ContainerPort
	}
	if overridePort.HostPort != 0 {
		mergedPort.HostPort = overridePort.HostPort
	}
	if overridePort.Protocol != "" {
		mergedPort.Protocol = overridePort.Protocol
	}
	if overridePort.HostIP != "" {
		mergedPort.HostIP = overridePort.HostIP
	}
	return mergedPort
}

// ContainerPortSlicesByName takes two slices of corev1.ContainerPorts, these values are merged by name.
// if there are elements present in the overridePorts that are not present in defaultPorts, they are
// appended to the end.
func ContainerPortSlicesByName(defaultPorts, overridePorts []corev1.ContainerPort) []corev1.ContainerPort {
	defaultPortMap := createContainerPortMap(defaultPorts)
	overridePortsMap := createContainerPortMap(overridePorts)

	mergedPorts := make([]corev1.ContainerPort, 0)

	for portName, defaultPort := range defaultPortMap {
		if overridePort, ok := overridePortsMap[portName]; ok {
			mergedPorts = append(mergedPorts, ContainerPorts(defaultPort, overridePort))
		} else {
			mergedPorts = append(mergedPorts, defaultPort)
		}
	}

	for portName, overridePort := range overridePortsMap {
		if _, ok := defaultPortMap[portName]; !ok {
			mergedPorts = append(mergedPorts, overridePort)
		}
	}

	sort.SliceStable(mergedPorts, func(i, j int) bool {
		return mergedPorts[i].Name < mergedPorts[j].Name
	})

	return mergedPorts
}

func createContainerPortMap(containerPorts []corev1.ContainerPort) map[string]corev1.ContainerPort {
	containerPortMap := make(map[string]corev1.ContainerPort)
	for _, m := range containerPorts {
		containerPortMap[m.Name] = m
	}
	return containerPortMap
}

// VolumeMount merges two corev1.VolumeMounts. Any fields in the override take precedence
// over the values in the original. Thw new VolumeMount is returned.
func VolumeMount(original, override corev1.VolumeMount) corev1.VolumeMount {
	merged := original

	if override.Name != "" {
		merged.Name = override.Name
	}

	if override.ReadOnly {
		merged.ReadOnly = override.ReadOnly
	}

	if override.MountPath != "" {
		merged.MountPath = override.MountPath
	}

	if override.SubPath != "" {
		merged.SubPath = override.SubPath
	}

	if override.MountPropagation != nil {
		merged.MountPropagation = override.MountPropagation
	}

	if override.SubPathExpr != "" {
		merged.SubPathExpr = override.SubPathExpr
	}
	return merged
}
