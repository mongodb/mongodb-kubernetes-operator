package mongodb

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
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
