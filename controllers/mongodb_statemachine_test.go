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

func TestOrderOfStates_NoTLS(t *testing.T) {
	mdb := newTestReplicaSet()

	inMemorySaveLoader := state.NewInMemorySaveLoader(startFreshStateName)

	mgr := client.NewManager(&mdb)
	r := NewReconciler(mgr, func(_ mdbv1.MongoDBCommunity, _ client.Client) state.SaveLoader {
		return inMemorySaveLoader
	})

	assertFullOrderOfStates(t, r, mdb,
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

func TestFullOrderOfStates_TLSEnabled(t *testing.T) {
	mdb := newTestReplicaSetWithTLS()

	inMemorySaveLoader := state.NewInMemorySaveLoader(startFreshStateName)

	mgr := client.NewManager(&mdb)

	err := createTLSSecretAndConfigMap(mgr.GetClient(), mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr, func(_ mdbv1.MongoDBCommunity, _ client.Client) state.SaveLoader {
		return inMemorySaveLoader
	})

	// if the StatefulSet does not exist, the automation config should be updated first.
	assertFullOrderOfStates(t, r, mdb,
		inMemorySaveLoader,
		startFreshStateName,
		validateSpecStateName,
		createServiceStateName,
		tlsValidationStateName,
		tlsResourcesStateName,
		deployAutomationConfigStateName,
		deployStatefulSetStateName,
		updateStatusStateName,
		reconciliationEndStateName,
	)

}

func TestPartialOrderOfStates_TLSEnabled(t *testing.T) {
	mdb := newTestReplicaSetWithTLS()

	inMemorySaveLoader := state.NewInMemorySaveLoader(startFreshStateName)

	mgr := client.NewManager(&mdb)

	err := createTLSSecretAndConfigMap(mgr.GetClient(), mdb)
	assert.NoError(t, err)

	r := NewReconciler(mgr, func(_ mdbv1.MongoDBCommunity, _ client.Client) state.SaveLoader {
		return inMemorySaveLoader
	})

	// if the StatefulSet does not exist, the automation config should be updated first.
	assertPartialOrderOfStates(t, r, mdb,
		inMemorySaveLoader,
		deployAutomationConfigStateName,
		deployStatefulSetStateName,
	)

	inMemorySaveLoader.Reset()
	// Once the StatefulSet exists, the stateful set should be updated first
	assertPartialOrderOfStates(t, r, mdb,
		inMemorySaveLoader,
		deployStatefulSetStateName,
		deployAutomationConfigStateName,
	)

}

func reconcileThroughAllStates(t *testing.T, r *ReplicaSetReconciler, mdb mdbv1.MongoDBCommunity) {
	res, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})

	for res.Requeue || res.RequeueAfter > 0 {
		assert.NoError(t, err)
		res, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: mdb.Namespace, Name: mdb.Name}})
	}
}

// assertFullOrderOfStates asserts that the order of states traversed is exactly as specfied.
func assertFullOrderOfStates(t *testing.T, r *ReplicaSetReconciler, mdb mdbv1.MongoDBCommunity, loader *state.InMemorySaveLoader, states ...string) {
	reconcileThroughAllStates(t, r, mdb)
	assert.Equal(t, len(states), len(loader.StateHistory))
	assert.Equal(t, states, loader.StateHistory)
}

// assertPartialOrderOfStates assert whether or not the subset of states exists in that order in the full history of states traversed.
func assertPartialOrderOfStates(t *testing.T, r *ReplicaSetReconciler, mdb mdbv1.MongoDBCommunity, loader *state.InMemorySaveLoader, subsetOfStates ...string) {
	reconcileThroughAllStates(t, r, mdb)
	startingIndex := -1
	for i, historyState := range loader.StateHistory {
		if historyState == subsetOfStates[0] {
			startingIndex = i
		}
	}

	if startingIndex == -1 {
		t.Fatalf("Subset %v did not exist within %v", subsetOfStates, loader.StateHistory)
	}

	var match []string
	for i := startingIndex; i < startingIndex+len(subsetOfStates); i++ {
		// prevent OOB
		if i >= len(loader.StateHistory) {
			break
		}
		match = append(match, loader.StateHistory[i])
	}

	assert.Equal(t, subsetOfStates, match)
}
