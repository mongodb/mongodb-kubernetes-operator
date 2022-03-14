package names

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"Valid input",
			"secret-name",
			"secret-name",
		},
		{
			"Allowed characters are sanitized",
			"?_normalize/_-username/?@with/[]?no]?/:allowed:chars[only?",
			"normalize-username-with-no-allowed-chars-only",
		},
		{
			"Name is converted to lowercase",
			"Capital-Letters",
			"capital-letters",
		},
		{
			"Name is shortened to correct length",
			strings.Repeat("a", 300),
			strings.Repeat("a", validation.DNS1123SubdomainMaxLength),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, NormalizeName(tc.input))
		})
	}
}
