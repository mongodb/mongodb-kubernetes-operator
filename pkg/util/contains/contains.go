package contains

import mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"

func String(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}

func AuthMode(slice []mdbv1.AuthMode, s mdbv1.AuthMode) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}
