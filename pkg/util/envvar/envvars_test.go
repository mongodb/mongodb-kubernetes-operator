package envvar

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvOrDefault(t *testing.T) {
	t.Setenv("env1", "val1")

	val := GetEnvOrDefault("env1", "defaultVal1")
	assert.Equal(t, "val1", val)

	val2 := GetEnvOrDefault("env2", "defaultVal2")
	assert.Equal(t, "defaultVal2", val2)
}
