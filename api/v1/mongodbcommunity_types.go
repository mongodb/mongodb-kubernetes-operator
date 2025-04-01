package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	"github.com/stretchr/objx"
)

type Type string

const (
	ReplicaSet       Type   = "ReplicaSet"
	defaultDBForUser string = "admin"
)

type Phase string

const (
	Running            Phase = "Running"
	Failed             Phase = "Failed"
	Pending            Phase = "Pending"
	defaultPasswordKey       = "password"

	// Keep in sync with controllers/prometheus.go
	defaultPrometheusPort = 9216
)

// SCRAM-SHA-256 and SCRAM-SHA-1 are the supported auth modes.
const (
	defaultMode AuthMode = "SCRAM-SHA-256"
)

const (
	defaultClusterDomain = "cluster.local"
)

// Connection string options that should be ignored as they are set through other means.
var (
	protectedConnectionStringOptions = map[string]struct{}{
		"replicaSet": {},
		"ssl":        {},
		"tls":        {},
	}
)

// MongoDBCommunitySpec defines the desired state of MongoDB
type MongoDBCommunitySpec struct {
	// Members is the number of members in the replica set
	// +optional
	Members int `json:"members"`
	// Type defines which type of MongoDB deployment the resource should create
	// +kubebuilder:validation:Enum=ReplicaSet
	Type Type `json:"type"`
	// Version defines which version of MongoDB will be used
	Version string `json:"version,omitempty"`

	// Arbiters is the number of arbiters to add to the Replica Set.
	// It is not recommended to have more than one arbiter per Replica Set.
	// More info: https://www.mongodb.com/docs/manual/tutorial/add-replica-set-arbiter/
	// +optional
	Arbiters int `json:"arbiters"`

	// FeatureCompatibilityVersion configures the feature compatibility version that will
	// be set for the deployment
	// +optional
	FeatureCompatibilityVersion string `json:"featureCompatibilityVersion,omitempty"`

	// ReplicaSetHorizons Add this parameter and values if you need your database
	// to be accessed outside of Kubernetes. This setting allows you to
	// provide different DNS settings within the Kubernetes cluster and
	// to the Kubernetes cluster. The Kubernetes Operator uses split horizon
	// DNS for replica set members. This feature allows communication both
	// within the Kubernetes cluster and from outside Kubernetes.
	// +optional
	ReplicaSetHorizons ReplicaSetHorizonConfiguration `json:"replicaSetHorizons,omitempty"`

	// Security configures security features, such as TLS, and authentication settings for a deployment
	// +required
	Security Security `json:"security"`

	// Users specifies the MongoDB users that should be configured in your deployment
	// +required
	Users []MongoDBUser `json:"users"`

	// +optional
	StatefulSetConfiguration StatefulSetConfiguration `json:"statefulSet,omitempty"`

	// AgentConfiguration sets options for the MongoDB automation agent
	// +optional
	AgentConfiguration AgentConfiguration `json:"agent,omitempty"`

	// AdditionalMongodConfig is additional configuration that can be passed to
	// each data-bearing mongod at runtime. Uses the same structure as the mongod
	// configuration file: https://www.mongodb.com/docs/manual/reference/configuration-options/
	// +kubebuilder:validation:Type=object
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	AdditionalMongodConfig MongodConfiguration `json:"additionalMongodConfig,omitempty"`

	// AutomationConfigOverride is merged on top of the operator created automation config. Processes are merged
	// by name. Currently Only the process.disabled field is supported.
	AutomationConfigOverride *AutomationConfigOverride `json:"automationConfig,omitempty"`

	// Prometheus configurations.
	// +optional
	Prometheus *Prometheus `json:"prometheus,omitempty"`

	// Additional options to be appended to the connection string. These options apply to the entire resource and to each user.
	// +kubebuilder:validation:Type=object
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	AdditionalConnectionStringConfig MapWrapper `json:"additionalConnectionStringConfig,omitempty"`

	// MemberConfig
	// +optional
	MemberConfig []automationconfig.MemberOptions `json:"memberConfig,omitempty"`
}

// MapWrapper is a wrapper for a map to be used by other structs.
// The CRD generator does not support map[string]interface{}
// on the top level and hence we need to work around this with
// a wrapping struct.
type MapWrapper struct {
	Object map[string]interface{} `json:"-"`
}

// MarshalJSON defers JSON encoding to the wrapped map
func (m *MapWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Object)
}

// UnmarshalJSON will decode the data into the wrapped map
func (m *MapWrapper) UnmarshalJSON(data []byte) error {
	if m.Object == nil {
		m.Object = map[string]interface{}{}
	}

	// Handle keys like net.port to be set as nested maps.
	// Without this after unmarshalling there is just key "net.port" which is not
	// a nested map and methods like GetPort() cannot access the value.
	tmpMap := map[string]interface{}{}
	err := json.Unmarshal(data, &tmpMap)
	if err != nil {
		return err
	}

	for k, v := range tmpMap {
		m.SetOption(k, v)
	}

	return nil
}

func (m *MapWrapper) DeepCopy() *MapWrapper {
	if m != nil && m.Object != nil {
		return &MapWrapper{
			Object: runtime.DeepCopyJSON(m.Object),
		}
	}
	c := NewMapWrapper()
	return &c
}

// NewMapWrapper returns an empty MapWrapper
func NewMapWrapper() MapWrapper {
	return MapWrapper{Object: map[string]interface{}{}}
}

