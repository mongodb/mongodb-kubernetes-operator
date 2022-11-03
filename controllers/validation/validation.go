package validation

import (
	"errors"
	"fmt"
	"strings"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scram"
	"go.uber.org/zap"
)

// ValidateInitalSpec checks if the resource's initial Spec is valid.
func ValidateInitalSpec(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	return validateSpec(mdb, log)
}

// ValidateUpdate validates that the new Spec, corresponding to the existing one, is still valid.
func ValidateUpdate(mdb mdbv1.MongoDBCommunity, oldSpec mdbv1.MongoDBCommunitySpec, log *zap.SugaredLogger) error {
	if oldSpec.Security.TLS.Enabled && !mdb.Spec.Security.TLS.Enabled {
		return errors.New("TLS can't be set to disabled after it has been enabled")
	}
	return validateSpec(mdb, log)
}

// validateSpec validates the specs of the given resource definition.
func validateSpec(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	if err := validateUsers(mdb); err != nil {
		return err
	}

	if err := validateArbiterSpec(mdb); err != nil {
		return err
	}

	if err := validateAuthModeSpec(mdb, log); err != nil {
		return err
	}

	return nil
}

// validateUsers checks if the users configuration is valid
func validateUsers(mdb mdbv1.MongoDBCommunity) error {
	connectionStringSecretNameMap := map[string]scram.User{}
	nameCollisions := []string{}

	scramSecretNameMap := map[string]scram.User{}
	scramSecretNameCollisions := []string{}

	for _, user := range mdb.GetScramUsers() {

		// Ensure no collisions in the connection string secret names
		connectionStringSecretName := user.ConnectionStringSecretName
		if previousUser, exists := connectionStringSecretNameMap[connectionStringSecretName]; exists {
			nameCollisions = append(nameCollisions,
				fmt.Sprintf(`[connection string secret name: "%s" for user: "%s", db: "%s" and user: "%s", db: "%s"]`,
					connectionStringSecretName,
					previousUser.Username,
					previousUser.Database,
					user.Username,
					user.Database))
		} else {
			connectionStringSecretNameMap[connectionStringSecretName] = user
		}

		// Ensure no collisions in the secret holding scram credentials
		scramSecretName := user.ScramCredentialsSecretName
		if previousUser, exists := scramSecretNameMap[scramSecretName]; exists {
			scramSecretNameCollisions = append(scramSecretNameCollisions,
				fmt.Sprintf(`[scram secret name: "%s" for user: "%s" and user: "%s"]`,
					scramSecretName,
					previousUser.Username,
					user.Username))
		} else {
			connectionStringSecretNameMap[connectionStringSecretName] = user
		}

	}
	if len(nameCollisions) > 0 {
		return fmt.Errorf("connection string secret names collision, update at least one of the users so that the resulted secret names (<resource name>-<user>-<db>) are unique: %s",
			strings.Join(nameCollisions, ", "))
	}

	if len(scramSecretNameCollisions) > 0 {
		return fmt.Errorf("scram credential secret names collision, update at least one of the users: %s",
			strings.Join(scramSecretNameCollisions, ", "))
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
func validateAuthModeSpec(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	allModes := mdb.Spec.Security.Authentication.Modes

	// Issue warning if Modes array is empty
	if len(allModes) == 0 {
		log.Warnf("An empty Modes array has been provided. The default mode (SCRAM-SHA-256) will be used.")
	}

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
