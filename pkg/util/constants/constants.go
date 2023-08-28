package constants

const (
	ExternalDB                            = "$external"
	Sha256                                = "SCRAM-SHA-256"
	Sha1                                  = "MONGODB-CR"
	X509                                  = "MONGODB-X509"
	AutomationAgentKeyFilePathInContainer = "/var/lib/mongodb-mms-automation/authentication/keyfile"
	AgentName                             = "mms-automation"
	AgentPasswordKey                      = "password"
	AgentKeyfileKey                       = "keyfile"
	AgentPemFile                          = "agent-certs-pem"
	AutomationAgentWindowsKeyFilePath     = "%SystemDrive%\\MMSAutomation\\versions\\keyfile"
)
