package x509

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/mocks"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/stretchr/testify/assert"
)

func TestEnable(t *testing.T) {
	ctx := context.Background()
	t.Run("X509 agent", func(t *testing.T) {
		auth := automationconfig.Auth{}
		mdb := buildX509Configurable("mdb", mocks.BuildX509MongoDBUser("my-user"), mocks.BuildScramMongoDBUser("my-scram-user"))

		agentSecret := CreateAgentCertificateSecret("tls.crt", false, mdb.AgentCertificateSecretNamespacedName())
		keyfileSecret := secret.Builder().
			SetName(mdb.GetAgentKeyfileSecretNamespacedName().Name).
			SetNamespace(mdb.GetAgentKeyfileSecretNamespacedName().Namespace).
			SetField(constants.AgentKeyfileKey, "RuPeMaIe2g0SNTTa").
			Build()
		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret, keyfileSecret)

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
		assert.NoError(t, err)

		expected := automationconfig.Auth{
			Users: []automationconfig.MongoDBUser{
				{
					Mechanisms: []string{},
					Roles: []automationconfig.Role{
						{
							Role:     "readWrite",
							Database: "admin",
						},
						{
							Role:     "clusterAdmin",
							Database: "admin",
						},
					},
					Username:                   "CN=my-user,OU=organizationalunit,O=organization",
					Database:                   "$external",
					AuthenticationRestrictions: []string{},
				},
			},
			Disabled:                 false,
			AuthoritativeSet:         false,
			AutoAuthMechanisms:       []string{constants.X509},
			AutoAuthMechanism:        constants.X509,
			DeploymentAuthMechanisms: []string{constants.X509},
			AutoUser:                 "CN=mms-automation-agent,OU=ENG,O=MongoDB,C=US",
			Key:                      "RuPeMaIe2g0SNTTa",
			KeyFile:                  "/path/to/keyfile",
			KeyFileWindows:           constants.AutomationAgentWindowsKeyFilePath,
			AutoPwd:                  "",
		}

		assert.Equal(t, expected, auth)
	})
	t.Run("SCRAM agent", func(t *testing.T) {
		auth := automationconfig.Auth{}
		mdb := buildScramConfigurable("mdb", mocks.BuildX509MongoDBUser("my-user"), mocks.BuildScramMongoDBUser("my-scram-user"))

		secrets := mocks.NewMockedSecretGetUpdateCreateDeleter()

		err := Enable(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
		assert.NoError(t, err)

		expected := automationconfig.Auth{
			Users: []automationconfig.MongoDBUser{{
				Mechanisms: []string{},
				Roles: []automationconfig.Role{
					{
						Role:     "readWrite",
						Database: "admin",
					},
					{
						Role:     "clusterAdmin",
						Database: "admin",
					},
				},
				Username:                   "CN=my-user,OU=organizationalunit,O=organization",
				Database:                   "$external",
				AuthenticationRestrictions: []string{},
			}},
			Disabled:                 false,
			AuthoritativeSet:         false,
			DeploymentAuthMechanisms: []string{constants.X509},
		}

		assert.Equal(t, expected, auth)
	})
}

func Test_ensureAgent(t *testing.T) {
	ctx := context.Background()
	auth := automationconfig.Auth{}
	mdb := buildX509Configurable("mdb")
	secrets := mocks.NewMockedSecretGetUpdateCreateDeleter()

	err := ensureAgent(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
	assert.Error(t, err)

	auth = automationconfig.Auth{}
	agentSecret := CreateAgentCertificateSecret("tls.pem", false, mdb.AgentCertificateSecretNamespacedName())
	secrets = mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret)

	err = ensureAgent(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "key \"tls.crt\" not present in the Secret")

	auth = automationconfig.Auth{}
	agentSecret = CreateAgentCertificateSecret("tls.crt", true, mdb.AgentCertificateSecretNamespacedName())
	secrets = mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret)

	err = ensureAgent(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "x509: malformed certificate")

	auth = automationconfig.Auth{}
	agentSecret = CreateAgentCertificateSecret("tls.crt", false, mdb.AgentCertificateSecretNamespacedName())
	secrets = mocks.NewMockedSecretGetUpdateCreateDeleter(agentSecret)

	err = ensureAgent(ctx, &auth, secrets, mdb, mdb.AgentCertificateSecretNamespacedName())
	assert.NoError(t, err)
}

