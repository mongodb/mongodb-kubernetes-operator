package controllers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/apierrors"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

//const (
//	cacheRefreshEnv     = "CACHE_REFRESH_TIME_SEC"
//	defaultCacheRefresh = int(2 * time.Second)
//)

func (r *ReplicaSetReconciler) getMongoDBCommunity(nsName types.NamespacedName) (mdbv1.MongoDBCommunity, error) {
	mdb := mdbv1.MongoDBCommunity{}
	err := r.client.Get(context.TODO(), nsName, &mdb)
	if err != nil {
		return mdbv1.MongoDBCommunity{}, err
	}
	return mdb, nil
}

func (r *ReplicaSetReconciler) LoadNextState(nsName types.NamespacedName) (string, error) {
	mdb, err := r.getMongoDBCommunity(nsName)
	if err != nil {
		return "", err
	}

	nextState, err := getNextState(mdb)
	if err != nil {
		return "", errors.Errorf("error fetching last state name from MongoDBCommunity annotations: %s", err)
	}

	if nextState == "" {
		nextState = reconciliationStartStateName
	}
	return nextState, nil
}

func (r *ReplicaSetReconciler) SaveNextState(nsName types.NamespacedName, stateName string) error {
	if stateName == "" {
		return nil
	}

	time.Sleep(time.Second * 2)

	var err error
	attempts := 3
	for i := 0; i < attempts; i++ {
		mdb, err := r.getMongoDBCommunity(nsName)
		if err != nil {
			return err
		}

		allStates, err := getExistingStateMachineStatesFromAnnotation(mdb)
		if err != nil {
			return err
		}

		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}

		// if the next state already exists within the state history, it means we are creating a cycle within the graph.
		// if this is the case we error out and the state machine will be reset.
		if allStates.ContainsState(stateName) {
			return errors.Errorf("attempting to save state [%s], that has already been completed. State history: %+v", stateName, allStates.StateHistory)
		}

		allStates.NextState = stateName
		allStates.StateHistory = append(allStates.StateHistory, stateName)

		bytes, err := json.Marshal(allStates)
		if err != nil {
			return err
		}

		mdb.Annotations[stateMachineAnnotation] = string(bytes)
		if err := r.client.Update(context.TODO(), &mdb); err == nil {
			return nil
		}

		if apierrors.IsTransientError(err) {
			zap.S().Debugf("Transient error updating the MongoDB resource, retrying in 1 second.")
			time.Sleep(1 * time.Second)
			continue
		}
		return err
	}

	return err

}

// getExistingStateMachineStatesFromAnnotation returns a MongoDBStates from
// on the annotation of the resource.
func getExistingStateMachineStatesFromAnnotation(mdb mdbv1.MongoDBCommunity) (MongoDBStates, error) {
	if mdb.Annotations == nil {
		return newStartingStates(), nil
	}

	stateAnnotation, ok := mdb.Annotations[stateMachineAnnotation]
	if !ok {
		return newStartingStates(), nil
	}

	allStates := MongoDBStates{}
	if err := json.Unmarshal([]byte(stateAnnotation), &allStates); err != nil {
		return MongoDBStates{}, err
	}
	return allStates, nil
}

// getNextState returns the next state that should be executed.
func getNextState(mdb mdbv1.MongoDBCommunity) (string, error) {
	allStates, err := getExistingStateMachineStatesFromAnnotation(mdb)
	if err != nil {
		return "", err
	}
	return allStates.NextState, nil
}
