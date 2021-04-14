package controllers

import (
	"context"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

func TestOrderOfStates(t *testing.T) {
	mdb := newTestReplicaSet()

	inMemorySaveLoader := state.NewInMemorySaveLoader(startFreshStateName)

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr, func(_ mdbv1.MongoDBCommunity, _ client.Client) state.SaveLoader {
		return inMemorySaveLoader
	})

	assertOrderOfStates(t, r, mdb,
		inMemorySaveLoader,
		startFreshStateName,
		validateSpecStateName,
		createServiceStateName,
		deployAutomationConfigStateName,
		deployStatefulSetStateName,
		updateStatusStateName,
		reconciliationEndStateName,
	)

}

func assertOrderOfStates(t *testing.T, r *ReplicaSetReconciler, mdb mdbv1.MongoDBCommunity, loader *state.InMemorySaveLoader, states ...string) {
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})

	for res.Requeue || res.RequeueAfter > 0 {
		assert.NoError(t, err)
		res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	}

	assert.Equal(t, len(states), len(loader.StateHistory))

	for i, s := range states {
		assert.Equal(t, s, loader.StateHistory[i])
	}
}
