package config

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"strconv"
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
	readinessProbeLoggerBackups      = "READINESS_PROBE_LOGGER_BACKUPS"
	readinessProbeLoggerMaxSize      = "READINESS_PROBE_LOGGER_MAX_SIZE"
	readinessProbeLoggerMaxAge       = "READINESS_PROBE_LOGGER_MAX_AGE"
)

type Config struct {
	ClientSet                  kubernetes.Interface
	Namespace                  string
	Hostname                   string
	AutomationConfigSecretName string
	HealthStatusReader         io.Reader
	LogFilePath                string
	Logger                     *lumberjack.Logger
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

	logger := &lumberjack.Logger{
		Filename:   readinessProbeLogFilePath(),
		MaxBackups: readIntOrDefault(readinessProbeLoggerBackups, 5),
		MaxSize:    readInt(readinessProbeLoggerMaxSize),
		MaxAge:     readInt(readinessProbeLoggerMaxAge),
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
		Logger:                     logger,
	}, nil
}

func readinessProbeLogFilePath() string {
	return getEnvOrDefault(logPathEnv, defaultLogPath)
}

func getEnvOrDefault(envVar, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(envVar))
	if value == "" {
		return defaultValue
	}
	return value
}

// readInt returns the int value of an envvar of the given name.
// defaults to 0.
func readInt(envVarName string) int {
	return readIntOrDefault(envVarName, 0)
}

// readIntOrDefault returns the int value of an envvar of the given name.
// defaults to the given value if not specified.
func readIntOrDefault(envVarName string, defaultValue int) int {
	envVar := getEnvOrDefault(envVarName, strconv.Itoa(defaultValue))
	intValue, err := strconv.Atoi(envVar)
	if err != nil {
		return defaultValue
	}
	return intValue
}
