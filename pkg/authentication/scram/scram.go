package scram

import (
	"context"
	"encoding/base64"
	"fmt"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
)

const (
	sha1SaltKey   = "sha1-salt"
	sha256SaltKey = "sha256-salt"

	sha1ServerKeyKey   = "sha-1-server-key"
	sha256ServerKeyKey = "sha-256-server-key"

	sha1StoredKeyKey   = "sha-1-stored-key"
	sha256StoredKeyKey = "sha-256-stored-key"
)

// Enable will configure all of the required Kubernetes resources for SCRAM-SHA to be enabled.
// The agent password and keyfile contents will be configured and stored in a secret.
// the user credentials will be generated if not present, or existing credentials will be read.
func Enable(ctx context.Context, auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) error {
	opts := mdb.GetAuthOptions()

	desiredUsers, err := convertMongoDBResourceUsersToAutomationConfigUsers(ctx, secretGetUpdateCreateDeleter, mdb)
	if err != nil {
		return fmt.Errorf("could not convert users to Automation Config users: %s", err)
	}

	if opts.AutoAuthMechanism == constants.Sha256 || opts.AutoAuthMechanism == constants.Sha1 {
		if err := ensureAgent(ctx, auth, secretGetUpdateCreateDeleter, mdb); err != nil {
			return err
		}
	}

	return enableClientAuthentication(auth, opts, desiredUsers)
}

func ensureAgent(ctx context.Context, auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) error {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return fmt.Errorf("could not generate password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return fmt.Errorf("could not generate keyfile contents: %s", err)
	}

	// ensure that the agent password secret exists or read existing password.
	agentPassword, err := secret.EnsureSecretWithKey(ctx, secretGetUpdateCreateDeleter, mdb.GetAgentPasswordSecretNamespacedName(), mdb.GetOwnerReferences(), constants.AgentPasswordKey, generatedPassword)
	if err != nil {
		return err
	}

	// ensure that the agent keyfile secret exists or read existing keyfile.
	agentKeyFile, err := secret.EnsureSecretWithKey(ctx, secretGetUpdateCreateDeleter, mdb.GetAgentKeyfileSecretNamespacedName(), mdb.GetOwnerReferences(), constants.AgentKeyfileKey, generatedContents)
	if err != nil {
		return err
	}

	return enableAgentAuthentication(auth, agentPassword, agentKeyFile, mdb.GetAuthOptions())
}

// ensureScramCredentials will ensure that the ScramSha1 & ScramSha256 credentials exist and are stored in the credentials
// secret corresponding to user of the given MongoDB deployment.
func ensureScramCredentials(ctx context.Context, getUpdateCreator secret.GetUpdateCreator, user authtypes.User, mdbNamespacedName types.NamespacedName, ownerRef []metav1.OwnerReference) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {

	password, err := secret.ReadKey(ctx, getUpdateCreator, user.PasswordSecretKey, types.NamespacedName{Name: user.PasswordSecretName, Namespace: mdbNamespacedName.Namespace})
	if err != nil {
		// if the password is deleted, that's fine we can read from the stored credentials that were previously generated
		if secret.SecretNotExist(err) {
			zap.S().Debugf("password secret was not found, reading from credentials from secret/%s", user.ScramCredentialsSecretName)
			return readExistingCredentials(ctx, getUpdateCreator, mdbNamespacedName, user.ScramCredentialsSecretName)
		}
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("could not read secret key: %s", err)
	}

	// we should only need to generate new credentials in two situations.
	// 1. We are creating the credentials for the first time
	// 2. We are changing the password
	shouldGenerateNewCredentials, err := needToGenerateNewCredentials(ctx, getUpdateCreator, user.Username, user.ScramCredentialsSecretName, mdbNamespacedName, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("could not determine if new credentials need to be generated: %s", err)
	}

	// there are no changes required, we can re-use the same credentials.
	if !shouldGenerateNewCredentials {
		zap.S().Debugf("Credentials have not changed, using credentials stored in: secret/%s", user.ScramCredentialsSecretName)
		return readExistingCredentials(ctx, getUpdateCreator, mdbNamespacedName, user.ScramCredentialsSecretName)
	}

	// the password has changed, or we are generating it for the first time
	zap.S().Debugf("Generating new credentials and storing in secret/%s", user.ScramCredentialsSecretName)
	sha1Creds, sha256Creds, err := generateScramShaCredentials(user.Username, password)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("failed generating scram credentials: %s", err)
	}

	// create or update our credentials secret for this user
	if err := createScramCredentialsSecret(ctx, getUpdateCreator, mdbNamespacedName, ownerRef, user.ScramCredentialsSecretName, sha1Creds, sha256Creds); err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("faild to create scram credentials secret %s: %s", user.ScramCredentialsSecretName, err)
	}

	zap.S().Debugf("Successfully generated SCRAM credentials")
	return sha1Creds, sha256Creds, nil
}

