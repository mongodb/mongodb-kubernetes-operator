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

const (
	// LastVersionAnnotationKey should indicate which version of MongoDB was last
	// configured
	LastVersionAnnotationKey = "mongodb.com/v1.lastVersion"

	// TLSRolledOutKey indicates if TLS has been fully rolled out
	TLSRolledOutKey = "mongodb.com/v1.tlsRolledOut"
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

	// Security configures security features, such as TLS, for a deployment
	// +optional
	Security Security `json:"security"`
}

type Security struct {
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

	// ServerSecretName is the name of a secret containing a private key and certificate to use for TLS
	// The key and cert are expected to be PEM encoded and available at "tls.key" and "tls.crt"
	// +optional
	ServerSecretName string `json:"serverSecretName"`

	// CAConfigMapName is the name of a ConfigMap containing the certificate for the CA which signed the server certificates
	// The certificate is expected to be available under the key "ca.crt"
	// +optional
	CAConfigMapName string `json:"caConfigMapName"`
}

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

func (m MongoDB) IsChangingVersion() bool {
	if lastVersion, ok := m.Annotations[LastVersionAnnotationKey]; ok {
		return (m.Spec.Version != lastVersion) && lastVersion != ""
	}
	return false
}

// HasRolledOutTLS indicates whether the certs and keys have been mounted to all pods.
// We need to roll out these first before we can enable TLS on the servers.
func (m MongoDB) HasRolledOutTLS() bool {
	_, completedRollOut := m.Annotations[TLSRolledOutKey]
	return completedRollOut
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

// ServiceName returns the name of the Service that should be created for
// this resource
func (m MongoDB) ServiceName() string {
	return m.Name + "-svc"
}

func (m MongoDB) ConfigMapName() string {
	return m.Name + "-config"
}

func (m MongoDB) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: m.Name, Namespace: m.Namespace}
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
