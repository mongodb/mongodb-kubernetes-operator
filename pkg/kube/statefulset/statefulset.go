package statefulset

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imdario/mergo"
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

// HaveEqualSpec accepts a StatefulSet builtSts, and a second existingSts, and compares
// the Spec of both inputs but only comparing the fields that were specified in builtSts
func HaveEqualSpec(builtSts appsv1.StatefulSet, existingSts appsv1.StatefulSet) (bool, error) {
	stsToMerge := *existingSts.DeepCopyObject().(*appsv1.StatefulSet)
	if err := mergo.Merge(&stsToMerge, builtSts, mergo.WithOverride); err != nil {
		return false, err
	}
	return reflect.DeepEqual(stsToMerge.Spec, existingSts.Spec), nil
}

type ModificationFunc func(*appsv1.StatefulSet)

func WithName(name string) ModificationFunc {
	return func(sts *appsv1.StatefulSet) {
		sts.Name = name
	}
}

func WithNamespace(namespace string) ModificationFunc {
	return func(sts *appsv1.StatefulSet) {
		sts.Namespace = namespace
	}
}

func WithServiceName(svcName string) ModificationFunc {
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.ServiceName = svcName
	}
}

func WithLabels(labels map[string]string) ModificationFunc {
	return func(set *appsv1.StatefulSet) {
		set.Labels = copyMap(labels)
	}
}
func WithMatchLabels(matchLabels map[string]string) ModificationFunc {
	return func(set *appsv1.StatefulSet) {
		if set.Spec.Selector == nil {
			set.Spec.Selector = &metav1.LabelSelector{}
		}
		set.Spec.Selector.MatchLabels = copyMap(matchLabels)
	}
}
func WithOwnerReference(ownerRefs []metav1.OwnerReference) ModificationFunc {
	ownerReference := make([]metav1.OwnerReference, len(ownerRefs))
	copy(ownerReference, ownerRefs)
	return func(set *appsv1.StatefulSet) {
		set.OwnerReferences = ownerReference
	}
}

func WithReplicas(replicas int) ModificationFunc {
	stsReplicas := int32(replicas)
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.Replicas = &stsReplicas
	}
}

func WithUpdateStrategyType(strategyType appsv1.StatefulSetUpdateStrategyType) ModificationFunc {
	return func(set *appsv1.StatefulSet) {
		set.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: strategyType,
		}
	}
}

func WithPodSpecTemplate(templateFunc func(*corev1.PodTemplateSpec)) ModificationFunc {
	return func(set *appsv1.StatefulSet) {
		template := &set.Spec.Template
		templateFunc(template)
	}
}

func WithVolumeClaims(volumeClaims []corev1.PersistentVolumeClaim) ModificationFunc {
	volumeClaimsTemplates := make([]corev1.PersistentVolumeClaim, len(volumeClaims))
	copy(volumeClaimsTemplates, volumeClaims)
	return func(set *appsv1.StatefulSet) {
		set.Spec.VolumeClaimTemplates = volumeClaimsTemplates
	}
}

func Modify(funcs ...ModificationFunc) func(*appsv1.StatefulSet) {
	return func(sts *appsv1.StatefulSet) {
		for _, f := range funcs {
			f(sts)
		}
	}
}