// SetOption updated the MapWrapper with a new option
func (m MapWrapper) SetOption(key string, value interface{}) MapWrapper {
	m.Object = objx.New(m.Object).Set(key, value)
	return m
}

// ReplicaSetHorizonConfiguration holds the split horizon DNS settings for
// replica set members.
type ReplicaSetHorizonConfiguration []automationconfig.ReplicaSetHorizons

// CustomRole defines a custom MongoDB role.
type CustomRole struct {
	// The name of the role.
	Role string `json:"role"`
	// The database of the role.
	DB string `json:"db"`
	// The privileges to grant the role.
	Privileges []Privilege `json:"privileges"`
	// An array of roles from which this role inherits privileges.
	// +optional
	Roles []Role `json:"roles"`
	// The authentication restrictions the server enforces on the role.
	// +optional
	AuthenticationRestrictions []AuthenticationRestriction `json:"authenticationRestrictions,omitempty"`
}

type Prometheus struct {
	// Port where metrics endpoint will bind to. Defaults to 9216.
	// +optional
	Port int `json:"port,omitempty"`

	// HTTP Basic Auth Username for metrics endpoint.
	Username string `json:"username"`

	// Name of a Secret containing a HTTP Basic Auth Password.
	PasswordSecretRef SecretKeyReference `json:"passwordSecretRef"`

	// Indicates path to the metrics endpoint.
	// +kubebuilder:validation:Pattern=^\/[a-z0-9]+$
	MetricsPath string `json:"metricsPath,omitempty"`

	// Name of a Secret (type kubernetes.io/tls) holding the certificates to use in the
	// Prometheus endpoint.
	// +optional
	TLSSecretRef SecretKeyReference `json:"tlsSecretKeyRef,omitempty"`
}

func (p Prometheus) GetPasswordKey() string {
	if p.PasswordSecretRef.Key != "" {
		return p.PasswordSecretRef.Key
	}

	return "password"
}

func (p Prometheus) GetPort() int {
	if p.Port != 0 {
		return p.Port
	}

	return defaultPrometheusPort
}

// ConvertToAutomationConfigCustomRole converts between a custom role defined by the crd and a custom role
// that can be used in the automation config.
func (c CustomRole) ConvertToAutomationConfigCustomRole() automationconfig.CustomRole {
	ac := automationconfig.CustomRole{Role: c.Role, DB: c.DB, Roles: []automationconfig.Role{}}

	// Add privileges.
	for _, privilege := range c.Privileges {
		ac.Privileges = append(ac.Privileges, automationconfig.Privilege{
			Resource: automationconfig.Resource{
				DB:          privilege.Resource.DB,
				Collection:  privilege.Resource.Collection,
				AnyResource: privilege.Resource.AnyResource,
				Cluster:     privilege.Resource.Cluster,
			},
			Actions: privilege.Actions,
		})
	}

	// Add roles.
	for _, dbRole := range c.Roles {
		ac.Roles = append(ac.Roles, automationconfig.Role{
			Role:     dbRole.Name,
			Database: dbRole.DB,
		})
	}

	// Add authentication restrictions (if any).
	for _, restriction := range c.AuthenticationRestrictions {
		ac.AuthenticationRestrictions = append(ac.AuthenticationRestrictions,
			automationconfig.AuthenticationRestriction{
				ClientSource:  restriction.ClientSource,
				ServerAddress: restriction.ServerAddress,
			})
	}

	return ac
}

// ConvertCustomRolesToAutomationConfigCustomRole converts custom roles to custom roles
// that can be used in the automation config.
func ConvertCustomRolesToAutomationConfigCustomRole(roles []CustomRole) []automationconfig.CustomRole {
	acRoles := []automationconfig.CustomRole{}
	for _, role := range roles {
		acRoles = append(acRoles, role.ConvertToAutomationConfigCustomRole())
	}
	return acRoles
}

// Privilege defines the actions a role is allowed to perform on a given resource.
type Privilege struct {
	Resource Resource `json:"resource"`
	Actions  []string `json:"actions"`
}

// Resource specifies specifies the resources upon which a privilege permits actions.
// See https://www.mongodb.com/docs/manual/reference/resource-document for more.
type Resource struct {
	// +optional
	DB *string `json:"db,omitempty"`
	// +optional
	Collection *string `json:"collection,omitempty"`
	// +optional
	Cluster bool `json:"cluster,omitempty"`
	// +optional
	AnyResource bool `json:"anyResource,omitempty"`
}

// AuthenticationRestriction specifies a list of IP addresses and CIDR ranges users
// are allowed to connect to or from.
type AuthenticationRestriction struct {
	ClientSource  []string `json:"clientSource"`
	ServerAddress []string `json:"serverAddress"`
}

// AutomationConfigOverride contains fields which will be overridden in the operator created config.
type AutomationConfigOverride struct {
	Processes  []OverrideProcess  `json:"processes,omitempty"`
	ReplicaSet OverrideReplicaSet `json:"replicaSet,omitempty"`
}

type OverrideReplicaSet struct {
	// Id can be used together with additionalMongodConfig.replication.replSetName
	// to manage clusters where replSetName differs from the MongoDBCommunity resource name
	Id *string `json:"id,omitempty"`
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Settings MapWrapper `json:"settings,omitempty"`
}

// Note: We do not use the automationconfig.Process type directly here as unmarshalling cannot happen directly
// with the Args26 which is a map[string]interface{}

// OverrideProcess contains fields that we can override on the AutomationConfig processes.
type OverrideProcess struct {
	Name      string                         `json:"name"`
	Disabled  bool                           `json:"disabled"`
	LogRotate *automationconfig.CrdLogRotate `json:"logRotate,omitempty"`
}

