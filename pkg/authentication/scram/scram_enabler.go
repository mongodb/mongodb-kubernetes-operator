package scram

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
func enableAgentAuthentication(auth *automationconfig.Auth, agentPassword, agentKeyFileContents string, opts authtypes.Options) error {
	if err := validateAgentOptions(opts); err != nil {
		return err
	}

	auth.Disabled = false
	auth.AuthoritativeSet = opts.AuthoritativeSet
	auth.KeyFile = opts.KeyFile

	// windows file is specified to pass validation, this will never be used
	auth.KeyFileWindows = constants.AutomationAgentWindowsKeyFilePath

	auth.AutoAuthMechanisms = make([]string, 0)
	if contains.Sha256(opts.AuthMechanisms) {
		auth.AutoAuthMechanisms = append(auth.AutoAuthMechanisms, constants.Sha256)
	}
	if contains.Sha1(opts.AuthMechanisms) {
		auth.AutoAuthMechanisms = append(auth.AutoAuthMechanisms, constants.Sha1)
	}

	// the username of the MongoDB Agent
	auth.AutoUser = opts.AgentName

	// the mechanism used by the Agent
	auth.AutoAuthMechanism = opts.AutoAuthMechanism

	// the password for the Agent user
	auth.AutoPwd = agentPassword

	// the contents the keyfile should have, this file is owned and managed
	// by the agent
	auth.Key = agentKeyFileContents

	return nil
}

func enableClientAuthentication(auth *automationconfig.Auth, opts authtypes.Options, users []automationconfig.MongoDBUser) error {
	if err := validateClientOptions(opts); err != nil {
		return err
	}

	if !contains.Sha256(auth.DeploymentAuthMechanisms) && contains.Sha256(opts.AuthMechanisms) {
		auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, constants.Sha256)
	}
	if !contains.Sha1(auth.DeploymentAuthMechanisms) && contains.Sha1(opts.AuthMechanisms) {
		auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, constants.Sha1)
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
	if opts.AgentName == "" {
		errs = multierror.Append(errs, errors.New("AgentName must be specified"))
	}
	if opts.KeyFile == "" {
		errs = multierror.Append(errs, errors.New("KeyFile must be specified"))
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
