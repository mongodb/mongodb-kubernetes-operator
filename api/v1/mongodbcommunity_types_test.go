package v1

import (
	"encoding/json"

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type args struct {
	members                          int
	name                             string
	namespace                        string
	clusterDomain                    string
	additionalMongodConfig           map[string]interface{}
	additionalConnectionStringConfig map[string]interface{}
	userConnectionStringConfig       map[string]interface{}
	connectionString                 string
}

func TestMongoDB_MongoURI(t *testing.T) {
	tests := []args{
		{
			members:          2,
			name:             "my-rs",
			namespace:        "my-namespace",
			clusterDomain:    "",
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/?replicaSet=my-rs",
		},
		{
			members:          2,
			name:             "my-rs",
			namespace:        "my-namespace",
			clusterDomain:    "my.cluster",
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.my.cluster:27017,my-rs-1.my-rs-svc.my-namespace.svc.my.cluster:27017/?replicaSet=my-rs",
		},
		{
			members:          1,
			name:             "my-single-rs",
			namespace:        "my-single-namespace",
			clusterDomain:    "",
			connectionString: "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.cluster.local:27017/?replicaSet=my-single-rs",
		},
		{
			members:          1,
			name:             "my-single-rs",
			namespace:        "my-single-namespace",
			clusterDomain:    "my.cluster",
			connectionString: "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.my.cluster:27017/?replicaSet=my-single-rs",
		},
		{
			members:          5,
			name:             "my-big-rs",
			namespace:        "my-big-namespace",
			clusterDomain:    "",
			connectionString: "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017/?replicaSet=my-big-rs",
		},
		{
			members:          5,
			name:             "my-big-rs",
			namespace:        "my-big-namespace",
			clusterDomain:    "my.cluster",
			connectionString: "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.my.cluster:27017/?replicaSet=my-big-rs",
		},
		{
			members:       2,
			name:          "my-rs",
			namespace:     "my-namespace",
			clusterDomain: "",
			additionalMongodConfig: map[string]interface{}{
				"net.port": 40333.,
			},
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:40333,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:40333/?replicaSet=my-rs",
		},
		{
			members:       2,
			name:          "my-rs",
			namespace:     "my-namespace",
			clusterDomain: "my.cluster",
			additionalMongodConfig: map[string]interface{}{
				"net.port": 40333.,
			},
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.my.cluster:40333,my-rs-1.my-rs-svc.my-namespace.svc.my.cluster:40333/?replicaSet=my-rs",
		},
	}

	for _, params := range tests {
		mdb := newReplicaSet(params.members, params.name, params.namespace)
		mdb.Spec.AdditionalMongodConfig.Object = params.additionalMongodConfig
		assert.Equal(t, mdb.MongoURI(params.clusterDomain), params.connectionString)
	}
}

func TestMongoDB_MongoURI_With_Options(t *testing.T) {
	tests := []args{
		{
			members:                          2,
			name:                             "my-rs",
			namespace:                        "my-namespace",
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			connectionString:                 "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/?replicaSet=my-rs&readPreference=primary",
		},
		{
			members:   2,
			name:      "my-rs",
			namespace: "my-namespace",
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true},
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/?replicaSet=my-rs&readPreference=primary",
		},
		{
			members:   1,
			name:      "my-single-rs",
			namespace: "my-single-namespace",
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary"},
			connectionString: "mongodb://my-single-rs-0.my-single-rs-svc.my-single-namespace.svc.cluster.local:27017/?replicaSet=my-single-rs&readPreference=primary",
		},
		{
			members:   5,
			name:      "my-big-rs",
			namespace: "my-big-namespace",
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary"},
			connectionString: "mongodb://my-big-rs-0.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-1.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-2.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-3.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017,my-big-rs-4.my-big-rs-svc.my-big-namespace.svc.cluster.local:27017/?replicaSet=my-big-rs&readPreference=primary",
		},
		{
			members:   2,
			name:      "my-rs",
			namespace: "my-namespace",
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary"},
			additionalMongodConfig: map[string]interface{}{
				"net.port": 40333.,
			},
			connectionString: "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:40333,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:40333/?replicaSet=my-rs&readPreference=primary",
		},
	}

	for _, params := range tests {
		mdb := newReplicaSet(params.members, params.name, params.namespace)
		mdb.Spec.AdditionalMongodConfig.Object = params.additionalMongodConfig
		mdb.Spec.AdditionalConnectionStringConfig.Object = params.additionalConnectionStringConfig
		assert.Equal(t, mdb.MongoURI(params.clusterDomain), params.connectionString)
	}
}

