package agent

import corev1 "k8s.io/api/core/v1"

type StartupParameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// StartupParametersToAgentFlag takes a slice of StartupParameters
// and concatenates them into a single string that is then
// returned as env variable AGENT_FLAGS
func StartupParametersToAgentFlag(parameters ...StartupParameter) corev1.EnvVar {
	agentParams := ""
	for _, param := range parameters {
		agentParams += " -" + param.Key + " " + param.Value
	}
	return corev1.EnvVar{Name: "AGENT_FLAGS", Value: agentParams}
}
