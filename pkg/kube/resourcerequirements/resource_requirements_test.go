package resourcerequirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewResourceRequirements_GetsConstructedCorrectly(t *testing.T) {
	requirements, err := newRequirements("0.5", "2.0", "500", "1000")
	assert.NoError(t, err)
	assert.Equal(t, resource.MustParse("0.5"), *requirements.Limits.Cpu())
	assert.Equal(t, resource.MustParse("2.0"), *requirements.Limits.Memory())
	assert.Equal(t, resource.MustParse("500"), *requirements.Requests.Cpu())
	assert.Equal(t, resource.MustParse("1000"), *requirements.Requests.Memory())
}

func TestBadInput_ReturnsError(t *testing.T) {
	_, err := newRequirements("BAD_INPUT", "2.0", "500", "1000")
	assert.Error(t, err)
}

func TestDefaultValues_DontReturnError(t *testing.T) {
	_, err := newDefaultRequirements()
	assert.NoError(t, err, "default requirements should never result in an error")
}
