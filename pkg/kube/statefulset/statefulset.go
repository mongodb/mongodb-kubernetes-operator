package statefulset

import (
	"context"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/merge"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	notFound = -1
)

type Getter interface {
	GetStatefulSet(ctx context.Context, objectKey client.ObjectKey) (appsv1.StatefulSet, error)
}

type Updater interface {
	UpdateStatefulSet(ctx context.Context, sts appsv1.StatefulSet) (appsv1.StatefulSet, error)
}

type Creator interface {
	CreateStatefulSet(ctx context.Context, sts appsv1.StatefulSet) error
}

type Deleter interface {
	DeleteStatefulSet(ctx context.Context, objectKey client.ObjectKey) error
}

type GetUpdater interface {
	Getter
	Updater
}

type GetUpdateCreator interface {
	Getter
	Updater
	Creator
}

type GetUpdateCreateDeleter interface {
	Getter
	Updater
	Creator
	Deleter
}

// CreateOrUpdate creates the given StatefulSet if it doesn't exist,
// or updates it if it does.
func CreateOrUpdate(ctx context.Context, getUpdateCreator GetUpdateCreator, statefulSet appsv1.StatefulSet) (appsv1.StatefulSet, error) {
	if sts, err := getUpdateCreator.UpdateStatefulSet(ctx, statefulSet); err != nil {
		if apiErrors.IsNotFound(err) {
			return statefulSet, getUpdateCreator.CreateStatefulSet(ctx, statefulSet)
		} else {
			return appsv1.StatefulSet{}, err
		}
	} else {
		return sts, nil
	}
}

// GetAndUpdate applies the provided function to the most recent version of the object
func GetAndUpdate(ctx context.Context, getUpdater GetUpdater, nsName types.NamespacedName, updateFunc func(*appsv1.StatefulSet)) (appsv1.StatefulSet, error) {
	sts, err := getUpdater.GetStatefulSet(ctx, nsName)
	if err != nil {
		return appsv1.StatefulSet{}, err
	}
	// apply the function on the most recent version of the resource
	updateFunc(&sts)
	return getUpdater.UpdateStatefulSet(ctx, sts)
}

// VolumeMountData contains values required for the MountVolume function
type VolumeMountData struct {
	Name      string
	MountPath string
	Volume    corev1.Volume
	ReadOnly  bool
}

func CreateVolumeFromConfigMap(name, sourceName string, options ...func(v *corev1.Volume)) corev1.Volume {
	volume := &corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: sourceName,
				},
			},
		},
	}

	for _, option := range options {
		option(volume)
	}
	return *volume
}

func CreateVolumeFromSecret(name, sourceName string, options ...func(v *corev1.Volume)) corev1.Volume {
	permission := int32(416)
	volumeMount := &corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  sourceName,
				DefaultMode: &permission,
			},
		},
	}
	for _, option := range options {
		option(volumeMount)
	}
	return *volumeMount

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

// NOOP is a valid Modification which applies no changes
func NOOP() Modification {
	return func(sts *appsv1.StatefulSet) {}
}

func WithSecretDefaultMode(mode *int32) func(*corev1.Volume) {
	return func(v *corev1.Volume) {
		if v.VolumeSource.Secret == nil {
			v.VolumeSource.Secret = &corev1.SecretVolumeSource{}
		}
		v.VolumeSource.Secret.DefaultMode = mode
	}
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
	atExpectedGeneration := sts.Generation == sts.Status.ObservedGeneration
	return allUpdated && allReady && atExpectedGeneration
}

type Modification func(*appsv1.StatefulSet)

func New(mods ...Modification) appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	for _, mod := range mods {
		mod(&sts)
	}
	return sts
}

func Apply(funcs ...Modification) func(*appsv1.StatefulSet) {
	return func(sts *appsv1.StatefulSet) {
		for _, f := range funcs {
			f(sts)
		}
	}
}

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

func WithAnnotations(annotations map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Annotations = merge.StringToStringMap(set.Annotations, annotations)
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

func WithRevisionHistoryLimit(revisionHistoryLimit int) Modification {
	rhl := int32(revisionHistoryLimit)
	return func(sts *appsv1.StatefulSet) {
		sts.Spec.RevisionHistoryLimit = &rhl
	}
}

func WithPodManagementPolicyType(policyType appsv1.PodManagementPolicyType) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Spec.PodManagementPolicy = policyType
	}
}

func WithSelector(selector *metav1.LabelSelector) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Spec.Selector = selector
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
		if idx == notFound {
			set.Spec.VolumeClaimTemplates = append(set.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{})
			idx = len(set.Spec.VolumeClaimTemplates) - 1
		}
		pvc := &set.Spec.VolumeClaimTemplates[idx]
		f(pvc)
	}
}

func WithVolumeClaimTemplates(pv []corev1.PersistentVolumeClaim) Modification {
	pvCopy := make([]corev1.PersistentVolumeClaim, len(pv))
	copy(pvCopy, pv)
	return func(set *appsv1.StatefulSet) {
		set.Spec.VolumeClaimTemplates = pvCopy
	}
}

func WithCustomSpecs(spec appsv1.StatefulSetSpec) Modification {
	return func(set *appsv1.StatefulSet) {
		set.Spec = merge.StatefulSetSpecs(set.Spec, spec)
	}
}

func WithObjectMetadata(labels map[string]string, annotations map[string]string) Modification {
	return func(set *appsv1.StatefulSet) {
		WithLabels(labels)(set)
		WithAnnotations(annotations)(set)
	}
}

func findVolumeClaimIndexByName(name string, pvcs []corev1.PersistentVolumeClaim) int {
	for idx, pvc := range pvcs {
		if pvc.Name == name {
			return idx
		}
	}
	return notFound
}

func VolumeMountWithNameExists(mounts []corev1.VolumeMount, volumeName string) bool {
	for _, mount := range mounts {
		if mount.Name == volumeName {
			return true
		}
	}
	return false
}

// ResetUpdateStrategy resets the statefulset update strategy to RollingUpdate.
// If a version change is in progress, it doesn't do anything.
func ResetUpdateStrategy(ctx context.Context, mdb annotations.Versioned, kubeClient GetUpdater) error {
	if !mdb.IsChangingVersion() {
		return nil
	}

	// if we changed the version, we need to reset the UpdatePolicy back to OnUpdate
	_, err := GetAndUpdate(ctx, kubeClient, mdb.NamespacedName(), func(sts *appsv1.StatefulSet) {
		sts.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	})
	return err
}
