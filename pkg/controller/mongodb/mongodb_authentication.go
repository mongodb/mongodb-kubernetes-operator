package mongodb

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
)

const (
	scramShaOption = "SCRAM"
)

// buildScramPodSpecModification will add the keyfile volume to the podTemplateSpec
// the keyfile is owned by the agent, and is required to have 0600 permissions.
func buildScramPodSpecModification(mdb mdbv1.MongoDB) podtemplatespec.Modification {
	mode := int32(0600)
	scramSecretNsName := mdb.ScramCredentialsNamespacedName()
	keyFileVolume := statefulset.CreateVolumeFromSecret(scramSecretNsName.Name, scramSecretNsName.Name, statefulset.WithSecretDefaultMode(&mode))
	keyFileVolumeVolumeMount := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))
	keyFileVolumeVolumeMountMongod := statefulset.CreateVolumeMount(keyFileVolume.Name, "/var/lib/mongodb-mms-automation/authentication", statefulset.WithReadOnly(false))

	return podtemplatespec.Apply(
		podtemplatespec.WithVolume(keyFileVolume),
		podtemplatespec.WithVolumeMounts(agentName, keyFileVolumeVolumeMount),
		podtemplatespec.WithVolumeMounts(mongodbName, keyFileVolumeVolumeMountMongod),
	)
}
