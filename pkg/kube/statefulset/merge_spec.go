package statefulset

import (
	"sort"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/merge"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MergeSpecs takes an original and an override spec. A new StatefulSetSpec is returned with has had
// any changes from the override applied on top of the original.
func MergeSpecs(originalSpec appsv1.StatefulSetSpec, overrideSpec appsv1.StatefulSetSpec) appsv1.StatefulSetSpec {
	mergedSpec := originalSpec

	if overrideSpec.Replicas != nil {
		mergedSpec.Replicas = overrideSpec.Replicas
	}

	if overrideSpec.Selector != nil {
		mergedSpec.Selector = mergeLabelSelectors(originalSpec.Selector, overrideSpec.Selector)
	}

	// TODO: merge Template corev1.PodTemplateSpec

	// TODO: merge VolumeClaimTemplates []v1.PersistentVolumeClaim

	if overrideSpec.ServiceName != "" {
		mergedSpec.ServiceName = overrideSpec.ServiceName
	}

	if overrideSpec.PodManagementPolicy != "" {
		mergedSpec.PodManagementPolicy = overrideSpec.PodManagementPolicy
	}

	if overrideSpec.RevisionHistoryLimit != nil {
		mergedSpec.RevisionHistoryLimit = overrideSpec.RevisionHistoryLimit
	}

	return mergedSpec
}

func mergeLabelSelectors(originalLabelSelector, overrideLabelSelector *metav1.LabelSelector) *metav1.LabelSelector {
	// we have only specified a label selector in the override
	if originalLabelSelector == nil && overrideLabelSelector != nil {
		return overrideLabelSelector
	}
	// we have only specified a label selector in the original
	if originalLabelSelector != nil && overrideLabelSelector == nil {
		return originalLabelSelector
	}

	// we have specified both, so we must merge them
	mergedLabelSelector := &metav1.LabelSelector{}
	mergedLabelSelector.MatchLabels = mergeStringToStringMaps(originalLabelSelector.MatchLabels, overrideLabelSelector.MatchLabels)
	mergedLabelSelector.MatchExpressions = mergeLabelSelectorRequirements(originalLabelSelector.MatchExpressions, overrideLabelSelector.MatchExpressions)

	return mergedLabelSelector
}

// mergeStringToStringMaps returns a map containing all the keys of the original and override provided.
// with any duplicate keys, values of the override will take precedence. A nil map is never returned.
func mergeStringToStringMaps(original, override map[string]string) map[string]string {
	mergedMap := make(map[string]string)
	for k, v := range original {
		mergedMap[k] = v
	}
	for k, v := range override {
		mergedMap[k] = v
	}
	return mergedMap
}

// mergeLabelSelectorRequirements accepts two slices of LabelSelectorRequirement. Any LabelSelectorRequirement in the override
// slice that has the same key as one from the original is merged. Otherwise they are appended to the list.
func mergeLabelSelectorRequirements(original, override []metav1.LabelSelectorRequirement) []metav1.LabelSelectorRequirement {
	mergedLsrs := make([]metav1.LabelSelectorRequirement, 0)
	for _, originalLsr := range original {
		mergedLsr := originalLsr
		overrideLsr := getLabelSelectorRequirementByKey(override, originalLsr.Key)
		if overrideLsr != nil {
			if overrideLsr.Operator != "" {
				mergedLsr.Operator = overrideLsr.Operator
			}
			if overrideLsr.Values != nil {
				mergedLsr.Values = merge.StringSlices(originalLsr.Values, overrideLsr.Values)
			}
		}
		sort.SliceStable(mergedLsr.Values, func(i, j int) bool {
			return mergedLsr.Values[i] < mergedLsr.Values[j]
		})

		mergedLsrs = append(mergedLsrs, mergedLsr)
	}

	// we need to add any override lsrs that do not exist in the original
	for _, overrideLsr := range override {
		existing := getLabelSelectorRequirementByKey(original, overrideLsr.Key)
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

// getLabelSelectorRequirementByKey returns the LabelSelectorRequirement with the given key if present in the slice.
// returns nil if not present.
func getLabelSelectorRequirementByKey(labelSelectorRequirements []metav1.LabelSelectorRequirement, key string) *metav1.LabelSelectorRequirement {
	for _, lsr := range labelSelectorRequirements {
		if lsr.Key == key {
			return &lsr
		}
	}
	return nil
}
