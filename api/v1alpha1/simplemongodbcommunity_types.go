package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Phase string

type Environment string

const (
	Running Phase = "Running"
	Failed  Phase = "Failed"
	Pending Phase = "Pending"

	Development Environment = "development"
	Production  Environment = "production"
)

type DataExpectations struct {
	Size resource.Quantity `json:"size,omitempty"`
}

type Expectations struct {
	// +optional
	// +kubebuilder:validation:Optional
	Data DataExpectations `json:"data,omitempty"`
}

// SimpleMongoDBCommunitySpec defines the desired state of SimpleMongoDBCommunity
type SimpleMongoDBCommunitySpec struct {
	// +optional
	// +kubebuilder:validation:Optional
	Environment Environment `json:"environment,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Expectations Expectations `json:"expectations,omitempty"`
}

// SimpleMongoDBCommunityStatus defines the observed state of SimpleMongoDBCommunity
type SimpleMongoDBCommunityStatus struct {
	Message string `json:"spec,message"`
	Phase   `json:"phase"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// SimpleMongoDBCommunity is the Schema for the simplemongodbcommunities API
type SimpleMongoDBCommunity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	// +kubebuilder:validation:Optional
	Spec   SimpleMongoDBCommunitySpec   `json:"spec,omitempty"`
	Status SimpleMongoDBCommunityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// SimpleMongoDBCommunityList contains a list of SimpleMongoDBCommunity
type SimpleMongoDBCommunityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SimpleMongoDBCommunity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SimpleMongoDBCommunity{}, &SimpleMongoDBCommunityList{})
}
