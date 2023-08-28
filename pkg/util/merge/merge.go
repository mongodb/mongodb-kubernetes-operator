package merge

import (
	"sort"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
	corev1 "k8s.io/api/core/v1"
)

// StringSlices accepts two slices of strings, and returns a string slice
// containing the distinct elements present in both.
func StringSlices(slice1, slice2 []string) []string {
	mergedStrings := make([]string, 0)
	mergedStrings = append(mergedStrings, slice1...)
	for _, s := range slice2 {
		if !contains.String(mergedStrings, s) {
			mergedStrings = append(mergedStrings, s)
		}
	}
	return mergedStrings
}

// StringToStringMap merges two string maps together with the second map
// overriding any values also specified in the first.
func StringToStringMap(map1, map2 map[string]string) map[string]string {
	if map1 == nil && map2 == nil {
		return nil
	}
	mergedMap := make(map[string]string)
	for k, v := range map1 {
		mergedMap[k] = v
	}
	for k, v := range map2 {
		mergedMap[k] = v
	}
	return mergedMap
}

// StringToBoolMap merges two string-to-bool maps together with the second map
// overriding any values also specified in the first.
func StringToBoolMap(map1, map2 map[string]bool) map[string]bool {
	mergedMap := make(map[string]bool)
	for k, v := range map1 {
		mergedMap[k] = v
	}
	for k, v := range map2 {
		mergedMap[k] = v
	}
	return mergedMap
}

// Containers merges two slices of containers merging each item by container name.
func Containers(defaultContainers, overrideContainers []corev1.Container) []corev1.Container {
	mergedContainerMap := map[string]corev1.Container{}

	originalMap := createContainerMap(defaultContainers)
	overrideMap := createContainerMap(overrideContainers)

	for k, v := range originalMap {
		mergedContainerMap[k] = v
	}

	for k, v := range overrideMap {
		if orig, ok := originalMap[k]; ok {
			mergedContainerMap[k] = Container(orig, v)
		} else {
			mergedContainerMap[k] = v
		}
	}

	var mergedContainers []corev1.Container
	for _, v := range mergedContainerMap {
		mergedContainers = append(mergedContainers, v)
	}

	sort.SliceStable(mergedContainers, func(i, j int) bool {
		return mergedContainers[i].Name < mergedContainers[j].Name
	})
	return mergedContainers

}

func createContainerMap(containers []corev1.Container) map[string]corev1.Container {
	m := make(map[string]corev1.Container)
	for _, v := range containers {
		m[v.Name] = v
	}
	return m
}

func Container(defaultContainer, overrideContainer corev1.Container) corev1.Container {
	merged := defaultContainer

	if overrideContainer.Name != "" {
		merged.Name = overrideContainer.Name
	}

	if overrideContainer.Image != "" {
		merged.Image = overrideContainer.Image
	}

	merged.Command = defaultContainer.Command
	if len(overrideContainer.Command) > 0 {
		merged.Command = overrideContainer.Command
	}
	merged.Args = defaultContainer.Args
	if len(overrideContainer.Args) > 0 {
		merged.Args = overrideContainer.Args
	}

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

	return merged
}

// Probe merges the contents of two probes together.
func Probe(original, override *corev1.Probe) *corev1.Probe {
	if override == nil {
		return original
	}
	if original == nil {
		return override
	}
	merged := *original
	if override.Exec != nil {
		merged.Exec = override.Exec
	}
	if override.HTTPGet != nil {
		merged.HTTPGet = override.HTTPGet
	}
	if override.TCPSocket != nil {
		merged.TCPSocket = override.TCPSocket
	}
	if override.InitialDelaySeconds != 0 {
		merged.InitialDelaySeconds = override.InitialDelaySeconds
	}
	if override.TimeoutSeconds != 0 {
		merged.TimeoutSeconds = override.TimeoutSeconds
	}
	if override.PeriodSeconds != 0 {
		merged.PeriodSeconds = override.PeriodSeconds
	}

	if override.SuccessThreshold != 0 {
		merged.SuccessThreshold = override.SuccessThreshold
	}

	if override.FailureThreshold != 0 {
		merged.FailureThreshold = override.FailureThreshold
	}
	return &merged
}

