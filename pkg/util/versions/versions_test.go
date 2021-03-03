package versions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateFCV(t *testing.T) {
	assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.0"))
	assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.5"))
	assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.8"))
	assert.Equal(t, "4.4", CalculateFeatureCompatibilityVersion("4.4.0"))
	assert.Equal(t, "4.4", CalculateFeatureCompatibilityVersion("4.4.1"))
	assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.8"))
	assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.5"))
	assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.12"))
}

func TestMajorMinor(t *testing.T) {
	res, err := MajorMinorVersion("4.2.3")
	assert.NoError(t, err)
	assert.Equal(t, "4.2", res)

	res, err = MajorMinorVersion("4.0.0")
	assert.NoError(t, err)
	assert.Equal(t, "4.0", res)

	res, err = MajorMinorVersion("2.2.0")
	assert.NoError(t, err)
	assert.Equal(t, "2.2", res)

	res, err = MajorMinorVersion("3.6.10")
	assert.NoError(t, err)
	assert.Equal(t, "3.6", res)
}