// StatefulSetConfiguration holds the optional custom StatefulSet
// that should be merged into the operator created one.
type StatefulSetConfiguration struct {
	// +kubebuilder:pruning:PreserveUnknownFields
	SpecWrapper StatefulSetSpecWrapper `json:"spec"`
	// +optional
	MetadataWrapper StatefulSetMetadataWrapper `json:"metadata"`
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

type AgentConfiguration struct {
	// +optional
	LogLevel LogLevel `json:"logLevel"`
	// +optional
	LogFile string `json:"logFile"`
	// +optional
	MaxLogFileDurationHours int `json:"maxLogFileDurationHours"`
	// +optional
	// LogRotate if enabled, will enable LogRotate for all processes.
	LogRotate *automationconfig.CrdLogRotate `json:"logRotate,omitempty"`
	// +optional
	// AuditLogRotate if enabled, will enable AuditLogRotate for all processes.
	AuditLogRotate *automationconfig.CrdLogRotate `json:"auditLogRotate,omitempty"`
	// +optional
	// SystemLog configures system log of mongod
	SystemLog *automationconfig.SystemLog `json:"systemLog,omitempty"`
}

// StatefulSetSpecWrapper is a wrapper around StatefulSetSpec with a custom implementation
// of MarshalJSON and UnmarshalJSON which delegate to the underlying Spec to avoid CRD pollution.

type StatefulSetSpecWrapper struct {
	Spec appsv1.StatefulSetSpec `json:"-"`
}

// MarshalJSON defers JSON encoding to the wrapped map
func (m *StatefulSetSpecWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Spec)
}

// UnmarshalJSON will decode the data into the wrapped map
func (m *StatefulSetSpecWrapper) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &m.Spec)
}

func (m *StatefulSetSpecWrapper) DeepCopy() *StatefulSetSpecWrapper {
	return &StatefulSetSpecWrapper{
		Spec: m.Spec,
	}
}

