package scram

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	corev1 "k8s.io/api/core/v1"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultPasswordKey = "password"
	sha1SaltKey        = "sha1-salt"
	sha256SaltKey      = "sha256-salt"
	day                = time.Hour * 24
	expireTime         = day * 10
)

// EnsureEnabler make sure that the agent password and keyfile exist in the secret and returns
// the scram authEnabler configured with this values
func EnsureEnabler(secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, secretNsName types.NamespacedName, mdb mdbv1.MongoDB) (automationconfig.AuthEnabler, error) {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating keyfile contents: %s", err)
	}

	desiredUsers, err := convertMongoDBResourceUsersToAutomationConfigUsers(secretGetUpdateCreateDeleter, mdb)
	if err != nil {
		return authEnabler{}, err
	}
	agentSecret, err := secretGetUpdateCreateDeleter.GetSecret(secretNsName)
	if err != nil {
		if errors.IsNotFound(err) {
			s := secret.Builder().
				SetNamespace(secretNsName.Namespace).
				SetName(secretNsName.Name).
				SetField(AgentPasswordKey, generatedPassword).
				SetField(AgentKeyfileKey, generatedContents).
				Build()
			return authEnabler{
				agentPassword: generatedPassword,
				agentKeyFile:  generatedContents,
				users:         desiredUsers,
			}, secretGetUpdateCreateDeleter.CreateSecret(s)
		}
		return authEnabler{}, err
	}

	if _, ok := agentSecret.Data[AgentPasswordKey]; !ok {
		agentSecret.Data[AgentPasswordKey] = []byte(generatedPassword)
	}

	if _, ok := agentSecret.Data[AgentKeyfileKey]; !ok {
		agentSecret.Data[AgentKeyfileKey] = []byte(generatedContents)
	}

	return authEnabler{
		agentPassword: string(agentSecret.Data[AgentPasswordKey]),
		agentKeyFile:  string(agentSecret.Data[AgentKeyfileKey]),
		users:         desiredUsers,
	}, secretGetUpdateCreateDeleter.UpdateSecret(agentSecret)
}

func convertMongoDBResourceUsersToAutomationConfigUsers(secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB) ([]automationconfig.MongoDBUser, error) {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.Spec.Users {
		passwordKey := u.PasswordSecretRef.Key
		if passwordKey == "" {
			passwordKey = defaultPasswordKey
		}
		password, err := secret.ReadKey(secretGetUpdateCreateDeleter, passwordKey, types.NamespacedName{Name: u.PasswordSecretRef.Name, Namespace: mdb.Namespace})
		if err != nil {
			return nil, err
		}
		acUser, err := convertMongoDBUserToAutomationConfigUser(secretGetUpdateCreateDeleter, mdb, u, password)
		if err != nil {
			return nil, err
		}
		usersWanted = append(usersWanted, acUser)
	}
	return usersWanted, nil
}

// convertMongoDBUserToAutomationConfigUser converts a single user configured in the MongoDB resource and converts it to a user
// that can be added directly to the AutomationConfig.
func convertMongoDBUserToAutomationConfigUser(secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB, user mdbv1.MongoDBUser, password string) (automationconfig.MongoDBUser, error) {
	acUser := automationconfig.MongoDBUser{
		Username: user.Name,
		Database: user.DB,
	}
	for _, role := range user.Roles {
		acUser.Roles = append(acUser.Roles, automationconfig.Role{
			Role:     role.Name,
			Database: role.DB,
		})
	}
	sha1Creds, sha256Creds, err := computeScram1AndScram256Credentials(secretGetUpdateCreateDeleter, mdb, acUser.Username, password)
	if err != nil {
		return automationconfig.MongoDBUser{}, err
	}
	acUser.AuthenticationRestrictions = []string{}
	acUser.Mechanisms = []string{}
	acUser.ScramSha1Creds = &sha1Creds
	acUser.ScramSha256Creds = &sha256Creds
	return acUser, nil
}

// generateSalt will create a salt for use with ComputeScramShaCreds based on the given hashConstructor.
// sha1.New should be used for MONGODB-CR/SCRAM-SHA-1 and sha256.New should be used for SCRAM-SHA-256
func generateSalt(hashConstructor func() hash.Hash) ([]byte, error) {
	saltSize := hashConstructor().Size() - scramcredentials.RFC5802MandatedSaltSize
	salt, err := generate.RandomFixedLengthStringOfSize(saltSize)
	if err != nil {
		return nil, err
	}
	return []byte(salt), nil
}

// ensureSalts makes sure that salts exist to be used with credential generation for the given user.
func ensureSalts(getUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB, username string) ([]byte, []byte, error) {
	secretName := fmt.Sprintf("%s-%s-salts", mdb.Name, username)
	var sha256Salt, sha1Salt []byte

	s, err := getUpdateCreateDeleter.GetSecret(types.NamespacedName{Name: secretName, Namespace: mdb.Namespace})
	if err == nil {
		// the secret exists and is fine, let's return the salts contained within
		if !secretHasExpired(s) {
			sha1Salt, hasSha1Salt := s.Data[sha1SaltKey]
			sha256Salt, hasSha256Salt := s.Data[sha256SaltKey]
			if hasSha1Salt && hasSha256Salt {
				return sha1Salt, sha256Salt, nil
			}
		}

		// the secret has expired, let's delete it and re-generate a new salt + credentials
		if err := getUpdateCreateDeleter.DeleteSecret(types.NamespacedName{Name: s.Name, Namespace: s.Namespace}); err != nil {
			return nil, nil, err
		}
	}

	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	sha256Salt, err = generateSalt(sha256.New)
	if err != nil {
		return nil, nil, err
	}

	sha1Salt, err = generateSalt(sha1.New)
	if err != nil {
		return nil, nil, err
	}

	saltSecret := secret.Builder().
		SetName(secretName).
		SetNamespace(mdb.Namespace).
		SetField(sha1SaltKey, string(sha1Salt)).
		SetField(sha256SaltKey, string(sha256Salt)).
		Build()

	if err := getUpdateCreateDeleter.CreateSecret(saltSecret); err != nil {
		return nil, nil, err
	}

	return sha1Salt, sha256Salt, nil
}

func secretHasExpired(s corev1.Secret) bool {
	return s.CreationTimestamp.Add(expireTime).Before(time.Now())
}

// computeScram1AndScram256Credentials takes in a username and password, and generates SHA1 & SHA256 credentials
// for that user. This should only be done if credentials do not already exist.
func computeScram1AndScram256Credentials(getUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB, username, password string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	sha1Salt, sha256Salt, err := ensureSalts(getUpdateCreateDeleter, mdb, username)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}
	scram256Creds, err := scramcredentials.ComputeScramSha256Creds(password, sha256Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("error generating scramSha256 creds: %s", err)
	}
	scram1Creds, err := scramcredentials.ComputeScramSha1Creds(username, password, sha1Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("error generating scramSha1Creds: %s", err)
	}
	return scram1Creds, scram256Creds, nil
}
