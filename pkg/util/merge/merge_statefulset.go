package merge

import (
	"sort"

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

	if overrideSpec.Selector != nil {
		mergedSpec.Selector = overrideSpec.Selector
	}

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
