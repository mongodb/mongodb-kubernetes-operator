package apis

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, mdbv1.SchemeBuilder.AddToScheme)
}
