package statefulset

import (
	"fmt"
	"sort"

	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Builder struct {
	name        string
	namespace   string
	replicas    int
	serviceName string

	// these fields need to be initialised
	labels                     map[string]string
	matchLabels                map[string]string
	ownerReference             []metav1.OwnerReference
	podTemplateSpec            corev1.PodTemplateSpec
	readinessProbePerContainer map[string]*corev1.Probe
	volumeClaimsTemplates      []corev1.PersistentVolumeClaim
	volumeMountsPerContainer   map[string][]corev1.VolumeMount
	updateStrategyType         appsv1.StatefulSetUpdateStrategyType
}

func (s *Builder) SetLabels(labels map[string]string) *Builder {
	s.labels = labels
	return s
}

func (s *Builder) SetName(name string) *Builder {
	s.name = name
	return s
}

func (s *Builder) SetNamespace(namespace string) *Builder {
	s.namespace = namespace
	return s
}

func (s *Builder) SetOwnerReference(ownerReference []metav1.OwnerReference) *Builder {
	s.ownerReference = ownerReference
	return s
}

func (s *Builder) SetServiceName(serviceName string) *Builder {
	s.serviceName = serviceName
	return s
}

func (s *Builder) SetReplicas(replicas int) *Builder {
	s.replicas = replicas
	return s
}

func (s *Builder) SetMatchLabels(matchLabels map[string]string) *Builder {
	s.matchLabels = matchLabels
	return s
}

func (s *Builder) SetReadinessProbe(probe *corev1.Probe, containerName string) *Builder {
	s.readinessProbePerContainer[containerName] = probe
	return s
}

func (s *Builder) SetPodTemplateSpec(podTemplateSpec corev1.PodTemplateSpec) *Builder {
	s.podTemplateSpec = podTemplateSpec
	return s
}

func (s *Builder) SetUpdateStrategy(updateStrategyType appsv1.StatefulSetUpdateStrategyType) *Builder {
	s.updateStrategyType = updateStrategyType
	return s
}

func (s *Builder) AddVolumeClaimTemplates(claims []corev1.PersistentVolumeClaim) *Builder {
	s.volumeClaimsTemplates = append(s.volumeClaimsTemplates, claims...)
	return s
}

func (s *Builder) AddVolumeMount(containerName string, mount corev1.VolumeMount) *Builder {
	s.volumeMountsPerContainer[containerName] = append(s.volumeMountsPerContainer[containerName], mount)
	return s
}

func (s *Builder) AddVolumeMounts(containerName string, mounts []corev1.VolumeMount) *Builder {
	for _, m := range mounts {
		s.AddVolumeMount(containerName, m)
	}
	return s
}

func (s *Builder) AddVolume(volume corev1.Volume) *Builder {
	s.podTemplateSpec.Spec.Volumes = append(s.podTemplateSpec.Spec.Volumes, volume)
	return s
}

func (s *Builder) AddVolumes(volumes []corev1.Volume) *Builder {
	for _, v := range volumes {
		s.AddVolume(v)
	}
	return s
}

// GetContainerIndexByName returns the index of the container with containerName.
func (s Builder) GetContainerIndexByName(containerName string) (int, error) {
	for i, c := range s.podTemplateSpec.Spec.Containers {
		if c.Name == containerName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no container with name [%s] found", containerName)
}

func (s *Builder) AddVolumeAndMount(volumeMountData VolumeMountData, containerNames ...string) *Builder {
	s.AddVolume(volumeMountData.Volume)
	for _, containerName := range containerNames {
		s.AddVolumeMount(
			containerName,
			corev1.VolumeMount{
				Name:      volumeMountData.Name,
				ReadOnly:  volumeMountData.ReadOnly,
				MountPath: volumeMountData.MountPath,
			},
		)
	}
	return s
}

func (s Builder) buildPodTemplateSpec() (corev1.PodTemplateSpec, error) {
	podTemplateSpec := s.podTemplateSpec.DeepCopy()
	var errs error
	for containerName, volumeMounts := range s.volumeMountsPerContainer {
		idx, err := s.GetContainerIndexByName(containerName)
		if err != nil {
			errs = multierror.Append(errs, err)
			// other containers may have valid mounts
			continue
		}
		existingVolumeMounts := map[string]bool{}
		for _, volumeMount := range volumeMounts {
			if prevMount, seen := existingVolumeMounts[volumeMount.MountPath]; seen {
				// Volume with the same path already mounted
				errs = multierror.Append(errs, fmt.Errorf("Volume %v already mounted as %v", volumeMount, prevMount))
				continue
			}
			podTemplateSpec.Spec.Containers[idx].VolumeMounts = append(podTemplateSpec.Spec.Containers[idx].VolumeMounts, volumeMount)
			existingVolumeMounts[volumeMount.MountPath] = true
		}
	}

	for containerName, overrideReadinessProbe := range s.readinessProbePerContainer {
		idx, err := s.GetContainerIndexByName(containerName)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		var readinessProbeRef *corev1.Probe
		if overrideReadinessProbe != nil {
			readinessProbe := *overrideReadinessProbe
			readinessProbeRef = &readinessProbe
		}
		podTemplateSpec.Spec.Containers[idx].ReadinessProbe = readinessProbeRef
	}

	// sorts environment variables for all containers
	for _, container := range podTemplateSpec.Spec.Containers {
		envVars := container.Env
		sort.SliceStable(envVars, func(i, j int) bool {
			return envVars[i].Name < envVars[j].Name
		})
	}
	return *podTemplateSpec, errs
}

func copyMap(originalMap map[string]string) map[string]string {
	newMap := map[string]string{}
	for k, v := range originalMap {
		newMap[k] = v
	}
	return newMap
}

func (s Builder) Build() (appsv1.StatefulSet, error) {
	podTemplateSpec, err := s.buildPodTemplateSpec()
	if err != nil {
		return appsv1.StatefulSet{}, err
	}

	replicas := int32(s.replicas)

	ownerReference := make([]metav1.OwnerReference, len(s.ownerReference))
	copy(ownerReference, s.ownerReference)

	volumeClaimsTemplates := make([]corev1.PersistentVolumeClaim, len(s.volumeClaimsTemplates))
	copy(volumeClaimsTemplates, s.volumeClaimsTemplates)

	sts := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            s.name,
			Namespace:       s.namespace,
			Labels:          copyMap(s.labels),
			OwnerReferences: ownerReference,
		},
		Spec: appsv1.StatefulSetSpec{
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: s.updateStrategyType,
			},
			ServiceName: s.serviceName,
			Replicas:    &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: copyMap(s.matchLabels),
			},
			Template:             podTemplateSpec,
			VolumeClaimTemplates: volumeClaimsTemplates,
		},
	}
	return sts, err
}

func NewBuilder() *Builder {
	return &Builder{
		labels:                     map[string]string{},
		matchLabels:                map[string]string{},
		ownerReference:             []metav1.OwnerReference{},
		podTemplateSpec:            corev1.PodTemplateSpec{},
		readinessProbePerContainer: map[string]*corev1.Probe{},
		volumeClaimsTemplates:      []corev1.PersistentVolumeClaim{},
		volumeMountsPerContainer:   map[string][]corev1.VolumeMount{},
		replicas:                   1,
	}
}
