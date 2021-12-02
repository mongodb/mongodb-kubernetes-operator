package versions

import (
	"fmt"

	"github.com/blang/semver"
)

// CalculateFeatureCompatibilityVersion returns a version in the format of "x.y"
func CalculateFeatureCompatibilityVersion(versionStr string) string {
	v1, err := semver.Make(versionStr)
	if err != nil {
		return ""
	}

	if v1.GTE(semver.MustParse("3.4.0")) {
		return fmt.Sprintf("%d.%d", v1.Major, v1.Minor)
	}

	return ""
}
