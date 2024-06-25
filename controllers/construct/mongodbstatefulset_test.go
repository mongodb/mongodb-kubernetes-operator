package construct

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/readiness/config"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestCollectEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		envSetup    map[string]string
		expectedEnv []corev1.EnvVar
	}{
		{
			name: "Basic env vars set",
			envSetup: map[string]string{
				config.ReadinessProbeLoggerBackups: "3",
				config.ReadinessProbeLoggerMaxSize: "10M",
				config.ReadinessProbeLoggerMaxAge:  "7",
				config.WithAgentFileLogging:        "enabled",
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  config.AgentHealthStatusFilePathEnv,
					Value: "/healthstatus/agent-health-status.json",
				},
				{
					Name:  config.ReadinessProbeLoggerBackups,
					Value: "3",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxSize,
					Value: "10M",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxAge,
					Value: "7",
				},
				{
					Name:  config.WithAgentFileLogging,
					Value: "enabled",
				},
			},
		},
		{
			name: "Additional env var set",
			envSetup: map[string]string{
				config.ReadinessProbeLoggerBackups:  "3",
				config.ReadinessProbeLoggerMaxSize:  "10M",
				config.ReadinessProbeLoggerMaxAge:   "7",
				config.ReadinessProbeLoggerCompress: "true",
				config.WithAgentFileLogging:         "enabled",
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  config.AgentHealthStatusFilePathEnv,
					Value: "/healthstatus/agent-health-status.json",
				},
				{
					Name:  config.ReadinessProbeLoggerBackups,
					Value: "3",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxSize,
					Value: "10M",
				},
				{
					Name:  config.ReadinessProbeLoggerMaxAge,
					Value: "7",
				},
				{
					Name:  config.ReadinessProbeLoggerCompress,
					Value: "true",
				},
				{
					Name:  config.WithAgentFileLogging,
					Value: "enabled",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			for key, value := range tt.envSetup {
				t.Setenv(key, value)
			}

			actualEnvVars := collectEnvVars()

			assert.EqualValues(t, tt.expectedEnv, actualEnvVars)
		})
	}
}
