package scram

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

const (
	scram256                              = "SCRAM-SHA-256"
	automationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/authentication/keyfile"
	automationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
	AgentName                             = "mms-automation"
	AgentPasswordKey                      = "password"
	AgentKeyfileKey                       = "keyfile"
)

type authEnabler struct {
	agentPassword string
	agentKeyFile  string
	users         []automationconfig.MongoDBUser
}

func (s authEnabler) EnableAuth(auth automationconfig.Auth) automationconfig.Auth {
	enableAgentAuthentication(&auth, s.agentPassword, s.agentKeyFile, s.users)
	enableDeploymentMechanisms(&auth)
	return auth
}

func enableAgentAuthentication(auth *automationconfig.Auth, agentPassword, agentKeyFileContents string, users []automationconfig.MongoDBUser) {
	auth.Disabled = false
	auth.AuthoritativeSet = true
	auth.KeyFile = automationAgentKeyFilePathInContainer

	// windows file is specified to pass validation, this will never be used
	auth.KeyFileWindows = automationAgentWindowsKeyFilePath
	auth.AutoAuthMechanisms = []string{scram256}

	// the username of the MongoDB Agent
	auth.AutoUser = AgentName

	// the mechanism used by the Agent
	auth.AutoAuthMechanism = scram256

	// the password for the Agent user
	auth.AutoPwd = agentPassword

	// the contents the keyfile should have, this file is owned and managed
	// by the agent
	auth.Key = agentKeyFileContents

	// assign all the users that should be added to the deployment
	auth.Users = users
}

func enableDeploymentMechanisms(auth *automationconfig.Auth) {
	if contains.String(auth.DeploymentAuthMechanisms, scram256) {
		return
	}
	auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, scram256)
}
