package automationconfig

import (
	"encoding/json"
	"reflect"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

func EnsureAutomationConfigSecret(secretGetUpdateCreator secret.GetUpdateCreator, ac AutomationConfig) (corev1.Secret, error) {
	return corev1.Secret{}, nil
}

// EnsureSecret fetches the existing Secret and applies the callback to it and pushes changes back.
// The callback is expected to update the data in Secret or return false if no update/create is needed
// Returns the final Secret (could be the initial one or the one after the update)
func EnsureSecret(secretGetUpdateCreator secret.GetUpdateCreator, nsName client.ObjectKey, owner metav1.OwnerReference, callback func(*corev1.Secret) bool) (corev1.Secret, error) {
	existingSecret, err := secretGetUpdateCreator.GetSecret(nsName)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			newSecret := secret.Builder().
				SetName(nsName.Name).
				SetNamespace(nsName.Namespace).
				SetOwnerReferences([]metav1.OwnerReference{owner}).
				Build()

			if !callback(&newSecret) {
				return corev1.Secret{}, nil
			}

			if err := secretGetUpdateCreator.CreateSecret(newSecret); err != nil {
				return corev1.Secret{}, err
			}
			return newSecret, nil
		}
		return corev1.Secret{}, err
	}
	// We are updating the existing Secret
	if !callback(&existingSecret) {
		return existingSecret, nil
	}
	if err := secretGetUpdateCreator.UpdateSecret(existingSecret); err != nil {
		return corev1.Secret{}, err
	}
	return existingSecret, nil
}

// ChangeAutomationConfigDataIfNecessary is a function that optionally changes the existing Automation Config Secret in
// case if its content is different from the desired Automation Config.
// Returns true if the data was changed.
func ChangeAutomationConfigDataIfNecessary(existingSecret *corev1.Secret, targetAutomationConfig *AutomationConfig, log *zap.SugaredLogger) bool {
	if len(existingSecret.Data) == 0 {
		log.Debugf("Secret for the Automation Config doesn't exist, it will be created")
	} else {
		if existingAutomationConfig, err := fromBytes(existingSecret.Data[ConfigKey]); err != nil {
			// in case of any problems deserializing the existing AutomationConfig - just ignore the error and update
			log.Warnf("There were problems deserializing existing automation config - it will be overwritten (%s)", err.Error())
		} else {
			// Otherwise there is an existing automation config and we need to compare it with the Operator version

			// Aligning the versions to make deep comparison correct
			targetAutomationConfig.Version = existingAutomationConfig.Version

			log.Debug("Ensuring authentication credentials")
			if err := ensureConsistentAgentAuthenticationCredentials(targetAutomationConfig, existingAutomationConfig, log); err != nil {
				log.Warnf("error ensuring consistent authentication credentials: %s", err)
				return false
			}

			// If the deployments are the same - we shouldn't perform the update
			// We cannot compare the deployments directly as the "operator" version contains some struct members
			// So we need to turn them into maps
			if reflect.DeepEqual(existingAutomationConfig, targetAutomationConfig) {
				log.Debugf("Automation Config hasn't changed - not updating Secret")
				return false
			}

			// Otherwise we increase the version
			targetAutomationConfig.Version = existingAutomationConfig.Version + 1
			log.Debugf("Automation Config change detected, increasing version: %d -> %d", existingAutomationConfig.Version, existingAutomationConfig.Version+1)
		}
	}

	// By this time we have the AutomationConfig we want to push
	bytes, err := json.Marshal(targetAutomationConfig)
	if err != nil {
		// this definitely cannot happen and means the dev error
		log.Errorf("Failed to serialize automation config! %s", err)
		return false
	}
	if existingSecret.Data == nil {
		existingSecret.Data = map[string][]byte{}
	}
	existingSecret.Data[ConfigKey] = bytes
	return true
}

// ensureConsistentAgentAuthenticationCredentials makes sure that if there are existing authentication credentials
// specified, we use those instead of always generating new ones which would cause constant remounting of the config map
func ensureConsistentAgentAuthenticationCredentials(newAutomationConfig *AutomationConfig, existingAutomationConfig *AutomationConfig, log *zap.SugaredLogger) error {
	// we will keep existing automation agent password
	if existingAutomationConfig.Auth.AutoPwd != "" {
		log.Debug("Agent password has already been generated, using existing password")
		newAutomationConfig.Auth.AutoPwd = existingAutomationConfig.Auth.AutoPwd
	} else {
		log.Debug("Generating new automation agent password")
		if _, err := newAutomationConfig.EnsurePassword(); err != nil {
			return err
		}
	}

	// keep existing keyfile contents
	if existingAutomationConfig.Auth.Key != "" {
		log.Debug("Keyfile contents have already been generated, using existing keyfile contents")
		newAutomationConfig.Auth.Key = existingAutomationConfig.Auth.Key
	} else {
		log.Debug("Generating new keyfile contents")
		if err := newAutomationConfig.EnsureKeyFileContents(); err != nil {
			return err
		}
	}
	return nil
}

// fromBytes takes in jsonBytes representing the Deployment
// and constructs an instance of AutomationConfig with all the concrete structs
// filled out.
func fromBytes(jsonBytes []byte) (*AutomationConfig, error) {
	ac := AutomationConfig{}
	if err := json.Unmarshal(jsonBytes, &ac); err != nil {
		return nil, err
	}
	return &ac, nil
}