// StatefulSetMetadataWrapper is a wrapper around Labels and Annotations
type StatefulSetMetadataWrapper struct {
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (m *StatefulSetMetadataWrapper) DeepCopy() *StatefulSetMetadataWrapper {
	return &StatefulSetMetadataWrapper{
		Labels:      m.Labels,
		Annotations: m.Annotations,
	}
}

// MongodConfiguration holds the optional mongod configuration
// that should be merged with the operator created one.
type MongodConfiguration struct {
	MapWrapper `json:"-"`
}

// NewMongodConfiguration returns an empty MongodConfiguration
func NewMongodConfiguration() MongodConfiguration {
	return MongodConfiguration{MapWrapper{map[string]interface{}{}}}
}

// GetDBDataDir returns the db path which should be used.
func (m MongodConfiguration) GetDBDataDir() string {
	return objx.New(m.Object).Get("storage.dbPath").Str(automationconfig.DefaultMongoDBDataDir)
}

// GetDBPort returns the port that should be used for the mongod process.
// If port is not specified, the default port of 27017 will be used.
func (m MongodConfiguration) GetDBPort() int {
	portValue := objx.New(m.Object).Get("net.port")

	// Underlying map could be manipulated in code, e.g. via SetDBPort (e.g. in unit tests) - then it will be as int,
	// or it could be deserialized from JSON and then integer in an untyped map will be deserialized as float64.
	// It's behavior of https://pkg.go.dev/encoding/json#Unmarshal that is converting JSON integers as float64.
	if portValue.IsInt() {
		return portValue.Int(automationconfig.DefaultDBPort)
	} else if portValue.IsFloat64() {
		return int(portValue.Float64(float64(automationconfig.DefaultDBPort)))
	}

	return automationconfig.DefaultDBPort
}

// SetDBPort ensures that port is stored as float64
func (m MongodConfiguration) SetDBPort(port int) MongodConfiguration {
	m.SetOption("net.port", float64(port))
	return m
}

type MongoDBUser struct {
	// Name is the username of the user
	Name string `json:"name"`

	// DB is the database the user is stored in. Defaults to "admin"
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=admin
	DB string `json:"db,omitempty"`

	// PasswordSecretRef is a reference to the secret containing this user's password
	// +optional
	PasswordSecretRef SecretKeyReference `json:"passwordSecretRef,omitempty"`

	// Roles is an array of roles assigned to this user
	Roles []Role `json:"roles"`

	// ScramCredentialsSecretName appended by string "scram-credentials" is the name of the secret object created by the mongoDB operator for storing SCRAM credentials
	// These secrets names must be different for each user in a deployment.
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$
	// +optional
	ScramCredentialsSecretName string `json:"scramCredentialsSecretName,omitempty"`

	// ConnectionStringSecretName is the name of the secret object created by the operator which exposes the connection strings for the user.
	// If provided, this secret must be different for each user in a deployment.
	// +optional
	ConnectionStringSecretName string `json:"connectionStringSecretName,omitempty"`

	// ConnectionStringSecretNamespace is the namespace of the secret object created by the operator which exposes the connection strings for the user.
	// +optional
	ConnectionStringSecretNamespace string `json:"connectionStringSecretNamespace,omitempty"`

	// Additional options to be appended to the connection string.
	// These options apply only to this user and will override any existing options in the resource.
	// +kubebuilder:validation:Type=object
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +nullable
	AdditionalConnectionStringConfig MapWrapper `json:"additionalConnectionStringConfig,omitempty"`
}

func (m MongoDBUser) GetPasswordSecretKey() string {
	if m.PasswordSecretRef.Key == "" {
		return defaultPasswordKey
	}
	return m.PasswordSecretRef.Key
}

// GetScramCredentialsSecretName gets the final SCRAM credentials secret-name by appending the user provided
// scramsCredentialSecretName with "scram-credentials"
func (m MongoDBUser) GetScramCredentialsSecretName() string {
	return fmt.Sprintf("%s-%s", m.ScramCredentialsSecretName, "scram-credentials")
}

// GetConnectionStringSecretName gets the connection string secret name provided by the user or generated
// from the SCRAM user configuration.
func (m MongoDBUser) GetConnectionStringSecretName(resourceName string) string {
	if m.ConnectionStringSecretName != "" {
		return m.ConnectionStringSecretName
	}

	return normalizeName(fmt.Sprintf("%s-%s-%s", resourceName, m.DB, m.Name))
}

// GetConnectionStringSecretNamespace gets the connection string secret namespace provided by the user or generated
// from the SCRAM user configuration.
func (m MongoDBUser) GetConnectionStringSecretNamespace(resourceNamespace string) string {
	if m.ConnectionStringSecretNamespace != "" {
		return m.ConnectionStringSecretNamespace
	}

	return resourceNamespace
}

// normalizeName returns a string that conforms to RFC-1123
func normalizeName(name string) string {
	errors := validation.IsDNS1123Subdomain(name)
	if len(errors) == 0 {
		return name
	}

	// convert name to lowercase and replace invalid characters with '-'
	name = strings.ToLower(name)
	re := regexp.MustCompile("[^a-z0-9-]+")
	name = re.ReplaceAllString(name, "-")

	// Remove duplicate `-` resulting from contiguous non-allowed chars.
	re = regexp.MustCompile(`\-+`)
	name = re.ReplaceAllString(name, "-")

	name = strings.Trim(name, "-")

	if len(name) > validation.DNS1123SubdomainMaxLength {
		name = name[0:validation.DNS1123SubdomainMaxLength]
	}
	return name
}

// SecretKeyReference is a reference to the secret containing the user's password
type SecretKeyReference struct {
	// Name is the name of the secret storing this user's password
	Name string `json:"name"`

	// Key is the key in the secret storing this password. Defaults to "password"
	// +optional
	Key string `json:"key"`
}

// Role is the database role this user should have
type Role struct {
	// DB is the database the role can act on
	DB string `json:"db"`
	// Name is the name of the role
	Name string `json:"name"`
}

type Security struct {
	// +optional
	Authentication Authentication `json:"authentication"`
	// TLS configuration for both client-server and server-server communication
	// +optional
	TLS TLS `json:"tls"`
	// User-specified custom MongoDB roles that should be configured in the deployment.
	// +optional
	Roles []CustomRole `json:"roles,omitempty"`
}

// TLS is the configuration used to set up TLS encryption
type TLS struct {
	Enabled bool `json:"enabled"`

	// Optional configures if TLS should be required or optional for connections
	// +optional
	Optional bool `json:"optional"`

	// CertificateKeySecret is a reference to a Secret containing a private key and certificate to use for TLS.
	// The key and cert are expected to be PEM encoded and available at "tls.key" and "tls.crt".
	// This is the same format used for the standard "kubernetes.io/tls" Secret type, but no specific type is required.
	// Alternatively, an entry tls.pem, containing the concatenation of cert and key, can be provided.
	// If all of tls.pem, tls.crt and tls.key are present, the tls.pem one needs to be equal to the concatenation of tls.crt and tls.key
	// +optional
	CertificateKeySecret corev1.LocalObjectReference `json:"certificateKeySecretRef"`

	// CaCertificateSecret is a reference to a Secret containing the certificate for the CA which signed the server certificates
	// The certificate is expected to be available under the key "ca.crt"
	// +optional
	CaCertificateSecret *corev1.LocalObjectReference `json:"caCertificateSecretRef,omitempty"`

	// CaConfigMap is a reference to a ConfigMap containing the certificate for the CA which signed the server certificates
	// The certificate is expected to be available under the key "ca.crt"
	// This field is ignored when CaCertificateSecretRef is configured
	// +optional
	CaConfigMap *corev1.LocalObjectReference `json:"caConfigMapRef,omitempty"`
}

type Authentication struct {
	// Modes is an array specifying which authentication methods should be enabled.
	Modes []AuthMode `json:"modes"`

	// AgentMode contains the authentication mode used by the automation agent.
	// +optional
	AgentMode AuthMode `json:"agentMode,omitempty"`

	// AgentCertificateSecret is a reference to a Secret containing the certificate and the key for the automation agent
	// The secret needs to have available:
	// - certificate under key: "tls.crt"
	// - private key under key: "tls.key"
	// If additionally, tls.pem is present, then it needs to be equal to the concatenation of tls.crt and tls.key
	// +optional
	AgentCertificateSecret *corev1.LocalObjectReference `json:"agentCertificateSecretRef,omitempty"`

	// IgnoreUnknownUsers set to true will ensure any users added manually (not through the CRD)
	// will not be removed.

	// TODO: defaults will work once we update to v1 CRD.

	// +optional
	// +kubebuilder:default:=true
	// +nullable
	IgnoreUnknownUsers *bool `json:"ignoreUnknownUsers,omitempty"`
}

// +kubebuilder:validation:Enum=SCRAM;SCRAM-SHA-256;SCRAM-SHA-1;X509
type AuthMode string

func IsAuthPresent(authModes []AuthMode, auth string) bool {
	for _, authMode := range authModes {
		if string(authMode) == auth {
			return true
		}
	}
	return false
}

// ConvertAuthModeToAuthMechanism acts as a map but is immutable. It allows users to use different labels to describe the
// same authentication mode.
func ConvertAuthModeToAuthMechanism(authModeLabel AuthMode) string {
	switch authModeLabel {
	case "SCRAM", "SCRAM-SHA-256":
		return constants.Sha256
	case "SCRAM-SHA-1":
		return constants.Sha1
	case "X509":
		return constants.X509
	default:
		return ""
	}
}

// MongoDBCommunityStatus defines the observed state of MongoDB
type MongoDBCommunityStatus struct {
	MongoURI string `json:"mongoUri"`
	Phase    Phase  `json:"phase"`
	Version  string `json:"version,omitempty"`

	CurrentStatefulSetReplicas int `json:"currentStatefulSetReplicas"`
	CurrentMongoDBMembers      int `json:"currentMongoDBMembers"`

	CurrentStatefulSetArbitersReplicas int `json:"currentStatefulSetArbitersReplicas,omitempty"`
	CurrentMongoDBArbiters             int `json:"currentMongoDBArbiters,omitempty"`

	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MongoDBCommunity is the Schema for the mongodbs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mongodbcommunity,scope=Namespaced,shortName=mdbc,singular=mongodbcommunity
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Current state of the MongoDB deployment"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Version of MongoDB server"
// +kubebuilder:metadata:annotations="service.binding/type=mongodb"
// +kubebuilder:metadata:annotations="service.binding/provider=community"
// +kubebuilder:metadata:annotations="service.binding=path={.metadata.name}-{.spec.users[0].db}-{.spec.users[0].name},objectType=Secret"
// +kubebuilder:metadata:annotations="service.binding/connectionString=path={.metadata.name}-{.spec.users[0].db}-{.spec.users[0].name},objectType=Secret,sourceKey=connectionString.standardSrv"
// +kubebuilder:metadata:annotations="service.binding/username=path={.metadata.name}-{.spec.users[0].db}-{.spec.users[0].name},objectType=Secret,sourceKey=username"
// +kubebuilder:metadata:annotations="service.binding/password=path={.metadata.name}-{.spec.users[0].db}-{.spec.users[0].name},objectType=Secret,sourceKey=password"
type MongoDBCommunity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MongoDBCommunitySpec   `json:"spec,omitempty"`
	Status MongoDBCommunityStatus `json:"status,omitempty"`
}

func (m *MongoDBCommunity) GetMongodConfiguration() MongodConfiguration {
	mongodConfig := NewMongodConfiguration()
	for k, v := range m.Spec.AdditionalMongodConfig.Object {
		mongodConfig.SetOption(k, v)
	}
	return mongodConfig
}

func (m *MongoDBCommunity) GetAgentPasswordSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-agent-password", Namespace: m.Namespace}
}

