package scram

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/stretchr/testify/assert"
)

func TestScramEnabler(t *testing.T) {
	enabler := Enabler{
		AgentPassword: "password",
		AgentKeyFile:  "keyfilecontents",
	}
	auth := enabler.EnableAuth(automationconfig.Auth{})
	t.Run("Authentication is correctly configured", func(t *testing.T) {
		assert.Equal(t, agentName, auth.AutoUser)
		assert.Equal(t, "keyfilecontents", auth.Key)
		assert.Equal(t, "password", auth.AutoPwd)
		assert.Equal(t, scram256, auth.AutoAuthMechanism)
		assert.Len(t, auth.DeploymentAuthMechanisms, 1)
		assert.Len(t, auth.AutoAuthMechanisms, 1)
		assert.Equal(t, []string{scram256}, auth.DeploymentAuthMechanisms)
		assert.Equal(t, []string{scram256}, auth.AutoAuthMechanisms)
		assert.Equal(t, automationAgentKeyFilePathInContainer, auth.KeyFile)
		assert.Equal(t, automationAgentWindowsKeyFilePath, auth.KeyFileWindows)
	})

	t.Run("Subsequent configuration doesn't add to deployment auth mechanisms", func(t *testing.T) {
		auth = enabler.EnableAuth(auth)
		assert.Equal(t, []string{scram256}, auth.DeploymentAuthMechanisms)
	})

}
