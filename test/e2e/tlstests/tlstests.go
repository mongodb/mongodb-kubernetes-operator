package tlstests

import (
	"context"
	"io/ioutil"
	"testing"

	f "github.com/operator-framework/operator-sdk/pkg/test"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	v1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
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
		cert, err := ioutil.ReadFile("testdata/tls/server_rotated.crt")
		assert.NoError(t, err)
		key, err := ioutil.ReadFile("testdata/tls/server_rotated.key")
		assert.NoError(t, err)

		certKeySecret := secret.Builder().
			SetName(mdb.Spec.Security.TLS.CertificateKeySecret.Name).
			SetNamespace(mdb.Namespace).
			SetField("tls.crt", string(cert)).
			SetField("tls.key", string(key)).
			Build()

		err = f.Global.Client.Update(context.TODO(), &certKeySecret)
		assert.NoError(t, err)
	}
}
