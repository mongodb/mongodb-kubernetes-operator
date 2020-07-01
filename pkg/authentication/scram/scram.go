package scram

import (
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// EnsureAgentSecret make sure that the agent password and keyfile exist in the secret and returns
// the scram authEnabler configured with this values
func EnsureAgentSecret(getUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName) (automationconfig.AuthEnabler, error) {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return authEnabler{}, fmt.Errorf("error generating keyfile contents: %s", err)
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
	}, getUpdateCreator.UpdateSecret(agentSecret)
}
