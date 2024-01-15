package authtypes

import (
	"fmt"
	"net/url"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/contains"
)

// Options contains a set of values that can be used for more fine-grained configuration of authentication.
type Options struct {
	// AuthoritativeSet indicates whether the agents will remove users not defined in the AutomationConfig.
	AuthoritativeSet bool

	// KeyFile is the path on disk to the keyfile that will be used for the deployment.
	KeyFile string

	// AuthMechanisms is a list of valid authentication mechanisms deployments/agents can use.
	AuthMechanisms []string

	// AgentName is username that the Automation Agent will have.
	AgentName string

	// AutoAuthMechanism is the desired authentication mechanism that the agents will use.
	AutoAuthMechanism string
}

func (o *Options) IsSha256Present() bool {
	return contains.String(o.AuthMechanisms, constants.Sha256)
}

// Role is a struct which will map to automationconfig.Role.
type Role struct {
	// Name is the name of the role.
	Name string

	// Database is the database this role applies to.
	Database string
}

// User is a struct which holds all the values required to create an AutomationConfig user
// and references to the credentials for that specific user.
type User struct {
	// Username is the username of the user.
	Username string

	// Database is the database this user will be created in.
	Database string

	// Roles is a slice of roles that this user should have.
	Roles []Role

	// PasswordSecretKey is the key which maps to the value of the user's password.
	PasswordSecretKey string

	// PasswordSecretName is the name of the secret which stores this user's password.
	PasswordSecretName string

	// ScramCredentialsSecretName returns the name of the secret which stores the generated credentials
	// for this user. These credentials will be generated if they do not exist, or used if they do.
	// Note: there will be one secret with credentials per user created.
	ScramCredentialsSecretName string

	// ConnectionStringSecretName is the name of the secret object created by the operator
	// which exposes the connection strings for the user.
	// Note: there will be one secret with connection strings per user created.
	ConnectionStringSecretName string

	// ConnectionStringSecretNamespace is the namespace of the secret object created by the operator which exposes the connection strings for the user.
	ConnectionStringSecretNamespace string `json:"connectionStringSecretNamespace,omitempty"`

	// ConnectionStringOptions contains connection string options for this user
	// These options will be appended at the end of the connection string and
	// will override any existing options from the resources.
	ConnectionStringOptions map[string]interface{}
}

func (u User) GetLoginString(password string) string {
	if u.Database != constants.ExternalDB {
		return fmt.Sprintf("%s:%s@",
			url.QueryEscape(u.Username),
			url.QueryEscape(password))
	}
	return ""
}

// Configurable is an interface which any resource which can configure ScramSha authentication should implement.
type Configurable interface {
	// GetAuthOptions returns a set of Options which can be used for fine-grained configuration.
	GetAuthOptions() Options

	// GetAuthUsers returns a list of users which will be mapped to users in the AutomationConfig.
	GetAuthUsers() []User

	// GetAgentPasswordSecretNamespacedName returns the NamespacedName of the secret which stores the generated password for the agent.
	GetAgentPasswordSecretNamespacedName() types.NamespacedName

	// GetAgentKeyfileSecretNamespacedName returns the NamespacedName of the secret which stores the keyfile for the agent.
	GetAgentKeyfileSecretNamespacedName() types.NamespacedName

	// NamespacedName returns the NamespacedName for the resource that is being configured.
	NamespacedName() types.NamespacedName

	// GetOwnerReferences returns the OwnerReferences pointing to the current resource.
	GetOwnerReferences() []v1.OwnerReference
}
