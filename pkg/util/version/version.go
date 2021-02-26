package version

import "github.com/blang/semver"

func MatchesRange(version, vRange string) (bool, error) {
	v, err := semver.Parse(version)
	if err != nil {
		return false, err
	}
	expectedRange, err := semver.ParseRange(vRange)
	if err != nil {
		return false, err
	}
	return expectedRange(v), nil
}
