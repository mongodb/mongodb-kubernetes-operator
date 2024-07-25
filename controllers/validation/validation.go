package validation

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
)

// ValidateInitialSpec checks if the resource's initial Spec is valid.
func ValidateInitialSpec(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
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

	if err := validateAgentCertSecret(mdb, log); err != nil {
		return err
	}

	if err := validateStatefulSet(mdb); err != nil {
		return err
	}

	return nil
}

// validateUsers checks if the users configuration is valid
func validateUsers(mdb mdbv1.MongoDBCommunity) error {
	connectionStringSecretNameMap := map[string]authtypes.User{}
	nameCollisions := []string{}

	scramSecretNameMap := map[string]authtypes.User{}
	scramSecretNameCollisions := []string{}
	expectedAuthMethods := map[string]struct{}{}

	if len(mdb.Spec.Security.Authentication.Modes) == 0 {
		expectedAuthMethods[constants.Sha256] = struct{}{}
	}

	for _, auth := range mdb.Spec.Security.Authentication.Modes {
		expectedAuthMethods[mdbv1.ConvertAuthModeToAuthMechanism(auth)] = struct{}{}
	}

	for _, user := range mdb.GetAuthUsers() {

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
			scramSecretNameMap[scramSecretName] = user
		}

		if user.Database == constants.ExternalDB {
			if _, ok := expectedAuthMethods[constants.X509]; !ok {
				return fmt.Errorf("X.509 user %s present but X.509 is not enabled", user.Username)
			}
			if user.PasswordSecretKey != "" {
				return fmt.Errorf("X509 user %s should not have a password secret key", user.Username)
			}
			if user.PasswordSecretName != "" {
				return fmt.Errorf("X509 user %s should not have a password secret name", user.Username)
			}
			if user.ScramCredentialsSecretName != "" {
				return fmt.Errorf("X509 user %s should not have scram credentials secret name", user.Username)
			}
		} else {
			_, sha1 := expectedAuthMethods[constants.Sha1]
			_, sha256 := expectedAuthMethods[constants.Sha256]
			if !sha1 && !sha256 {
				return fmt.Errorf("SCRAM user %s present but SCRAM is not enabled", user.Username)
			}
			if user.PasswordSecretKey == "" {
				return fmt.Errorf("SCRAM user %s is missing password secret key", user.Username)
			}
			if user.PasswordSecretName == "" {
				return fmt.Errorf("SCRAM user %s is missing password secret name", user.Username)
			}
			if user.ScramCredentialsSecretName == "" {
				return fmt.Errorf("SCRAM user %s is missing scram credentials secret name", user.Username)
			}
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
	mapMechanisms := make(map[string]struct{})

	// Issue warning if Modes array is empty
	if len(allModes) == 0 {
		mapMechanisms[constants.Sha256] = struct{}{}
		log.Warnf("An empty Modes array has been provided. The default mode (SCRAM-SHA-256) will be used.")
	}

	// Check that no auth is defined more than once
	for _, mode := range allModes {
		if value := mdbv1.ConvertAuthModeToAuthMechanism(mode); value == "" {
			return fmt.Errorf("unexpected value (%q) defined for supported authentication modes", value)
		} else if value == constants.X509 && !mdb.Spec.Security.TLS.Enabled {
			return fmt.Errorf("TLS must be enabled when using X.509 authentication")
		}
		mapMechanisms[mdbv1.ConvertAuthModeToAuthMechanism(mode)] = struct{}{}
	}

	if len(mapMechanisms) < len(allModes) {
		return fmt.Errorf("some authentication modes are declared twice or more")
	}

	agentMode := mdb.Spec.GetAgentAuthMode()
	if agentMode == "" && len(allModes) > 1 {
		return fmt.Errorf("If spec.security.authentication.modes contains different authentication modes, the agent mode must be specified ")
	}
	if _, present := mapMechanisms[mdbv1.ConvertAuthModeToAuthMechanism(agentMode)]; !present {
		return fmt.Errorf("Agent authentication mode: %s must be part of the spec.security.authentication.modes", agentMode)
	}

	return nil
}

func validateAgentCertSecret(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	agentMode := mdb.Spec.GetAgentAuthMode()
	if agentMode != "X509" &&
		mdb.Spec.Security.Authentication.AgentCertificateSecret != nil &&
		mdb.Spec.Security.Authentication.AgentCertificateSecret.Name != "" {
		log.Warnf("Agent authentication is not X.509, but the agent certificate secret is configured, it will be ignored")
	}
	return nil
}

func validateStatefulSet(mdb mdbv1.MongoDBCommunity) error {
	stsReplicas := mdb.Spec.StatefulSetConfiguration.SpecWrapper.Spec.Replicas

	if stsReplicas != nil && *stsReplicas != int32(mdb.Spec.Members) {
		return fmt.Errorf("spec.statefulset.spec.replicas has to be equal to spec.members")
	}

	return nil
}