func Test_convertMongoDBResourceUsersToAutomationConfigUsers(t *testing.T) {
	type args struct {
		mdb authtypes.Configurable
	}
	tests := []struct {
		name string
		args args
		want []automationconfig.MongoDBUser
	}{
		{
			name: "Only x.509 users",
			args: args{mdb: buildX509Configurable("mongodb", mocks.BuildX509MongoDBUser("my-user-1"), mocks.BuildX509MongoDBUser("my-user-2"))},
			want: []automationconfig.MongoDBUser{
				{
					Mechanisms: []string{},
					Roles: []automationconfig.Role{
						{
							Role:     "readWrite",
							Database: "admin",
						},
						{
							Role:     "clusterAdmin",
							Database: "admin",
						},
					},
					Username:                   "CN=my-user-1,OU=organizationalunit,O=organization",
					Database:                   "$external",
					AuthenticationRestrictions: []string{},
				},
				{
					Mechanisms: []string{},
					Roles: []automationconfig.Role{
						{
							Role:     "readWrite",
							Database: "admin",
						},
						{
							Role:     "clusterAdmin",
							Database: "admin",
						},
					},
					Username:                   "CN=my-user-2,OU=organizationalunit,O=organization",
					Database:                   "$external",
					AuthenticationRestrictions: []string{},
				},
			},
		},
		{
			name: "Only SCRAM users",
			args: args{mdb: buildX509Configurable("mongodb", mocks.BuildScramMongoDBUser("my-user-1"), mocks.BuildScramMongoDBUser("my-user-2"))},
			want: nil,
		},
		{
			name: "X.509 and SCRAM users",
			args: args{mdb: buildX509Configurable("mongodb", mocks.BuildX509MongoDBUser("my-user-1"), mocks.BuildScramMongoDBUser("my-user-2"))},
			want: []automationconfig.MongoDBUser{
				{
					Mechanisms: []string{},
					Roles: []automationconfig.Role{
						{
							Role:     "readWrite",
							Database: "admin",
						},
						{
							Role:     "clusterAdmin",
							Database: "admin",
						},
					},
					Username:                   "CN=my-user-1,OU=organizationalunit,O=organization",
					Database:                   "$external",
					AuthenticationRestrictions: []string{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertMongoDBResourceUsersToAutomationConfigUsers(tt.args.mdb)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertMongoDBResourceUsersToAutomationConfigUsers() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readAgentSubjectsFromCert(t *testing.T) {
	agentCert, _, _ := CreateAgentCertificate()

	subjectName, err := readAgentSubjectsFromCert(agentCert)
	assert.NoError(t, err)

	assert.Equal(t, "CN=mms-automation-agent,OU=ENG,O=MongoDB,C=US", subjectName)
}

func buildX509Configurable(name string, users ...authtypes.User) mocks.MockConfigurable {
	return mocks.NewMockConfigurable(
		authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           "/path/to/keyfile",
			AuthMechanisms:    []string{constants.X509},
			AutoAuthMechanism: constants.X509,
		},
		users,
		types.NamespacedName{
			Name:      name,
			Namespace: "default",
		},
		[]metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "mdbc",
			Name:       "my-ref",
		}},
	)
}

func buildScramConfigurable(name string, users ...authtypes.User) mocks.MockConfigurable {
	return mocks.NewMockConfigurable(
		authtypes.Options{
			AuthoritativeSet:  false,
			KeyFile:           "/path/to/keyfile",
			AuthMechanisms:    []string{constants.Sha256, constants.X509},
			AgentName:         constants.AgentName,
			AutoAuthMechanism: constants.Sha256,
		},
		users,
		types.NamespacedName{
			Name:      name,
			Namespace: "default",
		},
		[]metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "mdbc",
			Name:       "my-ref",
		}},
	)
}
