package secret

import (
	"context"
	"encoding/json"

	"github.com/spf13/cast"
	"k8s.io/client-go/kubernetes"
)

const (
	automationConfigKey = "cluster-config.json"
)

func ReadAutomationConfigVersionFromSecret(ctx context.Context, namespace string, clientSet kubernetes.Interface, automationConfigMap string) (int64, error) {
	secretReader := newKubernetesSecretReader(clientSet)
	theSecret, err := secretReader.ReadSecret(ctx, namespace, automationConfigMap)
	if err != nil {
		return -1, err
	}
	var existingDeployment map[string]interface{}
	if err := json.Unmarshal(theSecret.Data[automationConfigKey], &existingDeployment); err != nil {
		return -1, err
	}

	version, ok := existingDeployment["version"]
	if !ok {
		return -1, err
	}
	return cast.ToInt64(version), nil
}
