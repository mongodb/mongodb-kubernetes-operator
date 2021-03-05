package automationconfig

import (
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const ConfigKey = "cluster-config.json"

// ReadFromSecret returns the AutomationConfig present in the given Secret. If the Secret is not
// found, it is not considered an error and an empty AutomationConfig is returned.
func ReadFromSecret(secretGetter secret.Getter, secretNsName types.NamespacedName) (AutomationConfig, error) {
	acSecret, err := secretGetter.GetSecret(secretNsName)
	if err != nil {
		return AutomationConfig{}, client.IgnoreNotFound(err)
	}
	return FromBytes(acSecret.Data[ConfigKey])
}

// EnsureSecret makes sure that the AutomationConfig secret exists with the desired config.
// if the desired config is the same as the current contents, no change is made.
// The most recent AutomationConfig is returned. If no change is made, it will return the existing one, if there
// is a change, the new AutomationConfig is returned.
func EnsureSecret(secretGetUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName, owner []metav1.OwnerReference, desiredAutomationConfig AutomationConfig) (AutomationConfig, error) {
	existingSecret, err := secretGetUpdateCreator.GetSecret(secretNsName)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return createNewAutomationConfigSecret(secretGetUpdateCreator, secretNsName, owner, desiredAutomationConfig)
		}
		return AutomationConfig{}, err
	}

	acBytes, err := json.Marshal(desiredAutomationConfig)
	if err != nil {
		return AutomationConfig{}, err
	}
	if existingAcBytes, ok := existingSecret.Data[ConfigKey]; !ok {
		// the secret exists but the key is not present. We can update the secret
		existingSecret.Data[ConfigKey] = acBytes
	} else {
		// the secret already exists, we should check to see if we're making any changes.
		existingAutomationConfig, err := FromBytes(existingAcBytes)
		if err != nil {
			return AutomationConfig{}, err
		}
		// we are attempting to update with the same version, no change is required.
		areEqual, err := AreEqual(desiredAutomationConfig, existingAutomationConfig)
		if err != nil {
			return AutomationConfig{}, err
		}
		if areEqual {
			return existingAutomationConfig, nil
		}
		existingSecret.Data[ConfigKey] = acBytes
	}

	return desiredAutomationConfig, secretGetUpdateCreator.UpdateSecret(existingSecret)
}

func createNewAutomationConfigSecret(secretGetUpdateCreator secret.GetUpdateCreator, secretNsName types.NamespacedName, owner []metav1.OwnerReference, desiredAutomation AutomationConfig) (AutomationConfig, error) {
	acBytes, err := json.Marshal(desiredAutomation)
	if err != nil {
		return AutomationConfig{}, err
	}

	newSecret := secret.Builder().
		SetName(secretNsName.Name).
		SetNamespace(secretNsName.Namespace).
		SetField(ConfigKey, string(acBytes)).
		SetOwnerReferences(owner).
		Build()

	if err := secretGetUpdateCreator.CreateSecret(newSecret); err != nil {
		return AutomationConfig{}, err
	}
	return desiredAutomation, nil
}
