package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentFlagIsCorrectlyCreated(t *testing.T) {
	parameters := []StartupParameter{
		{
			Key:   "Key1",
			Value: "Value1",
		},
		{
			Key:   "Key2",
			Value: "Value2",
		},
	}

	envVar := StartupParametersToAgentFlag(parameters...)
	assert.Equal(t, "AGENT_FLAGS", envVar.Name)
	assert.Equal(t, " -Key1 Value1 -Key2 Value2", envVar.Value)

}

func TestAgentFlagEmptyParameters(t *testing.T) {
	parameters := []StartupParameter{}

	envVar := StartupParametersToAgentFlag(parameters...)
	assert.Equal(t, "AGENT_FLAGS", envVar.Name)
	assert.Equal(t, "", envVar.Value)

}