// needToGenerateNewCredentials determines if it is required to generate new credentials or not.
// this will be the case if we are either changing password, or are generating credentials for the first time.
func needToGenerateNewCredentials(ctx context.Context, secretGetter secret.Getter, username, scramCredentialsSecretName string, mdbNamespacedName types.NamespacedName, password string) (bool, error) {
	s, err := secretGetter.GetSecret(ctx, types.NamespacedName{Name: scramCredentialsSecretName, Namespace: mdbNamespacedName.Namespace})
	if err != nil {
		// haven't generated credentials yet, so we are changing password
		if secret.SecretNotExist(err) {
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
	sha1Creds, sha256Creds, err := computeScramShaCredentials(username, password, decodedSha1Salt, decodedSha256Salt)
	if err != nil {
		return false, err
	}

	existingSha1Creds, existingSha256Creds, err := readExistingCredentials(ctx, secretGetter, mdbNamespacedName, scramCredentialsSecretName)
	if err != nil {
		return false, err
	}

	sha1CredsAreDifferent := sha1Creds != existingSha1Creds
	sha256CredsAreDifferent := sha256Creds != existingSha256Creds

	return sha1CredsAreDifferent || sha256CredsAreDifferent, nil
}

// generateScramShaCredentials creates a new set of credentials using randomly generated salts. The first returned element is
// sha1 credentials, the second is sha256 credentials
func generateScramShaCredentials(username string, password string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	sha1Salt, sha256Salt, err := generate.Salts()
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}

	sha1Creds, sha256Creds, err := computeScramShaCredentials(username, password, sha1Salt, sha256Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, err
	}
	return sha1Creds, sha256Creds, nil
}

// computeScramShaCredentials computes ScramSha 1 & 256 credentials using the provided salts
func computeScramShaCredentials(username, password string, sha1Salt, sha256Salt []byte) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	scram1Creds, err := scramcredentials.ComputeScramSha1Creds(username, password, sha1Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("could not generate scramSha1Creds: %s", err)
	}

	scram256Creds, err := scramcredentials.ComputeScramSha256Creds(password, sha256Salt)
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("could not generate scramSha256Creds: %s", err)
	}

	return scram1Creds, scram256Creds, nil
}

// createScramCredentialsSecret will create a Secret that contains all of the fields required to read these credentials
// back in the future.
func createScramCredentialsSecret(ctx context.Context, getUpdateCreator secret.GetUpdateCreator, mdbObjectKey types.NamespacedName, ref []metav1.OwnerReference, scramCredentialsSecretName string, sha1Creds, sha256Creds scramcredentials.ScramCreds) error {
	scramCredsSecret := secret.Builder().
		SetName(scramCredentialsSecretName).
		SetNamespace(mdbObjectKey.Namespace).
		SetField(sha1SaltKey, sha1Creds.Salt).
		SetField(sha1StoredKeyKey, sha1Creds.StoredKey).
		SetField(sha1ServerKeyKey, sha1Creds.ServerKey).
		SetField(sha256SaltKey, sha256Creds.Salt).
		SetField(sha256StoredKeyKey, sha256Creds.StoredKey).
		SetField(sha256ServerKeyKey, sha256Creds.ServerKey).
		SetOwnerReferences(ref).
		Build()
	return secret.CreateOrUpdate(ctx, getUpdateCreator, scramCredsSecret)
}

