package names

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

// NormalizeName returns a string that conforms to RFC-1123
func NormalizeName(name string) string {
	errors := validation.IsDNS1123Subdomain(name)
	if len(errors) == 0 {
		return name
	}

	// convert name to lowercase and replace invalid characters with '-'
	name = strings.ToLower(name)
	re := regexp.MustCompile("[^a-z0-9-]+")
	name = re.ReplaceAllString(name, "-")

	// Remove duplicate `-` resulting from contiguous non-allowed chars.
	re = regexp.MustCompile(`\-+`)
	name = re.ReplaceAllString(name, "-")

	name = strings.Trim(name, "-")

	if len(name) > validation.DNS1123SubdomainMaxLength {
		name = name[0:validation.DNS1123SubdomainMaxLength]
	}
	return name
}