func (m *MongoDBCommunity) GetAgentKeyfileSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-keyfile", Namespace: m.Namespace}
}

func (m *MongoDBCommunity) GetOwnerReferences() []metav1.OwnerReference {
	ownerReference := *metav1.NewControllerRef(m, schema.GroupVersionKind{
		Group:   GroupVersion.Group,
		Version: GroupVersion.Version,
		Kind:    m.Kind,
	})
	return []metav1.OwnerReference{ownerReference}
}

// GetAuthOptions returns a set of Options that are used to configure scram
// authentication.
func (m *MongoDBCommunity) GetAuthOptions() authtypes.Options {
	ignoreUnknownUsers := true
	if m.Spec.Security.Authentication.IgnoreUnknownUsers != nil {
		ignoreUnknownUsers = *m.Spec.Security.Authentication.IgnoreUnknownUsers
	}

	authModes := m.Spec.Security.Authentication.Modes
	defaultAuthMechanism := ConvertAuthModeToAuthMechanism(defaultMode)
	autoAuthMechanism := ConvertAuthModeToAuthMechanism(m.Spec.GetAgentAuthMode())
	authMechanisms := make([]string, len(authModes))

	if autoAuthMechanism == "" {
		autoAuthMechanism = defaultAuthMechanism
	}

	if len(authModes) == 0 {
		authMechanisms = []string{defaultAuthMechanism}
	} else {
		for i, authMode := range authModes {
			if authMech := ConvertAuthModeToAuthMechanism(authMode); authMech != "" {
				authMechanisms[i] = authMech
			}
		}
	}

	return authtypes.Options{
		AuthoritativeSet:  !ignoreUnknownUsers,
		KeyFile:           constants.AutomationAgentKeyFilePathInContainer,
		AuthMechanisms:    authMechanisms,
		AgentName:         constants.AgentName,
		AutoAuthMechanism: autoAuthMechanism,
	}
}