func TestMongoDB_MongoSRVURI(t *testing.T) {
	mdb := newReplicaSet(2, "my-rs", "my-namespace")
	assert.Equal(t, mdb.MongoSRVURI(""), "mongodb+srv://my-rs-svc.my-namespace.svc.cluster.local/?replicaSet=my-rs")
	assert.Equal(t, mdb.MongoSRVURI("my.cluster"), "mongodb+srv://my-rs-svc.my-namespace.svc.my.cluster/?replicaSet=my-rs")
}

func TestMongoDB_MongoSRVURI_With_Options(t *testing.T) {
	mdb := newReplicaSet(2, "my-rs", "my-namespace")
	mdb.Spec.AdditionalConnectionStringConfig.Object = map[string]interface{}{
		"readPreference": "primary"}
	assert.Equal(t, mdb.MongoSRVURI(""), "mongodb+srv://my-rs-svc.my-namespace.svc.cluster.local/?replicaSet=my-rs&readPreference=primary")
	assert.Equal(t, mdb.MongoSRVURI("my.cluster"), "mongodb+srv://my-rs-svc.my-namespace.svc.my.cluster/?replicaSet=my-rs&readPreference=primary")

	mdb = newReplicaSet(2, "my-rs", "my-namespace")
	mdb.Spec.AdditionalConnectionStringConfig.Object = map[string]interface{}{
		"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true}
	assert.Equal(t, mdb.MongoSRVURI(""), "mongodb+srv://my-rs-svc.my-namespace.svc.cluster.local/?replicaSet=my-rs&readPreference=primary")
	assert.Equal(t, mdb.MongoSRVURI("my.cluster"), "mongodb+srv://my-rs-svc.my-namespace.svc.my.cluster/?replicaSet=my-rs&readPreference=primary")
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

func TestGetAuthOptions(t *testing.T) {
	t.Run("Default AutoAuthMechanism set if modes array empty", func(t *testing.T) {
		mdb := newModesArray(nil, "empty-modes-array", "my-namespace")

		options := mdb.GetAuthOptions()

		assert.EqualValues(t, defaultMode, options.AutoAuthMechanism)
		assert.EqualValues(t, []string{constants.Sha256}, options.AuthMechanisms)
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
		{
			MongoDBUser{
				Name:                            "mdb-2",
				DB:                              "admin",
				ScramCredentialsSecretName:      "scram-credential-secret-name-2",
				ConnectionStringSecretName:      "connection-string-secret-2",
				ConnectionStringSecretNamespace: "other-namespace",
			},
			"connection-string-secret-2",
		},
	}

	for _, tt := range testusers {
		assert.Equal(t, tt.exp, tt.in.GetConnectionStringSecretName("replica-set"))
	}
}

func TestMongoDBCommunity_MongoAuthUserURI(t *testing.T) {
	testuser := authtypes.User{
		Username: "testuser",
		Database: "admin",
	}
	mdb := newReplicaSet(2, "my-rs", "my-namespace")

	tests := []args{
		{
			additionalConnectionStringConfig: map[string]interface{}{},
			connectionString:                 "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			connectionString:                 "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true},
			connectionString: "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"readPreference": "primary"},
			connectionString:                 "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true},
			connectionString: "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"readPreference": "secondary"},
			connectionString:                 "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&readPreference=secondary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"retryReads": true},
			connectionString:                 "mongodb://testuser:password@my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/admin?replicaSet=my-rs&ssl=false&retryReads=true&readPreference=primary",
		},
	}

	for _, params := range tests {
		mdb.Spec.AdditionalConnectionStringConfig.Object = params.additionalConnectionStringConfig
		testuser.ConnectionStringOptions = params.userConnectionStringConfig
		assert.Equal(t, mdb.MongoAuthUserURI(testuser, "password", ""), params.connectionString)
	}

	testuser = authtypes.User{
		Username: "testuser",
		Database: "$external",
	}
	mdb = newReplicaSet(2, "my-rs", "my-namespace")

	assert.Equal(t, mdb.MongoAuthUserURI(testuser, "", ""), "mongodb://my-rs-0.my-rs-svc.my-namespace.svc.cluster.local:27017,my-rs-1.my-rs-svc.my-namespace.svc.cluster.local:27017/$external?replicaSet=my-rs&ssl=false")
}

func TestMongoDBCommunity_MongoAuthUserSRVURI(t *testing.T) {
	testuser := authtypes.User{
		Username: "testuser",
		Database: "admin",
	}
	mdb := newReplicaSet(2, "my-rs", "my-namespace")

	tests := []args{
		{
			additionalConnectionStringConfig: map[string]interface{}{},
			connectionString:                 "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			connectionString:                 "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true},
			connectionString: "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"readPreference": "primary"},
			connectionString:                 "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig: map[string]interface{}{
				"readPreference": "primary", "replicaSet": "differentName", "tls": true, "ssl": true},
			connectionString: "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&readPreference=primary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"readPreference": "secondary"},
			connectionString:                 "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&readPreference=secondary",
		},
		{
			additionalConnectionStringConfig: map[string]interface{}{"readPreference": "primary"},
			userConnectionStringConfig:       map[string]interface{}{"retryReads": true},
			connectionString:                 "mongodb+srv://testuser:password@my-rs-svc.my-namespace.svc.cluster.local/admin?replicaSet=my-rs&ssl=false&retryReads=true&readPreference=primary",
		},
	}

	for _, params := range tests {
		mdb.Spec.AdditionalConnectionStringConfig.Object = params.additionalConnectionStringConfig
		testuser.ConnectionStringOptions = params.userConnectionStringConfig
		assert.Equal(t, mdb.MongoAuthUserSRVURI(testuser, "password", ""), params.connectionString)
	}

	testuser = authtypes.User{
		Username: "testuser",
		Database: "$external",
	}
	mdb = newReplicaSet(2, "my-rs", "my-namespace")

	assert.Equal(t, mdb.MongoAuthUserSRVURI(testuser, "", ""), "mongodb+srv://my-rs-svc.my-namespace.svc.cluster.local/$external?replicaSet=my-rs&ssl=false")
}

