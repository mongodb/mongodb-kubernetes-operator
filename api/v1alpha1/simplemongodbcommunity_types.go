/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SimpleMongoDBCommunitySpec defines the desired state of SimpleMongoDBCommunity
type SimpleMongoDBCommunitySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of SimpleMongoDBCommunity. Edit simplemongodbcommunity_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// SimpleMongoDBCommunityStatus defines the observed state of SimpleMongoDBCommunity
type SimpleMongoDBCommunityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SimpleMongoDBCommunity is the Schema for the simplemongodbcommunities API
type SimpleMongoDBCommunity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
