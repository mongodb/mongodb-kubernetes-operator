package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"

	"k8s.io/client-go/kubernetes"
)

const (
	DefaultAgentHealthStatusFilePath = "/var/log/mongodb-mms-automation/agent-health-status.json"
	AgentHealthStatusFilePathEnv     = "AGENT_STATUS_FILEPATH"

	defaultLogPath              = "/var/log/mongodb-mms-automation/readiness.log"
	podNamespaceEnv             = "POD_NAMESPACE"
	automationConfigSecretEnv   = "AUTOMATION_CONFIG_MAP" //nolint
	logPathEnv                  = "LOG_FILE_PATH"
	hostNameEnv                 = "HOSTNAME"
	readinessProbeLoggerBackups = "READINESS_PROBE_LOGGER_BACKUPS"
	readinessProbeLoggerMaxSize = "READINESS_PROBE_LOGGER_MAX_SIZE"
	readinessProbeLoggerMaxAge  = "READINESS_PROBE_LOGGER_MAX_AGE"
)

type Config struct {
	ClientSet                  kubernetes.Interface
	Namespace                  string
	Hostname                   string
	AutomationConfigSecretName string
	HealthStatusReader         io.Reader
	LogFilePath                string
}

func BuildFromEnvVariables(clientSet kubernetes.Interface, isHeadless bool, file *os.File) (Config, error) {
	logFilePath := GetEnvOrDefault(logPathEnv, defaultLogPath)

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
	return Config{
		ClientSet:                  clientSet,
		Namespace:                  namespace,
		AutomationConfigSecretName: automationConfigName,
		Hostname:                   hostname,
		HealthStatusReader:         file,
		LogFilePath:                logFilePath,
	}, nil
}

func GetLogger() *lumberjack.Logger {
	logger := &lumberjack.Logger{
		Filename:   readinessProbeLogFilePath(),
		MaxBackups: readIntOrDefault(readinessProbeLoggerBackups, 5),
		MaxSize:    readInt(readinessProbeLoggerMaxSize),
		MaxAge:     readInt(readinessProbeLoggerMaxAge),
	}
	return logger
}

func readinessProbeLogFilePath() string {
	return GetEnvOrDefault(logPathEnv, defaultLogPath)
}

func GetEnvOrDefault(envVar, defaultValue string) string {
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
	envVar := GetEnvOrDefault(envVarName, strconv.Itoa(defaultValue))
	intValue, err := strconv.Atoi(envVar)
	if err != nil {
		return defaultValue
	}
	return intValue
}
