package versions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateFCV(t *testing.T) {
	t.Run("Version > 3.4 works as expected", func(t *testing.T) {
		assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.0"))
		assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.5"))
		assert.Equal(t, "4.2", CalculateFeatureCompatibilityVersion("4.2.8"))
		assert.Equal(t, "4.4", CalculateFeatureCompatibilityVersion("4.4.0"))
		assert.Equal(t, "4.4", CalculateFeatureCompatibilityVersion("4.4.1"))
		assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.8"))
		assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.5"))
		assert.Equal(t, "4.0", CalculateFeatureCompatibilityVersion("4.0.12"))
	})

	t.Run("Version == 3.4 works as expected", func(t *testing.T) {
		assert.Equal(t, "3.4", CalculateFeatureCompatibilityVersion("3.4.12"))
		assert.Equal(t, "3.4", CalculateFeatureCompatibilityVersion("3.4.10"))
		assert.Equal(t, "3.4", CalculateFeatureCompatibilityVersion("3.4.5"))
		assert.Equal(t, "3.4", CalculateFeatureCompatibilityVersion("3.4.0"))
	})

	t.Run("Version < 3.4 returns empty string", func(t *testing.T) {
		assert.Equal(t, "", CalculateFeatureCompatibilityVersion("3.2.1"))
		assert.Equal(t, "", CalculateFeatureCompatibilityVersion("3.3.20"))
		assert.Equal(t, "", CalculateFeatureCompatibilityVersion("2.0.12"))
		assert.Equal(t, "", CalculateFeatureCompatibilityVersion("1.4.5"))
	})
}
