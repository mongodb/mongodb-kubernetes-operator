package authentication

import (
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/x509"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
)

func Enable(auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) error {
	scramEnabled := false
	for _, authMode := range mdb.GetAuthOptions().AuthMechanisms {
		switch authMode {
		case constants.Sha1, constants.Sha256:
			if !scramEnabled {
				if err := scram.Enable(auth, secretGetUpdateCreateDeleter, mdb); err != nil {
					return fmt.Errorf("could not configure scram authentication: %s", err)
				}
				scramEnabled = true
			}
		case constants.X509:
			if err := x509.Enable(auth, secretGetUpdateCreateDeleter, mdb); err != nil {
				return fmt.Errorf("could not configure x509 authentication: %s", err)
			}
		}
	}
	return nil
}
