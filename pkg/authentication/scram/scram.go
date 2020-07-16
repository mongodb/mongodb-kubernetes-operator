package scram

import (
	"crypto/sha1" //nolint
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"reflect"

	"go.uber.org/zap"

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

	sha1ServerKeyKey   = "sha-1-server-key"
	sha256ServerKeyKey = "sha-256-server-key"

	sha1StoredKeyKey   = "sha-1-stored-key"
	sha256StoredKeyKey = "sha-256-stored-key"

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

// ensureScramCredentials will ensure that the ScramSha1 & ScramSha256 credentials exist and are stored in the credentials
// secret corresponding to user of the given MongoDB deployment.
func ensureScramCredentials(getUpdateCreator secret.GetUpdateCreator, user mdbv1.MongoDBUser, mdb mdbv1.MongoDB) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	passwordKey := user.PasswordSecretRef.Key
	if passwordKey == "" {
		passwordKey = defaultPasswordKey
	}
	password, err := secret.ReadKey(getUpdateCreator, passwordKey, types.NamespacedName{Name: user.PasswordSecretRef.Name, Namespace: mdb.Namespace})
	if err != nil {
		// if the password is deleted, that's fine we can read from the stored credentials that were previously generated
		if errors.IsNotFound(err) {
			zap.S().Debugf("password secret was not found, reading from credentials from secret/%s", scramCredentialsSecretName(mdb.Name, user.Name))
			return readExistingCredentials(getUpdateCreator, mdb.NamespacedName(), user.Name)
		}
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// we should only need to generate new credentials in two situations.
	// 1. We are creating the credentials for the first time
	// 2. We are changing the password
	needToGenerateNewCredentials, err := needToGenerateNewCredentials(getUpdateCreator, user, mdb, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// there are no changes required, we can re-use the same credentials.
	if !needToGenerateNewCredentials {
		zap.S().Debugf("Credentials have not changed, using credentials stored in: secret/%s", scramCredentialsSecretName(mdb.Name, user.Name))
		return readExistingCredentials(getUpdateCreator, mdb.NamespacedName(), user.Name)
	}

	// the password has changed, or we are generating it for the first time
	sha1Creds, sha256Creds, err := generateScramShaCredentials(user, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// create or update our credentials secret for this user
	zap.S().Debugf("Generating new credentials and storing in secret/%s", scramCredentialsSecretName(mdb.Name, user.Name))
	if err := createScramCredentialsSecret(getUpdateCreator, mdb.NamespacedName(), user.Name, sha1Creds, sha256Creds); err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	zap.S().Debugf("Successfully generated SCRAM credentials")
	return sha1Creds, sha256Creds, nil
}

// needToGenerateNewCredentials determines if it is required to generate new credentials or not.
// this will be the case if we are either changing password, or are generating credentials for the first time.
func needToGenerateNewCredentials(secretGetter secret.Getter, user mdbv1.MongoDBUser, mdb mdbv1.MongoDB, password string) (bool, error) {
	s, err := secretGetter.GetSecret(types.NamespacedName{Name: scramCredentialsSecretName(mdb.Name, user.Name), Namespace: mdb.Namespace})
	if err != nil {
		// haven't generated credentials yet, so we are changing password
		if errors.IsNotFound(err) {
			zap.S().Debugf("No existing credentials found, generating new credentials")
			return true, nil
		}
		return false, err
	}

	existingSha1Salt := s.Data[sha1SaltKey]
	existingSha256Salt := s.Data[sha256SaltKey]

	// the salts are stored encoded, we need to decode them before we use them for
	// salt generation
	decodedSha1Salt, err := base64.StdEncoding.DecodeString(string(existingSha1Salt))
	if err != nil {
		return false, err
	}
	decodedSha256Salt, err := base64.StdEncoding.DecodeString(string(existingSha256Salt))
	if err != nil {
		return false, err
	}

	// regenerate credentials using the existing salts in order to see if the password has changed.
	sha1Creds, sha256Creds, err := computeScramShaCredentials(user.Name, password, decodedSha1Salt, decodedSha256Salt)
	if err != nil {
		return false, err
	}

	existingSha1Creds, existingSha256Creds, err := readExistingCredentials(secretGetter, mdb.NamespacedName(), user.Name)
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

// generateSalt will create a salt which can be used to compute Scram Sha credentials based on the given hashConstructor.
// sha1.New should be used for MONGODB-CR/SCRAM-SHA-1 and sha256.New should be used for SCRAM-SHA-256
func generateSalt(hashConstructor func() hash.Hash) ([]byte, error) {
	saltSize := hashConstructor().Size() - scramcredentials.RFC5802MandatedSaltSize
	salt, err := generate.RandomFixedLengthStringOfSize(20)

	if err != nil {
		return nil, err
	}
	shaBytes32 := sha256.Sum256([]byte(salt))

	// the algorithms expect a salt of a specific size.
	return shaBytes32[:saltSize], nil
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

// createScramCredentialsSecret will create a Secret that contains all of the fields required to read these credentials
// back in the future.
func createScramCredentialsSecret(secretCreator secret.Creator, mdbObjectKey types.NamespacedName, username string, sha1Creds, sha256Creds scramcredentials.ScramCreds) error {
	scramCredsSecret := secret.Builder().
		SetName(scramCredentialsSecretName(mdbObjectKey.Name, username)).
		SetNamespace(mdbObjectKey.Namespace).
		SetField(sha1SaltKey, sha1Creds.Salt).
		SetField(sha1StoredKeyKey, sha1Creds.StoredKey).
		SetField(sha1ServerKeyKey, sha1Creds.ServerKey).
		SetField(sha256SaltKey, sha256Creds.Salt).
		SetField(sha256StoredKeyKey, sha256Creds.StoredKey).
		SetField(sha256ServerKeyKey, sha256Creds.ServerKey).
		Build()
	return secretCreator.CreateSecret(scramCredsSecret)
}

// readExistingCredentials reads the existing set of credentials for both ScramSha 1 & 256
func readExistingCredentials(secretGetter secret.Getter, mdbObjectKey types.NamespacedName, username string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	credentialsSecret, err := secretGetter.GetSecret(types.NamespacedName{Name: scramCredentialsSecretName(mdbObjectKey.Name, username), Namespace: mdbObjectKey.Namespace})
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	// we should really never hit this situation. It would only be possible if the secret storing credentials is manually edited.
	if !secret.HasAllKeys(credentialsSecret, sha1SaltKey, sha1ServerKeyKey, sha1ServerKeyKey, sha256SaltKey, sha256ServerKeyKey, sha256StoredKeyKey) {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("credentials secret did not have all of the required keys")
	}

	scramSha1Creds := scramcredentials.ScramCreds{
		IterationCount: 10000,
		Salt:           string(credentialsSecret.Data[sha1SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha1ServerKeyKey]),
		StoredKey:      string(credentialsSecret.Data[sha1StoredKeyKey]),
	}

	scramSha256Creds := scramcredentials.ScramCreds{
		IterationCount: 15000,
		Salt:           string(credentialsSecret.Data[sha256SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha256ServerKeyKey]),
		StoredKey:      string(credentialsSecret.Data[sha256StoredKeyKey]),
	}

	return scramSha1Creds, scramSha256Creds, nil
}

// convertMongoDBResourceUsersToAutomationConfigUsers returns a list of users that are able to be set in the AutomationConfig
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