// LifeCycle merges two LifeCycles.
func LifeCycle(original, override *corev1.Lifecycle) *corev1.Lifecycle {
	if override == nil {
		return original
	}
	if original == nil {
		return override
	}
	merged := *original

	if override.PostStart != nil {
		merged.PostStart = override.PostStart
	}
	if override.PreStop != nil {
		merged.PreStop = override.PreStop
	}
	return &merged
}

// SecurityContext merges two security contexts.
func SecurityContext(original, override *corev1.SecurityContext) *corev1.SecurityContext {
	if override == nil {
		return original
	}
	if original == nil {
		return override
	}
	merged := *original

	if override.Capabilities != nil {
		merged.Capabilities = override.Capabilities
	}

	if override.Privileged != nil {
		merged.Privileged = override.Privileged
	}

	if override.SELinuxOptions != nil {
		merged.SELinuxOptions = override.SELinuxOptions
	}

	if override.WindowsOptions != nil {
		merged.WindowsOptions = override.WindowsOptions
	}
	if override.RunAsUser != nil {
		merged.RunAsUser = override.RunAsUser
	}
	if override.RunAsGroup != nil {
		merged.RunAsGroup = override.RunAsGroup
	}
	if override.RunAsNonRoot != nil {
		merged.RunAsNonRoot = override.RunAsNonRoot
	}
	if override.ReadOnlyRootFilesystem != nil {
		merged.ReadOnlyRootFilesystem = override.ReadOnlyRootFilesystem
	}
	if override.AllowPrivilegeEscalation != nil {
		merged.AllowPrivilegeEscalation = override.AllowPrivilegeEscalation
	}
	if override.ProcMount != nil {
		merged.ProcMount = override.ProcMount
	}
	return &merged
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
		return volumeMountToString(mergedMounts[i]) < volumeMountToString(mergedMounts[j])
	})

	return mergedMounts
}

// volumeMountToString returns a string consisting of all components of the given VolumeMount.
func volumeMountToString(v corev1.VolumeMount) string {
	return strings.Join([]string{v.Name, v.MountPath, v.SubPath}, "_")
}

