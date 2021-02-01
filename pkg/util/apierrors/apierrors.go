package apierrors

import "strings"

// objectModifiedText is an error indicating that we are trying to update a resource that has since been updated.
// in this we just want to retry but not log it as an error.
var objectModifiedText = "the object has been modified; please apply your changes to the latest version and try again"

// IsTransient returns a boolean indicating if a given error is transient.
func IsTransient(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), objectModifiedText)
}