// GetAuthUsers converts all the users from the spec into users
// that can be used to configure authentication.
func (m *MongoDBCommunity) GetAuthUsers() []authtypes.User {
	users := make([]authtypes.User, len(m.Spec.Users))
	for i, u := range m.Spec.Users {
		roles := make([]authtypes.Role, len(u.Roles))
		for j, r := range u.Roles {

			roles[j] = authtypes.Role{
				Name:     r.Name,
				Database: r.DB,
			}
		}

		// When the MongoDB resource has been fetched from Kubernetes,
		// the User's database will be set to "admin" because this is set
		// by default on the CRD, but when running e2e tests, the resource
		// we are working with is local -- it has not been posted to the
		// Kubernetes API and the `u.DB` was not set to the default ("admin").
		// This is why the "admin" value is being set here.
		if u.DB == "" {
			u.DB = defaultDBForUser
		}

		users[i] = authtypes.User{
			Username:                        u.Name,
			Database:                        u.DB,
			Roles:                           roles,
			ConnectionStringSecretName:      u.GetConnectionStringSecretName(m.Name),
			ConnectionStringSecretNamespace: u.GetConnectionStringSecretNamespace(m.Namespace),
			ConnectionStringOptions:         u.AdditionalConnectionStringConfig.Object,
		}

		if u.DB != constants.ExternalDB {
			users[i].ScramCredentialsSecretName = u.GetScramCredentialsSecretName()
			users[i].PasswordSecretKey = u.GetPasswordSecretKey()
			users[i].PasswordSecretName = u.PasswordSecretRef.Name
		}
	}
	return users
}

// AgentCertificateSecretNamespacedName returns the namespaced name of the secret containing the agent certificate.
func (m *MongoDBCommunity) AgentCertificateSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.Namespace,
		Name:      m.Spec.GetAgentCertificateRef(),
	}
}

// AgentCertificatePemSecretNamespacedName returns the namespaced name of the secret containing the agent certificate in pem format.
func (m *MongoDBCommunity) AgentCertificatePemSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: m.Namespace,
		Name:      m.Spec.GetAgentCertificateRef() + "-pem",
	}
}

// GetAgentCertificateRef returns the name of the secret containing the agent certificate.
// If it is specified in the CR, it will return this. Otherwise, it default to agent-certs.
func (m *MongoDBCommunitySpec) GetAgentCertificateRef() string {
	agentCertSecret := "agent-certs"
	if m.Security.Authentication.AgentCertificateSecret != nil && m.Security.Authentication.AgentCertificateSecret.Name != "" {
		agentCertSecret = m.Security.Authentication.AgentCertificateSecret.Name
	}
	return agentCertSecret
}

// GetAgentAuthMode return the agent authentication mode. If the agent auth mode is specified, it will return this.
// Otherwise, if the spec.security.authentication.modes array is empty, it will default to SCRAM-SHA-256.
// If spec.security.authentication.modes has one element, the agent auth mode will default to that.
// If spec.security.authentication.modes has more than one element, then agent auth will need to be specified,
// with one exception: if spec.security.authentication.modes contains only SCRAM-SHA-256 and SCRAM-SHA-1, then it defaults to SCRAM-SHA-256 (for backwards compatibility).
func (m *MongoDBCommunitySpec) GetAgentAuthMode() AuthMode {
	if m.Security.Authentication.AgentMode != "" {
		return m.Security.Authentication.AgentMode
	}

	if len(m.Security.Authentication.Modes) == 0 {
		return "SCRAM-SHA-256"
	} else if len(m.Security.Authentication.Modes) == 1 {
		return m.Security.Authentication.Modes[0]
	} else if len(m.Security.Authentication.Modes) == 2 {
		if (IsAuthPresent(m.Security.Authentication.Modes, "SCRAM") || IsAuthPresent(m.Security.Authentication.Modes, "SCRAM-SHA-256")) &&
			IsAuthPresent(m.Security.Authentication.Modes, "SCRAM-SHA-1") {
			return "SCRAM-SHA-256"
		}
	}
	return ""
}

func (m *MongoDBCommunitySpec) IsAgentX509() bool {
	return m.GetAgentAuthMode() == "X509"
}

// IsStillScaling returns true if this resource is currently scaling,
// considering both arbiters and regular members.
func (m *MongoDBCommunity) IsStillScaling() bool {
	arbiters := automationConfigReplicasScaler{
		current:                m.CurrentArbiters(),
		desired:                m.DesiredArbiters(),
		forceIndividualScaling: true,
	}

	return scale.IsStillScaling(m) || scale.IsStillScaling(arbiters)
}

// AutomationConfigMembersThisReconciliation determines the correct number of
// automation config replica set members based on our desired number, and our
// current number.
func (m *MongoDBCommunity) AutomationConfigMembersThisReconciliation() int {
	return scale.ReplicasThisReconciliation(automationConfigReplicasScaler{
		current: m.Status.CurrentMongoDBMembers,
		desired: m.Spec.Members,
	})
}

// AutomationConfigArbitersThisReconciliation determines the correct number of
// automation config replica set arbiters based on our desired number, and our
// current number.
//
// Will not update arbiters until members have reached desired number.
func (m *MongoDBCommunity) AutomationConfigArbitersThisReconciliation() int {
	if scale.IsStillScaling(m) {
		return m.Status.CurrentMongoDBArbiters
	}

	return scale.ReplicasThisReconciliation(automationConfigReplicasScaler{
		desired:                m.Spec.Arbiters,
		current:                m.Status.CurrentMongoDBArbiters,
		forceIndividualScaling: true,
	})
}

// GetOptionsString return a string format of the connection string
// options that can be appended directly to the connection string.
//
// Only takes into account options for the resource and not any user.
func (m *MongoDBCommunity) GetOptionsString() string {
	generalOptionsMap := m.Spec.AdditionalConnectionStringConfig.Object
	optionValues := make([]string, len(generalOptionsMap))
	i := 0

	for key, value := range generalOptionsMap {
		if _, protected := protectedConnectionStringOptions[key]; !protected {
			optionValues[i] = fmt.Sprintf("%s=%v", key, value)
			i += 1
		}
	}

	optionValues = optionValues[:i]

	optionsString := ""
	if i > 0 {
		optionsString = "&" + strings.Join(optionValues, "&")
	}
	return optionsString
}

