package scram

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/stretchr/testify/assert"
)

func TestScramAutomationConfig(t *testing.T) {

	// Case 1: Both SHA-256 and SHA-1
	auth := automationconfig.Auth{}
	opts := authtypes.Options{
		AuthoritativeSet:  false,
		KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
		AuthMechanisms:    []string{constants.Sha256, constants.Sha1},
		AgentName:         "mms-automation",
		AutoAuthMechanism: constants.Sha256,
	}
	err := configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
	assert.NoError(t, err)

	t.Run("Authentication is correctly configured", func(t *testing.T) {
		assert.Equal(t, constants.AgentName, auth.AutoUser)
		assert.Equal(t, "keyfilecontents", auth.Key)
		assert.Equal(t, "password", auth.AutoPwd)
		assert.Equal(t, constants.Sha256, auth.AutoAuthMechanism)
		assert.Len(t, auth.DeploymentAuthMechanisms, 2)
		assert.Len(t, auth.AutoAuthMechanisms, 2)
		assert.Equal(t, []string{constants.Sha256, constants.Sha1}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha256, constants.Sha1}, auth.AutoAuthMechanisms)
		assert.Equal(t, constants.AutomationAgentKeyFilePathInContainer, auth.KeyFile)
		assert.Equal(t, constants.AutomationAgentWindowsKeyFilePath, auth.KeyFileWindows)
	})
	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		err := configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, []string{constants.Sha256, constants.Sha1}, auth.DeploymentAuthMechanisms)
	})

	// Case 2: only SHA-256
	auth = automationconfig.Auth{}
	opts = authtypes.Options{
		AuthoritativeSet:  false,
		KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
		AuthMechanisms:    []string{constants.Sha256},
		AgentName:         "mms-automation",
		AutoAuthMechanism: constants.Sha256,
	}
	err = configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
	assert.NoError(t, err)

	t.Run("Authentication is correctly configured", func(t *testing.T) {
		assert.Equal(t, constants.Sha256, auth.AutoAuthMechanism)
		assert.Len(t, auth.DeploymentAuthMechanisms, 1)
		assert.Len(t, auth.AutoAuthMechanisms, 1)
		assert.Equal(t, []string{constants.Sha256}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha256}, auth.AutoAuthMechanisms)
		assert.Equal(t, constants.AutomationAgentKeyFilePathInContainer, auth.KeyFile)
		assert.Equal(t, constants.AutomationAgentWindowsKeyFilePath, auth.KeyFileWindows)
	})
	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		err := configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, []string{constants.Sha256}, auth.DeploymentAuthMechanisms)
	})

	// Case 1: only SHA-1
	auth = automationconfig.Auth{}
	opts = authtypes.Options{
		AuthoritativeSet:  false,
		KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
		AuthMechanisms:    []string{constants.Sha1},
		AgentName:         "mms-automation",
		AutoAuthMechanism: constants.Sha1,
	}
	err = configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
	assert.NoError(t, err)

	t.Run("Authentication is correctly configured", func(t *testing.T) {
		assert.Equal(t, constants.Sha1, auth.AutoAuthMechanism)
		assert.Len(t, auth.DeploymentAuthMechanisms, 1)
		assert.Len(t, auth.AutoAuthMechanisms, 1)
		assert.Equal(t, []string{constants.Sha1}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{constants.Sha1}, auth.AutoAuthMechanisms)
		assert.Equal(t, constants.AutomationAgentKeyFilePathInContainer, auth.KeyFile)
		assert.Equal(t, constants.AutomationAgentWindowsKeyFilePath, auth.KeyFileWindows)
	})
	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		err := configureInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, []string{constants.Sha1}, auth.DeploymentAuthMechanisms)
	})
}

// configureInAutomationConfig updates the provided auth struct and fully configures Scram authentication.
func configureInAutomationConfig(auth *automationconfig.Auth, agentPassword, agentKeyFile string, users []automationconfig.MongoDBUser, opts authtypes.Options) error {
	err := enableAgentAuthentication(auth, agentPassword, agentKeyFile, opts)
	if err != nil {
		return err
	}
	err = enableClientAuthentication(auth, opts, users)
	if err != nil {
		return err
	}
	return nil
}
