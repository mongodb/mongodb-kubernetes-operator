package x509

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

// enableAgentAuthentication updates the provided auth struct and configures scram authentication based on the provided
// values and configuration options.
func enableAgentAuthentication(auth *automationconfig.Auth, agentKeyFileContents string, agentName string, opts authtypes.Options) error {
	if err := validateAgentOptions(opts); err != nil {
		return err
	}

	auth.Disabled = false
	auth.AuthoritativeSet = opts.AuthoritativeSet
	auth.KeyFile = opts.KeyFile

	// the contents the keyfile should have, this file is owned and managed
	// by the agent
	auth.Key = agentKeyFileContents

	// windows file is specified to pass validation, this will never be used
	auth.KeyFileWindows = constants.AutomationAgentWindowsKeyFilePath

	auth.AutoAuthMechanisms = []string{constants.X509}

	// the username of the MongoDB Agent
	auth.AutoUser = agentName

	// the mechanism used by the Agent
	auth.AutoAuthMechanism = constants.X509

	// the password for the Agent user
	auth.AutoPwd = ""

	return nil
}

func enableClientAuthentication(auth *automationconfig.Auth, opts authtypes.Options, users []automationconfig.MongoDBUser) error {
	if err := validateClientOptions(opts); err != nil {
		return err
	}

	if !contains.X509(auth.DeploymentAuthMechanisms) {
		auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, constants.X509)
	}

	auth.Users = append(auth.Users, users...)
	return nil
}

// validateAgentOptions validates that all the agent required fields have
// a non-empty value.
func validateAgentOptions(opts authtypes.Options) error {
	var errs error
	if opts.AutoAuthMechanism == "" {
		errs = multierror.Append(errs, errors.New("AutoAuthMechanism must not be empty"))
	}
	return errs
}

// validateClientOptions validates that all the deployment required fields have
// a non-empty value.
func validateClientOptions(opts authtypes.Options) error {
	var errs error
	if len(opts.AuthMechanisms) == 0 {
		errs = multierror.Append(errs, errors.New("at least one AuthMechanism must be specified"))
	}
	return errs
}
