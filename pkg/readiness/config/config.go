package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"
)

const (
	defaultAgentHealthStatusFilePath = "/var/log/mongodb-mms-automation/agent-health-status.json"
	defaultLogPath                   = "/var/log/mongodb-mms-automation/readiness.log"
	podNamespaceEnv                  = "POD_NAMESPACE"
	automationConfigSecretEnv        = "AUTOMATION_CONFIG_MAP" //nolint
	agentHealthStatusFilePathEnv     = "AGENT_STATUS_FILEPATH"
	logPathEnv                       = "LOG_FILE_PATH"
	hostNameEnv                      = "HOSTNAME"
)

type Config struct {
	ClientSet                  kubernetes.Interface
	Namespace                  string
	Hostname                   string
	AutomationConfigSecretName string
	HealthStatusReader         io.Reader
	LogFilePath                string
}

func BuildFromEnvVariables(clientSet kubernetes.Interface, isHeadless bool) (Config, error) {
	healthStatusFilePath := getEnvOrDefault(agentHealthStatusFilePathEnv, defaultAgentHealthStatusFilePath)
	logFilePath := getEnvOrDefault(logPathEnv, defaultLogPath)

	var namespace, automationConfigName, hostname string
	if isHeadless {
		var ok bool
		namespace, ok = os.LookupEnv(podNamespaceEnv)
		if !ok {
			return Config{}, fmt.Errorf("the '%s' environment variable must be set", podNamespaceEnv)
		}
		automationConfigName, ok = os.LookupEnv(automationConfigSecretEnv)
		if !ok {
			return Config{}, fmt.Errorf("the '%s' environment variable must be set", automationConfigSecretEnv)
		}
		hostname, ok = os.LookupEnv(hostNameEnv)
		if !ok {
			return Config{}, fmt.Errorf("the '%s' environment variable must be set", hostNameEnv)
		}
	}
	// Note, that we shouldn't close the file here - it will be closed very soon by the 'ioutil.ReadAll'
	// in main.go
	file, err := os.Open(healthStatusFilePath)
	if err != nil {
		return Config{}, err
	}
	return Config{
		ClientSet:                  clientSet,
		Namespace:                  namespace,
		AutomationConfigSecretName: automationConfigName,
		Hostname:                   hostname,
		HealthStatusReader:         file,
		LogFilePath:                logFilePath,
	}, nil
}

func getEnvOrDefault(envVar, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(envVar))
	if value == "" {
		return defaultValue
	}
	return value
}