// readExistingCredentials reads the existing set of credentials for both ScramSha 1 & 256
func readExistingCredentials(ctx context.Context, secretGetter secret.Getter, mdbObjectKey types.NamespacedName, scramCredentialsSecretName string) (scramcredentials.ScramCreds, scramcredentials.ScramCreds, error) {
	credentialsSecret, err := secretGetter.GetSecret(ctx, types.NamespacedName{Name: scramCredentialsSecretName, Namespace: mdbObjectKey.Namespace})
	if err != nil {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("could not get secret %s/%s: %s", mdbObjectKey.Namespace, scramCredentialsSecretName, err)
	}

	// we should really never hit this situation. It would only be possible if the secret storing credentials is manually edited.
	if !secret.HasAllKeys(credentialsSecret, sha1SaltKey, sha1ServerKeyKey, sha1ServerKeyKey, sha256SaltKey, sha256ServerKeyKey, sha256StoredKeyKey) {
		return scramcredentials.ScramCreds{}, scramcredentials.ScramCreds{}, fmt.Errorf("credentials secret did not have all of the required keys")
	}

	scramSha1Creds := scramcredentials.ScramCreds{
		IterationCount: scramcredentials.DefaultScramSha1Iterations,
		Salt:           string(credentialsSecret.Data[sha1SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha1ServerKeyKey]),
		StoredKey:      string(credentialsSecret.Data[sha1StoredKeyKey]),
	}

	scramSha256Creds := scramcredentials.ScramCreds{
		IterationCount: scramcredentials.DefaultScramSha256Iterations,
		Salt:           string(credentialsSecret.Data[sha256SaltKey]),
		ServerKey:      string(credentialsSecret.Data[sha256ServerKeyKey]),
		StoredKey:      string(credentialsSecret.Data[sha256StoredKeyKey]),
	}

	return scramSha1Creds, scramSha256Creds, nil
}

// convertMongoDBResourceUsersToAutomationConfigUsers returns a list of users that are able to be set in the AutomationConfig
func convertMongoDBResourceUsersToAutomationConfigUsers(ctx context.Context, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) ([]automationconfig.MongoDBUser, error) {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.GetAuthUsers() {
		if u.Database != constants.ExternalDB {
			acUser, err := convertMongoDBUserToAutomationConfigUser(ctx, secretGetUpdateCreateDeleter, mdb.NamespacedName(), mdb.GetOwnerReferences(), u)
			if err != nil {
				return nil, fmt.Errorf("failed to convert scram user %s to Automation Config user: %s", u.Username, err)
			}
			usersWanted = append(usersWanted, acUser)
		}
	}
	return usersWanted, nil
}

// convertMongoDBUserToAutomationConfigUser converts a single user configured in the MongoDB resource and converts it to a user
// that can be added directly to the AutomationConfig.
func convertMongoDBUserToAutomationConfigUser(ctx context.Context, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdbNsName types.NamespacedName, ownerRef []metav1.OwnerReference, user authtypes.User) (automationconfig.MongoDBUser, error) {
	acUser := automationconfig.MongoDBUser{
		Username: user.Username,
		Database: user.Database,
	}
	for _, role := range user.Roles {
		acUser.Roles = append(acUser.Roles, automationconfig.Role{
			Role:     role.Name,
			Database: role.Database,
		})
	}
	sha1Creds, sha256Creds, err := ensureScramCredentials(ctx, secretGetUpdateCreateDeleter, user, mdbNsName, ownerRef)
	if err != nil {
		return automationconfig.MongoDBUser{}, fmt.Errorf("could not ensure scram credentials: %s", err)
	}
	acUser.AuthenticationRestrictions = []string{}
	acUser.Mechanisms = []string{}
	acUser.ScramSha1Creds = &sha1Creds
	acUser.ScramSha256Creds = &sha256Creds
	return acUser, nil
}
