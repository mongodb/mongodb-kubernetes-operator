package scram

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockSecretGetUpdateCreateDeleter struct {
	secrets map[client.ObjectKey]corev1.Secret
}

func (c mockSecretGetUpdateCreateDeleter) DeleteSecret(ctx context.Context, objectKey client.ObjectKey) error {
	delete(c.secrets, objectKey)
	return nil
}

func (c mockSecretGetUpdateCreateDeleter) UpdateSecret(ctx context.Context, s corev1.Secret) error {
	c.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	return nil
}

func (c mockSecretGetUpdateCreateDeleter) CreateSecret(ctx context.Context, secret corev1.Secret) error {
	return c.UpdateSecret(ctx, secret)
}

func (c mockSecretGetUpdateCreateDeleter) GetSecret(ctx context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	if s, ok := c.secrets[objectKey]; !ok {
		return corev1.Secret{}, notFoundError()
	} else {
		return s, nil
	}
}

type mockConfigurable struct {
	opts   Options
	users  []User
	nsName types.NamespacedName
}

func (m mockConfigurable) GetAgentPasswordSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.nsName.Name + "-agent-password", Namespace: m.nsName.Namespace}
}

func (m mockConfigurable) GetAgentKeyfileSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.nsName.Name + "-keyfile", Namespace: m.nsName.Namespace}
}

func (m mockConfigurable) GetScramOptions() Options {
	return m.opts
}

func (m mockConfigurable) GetScramUsers() []User {
	return m.users
}

func (m mockConfigurable) NamespacedName() types.NamespacedName {
	return m.nsName
}

func (m mockConfigurable) GetOwnerReferences() []metav1.OwnerReference {
	return nil
}