func createVolumeMountMap(mounts []corev1.VolumeMount) map[string]corev1.VolumeMount {
	m := make(map[string]corev1.VolumeMount)
	for _, v := range mounts {
		m[volumeMountToString(v)] = v
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

func Volumes(defaultVolumes []corev1.Volume, overrideVolumes []corev1.Volume) []corev1.Volume {
	defaultVolumesMap := createVolumesMap(defaultVolumes)
	overrideVolumesMap := createVolumesMap(overrideVolumes)
	mergedVolumes := []corev1.Volume{}

	for _, defaultVolume := range defaultVolumes {
		mergedVolume := defaultVolume
		if overrideVolume, ok := overrideVolumesMap[defaultVolume.Name]; ok {
			mergedVolume = Volume(defaultVolume, overrideVolume)
		}
		mergedVolumes = append(mergedVolumes, mergedVolume)
	}

	for _, overrideVolume := range overrideVolumes {
		if _, ok := defaultVolumesMap[overrideVolume.Name]; ok {
			// Already Merged
			continue
		}
		mergedVolumes = append(mergedVolumes, overrideVolume)
	}

	sort.SliceStable(mergedVolumes, func(i, j int) bool {
		return mergedVolumes[i].Name < mergedVolumes[j].Name
	})
	return mergedVolumes
}

func Volume(defaultVolume corev1.Volume, overrideVolume corev1.Volume) corev1.Volume {
	// Volume contains only Name and VolumeSource

	// Merge VolumeSource
	overrideSource := overrideVolume.VolumeSource
	defaultSource := defaultVolume.VolumeSource
	mergedVolume := defaultVolume

	// Only one field must be non-nil.
	// We merge if it is one of the ones we fill from the operator side:
	// - EmptyDir
	if overrideSource.EmptyDir != nil && defaultSource.EmptyDir != nil {
		if overrideSource.EmptyDir.Medium != "" {
			mergedVolume.EmptyDir.Medium = overrideSource.EmptyDir.Medium
		}
		if overrideSource.EmptyDir.SizeLimit != nil {
			mergedVolume.EmptyDir.SizeLimit = overrideSource.EmptyDir.SizeLimit
		}
		return mergedVolume
	}
	// - Secret
	if overrideSource.Secret != nil && defaultSource.Secret != nil {
		if overrideSource.Secret.SecretName != "" {
			mergedVolume.Secret.SecretName = overrideSource.Secret.SecretName
		}
		mergedVolume.Secret.Items = mergeKeyToPathItems(defaultSource.Secret.Items, overrideSource.Secret.Items)
		if overrideSource.Secret.DefaultMode != nil {
			mergedVolume.Secret.DefaultMode = overrideSource.Secret.DefaultMode
		}
		return mergedVolume
	}
	// - ConfigMap
	if overrideSource.ConfigMap != nil && defaultSource.ConfigMap != nil {
		if overrideSource.ConfigMap.LocalObjectReference.Name != "" {
			mergedVolume.ConfigMap.LocalObjectReference.Name = overrideSource.ConfigMap.LocalObjectReference.Name
		}
		mergedVolume.ConfigMap.Items = mergeKeyToPathItems(defaultSource.ConfigMap.Items, overrideSource.ConfigMap.Items)
		if overrideSource.ConfigMap.DefaultMode != nil {
			mergedVolume.ConfigMap.DefaultMode = overrideSource.ConfigMap.DefaultMode
		}
		if overrideSource.ConfigMap.Optional != nil {
			mergedVolume.ConfigMap.Optional = overrideSource.ConfigMap.Optional
		}
		return mergedVolume
	}

	// Otherwise we assume that the user provides every field
	// and we just assign it and nil every other field
	// We also do that in the case the user provides one of the three listed above
	// but our volume has a different non-nil entry.

	// this is effectively the same as just returning the overrideSource
	mergedVolume.VolumeSource = overrideSource
	return mergedVolume
}

func createVolumesMap(volumes []corev1.Volume) map[string]corev1.Volume {
	volumesMap := make(map[string]corev1.Volume)
	for _, v := range volumes {
		volumesMap[v.Name] = v
	}
	return volumesMap
}

func mergeKeyToPathItems(defaultItems []corev1.KeyToPath, overrideItems []corev1.KeyToPath) []corev1.KeyToPath {
	// Merge Items array by KeyToPath.Key entry
	defaultItemsMap := createKeyToPathMap(defaultItems)
	overrideItemsMap := createKeyToPathMap(overrideItems)
	var mergedItems []corev1.KeyToPath
	for _, defaultItem := range defaultItemsMap {
		mergedKey := defaultItem
		if overrideItem, ok := overrideItemsMap[defaultItem.Key]; ok {
			// Need to merge
			mergedKey = mergeKeyToPath(defaultItem, overrideItem)
		}
		mergedItems = append(mergedItems, mergedKey)
	}
	for _, overrideItem := range overrideItemsMap {
		if _, ok := defaultItemsMap[overrideItem.Key]; ok {
			// Already merged
			continue
		}
		mergedItems = append(mergedItems, overrideItem)

	}
	return mergedItems
}

func mergeKeyToPath(defaultKey corev1.KeyToPath, overrideKey corev1.KeyToPath) corev1.KeyToPath {
	if defaultKey.Key != overrideKey.Key {
		// This should not happen as we always merge by Key.
		// If it does, we return the default as something's wrong
		return defaultKey
	}
	if overrideKey.Path != "" {
		defaultKey.Path = overrideKey.Path
	}
	if overrideKey.Mode != nil {
		defaultKey.Mode = overrideKey.Mode
	}
	return defaultKey
}

func createKeyToPathMap(items []corev1.KeyToPath) map[string]corev1.KeyToPath {
	itemsMap := make(map[string]corev1.KeyToPath)
	for _, v := range items {
		itemsMap[v.Key] = v
	}
	return itemsMap
}

// Affinity merges two corev1.Affinity types.
func Affinity(defaultAffinity, overrideAffinity *corev1.Affinity) *corev1.Affinity {
	if defaultAffinity == nil {
		return overrideAffinity
	}
	if overrideAffinity == nil {
		return defaultAffinity
	}
	mergedAffinity := defaultAffinity.DeepCopy()
	if overrideAffinity.NodeAffinity != nil {
		mergedAffinity.NodeAffinity = overrideAffinity.NodeAffinity
	}
	if overrideAffinity.PodAffinity != nil {
		mergedAffinity.PodAffinity = overrideAffinity.PodAffinity
	}
	if overrideAffinity.PodAntiAffinity != nil {
		mergedAffinity.PodAntiAffinity = overrideAffinity.PodAntiAffinity
	}
	return mergedAffinity
}
