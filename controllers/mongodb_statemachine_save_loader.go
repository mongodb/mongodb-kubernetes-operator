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

	startingStateName, err := getLastStateName(mdb)
	if err != nil {
		return "", errors.Errorf("error fetching last state name from MongoDBCommunity annotations: %s", err)
	}

	if startingStateName == "" {
		startingStateName = reconciliationStartStateName
	}
	return startingStateName, nil
}

func (r *ReplicaSetReconciler) SaveNextState(nsName types.NamespacedName, stateName string) error {
	if stateName == "" {
		return nil
	}

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

		allStates.NextState = stateName
		allStates.StateHistory = append(allStates.StateHistory, stateName)

		bytes, err := json.Marshal(allStates)
		if err != nil {
			return err
		}

		mdb.Annotations[stateMachineAnnotation] = string(bytes)
		err = r.client.Update(context.TODO(), &mdb)
		if err == nil {
			break
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

func getLastStateName(mdb mdbv1.MongoDBCommunity) (string, error) {
	allStates, err := getExistingStateMachineStatesFromAnnotation(mdb)
	if err != nil {
		return "", err
	}
	return allStates.NextState, nil
}
