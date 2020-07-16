package scram

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/stretchr/testify/assert"
)

func TestScramAutomationConfig(t *testing.T) {
	modificationFunc := automationConfigModification("password", "keyfilecontents", []automationconfig.MongoDBUser{})
	config := automationconfig.AutomationConfig{}

	t.Run("Authentication is correctly configured", func(t *testing.T) {
		modificationFunc(&config)

		assert.Equal(t, AgentName, config.Auth.AutoUser)
		assert.Equal(t, "keyfilecontents", config.Auth.Key)
		assert.Equal(t, "password", config.Auth.AutoPwd)
		assert.Equal(t, scram256, config.Auth.AutoAuthMechanism)
		assert.Len(t, config.Auth.DeploymentAuthMechanisms, 1)
		assert.Len(t, config.Auth.AutoAuthMechanisms, 1)
		assert.Equal(t, []string{scram256}, config.Auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{scram256}, config.Auth.AutoAuthMechanisms)
		assert.Equal(t, automationAgentKeyFilePathInContainer, config.Auth.KeyFile)
		assert.Equal(t, automationAgentWindowsKeyFilePath, config.Auth.KeyFileWindows)
	})

	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		modificationFunc(&config)
		assert.Equal(t, []string{scram256}, config.Auth.DeploymentAuthMechanisms)
	})
}
