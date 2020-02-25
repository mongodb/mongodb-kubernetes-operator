package statefulset

import (
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

// CreateVolumeMount convenience function to build a VolumeMount.
func CreateVolumeMount(name, path, subpath string) corev1.VolumeMount {
	volumeMount := corev1.VolumeMount{
		Name:      name,
		MountPath: path,
	}
	if subpath != "" {
		volumeMount.SubPath = subpath
	}
	return volumeMount
}
