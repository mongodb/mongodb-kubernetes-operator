package merge

import (
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// StatefulSets merges two StatefulSets together.
func StatefulSets(defaultStatefulSet, overrideStatefulSet appsv1.StatefulSet) appsv1.StatefulSet {
	mergedSts := defaultStatefulSet
	mergedSts.Labels = StringToStringMap(defaultStatefulSet.Labels, overrideStatefulSet.Labels)
	if overrideStatefulSet.Namespace != "" {
		mergedSts.Namespace = overrideStatefulSet.Namespace
	}
	if overrideStatefulSet.Name != "" {
		mergedSts.Name = overrideStatefulSet.Name
	}
	mergedSts.Spec = StatefulSetSpecs(defaultStatefulSet.Spec, overrideStatefulSet.Spec)
	return mergedSts
}

// StatefulSetSpecs merges two StatefulSetSpecs together.
func StatefulSetSpecs(defaultSpec, overrideSpec appsv1.StatefulSetSpec) appsv1.StatefulSetSpec {
	mergedSpec := defaultSpec
	if overrideSpec.Replicas != nil {
		mergedSpec.Replicas = overrideSpec.Replicas
	}

	mergedSpec.Selector = LabelSelectors(defaultSpec.Selector, overrideSpec.Selector)

	if overrideSpec.PodManagementPolicy != "" {
		mergedSpec.PodManagementPolicy = overrideSpec.PodManagementPolicy
	}

	if overrideSpec.RevisionHistoryLimit != nil {
		mergedSpec.RevisionHistoryLimit = overrideSpec.RevisionHistoryLimit
	}

	if overrideSpec.UpdateStrategy.Type != "" {
		mergedSpec.UpdateStrategy.Type = overrideSpec.UpdateStrategy.Type
	}

	if overrideSpec.UpdateStrategy.RollingUpdate != nil {
		mergedSpec.UpdateStrategy.RollingUpdate = overrideSpec.UpdateStrategy.RollingUpdate
	}

	if overrideSpec.ServiceName != "" {
		mergedSpec.ServiceName = overrideSpec.ServiceName
	}

	mergedSpec.Template = PodTemplateSpecs(defaultSpec.Template, overrideSpec.Template)
	mergedSpec.VolumeClaimTemplates = VolumeClaimTemplates(defaultSpec.VolumeClaimTemplates, overrideSpec.VolumeClaimTemplates)
	return mergedSpec
}

func LabelSelectors(originalLabelSelector, overrideLabelSelector *metav1.LabelSelector) *metav1.LabelSelector {
	// we have only specified a label selector in the override
	if overrideLabelSelector == nil {
		return originalLabelSelector
	}
	// we have only specified a label selector in the original
	if originalLabelSelector == nil {
		return overrideLabelSelector
	}

	// we have specified both, so we must merge them
	mergedLabelSelector := &metav1.LabelSelector{}
	mergedLabelSelector.MatchLabels = StringToStringMap(originalLabelSelector.MatchLabels, overrideLabelSelector.MatchLabels)
	mergedLabelSelector.MatchExpressions = LabelSelectorRequirements(originalLabelSelector.MatchExpressions, overrideLabelSelector.MatchExpressions)
	return mergedLabelSelector
}

// LabelSelectorRequirements accepts two slices of LabelSelectorRequirement. Any LabelSelectorRequirement in the override
// slice that has the same key as one from the original is merged. Otherwise they are appended to the list.
func LabelSelectorRequirements(original, override []metav1.LabelSelectorRequirement) []metav1.LabelSelectorRequirement {
	mergedLsrs := make([]metav1.LabelSelectorRequirement, 0)
	for _, originalLsr := range original {
		mergedLsr := originalLsr
		overrideLsr := LabelSelectorRequirementByKey(override, originalLsr.Key)
		if overrideLsr != nil {
			if overrideLsr.Operator != "" {
				mergedLsr.Operator = overrideLsr.Operator
			}
			if overrideLsr.Values != nil {
				mergedLsr.Values = StringSlices(originalLsr.Values, overrideLsr.Values)
			}
		}
		sort.SliceStable(mergedLsr.Values, func(i, j int) bool {
			return mergedLsr.Values[i] < mergedLsr.Values[j]
		})

		mergedLsrs = append(mergedLsrs, mergedLsr)
	}

	// we need to add any override lsrs that do not exist in the original
	for _, overrideLsr := range override {
		existing := LabelSelectorRequirementByKey(original, overrideLsr.Key)
		if existing == nil {
			sort.SliceStable(overrideLsr.Values, func(i, j int) bool {
				return overrideLsr.Values[i] < overrideLsr.Values[j]
			})
			mergedLsrs = append(mergedLsrs, overrideLsr)
		}
	}

	// sort them by key
	sort.SliceStable(mergedLsrs, func(i, j int) bool {
		return mergedLsrs[i].Key < mergedLsrs[j].Key
	})

	return mergedLsrs
}

// LabelSelectorRequirementByKey returns the LabelSelectorRequirement with the given key if present in the slice.
// returns nil if not present.
func LabelSelectorRequirementByKey(labelSelectorRequirements []metav1.LabelSelectorRequirement, key string) *metav1.LabelSelectorRequirement {
	for _, lsr := range labelSelectorRequirements {
		if lsr.Key == key {
			return &lsr
		}
	}
	return nil
}

func VolumeClaimTemplates(defaultTemplates []corev1.PersistentVolumeClaim, overrideTemplates []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	defaultMountsMap := createVolumeClaimMap(defaultTemplates)
	overrideMountsMap := createVolumeClaimMap(overrideTemplates)

	mergedMap := map[string]corev1.PersistentVolumeClaim{}

	for _, vct := range defaultMountsMap {
		mergedMap[vct.Name] = vct
	}

	for _, overrideClaim := range overrideMountsMap {
		if defaultClaim, ok := defaultMountsMap[overrideClaim.Name]; ok {
			mergedMap[overrideClaim.Name] = PersistentVolumeClaim(defaultClaim, overrideClaim)
		} else {
			mergedMap[overrideClaim.Name] = overrideClaim
		}
	}

	var mergedVolumes []corev1.PersistentVolumeClaim
	for _, v := range mergedMap {
		mergedVolumes = append(mergedVolumes, v)
	}

	sort.SliceStable(mergedVolumes, func(i, j int) bool {
		return mergedVolumes[i].Name < mergedVolumes[j].Name
	})

	return mergedVolumes
}

func createVolumeClaimMap(volumeMounts []corev1.PersistentVolumeClaim) map[string]corev1.PersistentVolumeClaim {
	mountMap := make(map[string]corev1.PersistentVolumeClaim)
	for _, m := range volumeMounts {
		mountMap[m.Name] = m
	}
	return mountMap
}

func PersistentVolumeClaim(defaultPvc corev1.PersistentVolumeClaim, overridePvc corev1.PersistentVolumeClaim) corev1.PersistentVolumeClaim {
	if overridePvc.Namespace != "" {
		defaultPvc.Namespace = overridePvc.Namespace
	}

	defaultPvc.Labels = StringToStringMap(defaultPvc.Labels, overridePvc.Labels)
	defaultPvc.Annotations = StringToStringMap(defaultPvc.Annotations, overridePvc.Annotations)

	if overridePvc.Spec.VolumeMode != nil {
		defaultPvc.Spec.VolumeMode = overridePvc.Spec.VolumeMode
	}

	if overridePvc.Spec.StorageClassName != nil {
		defaultPvc.Spec.StorageClassName = overridePvc.Spec.StorageClassName
	}

	for _, accessMode := range overridePvc.Spec.AccessModes {
		if !contains.AccessMode(defaultPvc.Spec.AccessModes, accessMode) {
			defaultPvc.Spec.AccessModes = append(defaultPvc.Spec.AccessModes, accessMode)
		}
	}

	if overridePvc.Spec.Selector != nil {
		defaultPvc.Spec.Selector = overridePvc.Spec.Selector
	}

	if overridePvc.Spec.Resources.Limits != nil {
		defaultPvc.Spec.Resources.Limits = overridePvc.Spec.Resources.Limits
	}

	if overridePvc.Spec.Resources.Requests != nil {
		defaultPvc.Spec.Resources.Requests = overridePvc.Spec.Resources.Requests
	}

	if overridePvc.Spec.DataSource != nil {
		defaultPvc.Spec.DataSource = overridePvc.Spec.DataSource
	}

	return defaultPvc
}
