package sts

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// getContainerIndexByName returns the index of the container with containerName
// in sts.Spec.Template.Spec.Containers of the provided StatefulSet
func getContainerIndexByName(sts appsv1.StatefulSet, containerName string) (int, error) {
	for i, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == containerName {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no container with name [%s] found", containerName)
}

// VolumeMountData contains values required for the MountVolume function
type VolumeMountData struct {
	MountPath  string
	Name       string
	SourceType interface{}
	SourceName string
}

// AddVolumeClaimTemplates mutates the provided stateful set by adding the provided PersistentVolumeClaims and
// VolumeMounts to the container with the given name
func AddVolumeClaimTemplates(set *appsv1.StatefulSet, containerName string, claims []corev1.PersistentVolumeClaim, mounts []corev1.VolumeMount) error {
	idx, err := getContainerIndexByName(*set, containerName)
	if err != nil {
		return err
	}
	set.Spec.VolumeClaimTemplates = append(set.Spec.VolumeClaimTemplates, claims...)
	set.Spec.Template.Spec.Containers[idx].VolumeMounts = append(set.Spec.Template.Spec.Containers[idx].VolumeMounts, mounts...)
	return nil
}

// MountVolume mutates the provided StatefulSet by adding a new volume to the StatefulSet
// and also by adding a new VolumeMount to the container with the given name
func MountVolume(set *appsv1.StatefulSet, mountData VolumeMountData, containerName string) error {
	volMount := corev1.VolumeMount{
		Name:      mountData.Name,
		ReadOnly:  true,
		MountPath: mountData.MountPath,
	}

	var vol corev1.Volume
	switch mountData.SourceType.(type) {
	case corev1.ConfigMapVolumeSource:
		vol = corev1.Volume{
			Name: mountData.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: mountData.SourceName,
					},
				},
			},
		}
	case corev1.SecretVolumeSource:
		vol = corev1.Volume{
			Name: mountData.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: mountData.SourceName,
				},
			},
		}
	default:
		panic("unrecognized volumeSource type. Must be either ConfigMapVolumeSource or SecretVolumeSource")
	}

	idx, err := getContainerIndexByName(*set, containerName)
	if err != nil {
		return err
	}

	set.Spec.Template.Spec.Containers[idx].VolumeMounts = append(set.Spec.Template.Spec.Containers[idx].VolumeMounts, volMount)
	set.Spec.Template.Spec.Volumes = append(set.Spec.Template.Spec.Volumes, vol)
	return nil
}