func TestConvertAuthModeToAuthMechanism(t *testing.T) {
	assert.Equal(t, constants.X509, ConvertAuthModeToAuthMechanism("X509"))
	assert.Equal(t, constants.Sha256, ConvertAuthModeToAuthMechanism("SCRAM"))
	assert.Equal(t, constants.Sha256, ConvertAuthModeToAuthMechanism("SCRAM-SHA-256"))
	assert.Equal(t, constants.Sha1, ConvertAuthModeToAuthMechanism("SCRAM-SHA-1"))
	assert.Equal(t, "", ConvertAuthModeToAuthMechanism("LDAP"))
}

func TestMongoDBCommunity_GetAuthOptions(t *testing.T) {
	mdb := newReplicaSet(3, "mdb", "mongodb")
	mdb.Spec.Security.Authentication.Modes = []AuthMode{"SCRAM", "X509"}

	opts := mdb.GetAuthOptions()

	assert.Equal(t, constants.Sha256, opts.AutoAuthMechanism)
	assert.Equal(t, []string{constants.Sha256, constants.X509}, opts.AuthMechanisms)
	assert.Equal(t, false, opts.AuthoritativeSet)

	mdb.Spec.Security.Authentication.Modes = []AuthMode{"X509"}
	mdb.Spec.Security.Authentication.AgentMode = "X509"

	opts = mdb.GetAuthOptions()
	assert.Equal(t, constants.X509, opts.AutoAuthMechanism)
	assert.Equal(t, []string{constants.X509}, opts.AuthMechanisms)
}

