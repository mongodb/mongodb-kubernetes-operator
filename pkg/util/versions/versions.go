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

	baseVersion, _ := semver.Make("3.4.0")
	if v1.GTE(baseVersion) {
		ans, _ := MajorMinorVersion(versionStr)
		return ans
	}

	return ""
}

func MajorMinorVersion(version string) (string, error) {
	v1, err := semver.Make(version)
	if err != nil {
		return "", nil
	}
	return fmt.Sprintf("%d.%d", v1.Major, v1.Minor), nil
}
