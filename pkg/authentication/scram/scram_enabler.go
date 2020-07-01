package scram

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

const (
	scram256                              = "SCRAM-SHA-256"
	automationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/authentication/keyfile"
	automationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
	agentName                             = "mms-automation"
	scramAgentPasswordKey                 = "password"
	scramAgentKeyfileKey                  = "keyfile"
)

type Enabler struct {
	AgentPassword string
	AgentKeyFile  string
}

func (s Enabler) EnableAuth(auth automationconfig.Auth) automationconfig.Auth {
	s.enableAgentAuthentication(&auth)
	enableDeploymentMechanisms(&auth)
	return auth
}

func (s Enabler) enableAgentAuthentication(auth *automationconfig.Auth) {
	auth.Disabled = false
	auth.AuthoritativeSet = true
	auth.KeyFile = automationAgentKeyFilePathInContainer

	// windows file is specified to pass validation, this will never be used
	auth.KeyFileWindows = automationAgentWindowsKeyFilePath
	auth.AutoAuthMechanisms = []string{scram256}
	auth.AutoUser = agentName
	auth.AutoAuthMechanism = scram256
	auth.AutoPwd = s.AgentPassword
	auth.Key = s.AgentKeyFile
}

func enableDeploymentMechanisms(auth *automationconfig.Auth) {
	if contains.String(auth.DeploymentAuthMechanisms, scram256) {
		return
	}
	auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, scram256)
}
