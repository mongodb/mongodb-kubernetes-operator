package scram

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
)

const (
	scram256                              = "SCRAM-SHA-256"
	automationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/keyfile"
	automationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
	agentName                             = "mms-automation"
)

type Enabler struct {
	AgentPassword string
}

func (s Enabler) Enable(ac *automationconfig.AutomationConfig) error {
	if err := enableAgentAuthentication(ac); err != nil {
		return err
	}
	if err := ensurePassword(s.AgentPassword, ac); err != nil {
		return err
	}
	enableDeploymentMechanisms(ac)

	return nil
}

func enableAgentAuthentication(ac *automationconfig.AutomationConfig) error {
	ac.Auth.Disabled = false
	ac.Auth.AuthoritativeSet = true
	ac.Auth.KeyFile = automationAgentKeyFilePathInContainer
	ac.Auth.KeyFileWindows = automationAgentWindowsKeyFilePath
	ac.Auth.AutoAuthMechanisms = []string{scram256}
	ac.Auth.AutoUser = agentName

	if err := ensureKeyFileContents(ac); err != nil {
		return err
	}
	return nil
}

func enableDeploymentMechanisms(ac *automationconfig.AutomationConfig) {
	if containsString(ac.Auth.DeploymentAuthMechanisms, scram256) {
		return
	}
	ac.Auth.DeploymentAuthMechanisms = append(ac.Auth.DeploymentAuthMechanisms, scram256)
}

func ensurePassword(existingPassword string, ac *automationconfig.AutomationConfig) error {
	if existingPassword != "" {
		return nil
	}
	automationAgentPassword, err := generate.KeyFileContents()
	if err != nil {
		return err
	}
	ac.Auth.AutoPwd = automationAgentPassword
	return nil
}

func ensureKeyFileContents(ac *automationconfig.AutomationConfig) error {
	if ac.Auth.Key == "" {
		keyfileContents, err := generate.KeyFileContents()
		if err != nil {
			return err
		}
		ac.Auth.Key = keyfileContents
	}
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
