package scram

import (
	"crypto/sha1" //nolint
	"crypto/sha256"
	"fmt"
	"hash"
	"reflect"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultPasswordKey = "password"

	sha1SaltKey   = "sha1-salt"
	sha256SaltKey = "sha256-salt"

	sha1ServerKey   = "sha-1-server-key"
	sha256ServerKey = "sha-256-server-key"

	sha1StoredKey   = "sha-1-stored-key"
	sha256StoredKey = "sha-256-stored-key"

	scramCredsSecretName = "scram-credentials" //nolint
)

func scramCredentialsSecretName(mdbName, username string) string {
	return fmt.Sprintf("%s-%s-%s", mdbName, username, scramCredsSecretName)
}

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

func ensureScramCredentials(c secret.GetUpdateCreateDeleter, user mdbv1.MongoDBUser, mdb mdbv1.MongoDB) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	passwordKey := user.PasswordSecretRef.Key
	if passwordKey == "" {
		passwordKey = defaultPasswordKey
	}
	password, err := secret.ReadKey(c, passwordKey, types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: mdb.Namespace})
	if err != nil {
		// if the password is deleted, that's fine we can read from the stored credentials that were previously generated
		if errors.IsNotFound(err) {
			return readExistingCredentials(c, mdb.NamespacedName(), user.Name)
		}
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	needToGenerateNewCredentials, err := needToGenerateNewCredentials(c, user, mdb, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// there are no changes required, we can re-use the same credentials.
	if !needToGenerateNewCredentials {
		return readExistingCredentials(c, mdb.NamespacedName(), user.Name)
	}

	// the password has changed, or we are generating it for the first time
	sha1Creds, sha256Creds, err := generateScramShaCredentials(user, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// create or update our credentials secret for this user
	if err := createScramCredentialsSecret(c, mdb.NamespacedName(), user.Name, sha1Creds, sha256Creds); err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	return sha1Creds, sha256Creds, nil
}

func needToGenerateNewCredentials(c secret.GetUpdateCreateDeleter, user mdbv1.MongoDBUser, mdb mdbv1.MongoDB, password string) (bool, error) {
	s, err := c.GetSecret(types.NamespacedName{Name: scramCredentialsSecretName(mdb.Name, user.Name), Namespace: mdb.Namespace})
	if err != nil {
		// haven't generated credentials yet, so we are changing password
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	existingSha1Salt := s.Data[sha1SaltKey]
	existingSha256Salt := s.Data[sha256SaltKey]

	sha1Creds, sha256Creds, err := computeScramShaCredentials(user.Name, password, existingSha1Salt, existingSha256Salt)
	if err != nil {
		return false, err
	}

	existingSha1Creds, existingSha256Creds, err := readExistingCredentials(c, mdb.NamespacedName(), user.Name)
	if err != nil {
		return false, err
	}

	sha1CredsAreTheSame := reflect.DeepEqual(sha1Creds, existingSha1Creds)
	sha256CredsAreTheSame := reflect.DeepEqual(sha256Creds, existingSha256Creds)

	return !sha1CredsAreTheSame || !sha256CredsAreTheSame, nil
}

// generateSalts generates 2 different salts. The first is for the sha1 algorithm
// the second is for sha256
func generateSalts() ([]byte, []byte, error) {
	sha1Salt, err := generateSalt(sha1.New)
	if err != nil {
		return nil, nil, err
	}

	sha256Salt, err := generateSalt(sha256.New)
	if err != nil {
		return nil, nil, err
	}
	return sha1Salt, sha256Salt, nil
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

// generateScramShaCredentials creates a new set of credentials using randomly generated salts. The first returned element is
// sha1 credentials, the second is sha256 credentials
func generateScramShaCredentials(user mdbv1.MongoDBUser, password string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	sha1Salt, sha256Salt, err := generateSalts()
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	sha1Creds, sha256Creds, err := computeScramShaCredentials(user.Name, password, sha1Salt, sha256Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}
	return sha1Creds, sha256Creds, nil
}

// computeScramShaCredentials computes ScramSha 1 & 256 credentials using the provided salts
func computeScramShaCredentials(username, password string, sha1Salt, sha256Salt []byte) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	scram1Creds, err := scramcredentials.ComputeScramSha1Creds(username, password, sha1Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("error generating scramSha1Creds: %s", err)
	}

	scram256Creds, err := scramcredentials.ComputeScramSha256Creds(password, sha256Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("error generating scramSha256Creds: %s", err)
	}

	return scram1Creds, scram256Creds, nil
}

func createScramCredentialsSecret(secretCreator secret.Creator, mdbObjectKey types.NamespacedName, username string, sha1Creds, sha256Creds scramcredentials.ScramCreds) error {
	scramCredsSecret := secret.Builder().
		SetName(scramCredentialsSecretName(mdbObjectKey.Name, username)).
		SetNamespace(mdbObjectKey.Namespace).
		SetField(sha1SaltKey, sha1Creds.Salt).
		SetField(sha1StoredKey, sha1Creds.StoredKey).
		SetField(sha1ServerKey, sha1Creds.ServerKey).
		SetField(sha256SaltKey, sha256Creds.Salt).
		SetField(sha256StoredKey, sha256Creds.StoredKey).
		SetField(sha256ServerKey, sha256Creds.ServerKey).
		Build()
	return secretCreator.CreateSecret(scramCredsSecret)
}

func readExistingCredentials(secretGetter secret.Getter, mdbObjectKey types.NamespacedName, username string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	credentialsSecret, err := secretGetter.GetSecret(types.NamespacedName{Name: scramCredentialsSecretName(mdbObjectKey.Name, username), Namespace: mdbObjectKey.Namespace})
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	if !secret.HasAllKeys(credentialsSecret, sha1SaltKey, sha1ServerKey, sha1ServerKey, sha256SaltKey, sha256ServerKey, sha256StoredKey) {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("credentials secret did not have all of the required keys")
	}

	scramSha1Creds := scramcredentials.ScramCreds{
		IterationCount: 10000,
		Salt:           string(credentialsSecret.Data[sha1SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha1ServerKey]),
		StoredKey:      string(credentialsSecret.Data[sha1StoredKey]),
	}

	scramSha256Creds := scramcredentials.ScramCreds{
		IterationCount: 15000,
		Salt:           string(credentialsSecret.Data[sha256SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha256ServerKey]),
		StoredKey:      string(credentialsSecret.Data[sha256StoredKey]),
	}

	return scramSha1Creds, scramSha256Creds, nil
}

func convertMongoDBResourceUsersToAutomationConfigUsers(secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB) ([]automationconfig.MongoDBUser, error) {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.Spec.Users {
		acUser, err := convertMongoDBUserToAutomationConfigUser(secretGetUpdateCreateDeleter, mdb, u)
		if err != nil {
			return nil, err
		}
		usersWanted = append(usersWanted, acUser)
	}
	return usersWanted, nil
}

// convertMongoDBUserToAutomationConfigUser converts a single user configured in the MongoDB resource and converts it to a user
// that can be added directly to the AutomationConfig.
func convertMongoDBUserToAutomationConfigUser(secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb mdbv1.MongoDB, user mdbv1.MongoDBUser) (automationconfig.MongoDBUser, error) {
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
	sha1Creds, sha256Creds, err := ensureScramCredentials(secretGetUpdateCreateDeleter, user, mdb)
	if err != nil {
		return automationconfig.MongoDBUser{}, err
	}
	acUser.AuthenticationRestrictions = []string{}
	acUser.Mechanisms = []string{}
	acUser.ScramSha1Creds = &sha1Creds
	acUser.ScramSha256Creds = &sha256Creds
	return acUser, nil
}