// GetUserOptionsString return a string format of the connection string
// options that can be appended directly to the connection string.
//
// Takes into account both user options and resource options.
// User options will override any existing options in the resource.
func (m *MongoDBCommunity) GetUserOptionsString(user authtypes.User) string {
	generalOptionsMap := m.Spec.AdditionalConnectionStringConfig.Object
	userOptionsMap := user.ConnectionStringOptions
	optionValues := make([]string, len(generalOptionsMap)+len(userOptionsMap))
	i := 0
	for key, value := range userOptionsMap {
		if _, protected := protectedConnectionStringOptions[key]; !protected {
			optionValues[i] = fmt.Sprintf("%s=%v", key, value)
			i += 1
		}
	}

	for key, value := range generalOptionsMap {
		_, ok := userOptionsMap[key]
		if _, protected := protectedConnectionStringOptions[key]; !ok && !protected {
			optionValues[i] = fmt.Sprintf("%s=%v", key, value)
			i += 1
		}
	}

	optionValues = optionValues[:i]

	optionsString := ""
	if i > 0 {
		optionsString = "&" + strings.Join(optionValues, "&")
	}
	return optionsString
}

// MongoURI returns a mongo uri which can be used to connect to this deployment
func (m *MongoDBCommunity) MongoURI(clusterDomain string) string {
	optionsString := m.GetOptionsString()

	return fmt.Sprintf("mongodb://%s/?replicaSet=%s%s", strings.Join(m.Hosts(clusterDomain), ","), m.Name, optionsString)
}

// MongoSRVURI returns a mongo srv uri which can be used to connect to this deployment
func (m *MongoDBCommunity) MongoSRVURI(clusterDomain string) string {
	if clusterDomain == "" {
		clusterDomain = defaultClusterDomain
	}

	optionsString := m.GetOptionsString()

	return fmt.Sprintf("mongodb+srv://%s.%s.svc.%s/?replicaSet=%s%s", m.ServiceName(), m.Namespace, clusterDomain, m.Name, optionsString)
}

// MongoAuthUserURI returns a mongo uri which can be used to connect to this deployment
// and includes the authentication data for the user
func (m *MongoDBCommunity) MongoAuthUserURI(user authtypes.User, password string, clusterDomain string) string {
	optionsString := m.GetUserOptionsString(user)
	return fmt.Sprintf("mongodb://%s%s/%s?replicaSet=%s&ssl=%t%s",
		user.GetLoginString(password),
		strings.Join(m.Hosts(clusterDomain), ","),
		user.Database,
		m.Name,
		m.Spec.Security.TLS.Enabled,
		optionsString)
}

// MongoAuthUserSRVURI returns a mongo srv uri which can be used to connect to this deployment
// and includes the authentication data for the user
func (m *MongoDBCommunity) MongoAuthUserSRVURI(user authtypes.User, password string, clusterDomain string) string {
	if clusterDomain == "" {
		clusterDomain = defaultClusterDomain
	}

	optionsString := m.GetUserOptionsString(user)
	return fmt.Sprintf("mongodb+srv://%s%s.%s.svc.%s/%s?replicaSet=%s&ssl=%t%s",
		user.GetLoginString(password),
		m.ServiceName(),
		m.Namespace,
		clusterDomain,
		user.Database,
		m.Name,
		m.Spec.Security.TLS.Enabled,
		optionsString)
}

func (m *MongoDBCommunity) Hosts(clusterDomain string) []string {
	hosts := make([]string, m.Spec.Members)

	if clusterDomain == "" {
		clusterDomain = defaultClusterDomain
	}

	for i := 0; i < m.Spec.Members; i++ {
		hosts[i] = fmt.Sprintf("%s-%d.%s.%s.svc.%s:%d",
			m.Name, i,
			m.ServiceName(),
			m.Namespace,
			clusterDomain,
			m.GetMongodConfiguration().GetDBPort())
	}
	return hosts
}

// ServiceName returns the name of the Service that should be created for this resource.
func (m *MongoDBCommunity) ServiceName() string {
	serviceName := m.Spec.StatefulSetConfiguration.SpecWrapper.Spec.ServiceName
	if serviceName != "" {
		return serviceName
	}
	return m.Name + "-svc"
}

func (m *MongoDBCommunity) ArbiterNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: m.Namespace, Name: m.Name + "-arb"}
}

func (m *MongoDBCommunity) AutomationConfigSecretName() string {
	return m.Name + "-config"
}

// TLSCaCertificateSecretNamespacedName will get the namespaced name of the Secret containing the CA certificate
// As the Secret will be mounted to our pods, it has to be in the same namespace as the MongoDB resource
func (m *MongoDBCommunity) TLSCaCertificateSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Security.TLS.CaCertificateSecret.Name, Namespace: m.Namespace}
}

// TLSConfigMapNamespacedName will get the namespaced name of the ConfigMap containing the CA certificate
// As the ConfigMap will be mounted to our pods, it has to be in the same namespace as the MongoDB resource
func (m *MongoDBCommunity) TLSConfigMapNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Security.TLS.CaConfigMap.Name, Namespace: m.Namespace}
}

// TLSSecretNamespacedName will get the namespaced name of the Secret containing the server certificate and key
func (m *MongoDBCommunity) TLSSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Security.TLS.CertificateKeySecret.Name, Namespace: m.Namespace}
}

