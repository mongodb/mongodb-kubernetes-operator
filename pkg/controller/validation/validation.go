package validation

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/pkg/errors"
)

func Validate(oldSpec, newSpec mdbv1.MongoDBSpec) error {
	if oldSpec.Security.TLS.Enabled && !newSpec.Security.TLS.Enabled {
		return errors.New("TLS can't be set to disabled after it has been enabled")
	}

	return nil
}
