package statefulset

import (
	"sort"

	"github.com/imdario/mergo"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
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
	GetStatefulSet(objectKey client.ObjectKey) (appsv1.StatefulSet, error)
}

type Updater interface {
	UpdateStatefulSet(sts appsv1.StatefulSet) error
}

type Creator interface {
	CreateStatefulSet(sts appsv1.StatefulSet) error
}

type Deleter interface {
	DeleteStatefulSet(objectKey client.ObjectKey) error
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
func CreateOrUpdate(getUpdateCreator GetUpdateCreator, sts appsv1.StatefulSet) error {
	_, err := getUpdateCreator.GetStatefulSet(types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace})
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return getUpdateCreator.CreateStatefulSet(sts)
		}
		return err
	}
	return getUpdateCreator.UpdateStatefulSet(sts)
}

// GetAndUpdate applies the provided function to the most recent version of the object
func GetAndUpdate(getUpdater GetUpdater, nsName types.NamespacedName, updateFunc func(*appsv1.StatefulSet)) error {
	sts, err := getUpdater.GetStatefulSet(nsName)
	if err != nil {
		return err
	}
	// apply the function on the most recent version of the resource
	updateFunc(&sts)
	return getUpdater.UpdateStatefulSet(sts)
}

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

func CreateVolumeFromSecret(name, sourceName string, options ...func(v *corev1.Volume)) corev1.Volume {
	volumeMount := &corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: sourceName,
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
	return allUpdated && allReady
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

func NOOP() Modification {
	return func(sts *appsv1.StatefulSet) {}
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
		if idx == notFound {
			set.Spec.VolumeClaimTemplates = append(set.Spec.VolumeClaimTemplates, corev1.PersistentVolumeClaim{})
			idx = len(set.Spec.VolumeClaimTemplates) - 1
		}
		pvc := &set.Spec.VolumeClaimTemplates[idx]
		f(pvc)
	}
}

func createVolumeClaimMap(volumeMounts []corev1.PersistentVolumeClaim) map[string]corev1.PersistentVolumeClaim {
	mountMap := make(map[string]corev1.PersistentVolumeClaim)
	for _, m := range volumeMounts {
		mountMap[m.Name] = m
	}
	return mountMap
}

func MergeVolumeClaimTemplates(defaultTemplates []corev1.PersistentVolumeClaim, overrideTemplates []corev1.PersistentVolumeClaim) ([]corev1.PersistentVolumeClaim, error) {
	defaultMountsMap := createVolumeClaimMap(defaultTemplates)
	overrideMountsMap := createVolumeClaimMap(overrideTemplates)
	var mergedVolumes []corev1.PersistentVolumeClaim
	for _, defaultMount := range defaultMountsMap {
		if overrideMount, ok := overrideMountsMap[defaultMount.Name]; ok {
			// needs merge
			if err := mergo.Merge(&defaultMount, overrideMount, mergo.WithAppendSlice, mergo.WithOverride); err != nil {
				return nil, err
			}
		}
		mergedVolumes = append(mergedVolumes, defaultMount)
	}
	for _, overrideMount := range overrideMountsMap {
		if _, ok := defaultMountsMap[overrideMount.Name]; ok {
			// already merged
			continue
		}
		mergedVolumes = append(mergedVolumes, overrideMount)
	}
	sort.SliceStable(mergedVolumes, func(i, j int) bool {
		return mergedVolumes[i].Name < mergedVolumes[j].Name
	})
	return mergedVolumes, nil
}

func mergeStatefulSetSpecs(defaultSpec, overrideSpec appsv1.StatefulSetSpec) (appsv1.StatefulSetSpec, error) {

	// PodTemplateSpec needs to be manually merged
	mergedPodTemplateSpec, err := podtemplatespec.MergePodTemplateSpecs(defaultSpec.Template, overrideSpec.Template)
	if err != nil {
		return appsv1.StatefulSetSpec{}, err
	}
	// VolumeClaimTemplates needs to be manually merged
	mergedVolumeClaimTemplates, err := MergeVolumeClaimTemplates(defaultSpec.VolumeClaimTemplates, overrideSpec.VolumeClaimTemplates)

	// Merging the rest with mergo

	if err := mergo.Merge(&defaultSpec, overrideSpec, mergo.WithOverride); err != nil {
		return appsv1.StatefulSetSpec{}, err
	}
	defaultSpec.Template = mergedPodTemplateSpec
	defaultSpec.VolumeClaimTemplates = mergedVolumeClaimTemplates
	return defaultSpec, nil
}

func WithCustomSpecs(spec appsv1.StatefulSetSpec) Modification {
	return func(set *appsv1.StatefulSet) {
		m, err := mergeStatefulSetSpecs(set.Spec, spec)
		if err != nil {
			return
		}
		set.Spec = m
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
