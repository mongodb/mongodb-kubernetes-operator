package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMongoDB_MongoURI(t *testing.T) {
	mdb := newReplicaSet(2, "my-rs", "my-namespace")
	assert.Equal(t, mdb.MongoURI(), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017")
	mdb = newReplicaSet(1, "my-single-rs", "my-single-namespace")
	assert.Equal(t, mdb.MongoURI(), "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.cluster.local:27017")
	mdb = newReplicaSet(5, "my-big-rs", "my-big-namespace")
	assert.Equal(t, mdb.MongoURI(), "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017")
}

func TestGetFCV(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")
	mdb.Spec.Version = "4.2.0"
	assert.Equal(t, "4.2", mdb.GetFCV())

	mdb.Spec.FeatureCompatibilityVersion = "4.0"
	assert.Equal(t, "4.0", mdb.GetFCV())

	mdb.Spec.FeatureCompatibilityVersion = ""
	assert.Equal(t, "4.2", mdb.GetFCV())
}

func TestGetScramCredentialsSecretName(t *testing.T) {
	testusers := []struct {
		in  MongoDBUser
		exp string
	}{
		{
			MongoDBUser{
				Name: "mdb-0",
				DB:   "admin",
				Roles: []Role{
					// roles on testing db for general connectivity
					{
						DB:   "testing",
						Name: "readWrite",
					},
					{
						DB:   "testing",
						Name: "clusterAdmin",
					},
					// admin roles for reading FCV
					{
						DB:   "admin",
						Name: "readWrite",
					},
					{
						DB:   "admin",
						Name: "clusterAdmin",
					},
				},
				ScramCredentialsSecretName: "scram-credential-secret-name-0",
			},
			"scram-credential-secret-name-0-scram-credentials",
		},
		{
			MongoDBUser{
				Name: "mdb-1",
				DB:   "admin",
				Roles: []Role{
					// roles on testing db for general connectivity
					{
						DB:   "testing",
						Name: "readWrite",
					},
					{
						DB:   "testing",
						Name: "clusterAdmin",
					},
					// admin roles for reading FCV
					{
						DB:   "admin",
						Name: "readWrite",
					},
					{
						DB:   "admin",
						Name: "clusterAdmin",
					},
				},
				ScramCredentialsSecretName: "scram-credential-secret-name-1",
			},
			"scram-credential-secret-name-1-scram-credentials",
		},
	}

	for _, tt := range testusers {
		assert.Equal(t, tt.exp, tt.in.GetScramCredentialsSecretName())
	}

}

func newReplicaSet(members int, name, namespace string) MongoDBCommunity {
	return MongoDBCommunity{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: MongoDBCommunitySpec{
			Members: members,
		},
	}
}
