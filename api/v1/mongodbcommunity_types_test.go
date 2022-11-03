package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMongoDB_MongoURI(t *testing.T) {
	mdb := newReplicaSet(2, "my-rs", "my-namespace")
	assert.Equal(t, mdb.MongoURI(""), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/?replicaSet=my-rs")
	assert.Equal(t, mdb.MongoURI("my.cluster"), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.my.cluster:27017,my-rs-1.my-rs-svc.my-namespace.svc.my.cluster:27017/?replicaSet=my-rs")
	mdb = newReplicaSet(1, "my-single-rs", "my-single-namespace")
	assert.Equal(t, mdb.MongoURI(""), "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.cluster.local:27017/?replicaSet=my-single-rs")
	assert.Equal(t, mdb.MongoURI("my.cluster"), "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.my.cluster:27017/?replicaSet=my-single-rs")
	mdb = newReplicaSet(5, "my-big-rs", "my-big-namespace")
	assert.Equal(t, mdb.MongoURI(""), "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017/?replicaSet=my-big-rs")
	assert.Equal(t, mdb.MongoURI("my.cluster"), "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017/?replicaSet=my-big-rs")
	mdb = newReplicaSet(2, "my-rs", "my-namespace")
	mdb.Spec.AdditionalMongodConfig.Object = map[string]interface{}{
		"net.port": 40333.,
	}
	assert.Equal(t, mdb.MongoURI(""), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:40333,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:40333/?replicaSet=my-rs")
	assert.Equal(t, mdb.MongoURI("my.cluster"), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.my.cluster:40333,my-rs-1.my-rs-svc.my-namespace.svc.my.cluster:40333/?replicaSet=my-rs")
}

func TestMongodConfiguration(t *testing.T) {
	mc := NewMongodConfiguration()
	assert.Equal(t, mc.Object, map[string]interface{}{})
	assert.Equal(t, mc.GetDBDataDir(), "/data")
	assert.Equal(t, mc.GetDBPort(), 27017)
	mc.SetOption("net.port", 40333.)
	assert.Equal(t, mc.GetDBPort(), 40333)
	mc.SetOption("storage", map[string]interface{}{"dbPath": "/other/data/path"})
	assert.Equal(t, mc.GetDBDataDir(), "/other/data/path")
	assert.Equal(t, mc.Object, map[string]interface{}{
		"net": map[string]interface{}{
			"port": 40333.,
		},
		"storage": map[string]interface{}{
			"dbPath": "/other/data/path",
		},
	})
}

func TestMongodConfigurationWithNestedMapsAfterUnmarshalling(t *testing.T) {
	jsonStr := `
		{
			"net.port": 40333,
			"storage.dbPath": "/other/data/path"
		}
	`
	mc := NewMongodConfiguration()
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &mc))
	assert.Equal(t, map[string]interface{}{
		"net": map[string]interface{}{
			"port": 40333.,
		},
		"storage": map[string]interface{}{
			"dbPath": "/other/data/path",
		},
	}, mc.Object)
}

func TestGetScramOptions(t *testing.T) {
	t.Run("Default AutoAuthMechanism set if modes array empty", func(t *testing.T) {
		mdb := newModesArray(nil, "empty-modes-array", "my-namespace")

		options := mdb.GetScramOptions()

		assert.EqualValues(t, defaultMode, options.AutoAuthMechanism)
		assert.EqualValues(t, []string{}, options.AutoAuthMechanisms)
	})
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

func TestGetConnectionStringSecretName(t *testing.T) {
	testusers := []struct {
		in  MongoDBUser
		exp string
	}{
		{
			MongoDBUser{
				Name:                       "mdb-0",
				DB:                         "admin",
				ScramCredentialsSecretName: "scram-credential-secret-name-0",
			},
			"replica-set-admin-mdb-0",
		},
		{
			MongoDBUser{
				Name:                       "?_normalize/_-username/?@with/[]?no]?/:allowed:chars[only?",
				DB:                         "admin",
				ScramCredentialsSecretName: "scram-credential-secret-name-0",
			},
			"replica-set-admin-normalize-username-with-no-allowed-chars-only",
		},
		{
			MongoDBUser{
				Name:                       "AppUser",
				DB:                         "Administrators",
				ScramCredentialsSecretName: "scram-credential-secret-name-0",
			},
			"replica-set-administrators-appuser",
		},
		{
			MongoDBUser{
				Name:                       "mdb-0",
				DB:                         "admin",
				ScramCredentialsSecretName: "scram-credential-secret-name-0",
				ConnectionStringSecretName: "connection-string-secret",
			},
			"connection-string-secret",
		},
	}

	for _, tt := range testusers {
		assert.Equal(t, tt.exp, tt.in.GetConnectionStringSecretName("replica-set"))
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

func newModesArray(modes []AuthMode, name, namespace string) MongoDBCommunity {
	return MongoDBCommunity{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: MongoDBCommunitySpec{
			Security: Security{
				Authentication: Authentication{
					Modes:              modes,
					IgnoreUnknownUsers: nil,
				},
			},
		},
	}
}
