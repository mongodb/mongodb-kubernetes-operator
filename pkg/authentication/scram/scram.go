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
// an automation config modification function with these values
func EnsureAgentSecret(getUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName) (automationconfig.Modification, error) {
	generatedPassword, err := generate.RandomFixedLengthStringOfSize(20)
	if err != nil {
		return automationconfig.NOOP(), fmt.Errorf("error generating password: %s", err)
	}

	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return automationconfig.NOOP(), fmt.Errorf("error generating keyfile contents: %s", err)
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
			return automationConfigModification(generatedPassword, generatedContents, []automationconfig.MongoDBUser{}), getUpdateCreator.CreateSecret(s)
		}

		return automationconfig.NOOP(), err
	}

	if _, ok := agentSecret.Data[AgentPasswordKey]; !ok {
		agentSecret.Data[AgentPasswordKey] = []byte(generatedPassword)
	}

	if _, ok := agentSecret.Data[AgentKeyfileKey]; !ok {
		agentSecret.Data[AgentKeyfileKey] = []byte(generatedContents)
	}

	return automationConfigModification(
		string(agentSecret.Data[AgentPasswordKey]),
		string(agentSecret.Data[AgentKeyfileKey]),
		[]automationconfig.MongoDBUser{},
	), getUpdateCreator.UpdateSecret(agentSecret)
}
