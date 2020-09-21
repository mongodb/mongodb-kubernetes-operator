package podtemplatespec

import (
	"sort"

	"github.com/imdario/mergo"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/envvar"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Modification func(*corev1.PodTemplateSpec)

const (
	notFound = -1
)

func New(templateMods ...Modification) corev1.PodTemplateSpec {
	podTemplateSpec := corev1.PodTemplateSpec{}
	for _, templateMod := range templateMods {
		templateMod(&podTemplateSpec)
	}
	return podTemplateSpec
}

// Apply returns a function which applies a series of Modification functions to a *corev1.PodTemplateSpec
func Apply(templateMods ...Modification) Modification {
	return func(template *corev1.PodTemplateSpec) {
		for _, f := range templateMods {
			f(template)
		}
	}
}

// NOOP is a valid Modification which applies no changes
func NOOP() Modification {
	return func(spec *corev1.PodTemplateSpec) {}
}

// WithContainer applies the modifications to the container with the provided name
func WithContainer(name string, containerfunc func(*corev1.Container)) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		idx := findIndexByName(name, podTemplateSpec.Spec.Containers)
		if idx == notFound {
			// if we are attempting to modify a container that does not exist, we will add a new one
			podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, corev1.Container{})
			idx = len(podTemplateSpec.Spec.Containers) - 1
		}
		c := &podTemplateSpec.Spec.Containers[idx]
		containerfunc(c)
	}
}

// WithContainerByIndex applies the modifications to the container with the provided index
// if the index is out of range, a new container is added to accept these changes.
func WithContainerByIndex(index int, funcs ...func(container *corev1.Container)) func(podTemplateSpec *corev1.PodTemplateSpec) {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		if index >= len(podTemplateSpec.Spec.Containers) {
			podTemplateSpec.Spec.Containers = append(podTemplateSpec.Spec.Containers, corev1.Container{})
		}
		c := &podTemplateSpec.Spec.Containers[index]
		for _, f := range funcs {
			f(c)
		}
	}
}

// WithInitContainer applies the modifications to the init container with the provided name
func WithInitContainer(name string, containerfunc func(*corev1.Container)) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		idx := findIndexByName(name, podTemplateSpec.Spec.InitContainers)
		if idx == notFound {
			// if we are attempting to modify a container that does not exist, we will add a new one
			podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, corev1.Container{})
			idx = len(podTemplateSpec.Spec.InitContainers) - 1
		}
		c := &podTemplateSpec.Spec.InitContainers[idx]
		containerfunc(c)
	}
}

// WithInitContainerByIndex applies the modifications to the container with the provided index
// if the index is out of range, a new container is added to accept these changes.
func WithInitContainerByIndex(index int, funcs ...func(container *corev1.Container)) func(podTemplateSpec *corev1.PodTemplateSpec) {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		if index >= len(podTemplateSpec.Spec.InitContainers) {
			podTemplateSpec.Spec.InitContainers = append(podTemplateSpec.Spec.InitContainers, corev1.Container{})
		}
		c := &podTemplateSpec.Spec.InitContainers[index]
		for _, f := range funcs {
			f(c)
		}
	}
}

// WithPodLabels sets the PodTemplateSpec's Labels
func WithPodLabels(labels map[string]string) Modification {
	if labels == nil {
		labels = map[string]string{}
	}
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.ObjectMeta.Labels = labels
	}
}

// WithServiceAccount sets the PodTemplateSpec's ServiceAccount name
func WithServiceAccount(serviceAccountName string) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.ServiceAccountName = serviceAccountName
	}
}

// WithVolume ensures the given volume exists
func WithVolume(volume corev1.Volume) Modification {
	return func(template *corev1.PodTemplateSpec) {
		for _, v := range template.Spec.Volumes {
			if v.Name == volume.Name {
				return
			}
		}
		template.Spec.Volumes = append(template.Spec.Volumes, volume)
	}
}

func findIndexByName(name string, containers []corev1.Container) int {
	for idx, c := range containers {
		if c.Name == name {
			return idx
		}
	}
	return notFound
}

// WithTerminationGracePeriodSeconds sets the PodTemplateSpec's termination grace period seconds
func WithTerminationGracePeriodSeconds(seconds int) Modification {
	s := int64(seconds)
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.TerminationGracePeriodSeconds = &s
	}
}

// WithSecurityContext sets the PodTemplateSpec's SecurityContext
func WithSecurityContext(securityContext corev1.PodSecurityContext) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		spec := &podTemplateSpec.Spec
		spec.SecurityContext = &securityContext
	}
}

// WithImagePullSecrets adds an ImagePullSecrets local reference with the given name
func WithImagePullSecrets(name string) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		for _, v := range podTemplateSpec.Spec.ImagePullSecrets {
			if v.Name == name {
				return
			}
		}
		podTemplateSpec.Spec.ImagePullSecrets = append(podTemplateSpec.Spec.ImagePullSecrets, corev1.LocalObjectReference{
			Name: name,
		})
	}
}

