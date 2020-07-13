package mongodb

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

const (
	scramShaOption = "SCRAM"
)

// noOpAuthEnabler performs no changes, leaving authentication settings untouched
type noOpAuthEnabler struct{}

func (n noOpAuthEnabler) EnableAuth(auth automationconfig.Auth) automationconfig.Auth {
	return auth
}

// getAuthenticationEnabler returns a type that is able to configure the automation config's
// authentication settings
func getAuthenticationEnabler(getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDB) (automationconfig.AuthEnabler, error) {
	if !mdb.Spec.Security.Authentication.Enabled {
		return noOpAuthEnabler{}, nil
	}

	// currently, just enable auth if it's in the list as there is only one option
	if contains.AuthMode(mdb.Spec.Security.Authentication.Modes, scramShaOption) {
		enabler, err := scram.EnsureAgentSecret(getUpdateCreator, mdb.ScramCredentialsNamespacedName())
		if err != nil {
			return noOpAuthEnabler{}, err
		}
		return enabler, nil
	}
	return noOpAuthEnabler{}, nil
}

// buildScramPodSpecModification will add the keyfile volume to the podTemplateSpec
// the keyfile is owned by the agent, and is required to have 0600 permissions.
func buildScramPodSpecModification(mdb mdbv1.MongoDB) podtemplatespec.Modification {
	if !mdb.Spec.Security.Authentication.Enabled {
		return podtemplatespec.NOOP()
	}

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
