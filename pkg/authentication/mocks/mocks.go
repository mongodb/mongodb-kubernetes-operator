package mocks

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
)

type MockSecretGetUpdateCreateDeleter struct {
	secrets map[client.ObjectKey]corev1.Secret
}

func NewMockedSecretGetUpdateCreateDeleter(secrets ...corev1.Secret) secret.GetUpdateCreateDeleter {
	mockSecretGetUpdateCreateDeleter := MockSecretGetUpdateCreateDeleter{}
	mockSecretGetUpdateCreateDeleter.secrets = make(map[client.ObjectKey]corev1.Secret)
	for _, s := range secrets {
		mockSecretGetUpdateCreateDeleter.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	}
	return mockSecretGetUpdateCreateDeleter
}

func (c MockSecretGetUpdateCreateDeleter) DeleteSecret(_ context.Context, key client.ObjectKey) error {
	delete(c.secrets, key)
	return nil
}

func (c MockSecretGetUpdateCreateDeleter) UpdateSecret(_ context.Context, s corev1.Secret) error {
	c.secrets[types.NamespacedName{Name: s.Name, Namespace: s.Namespace}] = s
	return nil
}

func (c MockSecretGetUpdateCreateDeleter) CreateSecret(ctx context.Context, secret corev1.Secret) error {
	return c.UpdateSecret(ctx, secret)
}

func (c MockSecretGetUpdateCreateDeleter) GetSecret(_ context.Context, objectKey client.ObjectKey) (corev1.Secret, error) {
	if s, ok := c.secrets[objectKey]; !ok {
		return corev1.Secret{}, &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}}
	} else {
		return s, nil
	}
}

type MockConfigurable struct {
	opts   authtypes.Options
	users  []authtypes.User
	nsName types.NamespacedName
	refs   []metav1.OwnerReference
}

func NewMockConfigurable(opts authtypes.Options, users []authtypes.User, nsName types.NamespacedName, refs []metav1.OwnerReference) MockConfigurable {
	return MockConfigurable{opts: opts, users: users, nsName: nsName, refs: refs}
}

func (m MockConfigurable) AgentCertificateSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.nsName.Namespace,
		Name:      "agent-certs",
	}
}

func (m MockConfigurable) GetAgentPasswordSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.nsName.Name + "-agent-password", Namespace: m.nsName.Namespace}
}

func (m MockConfigurable) GetAgentKeyfileSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.nsName.Name + "-keyfile", Namespace: m.nsName.Namespace}
}

func (m MockConfigurable) GetAuthOptions() authtypes.Options {
	return m.opts
}

func (m MockConfigurable) GetAuthUsers() []authtypes.User {
	return m.users
}

func (m MockConfigurable) NamespacedName() types.NamespacedName {
	return m.nsName
}

func (m MockConfigurable) GetOwnerReferences() []metav1.OwnerReference {
	return m.refs
}

func BuildX509MongoDBUser(name string) authtypes.User {
	return authtypes.User{
		Username: fmt.Sprintf("CN=%s,OU=organizationalunit,O=organization", name),
		Database: "$external",
		Roles: []authtypes.Role{
			{
				Database: "admin",
				Name:     "readWrite",
			},
			{
				Database: "admin",
				Name:     "clusterAdmin",
			},
		},
	}

}

func BuildScramMongoDBUser(name string) authtypes.User {
	return authtypes.User{
		Username: name,
		Database: "admin",
		Roles: []authtypes.Role{
			{
				Database: "testing",
				Name:     "readWrite",
			},
			{
				Database: "testing",
				Name:     "clusterAdmin",
			},
			// admin roles for reading FCV
			{
				Database: "admin",
				Name:     "readWrite",
			},
			{
				Database: "admin",
				Name:     "clusterAdmin",
			},
		},
		PasswordSecretKey:          fmt.Sprintf("%s-password", name),
		PasswordSecretName:         fmt.Sprintf("%s-password-secret", name),
		ScramCredentialsSecretName: fmt.Sprintf("%s-scram", name),
	}

}
