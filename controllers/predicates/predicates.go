package predicates

import (
	"reflect"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// OnlyOnSpecChange returns a set of predicates indicating
// that reconciliations should only happen on changes to the Spec of the resource.
// any other changes won't trigger a reconciliation. This allows us to freely update the annotations
// of the resource without triggering unintentional reconciliations.
func OnlyOnSpecChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldResource := e.ObjectOld.(*mdbv1.MongoDBCommunity)
			newResource := e.ObjectNew.(*mdbv1.MongoDBCommunity)
			specChanged := !reflect.DeepEqual(oldResource.Spec, newResource.Spec)
			return specChanged
		},
	}
}
