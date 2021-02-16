package validation

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/pkg/errors"
)

func Validate(oldSpec, newSpec mdbv1.MongoDBCommunitySpec) error {
	if oldSpec.Security.TLS.Enabled && !newSpec.Security.TLS.Enabled {
		return errors.New("TLS can't be set to disabled after it has been enabled")
	}

	return nil
}