func TestMongoDBCommunity_GetAuthUsers(t *testing.T) {
	mdb := newReplicaSet(3, "mdb", "mongodb")
	mdb.Spec.Users = []MongoDBUser{
		{
			Name:              "my-user",
			DB:                "admin",
			PasswordSecretRef: SecretKeyReference{Name: "my-user-password"},
			Roles: []Role{
				{
					DB:   "admin",
					Name: "readWriteAnyDatabase",
				},
			},
			ScramCredentialsSecretName:       "my-scram",
			ConnectionStringSecretName:       "",
			AdditionalConnectionStringConfig: MapWrapper{},
		},
		{
			Name:              "CN=my-x509-authenticated-user,OU=organizationalunit,O=organization",
			DB:                "$external",
			PasswordSecretRef: SecretKeyReference{},
			Roles: []Role{
				{
					DB:   "admin",
					Name: "readWriteAnyDatabase",
				},
			},
			ScramCredentialsSecretName:       "",
			ConnectionStringSecretName:       "",
			AdditionalConnectionStringConfig: MapWrapper{},
		},
	}

	authUsers := mdb.GetAuthUsers()

	assert.Equal(t, authtypes.User{
		Username: "my-user",
		Database: "admin",
		Roles: []authtypes.Role{{
			Database: "admin",
			Name:     "readWriteAnyDatabase",
		}},
		PasswordSecretKey:               "password",
		PasswordSecretName:              "my-user-password",
		ScramCredentialsSecretName:      "my-scram-scram-credentials",
		ConnectionStringSecretName:      "mdb-admin-my-user",
		ConnectionStringSecretNamespace: mdb.Namespace,
		ConnectionStringOptions:         nil,
	}, authUsers[0])
	assert.Equal(t, authtypes.User{
		Username: "CN=my-x509-authenticated-user,OU=organizationalunit,O=organization",
		Database: "$external",
		Roles: []authtypes.Role{{
			Database: "admin",
			Name:     "readWriteAnyDatabase",
		}},
		PasswordSecretKey:               "",
		PasswordSecretName:              "",
		ScramCredentialsSecretName:      "",
		ConnectionStringSecretName:      "mdb-external-cn-my-x509-authenticated-user-ou-organizationalunit-o-organization",
		ConnectionStringSecretNamespace: mdb.Namespace,
		ConnectionStringOptions:         nil,
	}, authUsers[1])
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

func TestMongoDBCommunitySpec_GetAgentCertificateRef(t *testing.T) {
	m := newReplicaSet(3, "mdb", "mdb")

	assert.Equal(t, "agent-certs", m.Spec.GetAgentCertificateRef())

	m.Spec.Security.Authentication.AgentCertificateSecret = &corev1.LocalObjectReference{Name: "my-agent-certificate"}

	assert.Equal(t, "my-agent-certificate", m.Spec.GetAgentCertificateRef())
}

func TestMongoDBCommunity_AgentCertificateSecretNamespacedName(t *testing.T) {
	m := newReplicaSet(3, "mdb", "mdb")

	assert.Equal(t, "agent-certs", m.AgentCertificateSecretNamespacedName().Name)
	assert.Equal(t, "mdb", m.AgentCertificateSecretNamespacedName().Namespace)

	m.Spec.Security.Authentication.AgentCertificateSecret = &corev1.LocalObjectReference{Name: "agent-certs-custom"}
	assert.Equal(t, "agent-certs-custom", m.AgentCertificateSecretNamespacedName().Name)
}

func TestMongoDBCommunity_AgentCertificatePemSecretNamespacedName(t *testing.T) {
	m := newReplicaSet(3, "mdb", "mdb")

	assert.Equal(t, "agent-certs-pem", m.AgentCertificatePemSecretNamespacedName().Name)
	assert.Equal(t, "mdb", m.AgentCertificatePemSecretNamespacedName().Namespace)

	m.Spec.Security.Authentication.AgentCertificateSecret = &corev1.LocalObjectReference{Name: "agent-certs-custom"}
	assert.Equal(t, "agent-certs-custom-pem", m.AgentCertificatePemSecretNamespacedName().Name)

}

func TestMongoDBCommunitySpec_GetAgentAuthMode(t *testing.T) {
	type fields struct {
		agentAuth AuthMode
		modes     []AuthMode
	}
	tests := []struct {
		name   string
		fields fields
		want   AuthMode
	}{
		{
			name: "Agent auth not specified and modes array empty",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{},
			},
			want: AuthMode("SCRAM-SHA-256"),
		},
		{
			name: "Agent auth specified and modes array empty",
			fields: fields{
				agentAuth: "X509",
				modes:     []AuthMode{},
			},
			want: AuthMode("X509"),
		},
		{
			name: "Modes array one element",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{"X509"},
			},
			want: AuthMode("X509"),
		},
		{
			name: "Modes array has sha256 and sha1",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{"SCRAM-SHA-256", "SCRAM-SHA-1"},
			},
			want: AuthMode("SCRAM-SHA-256"),
		},
		{
			name: "Modes array has scram and sha1",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{"SCRAM", "SCRAM-SHA-1"},
			},
			want: AuthMode("SCRAM-SHA-256"),
		},
		{
			name: "Modes array has 2 different auth modes",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{"SCRAM", "X509"},
			},
			want: AuthMode(""),
		},
		{
			name: "Modes array has 3 auth modes",
			fields: fields{
				agentAuth: "",
				modes:     []AuthMode{"SCRAM-SHA-256", "SCRAM-SHA-1", "X509"},
			},
			want: AuthMode(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newReplicaSet(3, "mdb", "mdb")
			m.Spec.Security.Authentication.Modes = tt.fields.modes
			m.Spec.Security.Authentication.AgentMode = tt.fields.agentAuth
			assert.Equalf(t, tt.want, m.Spec.GetAgentAuthMode(), "GetAgentAuthMode()")
		})
	}
}
