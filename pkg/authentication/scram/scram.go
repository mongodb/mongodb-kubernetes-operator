package scram

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
)

const (
	scram256                              = "SCRAM-SHA-256"
	automationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/keyfile/keyfile"
	automationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
	agentName                             = "mms-automation"
)

type Enabler struct {
	AgentPassword string
	AgentKeyFile  string
}

func (s Enabler) Enable(auth automationconfig.Auth) (automationconfig.Auth, error) {
	if err := s.enableAgentAuthentication(&auth); err != nil {
		return automationconfig.Auth{}, err
	}
	if err := ensurePassword(s.AgentPassword, &auth); err != nil {
		return automationconfig.Auth{}, err
	}
	enableDeploymentMechanisms(&auth)

	return auth, nil
}

func (s Enabler) enableAgentAuthentication(auth *automationconfig.Auth) error {
	auth.Disabled = false
	auth.AuthoritativeSet = true
	auth.KeyFile = automationAgentKeyFilePathInContainer
	auth.KeyFileWindows = automationAgentWindowsKeyFilePath
	auth.AutoAuthMechanisms = []string{scram256}
	auth.AutoUser = agentName

	if err := s.ensureKeyFileContents(auth); err != nil {
		return err
	}
	return nil
}

func enableDeploymentMechanisms(auth *automationconfig.Auth) {
	if containsString(auth.DeploymentAuthMechanisms, scram256) {
		return
	}
	auth.DeploymentAuthMechanisms = append(auth.DeploymentAuthMechanisms, scram256)
}

func ensurePassword(existingPassword string, auth *automationconfig.Auth) error {
	auth.AutoPwd = existingPassword
	//if existingPassword != "" {
	//	return nil
	//}
	//automationAgentPassword, err := generate.KeyFileContents()
	//if err != nil {
	//	return err
	//}
	//auth.AutoPwd = automationAgentPassword
	return nil
}

func (s Enabler) ensureKeyFileContents(auth *automationconfig.Auth) error {

	auth.Key = s.AgentKeyFile

	//if auth.Key == "" {
	//	keyfileContents, err := generate.KeyFileContents()
	//	if err != nil {
	//		return err
	//	}
	//	auth.Key = keyfileContents
	//}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}
