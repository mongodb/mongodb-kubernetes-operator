package x509

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/stretchr/testify/assert"
)

func TestX509AutomationConfig(t *testing.T) {
	t.Run("Only X509", func(t *testing.T) {
		auth := automationconfig.Auth{}
		opts := authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
			AuthMechanisms:    []string{constants.X509},
			AutoAuthMechanism: constants.X509,
		}
		err := configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)

		t.Run("Authentication is correctly configured", func(t *testing.T) {
			assert.Equal(t, "CN=my-agent,O=MongoDB", auth.AutoUser)
			assert.Equal(t, "keyfilecontents", auth.Key)
			assert.Equal(t, "", auth.AutoPwd)
			assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
			assert.Len(t, auth.DeploymentAuthMechanisms, 1)
			assert.Len(t, auth.AutoAuthMechanisms, 1)
			assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
			assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
			assert.Equal(t, constants.AutomationAgentKeyFilePathInContainer, auth.KeyFile)
			assert.Equal(t, constants.AutomationAgentWindowsKeyFilePath, auth.KeyFileWindows)
		})
		t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
			err := configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
			assert.NoError(t, err)
			assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
		})
	})

	t.Run("X509 and SHA-256", func(t *testing.T) {
		auth := automationconfig.Auth{}
		opts := authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
			AuthMechanisms:    []string{constants.X509, constants.Sha256},
			AutoAuthMechanism: constants.X509,
		}
		err := configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)

		t.Run("Authentication is correctly configured", func(t *testing.T) {
			assert.Equal(t, "CN=my-agent,O=MongoDB", auth.AutoUser)
			assert.Equal(t, "keyfilecontents", auth.Key)
			assert.Equal(t, "", auth.AutoPwd)
			assert.Equal(t, constants.X509, auth.AutoAuthMechanism)
			assert.Len(t, auth.DeploymentAuthMechanisms, 1)
			assert.Len(t, auth.AutoAuthMechanisms, 1)
			assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
			assert.Equal(t, []string{constants.X509}, auth.AutoAuthMechanisms)
			assert.Equal(t, constants.AutomationAgentKeyFilePathInContainer, auth.KeyFile)
			assert.Equal(t, constants.AutomationAgentWindowsKeyFilePath, auth.KeyFileWindows)
		})
		t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
			err := configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
			assert.NoError(t, err)
			assert.Equal(t, []string{constants.X509}, auth.DeploymentAuthMechanisms)
		})
	})

	t.Run("Fail validation", func(t *testing.T) {
		auth := automationconfig.Auth{}
		opts := authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
			AuthMechanisms:    []string{},
			AutoAuthMechanism: constants.X509,
		}
		err := configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
		assert.Error(t, err)

		auth = automationconfig.Auth{}
		opts = authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
			AuthMechanisms:    []string{constants.X509},
			AutoAuthMechanism: "",
		}
		err = configureInAutomationConfig(&auth, "keyfilecontents", "CN=my-agent,O=MongoDB", []automationconfig.MongoDBUser{}, opts)
		assert.Error(t, err)
	})
}

// configureInAutomationConfig updates the provided auth struct and fully configures Scram authentication.
func configureInAutomationConfig(auth *automationconfig.Auth, agentKeyFile, agentName string, users []automationconfig.MongoDBUser, opts authtypes.Options) error {
	err := enableAgentAuthentication(auth, agentKeyFile, agentName, opts)
	if err != nil {
		return err
	}
	err = enableClientAuthentication(auth, opts, users)
	if err != nil {
		return err
	}
	return nil
}
