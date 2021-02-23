package scram

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

const (
	Sha256                                = "SCRAM-SHA-256"
	Sha1                                  = "MONGODB-CR"
	AutomationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/authentication/keyfile"
	automationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
	AgentName                             = "mms-automation"
	AgentPasswordKey                      = "password"
	AgentKeyfileKey                       = "keyfile"
)

// configureScramInAutomationConfig updates the provided auth struct and fully configures Scram authentication.
func configureScramInAutomationConfig(auth *automationconfig.Auth, agentPassword, agentKeyFile string, users []automationconfig.MongoDBUser, opts Options) error {
	if err := validateScramOptions(opts); err != nil {
		return err
	}
	enableAgentAuthentication(auth, agentPassword, agentKeyFile, users, opts)
	enableDeploymentMechanisms(auth, opts)
	return nil
}

// enableAgentAuthentication updates the provided auth struct and configures scram authentication based on the provided
// values and configuration options.
func enableAgentAuthentication(auth *automationconfig.Auth, agentPassword, agentKeyFileContents string, users []automationconfig.MongoDBUser, opts Options) {
	auth.Disabled = false
	auth.AuthoritativeSet = opts.AuthoritativeSet
	auth.KeyFile = opts.KeyFile

	// windows file is specified to pass validation, this will never be used
	auth.KeyFileWindows = automationAgentWindowsKeyFilePath

	for _, authMode := range opts.AutoAuthMechanisms {
		if !contains.String(auth.AutoAuthMechanisms, authMode) {
			auth.AutoAuthMechanisms = append(auth.AutoAuthMechanisms, authMode)
		}
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

	// assign all the users that should be added to the deployment
	auth.Users = users
}

func enableDeploymentMechanisms(auth *automationconfig.Auth, opts Options) {
	for _, authMode := range opts.AutoAuthMechanisms {
		if !contains.String(auth.DeploymentAuthMechanisms, authMode) {
			auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, authMode)
		}
	}
}

// validateScramOptions validates that all the required fields have
// a non empty value.
func validateScramOptions(opts Options) error {
	var errs error
	if len(opts.AutoAuthMechanisms) == 0 {
		errs = multierror.Append(errs, errors.New("at least one AutoAuthMechanism must be specified"))
	}
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
