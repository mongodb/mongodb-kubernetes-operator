package controllers

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
)

// buildScramPodSpecModification will add the keyfile volume to the podTemplateSpec
// the keyfile is owned by the agent, and is required to have 0600 permissions.
func buildScramPodSpecModification() podtemplatespec.Modification {
	mode := int32(0600)
	keyfileVolumeName := "keyfile-volume"
	keyFileVolume := statefulset.CreateVolumeFromSecret(keyfileVolumeName, keyfileVolumeName, statefulset.WithSecretDefaultMode(&mode))
	keyFileVolumeVolumeMount := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))
	keyFileVolumeVolumeMountMongod := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))

	return podtemplatespec.Apply(
		podtemplatespec.WithVolume(keyFileVolume),
		podtemplatespec.WithVolumeMounts(agentName, keyFileVolumeVolumeMount),
		podtemplatespec.WithVolumeMounts(mongodbName, keyFileVolumeVolumeMountMongod),
	)
}
