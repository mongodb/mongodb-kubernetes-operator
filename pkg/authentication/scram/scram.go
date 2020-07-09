package scram

import (
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultPasswordKey = "password"
)

// EnsureEnabler make sure that the agent password and keyfile exist in the secret and returns
// the scram authEnabler configured with this values
func EnsureEnabler(getUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName, mdb mdbv1.MongoDB) (automationconfig.AuthEnabler, error) {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating keyfile contents: %s", err)
	}

	desiredUsers, err := convertMongoDBResourceUsersToAutomationConfigUsers(getUpdateCreator, mdb)
	if err != nil {
		return authEnabler{}, err
	}
	agentSecret, err := getUpdateCreator.GetSecret(secretNsName)
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
			}, getUpdateCreator.CreateSecret(s)
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
	}, getUpdateCreator.UpdateSecret(agentSecret)
}

func convertMongoDBResourceUsersToAutomationConfigUsers(getter secret.Getter, mdb mdbv1.MongoDB) ([]automationconfig.MongoDBUser, error) {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.Spec.Users {
		passwordKey := u.PasswordSecretRef.Key
		if passwordKey == "" {
			passwordKey = defaultPasswordKey
		}
		password, err := secret.ReadKey(getter, passwordKey, types.NamespacedName{Name: u.PasswordSecretRef.Name, Namespace: mdb.Namespace})
		if err != nil {
			return nil, err
		}
		acUser, err := convertMongoDBUserToAutomationConfigUser(u, password)
		if err != nil {
			return nil, err
		}
		usersWanted = append(usersWanted, acUser)
	}
	return usersWanted, nil
}

func convertMongoDBUserToAutomationConfigUser(user mdbv1.MongoDBUser, password string) (automationconfig.MongoDBUser, error) {
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
	sha1Creds, sha256Creds, err := computeScram1AndScram256Credentials(acUser.Username, password)
	if err != nil {
		return automationconfig.MongoDBUser{}, err
	}
	acUser.AuthenticationRestrictions = []string{}
	acUser.Mechanisms = []string{}
	acUser.ScramSha1Creds = sha1Creds
	acUser.ScramSha256Creds = sha256Creds
	return acUser, nil
}
