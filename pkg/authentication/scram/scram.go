package scram

import (
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// EnsureAgentSecret make sure that the agent password and keyfile exist in the secret and returns
// the scram enabler configured with this values
func EnsureAgentSecret(getUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName) (Enabler, error) {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return Enabler{}, fmt.Errorf("error generating password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return Enabler{}, fmt.Errorf("error generating keyfile contents: %s", err)
	}

	agentSecret, err := getUpdateCreator.GetSecret(secretNsName)
	if err != nil {
		if errors.IsNotFound(err) {
			s := secret.Builder().
				SetNamespace(secretNsName.Namespace).
				SetName(secretNsName.Name).
				SetField(scramAgentPasswordKey, generatedPassword).
				SetField(scramAgentKeyfileKey, generatedContents).
				Build()
			return Enabler{
				AgentPassword: generatedPassword,
				AgentKeyFile:  generatedContents,
			}, getUpdateCreator.CreateSecret(s)
		}
	}

	if _, ok := agentSecret.Data[scramAgentPasswordKey]; !ok {
		agentSecret.Data[scramAgentPasswordKey] = []byte(generatedPassword)
	}

	if _, ok := agentSecret.Data[scramAgentKeyfileKey]; !ok {
		agentSecret.Data[scramAgentKeyfileKey] = []byte(generatedContents)
	}

	return Enabler{
		AgentPassword: string(agentSecret.Data[scramAgentPasswordKey]),
		AgentKeyFile:  string(agentSecret.Data[scramAgentKeyfileKey]),
	}, getUpdateCreator.UpdateSecret(agentSecret)
}
