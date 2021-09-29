package tlstests

import (
	"bytes"
	"context"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

// EnableTLS will upgrade an existing TLS cluster to use TLS.
func EnableTLS(mdb *mdbv1.MongoDBCommunity, optional bool) func(*testing.T) {
	return func(t *testing.T) {
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Security.TLS = e2eutil.NewTestTLSConfig(optional)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func RotateCertificate(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		certKeySecretName := types.NamespacedName{Name: mdb.Spec.Security.TLS.CertificateKeySecret.Name, Namespace: mdb.Namespace}

		currentSecret := corev1.Secret{}
		err := e2eutil.TestClient.Get(context.TODO(), certKeySecretName, &currentSecret)
		assert.NoError(t, err)

		// delete current cert secret, cert-manager should generate a new one
		err = e2eutil.TestClient.Delete(context.TODO(), &currentSecret)
		assert.NoError(t, err)

		newSecret := corev1.Secret{}
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (done bool, err error) {
			if err := e2eutil.TestClient.Get(context.TODO(), certKeySecretName, &newSecret); err != nil {
				return false, nil
			}
			return true, nil
		})
		assert.NoError(t, err)
		assert.False(t, bytes.Equal(currentSecret.Data["tls.crt"], newSecret.Data["tls.crt"]))
	}
}
