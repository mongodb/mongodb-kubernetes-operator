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
	merged.Env = Envs(defaultContainer.Env, overrideContainer.Env)
	merged.Resources = ResourceRequirements(defaultContainer.Resources, overrideContainer.Resources)
	merged.VolumeMounts = VolumeMounts(defaultContainer.VolumeMounts, overrideContainer.VolumeMounts)
	merged.VolumeDevices = VolumeDevices(defaultContainer.VolumeDevices, overrideContainer.VolumeDevices)

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

// VolumeDevices merges two slices of VolumeDevices by name.
func VolumeDevices(original, override []corev1.VolumeDevice) []corev1.VolumeDevice {
	mergedDevicesMap := map[string]corev1.VolumeDevice{}
	originalDevicesMap := createVolumeDevicesMap(original)
	overrideDevicesMap := createVolumeDevicesMap(override)

	for k, v := range originalDevicesMap {
		mergedDevicesMap[k] = v
	}

	for k, v := range overrideDevicesMap {
		if orig, ok := originalDevicesMap[k]; ok {
			mergedDevicesMap[k] = mergeVolumeDevice(orig, v)
		} else {
			mergedDevicesMap[k] = v
		}
	}

	var mergedDevices []corev1.VolumeDevice
	for _, v := range mergedDevicesMap {
		mergedDevices = append(mergedDevices, v)
	}

	sort.SliceStable(mergedDevices, func(i, j int) bool {
		return mergedDevices[i].Name < mergedDevices[j].Name
	})
	return mergedDevices
}

func createVolumeDevicesMap(devices []corev1.VolumeDevice) map[string]corev1.VolumeDevice {
	m := make(map[string]corev1.VolumeDevice)
	for _, v := range devices {
		m[v.Name] = v
	}
	return m
}

func mergeVolumeDevice(original, override corev1.VolumeDevice) corev1.VolumeDevice {
	merged := original
	if override.Name != "" {
		merged.Name = override.Name
	}
	if override.DevicePath != "" {
		merged.DevicePath = override.DevicePath
	}
	return merged
}

// Envs merges two slices of EnvVars using their name as the unique
// identifier.
func Envs(original, override []corev1.EnvVar) []corev1.EnvVar {
	mergedEnvsMap := map[string]corev1.EnvVar{}

	originalMap := createEnvMap(original)
	overrideMap := createEnvMap(override)

	for k, v := range originalMap {
		mergedEnvsMap[k] = v
	}

	for k, v := range overrideMap {
		if orig, ok := originalMap[k]; ok {
			mergedEnvsMap[k] = mergeSingleEnv(orig, v)
		} else {
			mergedEnvsMap[k] = v
		}
	}

	var mergedEnvs []corev1.EnvVar
	for _, v := range mergedEnvsMap {
		mergedEnvs = append(mergedEnvs, v)
	}

	sort.SliceStable(mergedEnvs, func(i, j int) bool {
		return mergedEnvs[i].Name < mergedEnvs[j].Name
	})
	return mergedEnvs
}

func mergeSingleEnv(original, override corev1.EnvVar) corev1.EnvVar {
	merged := original
	if override.Value != "" {
		merged.Value = override.Value
		merged.ValueFrom = nil
	}

	if override.ValueFrom != nil {
		merged.ValueFrom = override.ValueFrom
		merged.Value = ""
	}
	return merged
}

func createEnvMap(env []corev1.EnvVar) map[string]corev1.EnvVar {
	m := make(map[string]corev1.EnvVar)
	for _, e := range env {
		m[e.Name] = e
	}
	return m
}

// ResourceRequirements merges two resource requirements.
func ResourceRequirements(original, override corev1.ResourceRequirements) corev1.ResourceRequirements {
	merged := original
	if override.Limits != nil {
		merged.Limits = override.Limits
	}

	if override.Requests != nil {
		merged.Requests = override.Requests
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

// VolumeMounts merges two slices of volume mounts by name.
func VolumeMounts(original, override []corev1.VolumeMount) []corev1.VolumeMount {
	mergedMountsMap := map[string]corev1.VolumeMount{}
	originalMounts := createVolumeMountMap(original)
	overrideMounts := createVolumeMountMap(override)

	for k, v := range originalMounts {
		mergedMountsMap[k] = v
	}

	for k, v := range overrideMounts {
		if orig, ok := originalMounts[k]; ok {
			mergedMountsMap[k] = VolumeMount(orig, v)
		} else {
			mergedMountsMap[k] = v
		}
	}

	var mergedMounts []corev1.VolumeMount
	for _, mount := range mergedMountsMap {
		mergedMounts = append(mergedMounts, mount)
	}

	sort.SliceStable(mergedMounts, func(i, j int) bool {
		return mergedMounts[i].Name < mergedMounts[j].Name
	})

	return mergedMounts
}

func createVolumeMountMap(mounts []corev1.VolumeMount) map[string]corev1.VolumeMount {
	m := make(map[string]corev1.VolumeMount)
	for _, v := range mounts {
		m[v.Name] = v
	}
	return m
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
