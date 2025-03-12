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
	WithAgentFileLogging             = "MDB_WITH_AGENT_FILE_LOGGING"

	defaultLogPath               = "/var/log/mongodb-mms-automation/readiness.log"
	podNamespaceEnv              = "POD_NAMESPACE"
	automationConfigSecretEnv    = "AUTOMATION_CONFIG_MAP" //nolint
	logPathEnv                   = "LOG_FILE_PATH"
	hostNameEnv                  = "HOSTNAME"
	ReadinessProbeLoggerBackups  = "READINESS_PROBE_LOGGER_BACKUPS"
	ReadinessProbeLoggerMaxSize  = "READINESS_PROBE_LOGGER_MAX_SIZE"
	ReadinessProbeLoggerMaxAge   = "READINESS_PROBE_LOGGER_MAX_AGE"
	ReadinessProbeLoggerCompress = "READINESS_PROBE_LOGGER_COMPRESS"
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
		namespace, ok = os.LookupEnv(podNamespaceEnv) // nolint:forbidigo
		if !ok {
			return Config{}, fmt.Errorf("the '%s' environment variable must be set", podNamespaceEnv)
		}
		automationConfigName, ok = os.LookupEnv(automationConfigSecretEnv) // nolint:forbidigo
		if !ok {
			return Config{}, fmt.Errorf("the '%s' environment variable must be set", automationConfigSecretEnv)
		}
		hostname, ok = os.LookupEnv(hostNameEnv) // nolint:forbidigo
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
		MaxBackups: readIntOrDefault(ReadinessProbeLoggerBackups, 5),
		MaxSize:    readIntOrDefault(ReadinessProbeLoggerMaxSize, 5),
		MaxAge:     readInt(ReadinessProbeLoggerMaxAge),
		Compress:   ReadBoolWitDefault(ReadinessProbeLoggerCompress, "false"),
	}
	return logger
}

func readinessProbeLogFilePath() string {
	return GetEnvOrDefault(logPathEnv, defaultLogPath)
}

func GetEnvOrDefault(envVar, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(envVar)) // nolint:forbidigo
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

// ReadBoolWitDefault returns the boolean value of an envvar of the given name.
func ReadBoolWitDefault(envVarName string, defaultValue string) bool {
	envVar := GetEnvOrDefault(envVarName, defaultValue)
	return strings.TrimSpace(strings.ToLower(envVar)) == "true"
}
