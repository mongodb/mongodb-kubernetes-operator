package scram

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/stretchr/testify/assert"
)

func TestScramAutomationConfig(t *testing.T) {
	auth := automationconfig.Auth{}
	opts := Options{
		AuthoritativeSet:   false,
		KeyFile:            AutomationAgentKeyFilePathInContainer,
		AutoAuthMechanisms: []string{Sha256, Sha1},
		AgentName:          "mms-automation",
		AutoAuthMechanism:  Sha256,
	}

	err := configureScramInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
	assert.NoError(t, err)

	t.Run("Authentication is correctly configured", func(t *testing.T) {
		assert.Equal(t, AgentName, auth.AutoUser)
		assert.Equal(t, "keyfilecontents", auth.Key)
		assert.Equal(t, "password", auth.AutoPwd)
		assert.Equal(t, Sha256, auth.AutoAuthMechanism)
		assert.Len(t, auth.DeploymentAuthMechanisms, 2)
		assert.Len(t, auth.AutoAuthMechanisms, 2)
		assert.Equal(t, []string{Sha256, Sha1}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{Sha256, Sha1}, auth.AutoAuthMechanisms)
		assert.Equal(t, AutomationAgentKeyFilePathInContainer, auth.KeyFile)
		assert.Equal(t, automationAgentWindowsKeyFilePath, auth.KeyFileWindows)
	})

	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		err := configureScramInAutomationConfig(&auth, "password", "keyfilecontents", []automationconfig.MongoDBUser{}, opts)
		assert.NoError(t, err)
		assert.Equal(t, []string{Sha256, Sha1}, auth.DeploymentAuthMechanisms)
	})
}
