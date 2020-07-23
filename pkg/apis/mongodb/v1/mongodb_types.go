package v1

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Type string

const (
	ReplicaSet Type = "ReplicaSet"
)

type Phase string

const (
	Running Phase = "Running"
)

// MongoDBSpec defines the desired state of MongoDB
type MongoDBSpec struct {
	// Members is the number of members in the replica set
	// +optional
	Members int `json:"members"`
	// Type defines which type of MongoDB deployment the resource should create
	// +kubebuilder:validation:Enum=ReplicaSet
	Type Type `json:"type"`
	// Version defines which version of MongoDB will be used
	Version string `json:"version"`

	// FeatureCompatibilityVersion configures the feature compatibility version that will
	// be set for the deployment
	// +optional
	FeatureCompatibilityVersion string `json:"featureCompatibilityVersion,omitempty"`

	// Security configures security features, such as TLS, and authentication settings for a deployment
	// +optional
	Security Security `json:"security"`

	// Users specifies the MongoDB users that should be configured in your deployment
	// +required
	Users []MongoDBUser `json:"users"`
}

type MongoDBUser struct {
	// Name is the username of the user
	Name string `json:"name"`

	// DB is the database the user is stored in. Defaults to "admin"
	// +optional
	DB string `json:"db"`

	// PasswordSecretRef is a reference to the secret containing this user's password
	PasswordSecretRef SecretKeyReference `json:"passwordSecretRef"`

	// Roles is an array of roles assigned to this user
	Roles []Role `json:"roles"`
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
	// +optional
	CertificateKeySecret LocalObjectReference `json:"certificateKeySecretRef"`

	// CaConfigMap is a reference to a ConfigMap containing the certificate for the CA which signed the server certificates
	// The certificate is expected to be available under the key "ca.crt"
	// +optional
	CaConfigMap LocalObjectReference `json:"caConfigMapRef"`
}

// LocalObjectReference is a reference to another Kubernetes object by name.
// TODO: Replace with a type from the K8s API. CoreV1 has an equivalent
// 	"LocalObjectReference" type but it contains a TODO in its
// 	description that we don't want in our CRD.
type LocalObjectReference struct {
	Name string `json:"name"`
}

type Authentication struct {
	// Enabled specifies if authentication should be enabled
	Enabled bool `json:"enabled"`

	// Modes is an array specifying which authentication methods should be enabled
	Modes []AuthMode `json:"modes"`
}

// +kubebuilder:validation:Enum=SCRAM
type AuthMode string

// MongoDBStatus defines the observed state of MongoDB
type MongoDBStatus struct {
	MongoURI string `json:"mongoUri"`
	Phase    Phase  `json:"phase"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MongoDB is the Schema for the mongodbs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mongodb,scope=Namespaced,shortName=mdb
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Current state of the MongoDB deployment"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Version of MongoDB server"
type MongoDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MongoDBSpec   `json:"spec,omitempty"`
	Status MongoDBStatus `json:"status,omitempty"`
}

func (m *MongoDB) UpdateSuccess() {
	m.Status.MongoURI = m.MongoURI()
	m.Status.Phase = Running
}

// MongoURI returns a mongo uri which can be used to connect to this deployment
func (m MongoDB) MongoURI() string {
	members := make([]string, m.Spec.Members)
	clusterDomain := "svc.cluster.local" // TODO: make this configurable
	for i := 0; i < m.Spec.Members; i++ {
		members[i] = fmt.Sprintf("%s-%d.%s.%s.%s:%d", m.Name, i, m.ServiceName(), m.Namespace, clusterDomain, 27017)
	}
	return fmt.Sprintf("mongodb://%s", strings.Join(members, ","))
}

// TODO: this is a temporary function which will be used in the e2e tests
// which will be removed in the following PR to clean up our mongo client testing
func (m MongoDB) SCRAMMongoURI(username, password string) string {
	members := make([]string, m.Spec.Members)
	clusterDomain := "svc.cluster.local" // TODO: make this configurable
	for i := 0; i < m.Spec.Members; i++ {
		members[i] = fmt.Sprintf("%s-%d.%s.%s.%s:%d", m.Name, i, m.ServiceName(), m.Namespace, clusterDomain, 27017)
	}
	return fmt.Sprintf("mongodb://%s:%s@%s/?authMechanism=SCRAM-SHA-256", username, password, strings.Join(members, ","))
}

// ServiceName returns the name of the Service that should be created for
// this resource
func (m MongoDB) ServiceName() string {
	return m.Name + "-svc"
}

func (m MongoDB) ConfigMapName() string {
	return m.Name + "-config"
}

// TLSConfigMapNamespacedName will get the namespaced name of the ConfigMap containing the CA certificate
// As the ConfigMap will be mounted to our pods, it has to be in the same namespace as the MongoDB resource
func (m MongoDB) TLSConfigMapNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Security.TLS.CaConfigMap.Name, Namespace: m.Namespace}
}

// TLSSecretNamespacedName will get the namespaced name of the Secret containing the server certificate and key
func (m MongoDB) TLSSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Spec.Security.TLS.CertificateKeySecret.Name, Namespace: m.Namespace}
}

// TLSOperatorSecretNamespacedName will get the namespaced name of the Secret created by the operator
// containing the combined certificate and key.
func (m MongoDB) TLSOperatorSecretNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name + "-server-certificate-key", Namespace: m.Namespace}
}

func (m MongoDB) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name, Namespace: m.Namespace}
}

func (m *MongoDB) ScramCredentialsNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: "agent-scram-credentials", Namespace: m.Namespace}
}

// GetFCV returns the feature compatibility version. If no FeatureCompatibilityVersion is specified.
// It uses the major and minor version for whichever version of MongoDB is configured.
func (m MongoDB) GetFCV() string {
	versionToSplit := m.Spec.FeatureCompatibilityVersion
	if versionToSplit == "" {
		versionToSplit = m.Spec.Version
	}
	minorIndex := 1
	parts := strings.Split(versionToSplit, ".")
	return strings.Join(parts[:minorIndex+1], ".")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MongoDBList contains a list of MongoDB
type MongoDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MongoDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MongoDB{}, &MongoDBList{})
}
