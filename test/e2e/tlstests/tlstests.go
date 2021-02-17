package tlstests

import (
	"context"
	"io/ioutil"
	"path"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
)

// EnableTLS will upgrade an existing TLS cluster to use TLS.
func EnableTLS(mdb *v1.MongoDBCommunity, optional bool) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *v1.MongoDBCommunity) {
			db.Spec.Security.TLS = e2eutil.NewTestTLSConfig(optional)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func RotateCertificate(mdb *v1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		// Load new certificate and key
		cert, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "server_rotated.crt"))
		assert.NoError(t, err)
		key, err := ioutil.ReadFile(path.Join(e2eutil.TestdataDir, "server_rotated.key"))
		assert.NoError(t, err)

		certKeySecret := secret.Builder().
			SetName(mdb.Spec.Security.TLS.CertificateKeySecret.Name).
			SetNamespace(mdb.Namespace).
			SetField("tls.crt", string(cert)).
			SetField("tls.key", string(key)).
			Build()

		err = e2eutil.TestClient.Update(context.TODO(), &certKeySecret)
		assert.NoError(t, err)
	}
}
