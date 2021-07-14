package validation

import (
	"fmt"
	"strings"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"github.com/pkg/errors"
)

// ValidateInitalSpec checks if the resource's initial Spec is valid.
func ValidateInitalSpec(mdb mdbv1.MongoDBCommunity) error {
	return validateSpec(mdb)
}

// ValidateUpdate validates that the new Spec, corresponding to the existing one, is still valid.
func ValidateUpdate(mdb mdbv1.MongoDBCommunity, oldSpec mdbv1.MongoDBCommunitySpec) error {
	if oldSpec.Security.TLS.Enabled && !mdb.Spec.Security.TLS.Enabled {
		return errors.New("TLS can't be set to disabled after it has been enabled")
	}
	return validateSpec(mdb)
}

// validateSpec validates the specs of the given resource definition.
func validateSpec(mdb mdbv1.MongoDBCommunity) error {
	if err := validateUsers(mdb); err != nil {
		return err
	}

	if err := validateArbiterSpec(mdb); err != nil {
		return err
	}

	if err := validateAuthModeSpec(mdb); err != nil {
		return err
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

// validateArbiterSpec checks if the initial Member spec is valid.
func validateArbiterSpec(mdb mdbv1.MongoDBCommunity) error {
	if mdb.Spec.Arbiters < 0 {
		return fmt.Errorf("number of arbiters must be greater or equal than 0")
	}
	if mdb.Spec.Arbiters >= mdb.Spec.Members {
		return fmt.Errorf("number of arbiters specified (%v) is greater or equal than the number of members in the replicaset (%v). At least one member must not be an arbiter", mdb.Spec.Arbiters, mdb.Spec.Members)
	}

	return nil
}

// validateAuthModeSpec checks that the list of modes does not contain duplicates.
func validateAuthModeSpec(mdb mdbv1.MongoDBCommunity) error {
	allModes := mdb.Spec.Security.Authentication.Modes

	// Check that no auth is defined more than once
	mapModes := make(map[mdbv1.AuthMode]struct{})
	for i, mode := range allModes {
		if value := mdbv1.ConvertAuthModeToAuthMechanism(mode); value == "" {
			return fmt.Errorf("unexpected value (%q) defined for supported authentication modes", value)
		}
		mapModes[allModes[i]] = struct{}{}
	}
	if len(mapModes) != len(allModes) {
		return fmt.Errorf("some authentication modes are declared twice or more")
	}

	return nil
}
