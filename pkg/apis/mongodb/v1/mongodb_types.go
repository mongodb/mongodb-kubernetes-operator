package v1

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Type string

var (
	ReplicaSet Type = "ReplicaSet"
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
}

// MongoDBStatus defines the observed state of MongoDB
type MongoDBStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MongoDB is the Schema for the mongodbs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mongodbs,scope=Namespaced
type MongoDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MongoDBSpec   `json:"spec,omitempty"`
	Status MongoDBStatus `json:"status,omitempty"`
}

// ServiceName returns the name of the Service that should be created for
// this resource
func (m *MongoDB) ServiceName() string {
	return m.Name + "-svc"
}

//func (m MongoDB) BuildAutomationConfig() automationconfig.AutomationConfig {
//	sts := m.BuildStatefulSet()
//	clusterDomain := "svc.cluster.local" // TODO: make configurable
//	hostnames, names := automationconfig.getDnsForStatefulSet(sts, clusterDomain)
//	processes := make([]automationconfig.Process, len(hostnames))
//	wiredTigerCache := automationconfig.CalculateWiredTigerCache(sts, m.Spec.Version)
//	for idx, hostname := range hostnames {
//		processes[idx] = automationconfig.NewProcess(names[idx], hostname, m.Spec.Version, *wiredTigerCache)
//	}
//	return automationconfig.NewBuilder().Build()
//}

// TODO: build the correct statefulset - this is a dummy implementation
// BuildStatefulSet constructs an instance of appsv1.StatefulSet
// which should be created during reconciliation
func (m MongoDB) BuildStatefulSet() appsv1.StatefulSet {
	labels := map[string]string{
		"app": m.ServiceName(),
	}
	replicas := int32(m.Spec.Members)
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      m.Name,
					Namespace: m.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mongo",
							Image: fmt.Sprintf("mongo:%s", m.Spec.Version),
						},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}
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
