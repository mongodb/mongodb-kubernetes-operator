package resourcerequirements

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"testing"
)

func TestNewResourceRequirements_GetsConstructedCorrectly(t *testing.T) {
	requirements, err := New("0.5", "2.0", "500", "1000")
	assert.NoError(t, err)
	assert.Equal(t, resource.MustParse("0.5"), *requirements.Limits.Cpu())
	assert.Equal(t, resource.MustParse("2.0"), *requirements.Limits.Memory())
	assert.Equal(t, resource.MustParse("500"), *requirements.Requests.Cpu())
	assert.Equal(t, resource.MustParse("1000"), *requirements.Requests.Memory())
}

func TestBadInput_ReturnsError(t *testing.T) {
	_, err := New("BAD_INPUT", "2.0", "500", "1000")
	assert.Error(t, err)
}
