package validation

import (
	"fmt"
	"strings"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/pkg/errors"
)

// ValidateInitalSpec checks if the resource initial Spec is valid
func ValidateInitalSpec(mdb mdbv1.MongoDBCommunity) error {
	if err := validateUsers(mdb); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate validates that the new Spec, corresponding to the existing one is still valid
func ValidateUpdate(oldSpec, newSpec mdbv1.MongoDBCommunitySpec) error {
	if oldSpec.Security.TLS.Enabled && !newSpec.Security.TLS.Enabled {
		return errors.New("TLS can't be set to disabled after it has been enabled")
	}

	return nil
}

// validateUsers checks if the users configuration is valid
func validateUsers(mdb mdbv1.MongoDBCommunity) error {
	connectionStringSecretNameMap := map[string]scram.User{}
	nameCollisions := []string{}
	for _, user := range mdb.GetScramUsers() {
		secretName := user.GetConnectionStringSecretName(mdb)
		if previousUser, exists := connectionStringSecretNameMap[secretName]; exists {
			nameCollisions = append(nameCollisions,
				fmt.Sprintf(`[secret name: "%s" for user: "%s", db: "%s" and user: "%s", db: "%s"]`,
					secretName,
					previousUser.Username,
					previousUser.Database,
					user.Username,
					user.Database))
		} else {
			connectionStringSecretNameMap[secretName] = user
		}
	}
	if len(nameCollisions) > 0 {
		return errors.Errorf("connection string secret names collision, update at least one of the users so that the resulted secret names (<resource name>-<user>-<db>) are unique: %s",
			strings.Join(nameCollisions, ", "))
	}

	return nil
}
