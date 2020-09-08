package envvar

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvOrDefault(t *testing.T) {

	err := os.Setenv("env1", "val1")
	assert.NoError(t, err)

	val := GetEnvOrDefault("env1", "defaultVal1")
	assert.Equal(t, "val1", val)

	val2 := GetEnvOrDefault("env2", "defaultVal2")
	assert.Equal(t, "defaultVal2", val2)
}