// WithTopologyKey sets the PodTemplateSpec's topology at a given index
func WithTopologyKey(topologyKey string, idx int) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[idx].PodAffinityTerm.TopologyKey = topologyKey
	}
}

// WithAffinity updates the name, antiAffinityLabelKey and weight of the PodTemplateSpec's Affinity
func WithAffinity(stsName, antiAffinityLabelKey string, weight int) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.Affinity =
			&corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
						Weight: int32(weight),
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{antiAffinityLabelKey: stsName}},
						},
					}},
				},
			}
	}
}

// WithNodeAffinity sets the PodTemplateSpec's node affinity
func WithNodeAffinity(nodeAffinity *corev1.NodeAffinity) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.Affinity.NodeAffinity = nodeAffinity
	}
}

// WithPodAffinity sets the PodTemplateSpec's pod affinity
func WithPodAffinity(podAffinity *corev1.PodAffinity) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.Affinity.PodAffinity = podAffinity
	}
}

// WithTolerations sets the PodTemplateSpec's tolerations
func WithTolerations(tolerations []corev1.Toleration) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Spec.Tolerations = tolerations
	}
}

// WithAnnotations sets the PodTemplateSpec's annotations
func WithAnnotations(annotations map[string]string) Modification {
	if annotations == nil {
		annotations = map[string]string{}
	}
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		podTemplateSpec.Annotations = annotations
	}
}

// WithVolumeMounts will add volume mounts to a container or init container by name
func WithVolumeMounts(containerName string, volumeMounts ...corev1.VolumeMount) Modification {
	return func(podTemplateSpec *corev1.PodTemplateSpec) {
		c := findContainerByName(containerName, podTemplateSpec)
		if c == nil {
			return
		}
		container.WithVolumeMounts(volumeMounts)(c)
	}
}

