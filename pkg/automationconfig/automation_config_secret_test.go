package automationconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEnsureSecret(t *testing.T) {
	ctx := context.Background()
	secretNsName := types.NamespacedName{Name: "ac-secret", Namespace: "test-namespace"}
	desiredAutomationConfig, err := newAutomationConfig()
	assert.NoError(t, err)

	t.Run("When the secret exists, but does not have the correct key, it is created correctly", func(t *testing.T) {

		s := secret.Builder().
			SetName(secretNsName.Name).
			SetNamespace(secretNsName.Namespace).
			Build()

		secretGetUpdateCreator := &mockSecretGetUpdateCreator{secret: &s}

		ac, err := EnsureSecret(ctx, secretGetUpdateCreator, secretNsName, []metav1.OwnerReference{}, desiredAutomationConfig)
		assert.NoError(t, err)
		assert.Equal(t, desiredAutomationConfig, ac, "The config should be returned if there is not one currently.")

		acSecret, err := secretGetUpdateCreator.GetSecret(ctx, secretNsName)
		assert.NoError(t, err)

		assert.Contains(t, acSecret.Data, ConfigKey, "The secret of the given name should have been updated with the config.")

	})

	t.Run("test LogRotate marshal and unmarshal", func(t *testing.T) {
		ctx := context.Background()

		desiredAutomationConfig, err = NewBuilder().SetMembers(3).AddProcessModification(func(i_ int, p *Process) {
			lr := &CrdLogRotate{
				SizeThresholdMB: "0.001",
				LogRotate: LogRotate{
					TimeThresholdHrs:                1,
					NumUncompressed:                 1,
					NumTotal:                        1,
					IncludeAuditLogsWithMongoDBLogs: false,
				},
				PercentOfDiskspace: "1",
			}
			p.SetLogRotate(lr)
			p.SetAuditLogRotate(lr)
		}).Build()
		assert.NoError(t, err)

		s := secret.Builder().
			SetName(secretNsName.Name).
			SetNamespace(secretNsName.Namespace).
			Build()

		secretGetUpdateCreator := &mockSecretGetUpdateCreator{secret: &s}

		ac, err := EnsureSecret(ctx, secretGetUpdateCreator, secretNsName, []metav1.OwnerReference{}, desiredAutomationConfig)
		assert.NoError(t, err)
		assert.Equal(t, desiredAutomationConfig, ac, "The config should be returned if there is not one currently.")

		bytes := s.Data[ConfigKey]
		acFromBytes, err := FromBytes(bytes)
		assert.NoError(t, err)
		assert.Equal(t, 0.001, acFromBytes.Processes[0].LogRotate.SizeThresholdMB)
		assert.Equal(t, 0.001, acFromBytes.Processes[0].AuditLogRotate.SizeThresholdMB)
		assert.Equal(t, float64(1), acFromBytes.Processes[0].LogRotate.PercentOfDiskspace)
		assert.Equal(t, float64(1), acFromBytes.Processes[0].AuditLogRotate.PercentOfDiskspace)
	})

	t.Run("test LogRotate marshal and unmarshal if not set", func(t *testing.T) {
		ctx := context.Background()

		desiredAutomationConfig, err = NewBuilder().SetMembers(3).AddProcessModification(func(i_ int, p *Process) {}).Build()
		assert.NoError(t, err)

		s := secret.Builder().
			SetName(secretNsName.Name).
			SetNamespace(secretNsName.Namespace).
			Build()

		secretGetUpdateCreator := &mockSecretGetUpdateCreator{secret: &s}

		ac, err := EnsureSecret(ctx, secretGetUpdateCreator, secretNsName, []metav1.OwnerReference{}, desiredAutomationConfig)
		assert.NoError(t, err)
		assert.Equal(t, desiredAutomationConfig, ac, "The config should be returned if there is not one currently.")

		bytes := s.Data[ConfigKey]
		acFromBytes, err := FromBytes(bytes)
		assert.NoError(t, err)
		assert.NotEqual(t, &AcLogRotate{}, acFromBytes.Processes[0].LogRotate)
		assert.Nil(t, acFromBytes.Processes[0].LogRotate)
	})

	t.Run("When the existing Automation Config is different the Automation Config Changes", func(t *testing.T) {

		oldAc, err := newAutomationConfig()
		assert.NoError(t, err)
		existingSecret, err := newAutomationConfigSecret(oldAc, secretNsName)
		assert.NoError(t, err)

		secretGetUpdateCreator := &mockSecretGetUpdateCreator{secret: &existingSecret}

		newAc, err := newAutomationConfigBuilder().SetDomain("different-domain").Build()
		assert.NoError(t, err)

		res, err := EnsureSecret(ctx, secretGetUpdateCreator, secretNsName, []metav1.OwnerReference{}, newAc)
		assert.NoError(t, err)
		assert.Equal(t, newAc, res)

	})

}
func newAutomationConfig() (AutomationConfig, error) {
	return NewBuilder().Build()
}

func newAutomationConfigBuilder() *Builder {
	return NewBuilder().SetName("test-name").SetMembers(3).SetDomain("some-domain")
}

func newAutomationConfigSecret(ac AutomationConfig, nsName types.NamespacedName) (corev1.Secret, error) {
	acBytes, err := json.Marshal(ac)
	if err != nil {
		return corev1.Secret{}, err
	}

	return secret.Builder().
		SetName(nsName.Name).
		SetNamespace(nsName.Namespace).
		SetField(ConfigKey, string(acBytes)).
		Build(), nil

}

type mockSecretGetUpdateCreator struct {
	secret *corev1.Secret
}

func (m *mockSecretGetUpdateCreator) GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	if m.secret != nil {
		if objectKey.Name == m.secret.Name && objectKey.Namespace == m.secret.Namespace {
			return *m.secret, nil
		}
	}
	return corev1.Secret{}, notFoundError()
}

func (m *mockSecretGetUpdateCreator) UpdateSecret(ctx context.Context, secret corev1.Secret) error {
	m.secret = &secret
	return nil
}

func (m *mockSecretGetUpdateCreator) CreateSecret(ctx context.Context, secret corev1.Secret) error {
	if m.secret == nil {
		m.secret = &secret
		return nil
	}
	return alreadyExistsError()
}

// notFoundError returns an error which returns true for "errors.IsNotFound"
func notFoundError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
}

func alreadyExistsError() error {
	return &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonAlreadyExists}}
}
