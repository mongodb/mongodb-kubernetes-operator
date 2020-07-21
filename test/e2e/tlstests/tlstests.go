package tlstests

import (
	"testing"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
)

// EnableTLS will upgrade an existing TLS cluster to use TLS.
func EnableTLS(mdb *v1.MongoDB, optional bool) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *v1.MongoDB) {
			db.Spec.Security.TLS = e2eutil.NewTestTLSConfig(optional)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