func MergePodTemplateSpecs(defaultTemplate, overrideTemplate corev1.PodTemplateSpec) (corev1.PodTemplateSpec, error) {
	// Containers need to be merged manually
	mergedContainers, err := mergeContainers(defaultTemplate.Spec.Containers, overrideTemplate.Spec.Containers)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	mergedTolerations := mergeTolerations(defaultTemplate.Spec.Tolerations, overrideTemplate.Spec.Tolerations)

	// InitContainers need to be merged manually
	mergedInitContainers, err := mergeContainers(defaultTemplate.Spec.InitContainers, overrideTemplate.Spec.InitContainers)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	// Affinity needs to be merged manually
	mergedAffinity, err := mergeAffinity(defaultTemplate.Spec.Affinity, overrideTemplate.Spec.Affinity)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	mergedVolumes := mergeVolumes(defaultTemplate.Spec.Volumes, overrideTemplate.Spec.Volumes)

	// Everything else can be merged with mergo
	mergedPodTemplateSpec := *defaultTemplate.DeepCopy()
	if err = mergo.Merge(&mergedPodTemplateSpec, overrideTemplate, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	mergedPodTemplateSpec.Spec.Containers = mergedContainers
	mergedPodTemplateSpec.Spec.Tolerations = mergedTolerations
	mergedPodTemplateSpec.Spec.InitContainers = mergedInitContainers
	mergedPodTemplateSpec.Spec.Affinity = mergedAffinity
	mergedPodTemplateSpec.Spec.Volumes = mergedVolumes
	return mergedPodTemplateSpec, nil
}

func createKeyToPathMap(items []corev1.KeyToPath) map[string]corev1.KeyToPath {
	itemsMap := make(map[string]corev1.KeyToPath)
	for _, v := range items {
		itemsMap[v.Key] = v
	}
	return itemsMap
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

func mergeKeyToPathItems(defaultItems []corev1.KeyToPath, overrideItems []corev1.KeyToPath) []corev1.KeyToPath {
	// Merge Items array by KeyToPath.Key entry
	defaultItemsMap := createKeyToPathMap(defaultItems)
	overrideItemsMap := createKeyToPathMap(overrideItems)
	mergedItems := []corev1.KeyToPath{}
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

func mergeVolume(defaultVolume corev1.Volume, overrideVolume corev1.Volume) corev1.Volume {
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
		if len(overrideSource.Secret.Items) > 0 {
			mergedVolume.Secret.Items = mergeKeyToPathItems(defaultSource.Secret.Items, overrideSource.Secret.Items)
		}
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
		if len(overrideSource.ConfigMap.Items) > 0 {
			mergedVolume.ConfigMap.Items = mergeKeyToPathItems(defaultSource.ConfigMap.Items, overrideSource.ConfigMap.Items)
		}
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

func mergeVolumes(defaultVolumes []corev1.Volume, overrideVolumes []corev1.Volume) []corev1.Volume {
	defaultVolumesMap := createVolumesMap(defaultVolumes)
	overrideVolumesMap := createVolumesMap(overrideVolumes)
	mergedVolumes := []corev1.Volume{}

	for _, defaultVolume := range defaultVolumes {
		mergedVolume := defaultVolume
		if overrideVolume, ok := overrideVolumesMap[defaultVolume.Name]; ok {
			mergedVolume = mergeVolume(defaultVolume, overrideVolume)
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

func mergeVolumeMounts(defaultMounts, overrideMounts []corev1.VolumeMount) ([]corev1.VolumeMount, error) {
	defaultMountsMap := createMountsMap(defaultMounts)
	overrideMountsMap := createMountsMap(overrideMounts)
	mergedVolumeMounts := []corev1.VolumeMount{}
	for _, defaultMount := range defaultMounts {
		if overrideMount, ok := overrideMountsMap[defaultMount.Name]; ok {
			// needs merge
			if err := mergo.Merge(&defaultMount, overrideMount, mergo.WithAppendSlice); err != nil { //nolint
				return nil, err
			}
		}
		mergedVolumeMounts = append(mergedVolumeMounts, defaultMount)
	}
	for _, overrideMount := range overrideMounts {
		if _, ok := defaultMountsMap[overrideMount.Name]; ok {
			// already merged
			continue
		}
		mergedVolumeMounts = append(mergedVolumeMounts, overrideMount)
	}
	return mergedVolumeMounts, nil
}

func createMountsMap(volumeMounts []corev1.VolumeMount) map[string]corev1.VolumeMount {
	mountMap := make(map[string]corev1.VolumeMount)
	for _, m := range volumeMounts {
		mountMap[m.Name] = m
	}
	return mountMap
}

func createTolerationsMap(tolerations []corev1.Toleration) map[string]corev1.Toleration {
	tolerationsMap := make(map[string]corev1.Toleration)
	for _, t := range tolerations {
		tolerationsMap[t.Key] = t
	}
	return tolerationsMap
}

func mergeTolerations(defaultTolerations, overrideTolerations []corev1.Toleration) []corev1.Toleration {
	mergedTolerations := make([]corev1.Toleration, 0)
	defaultMap := createTolerationsMap(defaultTolerations)
	for _, v := range overrideTolerations {
		defaultMap[v.Key] = v
	}

	for _, v := range defaultMap {
		mergedTolerations = append(mergedTolerations, v)
	}

	if len(mergedTolerations) == 0 {
		return nil
	}

	sort.SliceStable(mergedTolerations, func(i, j int) bool {
		return mergedTolerations[i].Key < mergedTolerations[j].Key
	})

	return mergedTolerations
}

func mergeContainers(defaultContainers, customContainers []corev1.Container) ([]corev1.Container, error) {
	defaultMap := createContainerMap(defaultContainers)
	customMap := createContainerMap(customContainers)
	mergedContainers := []corev1.Container{}
	for _, defaultContainer := range defaultContainers {
		if customContainer, ok := customMap[defaultContainer.Name]; ok {
			// The container is present in both maps, so we need to merge
			// MergeWithOverride mounts
			mergedMounts, err := mergeVolumeMounts(defaultContainer.VolumeMounts, customContainer.VolumeMounts)
			if err != nil {
				return nil, err
			}

			mergedEnvs := envvar.MergeWithOverride(defaultContainer.Env, customContainer.Env)

			if err := mergo.Merge(&defaultContainer, customContainer, mergo.WithOverride); err != nil { //nolint
				return nil, err
			}
			// completely override any resources that were provided
			// this prevents issues with custom requests giving errors due
			// to the defaulted limits
			defaultContainer.Resources = customContainer.Resources
			defaultContainer.VolumeMounts = mergedMounts
			defaultContainer.Env = mergedEnvs
		}
		// The default container was not modified by the override, so just add it
		mergedContainers = append(mergedContainers, defaultContainer)
	}

	// Look for customContainers that were not merged into existing ones
	for _, customContainer := range customContainers {
		if _, ok := defaultMap[customContainer.Name]; ok {
			continue
		}
		// Need to add it
		mergedContainers = append(mergedContainers, customContainer)
	}

	return mergedContainers, nil
}

func createContainerMap(containers []corev1.Container) map[string]corev1.Container {
	containerMap := make(map[string]corev1.Container)
	for _, c := range containers {
		containerMap[c.Name] = c
	}
	return containerMap
}

func mergeAffinity(defaultAffinity, overrideAffinity *corev1.Affinity) (*corev1.Affinity, error) {
	if defaultAffinity == nil {
		return overrideAffinity, nil
	}
	if overrideAffinity == nil {
		return defaultAffinity, nil
	}
	mergedAffinity := defaultAffinity.DeepCopy()
	if err := mergo.Merge(mergedAffinity, *overrideAffinity, mergo.WithOverride); err != nil {
		return nil, err
	}
	return mergedAffinity, nil
}

// findContainerByName will find either a container or init container by name in a pod template spec
func findContainerByName(name string, podTemplateSpec *corev1.PodTemplateSpec) *corev1.Container {
	containerIdx := findIndexByName(name, podTemplateSpec.Spec.Containers)
	if containerIdx != notFound {
		return &podTemplateSpec.Spec.Containers[containerIdx]
	}

	initIdx := findIndexByName(name, podTemplateSpec.Spec.InitContainers)
	if initIdx != notFound {
		return &podTemplateSpec.Spec.InitContainers[initIdx]
	}

	return nil
}
