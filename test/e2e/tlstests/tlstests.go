package tlstests

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func ExtendCACertificate(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		certGVR := schema.GroupVersionResource{
			Group:    "cert-manager.io",
			Version:  "v1",
			Resource: "certificates",
		}
		caCertificateClient := e2eutil.TestClient.DynamicClient.Resource(certGVR).Namespace(mdb.Namespace)
		patch := []interface{}{
			map[string]interface{}{
				"op":    "replace",
				"path":  "/spec/duration",
				"value": "8760h0m0s",
			},
			map[string]interface{}{
				"op":    "replace",
				"path":  "/spec/renewBefore",
				"value": "720h0m0s",
			},
			map[string]interface{}{
				"op":    "add",
				"path":  "/spec/dnsNames",
				"value": []string{"*.ca-example.domain"},
			},
		}
		payload, err := json.Marshal(patch)
		assert.NoError(t, err)
		_, err = caCertificateClient.Patch(context.TODO(), "tls-selfsigned-ca", types.JSONPatchType, payload, metav1.PatchOptions{})
		assert.NoError(t, err)
	}
}

func RotateCertificate(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		certKeySecretName := mdb.TLSSecretNamespacedName()
		rotateCertManagerSecret(certKeySecretName, t)
	}
}

func RotateCACertificate(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		caCertSecretName := mdb.TLSCaCertificateSecretNamespacedName()
		rotateCertManagerSecret(caCertSecretName, t)
	}
}

func rotateCertManagerSecret(secretName types.NamespacedName, t *testing.T) {
	currentSecret := corev1.Secret{}
	err := e2eutil.TestClient.Get(context.TODO(), secretName, &currentSecret)
	assert.NoError(t, err)

	// delete current cert secret, cert-manager should generate a new one
	err = e2eutil.TestClient.Delete(context.TODO(), &currentSecret)
	assert.NoError(t, err)

	newSecret := corev1.Secret{}
	err = wait.Poll(5*time.Second, 1*time.Minute, func() (done bool, err error) {
		if err := e2eutil.TestClient.Get(context.TODO(), secretName, &newSecret); err != nil {
			return false, nil
		}
		return true, nil
	})
	assert.NoError(t, err)
	assert.False(t, bytes.Equal(currentSecret.Data[corev1.TLSCertKey], newSecret.Data[corev1.TLSCertKey]))
}