// PrometheusTLSSecretNamespacedName will get the namespaced name of the Secret containing the server certificate and key
func (m *MongoDBCommunity) PrometheusTLSSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Prometheus.TLSSecretRef.Name, Namespace: m.Namespace}
}

func (m *MongoDBCommunity) TLSOperatorCASecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-ca-certificate", Namespace: m.Namespace}
}

// TLSOperatorSecretNamespacedName will get the namespaced name of the Secret created by the operator
// containing the combined certificate and key.
func (m *MongoDBCommunity) TLSOperatorSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-server-certificate-key", Namespace: m.Namespace}
}

// PrometheusTLSOperatorSecretNamespacedName will get the namespaced name of the Secret created by the operator
// containing the combined certificate and key.
func (m *MongoDBCommunity) PrometheusTLSOperatorSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-prometheus-certificate-key", Namespace: m.Namespace}
}

func (m *MongoDBCommunity) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name, Namespace: m.Namespace}
}

func (m *MongoDBCommunity) DesiredReplicas() int {
	return m.Spec.Members
}

func (m *MongoDBCommunity) CurrentReplicas() int {
	return m.Status.CurrentStatefulSetReplicas
}

// ForcedIndividualScaling if set to true, will always scale the deployment 1 by
// 1, even if the resource has been just created.
//
// The reason for this is that we have 2 types of resources that are scaled at
// different times: a) Regular members, which can be scaled from 0->n, for
// instance, when the resource was just created; and b) Arbiters, which will be
// scaled from 0->M 1 by 1 at all times.
//
// This was done to simplify the process of scaling arbiters, *after* members
// have reached the desired amount of replicas.
func (m *MongoDBCommunity) ForcedIndividualScaling() bool {
	return false
}

func (m *MongoDBCommunity) DesiredArbiters() int {
	return m.Spec.Arbiters
}

func (m *MongoDBCommunity) CurrentArbiters() int {
	return m.Status.CurrentStatefulSetArbitersReplicas
}

func (m *MongoDBCommunity) GetMongoDBVersion() string {
	return m.Spec.Version
}

// GetMongoDBVersionForAnnotation returns the MDB version used to annotate the object.
// Here it's the same as GetMongoDBVersion, but a different name is used in order to make
// the usage clearer in enterprise (where it's a method of OpsManager but is used for the AppDB)
func (m *MongoDBCommunity) GetMongoDBVersionForAnnotation() string {
	return m.GetMongoDBVersion()
}

func (m *MongoDBCommunity) StatefulSetReplicasThisReconciliation() int {
	return scale.ReplicasThisReconciliation(automationConfigReplicasScaler{
		desired:                m.DesiredReplicas(),
		current:                m.CurrentReplicas(),
		forceIndividualScaling: false,
	})
}

func (m *MongoDBCommunity) StatefulSetArbitersThisReconciliation() int {
	return scale.ReplicasThisReconciliation(automationConfigReplicasScaler{
		desired:                m.DesiredArbiters(),
		current:                m.CurrentArbiters(),
		forceIndividualScaling: true,
	})
}

// GetUpdateStrategyType returns the type of RollingUpgradeStrategy that the
// MongoDB StatefulSet should be configured with.
func (m *MongoDBCommunity) GetUpdateStrategyType() appsv1.StatefulSetUpdateStrategyType {
	if !m.IsChangingVersion() {
		return appsv1.RollingUpdateStatefulSetStrategyType
	}
	return appsv1.OnDeleteStatefulSetStrategyType
}

// IsChangingVersion returns true if an attempted version change is occurring.
func (m *MongoDBCommunity) IsChangingVersion() bool {
	lastVersion := m.getLastVersion()
	return lastVersion != "" && lastVersion != m.Spec.Version
}

// GetLastVersion returns the MDB version the statefulset was configured with.
func (m *MongoDBCommunity) getLastVersion() string {
	return annotations.GetAnnotation(m, annotations.LastAppliedMongoDBVersion)
}

func (m *MongoDBCommunity) HasSeparateDataAndLogsVolumes() bool {
	return true
}

func (m *MongoDBCommunity) GetAnnotations() map[string]string {
	return m.Annotations
}

func (m *MongoDBCommunity) DataVolumeName() string {
	return "data-volume"
}

func (m *MongoDBCommunity) LogsVolumeName() string {
	return "logs-volume"
}

func (m *MongoDBCommunity) NeedsAutomationConfigVolume() bool {
	return true
}

func (m MongoDBCommunity) GetAgentLogLevel() LogLevel {
	return m.Spec.AgentConfiguration.LogLevel
}

func (m MongoDBCommunity) GetAgentLogFile() string {
	return m.Spec.AgentConfiguration.LogFile
}

func (m MongoDBCommunity) GetAgentMaxLogFileDurationHours() int {
	return m.Spec.AgentConfiguration.MaxLogFileDurationHours
}

type automationConfigReplicasScaler struct {
	current, desired       int
	forceIndividualScaling bool
}

func (a automationConfigReplicasScaler) DesiredReplicas() int {
	return a.desired
}

func (a automationConfigReplicasScaler) CurrentReplicas() int {
	return a.current
}

func (a automationConfigReplicasScaler) ForcedIndividualScaling() bool {
	return a.forceIndividualScaling
}

// +kubebuilder:object:root=true

// MongoDBCommunityList contains a list of MongoDB
type MongoDBCommunityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MongoDBCommunity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MongoDBCommunity{}, &MongoDBCommunityList{})
}
