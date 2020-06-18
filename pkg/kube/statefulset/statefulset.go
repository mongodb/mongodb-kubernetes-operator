package statefulset

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// VolumeMountData contains values required for the MountVolume function
type VolumeMountData struct {
	Name      string
	MountPath string
	Volume    corev1.Volume
}

func CreateVolumeFromConfigMap(name, sourceName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: sourceName,
				},
			},
		},
	}
}

func CreateVolumeFromSecret(name, sourceName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: sourceName,
			},
		},
	}
}

func CreateVolumeFromEmptyDir(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			// No options EmptyDir means default storage medium and size.
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// CreateVolumeMount returns a corev1.VolumeMount with options.
func CreateVolumeMount(name, path string, options ...func(*corev1.VolumeMount)) corev1.VolumeMount {
	volumeMount := &corev1.VolumeMount{
		Name:      name,
		MountPath: path,
	}
	for _, option := range options {
		option(volumeMount)
	}
	return *volumeMount
}

// WithSubPath sets the SubPath for this VolumeMount
func WithSubPath(subPath string) func(*corev1.VolumeMount) {
	return func(v *corev1.VolumeMount) {
		v.SubPath = subPath
	}
}

// WithReadOnly sets the ReadOnly attribute of this VolumeMount
func WithReadOnly(readonly bool) func(*corev1.VolumeMount) {
	return func(v *corev1.VolumeMount) {
		v.ReadOnly = readonly
	}
}

func IsReady(sts appsv1.StatefulSet, expectedReplicas int) bool {
	allUpdated := int32(expectedReplicas) == sts.Status.UpdatedReplicas
	allReady := int32(expectedReplicas) == sts.Status.ReadyReplicas
	return allUpdated && allReady
}

type Modification func(*appsv1.StatefulSet)

func WithName(name string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Name = name
	}
}

func WithNamespace(namespace string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Namespace = namespace
	}
}

func WithServiceName(svcName string) Modification {
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.ServiceName = svcName
	}
}

func WithLabels(labels map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Labels = copyMap(labels)
	}
}
func WithMatchLabels(matchLabels map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		if set.Spec.Selector == nil {
			set.Spec.Selector = &metav1.LabelSelector{}
		}
		set.Spec.Selector.MatchLabels = copyMap(matchLabels)
	}
}
func WithOwnerReference(ownerRefs []metav1.OwnerReference) Modification {
	ownerReference := make([]metav1.OwnerReference, len(ownerRefs))
	copy(ownerReference, ownerRefs)
	return func(set *appsv1.StatefulSet) {
		set.OwnerReferences = ownerReference
	}
}

func WithReplicas(replicas int) Modification {
	stsReplicas := int32(replicas)
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.Replicas = &stsReplicas
	}
}

func WithUpdateStrategyType(strategyType appsv1.StatefulSetUpdateStrategyType) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: strategyType,
		}
	}
}

func WithPodSpecTemplate(templateFunc func(*corev1.PodTemplateSpec)) Modification {
	return func(set *appsv1.StatefulSet) {
		template := &set.Spec.Template
		templateFunc(template)
	}
}

func WithVolumeClaim(name string, f func(*corev1.PersistentVolumeClaim)) Modification {
	return func(set *appsv1.StatefulSet) {
		idx := findVolumeClaimIndexByName(name, set.Spec.VolumeClaimTemplates)
		if idx == -1 {
			set.Spec.VolumeClaimTemplates = append(set.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{})
			idx = len(set.Spec.VolumeClaimTemplates) - 1
		}
		pvc := &set.Spec.VolumeClaimTemplates[idx]
		f(pvc)
	}
}

func findVolumeClaimIndexByName(name string, pvcs []corev1.PersistentVolumeClaim) int {
	for idx, pvc := range pvcs {
		if pvc.Name == name {
			return idx
		}
	}
	return -1
}

func Apply(funcs ...Modification) func(*appsv1.StatefulSet) {
	return func(sts *appsv1.StatefulSet) {
		for _, f := range funcs {
			f(sts)
		}
	}
}
