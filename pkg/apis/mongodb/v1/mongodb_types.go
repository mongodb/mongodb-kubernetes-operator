package v1

import (
	"fmt"
	"strings"

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
	LastVersionAnnotationKey = "lastVersion"
)

// MongoDBSpec defines the desired state of MongoDB
type MongoDBSpec struct {
	// Members is the number of members in the replica set
	// +optional
	Members int `json:"members"`
	// Type defines which type of MongoDB deployment the resource should create
	Type Type `json:"type"`
	// Version defines which version of MongoDB will be used
	Version string `json:"version"`

	// FeatureCompatibilityVersion configures the feature compatibility version that will
	// be set for the deployment
	// +optional
	FeatureCompatibilityVersion string `json:"featureCompatibilityVersion,omitempty"`
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

func (m MongoDB) ChangingVersion() bool {
	if lastVersion, ok := m.Annotations[LastVersionAnnotationKey]; ok {
		return (m.Spec.Version != lastVersion) && lastVersion != ""
	}
	return false
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
