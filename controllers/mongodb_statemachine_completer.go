package controllers

import (
	"context"
	"encoding/json"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/apierrors"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type MongoDBCommunityStateSaver struct {
	mdb    mdbv1.MongoDBCommunity
	client k8sClient.Client
}

func (m *MongoDBCommunityStateSaver) LoadNextState() (string, error) {
	startingStateName, err := getLastStateName(m.mdb)
	if err != nil {
		return "", errors.Errorf("error fetching last state name from MongoDBCommunity annotations: %s", err)
	}

	if startingStateName == "" {
		startingStateName = startFreshStateName
	}
	return startingStateName, nil
}

func (m *MongoDBCommunityStateSaver) SaveNextState(stateName string) error {
	if stateName == "" {
		return nil
	}

	var err error
	attempts := 3
	for i := 0; i < attempts; i++ {
		mdb := mdbv1.MongoDBCommunity{}
		if err := m.client.Get(context.TODO(), m.mdb.NamespacedName(), &mdb); err != nil {
			return err
		}

		allStates, err := getAllStates(mdb)
		if err != nil {
			return err
		}

		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}

		allStates.NextState = stateName

		bytes, err := json.Marshal(allStates)
		if err != nil {
			return err
		}

		mdb.Annotations[stateMachineAnnotation] = string(bytes)
		err = m.client.Update(context.TODO(), &mdb)
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

func getAllStates(mdb mdbv1.MongoDBCommunity) (MongoDBStates, error) {
	if mdb.Annotations == nil {
		return newAllStates(), nil
	}

	stateAnnotation, ok := mdb.Annotations[stateMachineAnnotation]
	if !ok {
		return newAllStates(), nil
	}

	allStates := MongoDBStates{}
	if err := json.Unmarshal([]byte(stateAnnotation), &allStates); err != nil {
		return MongoDBStates{}, err
	}
	return allStates, nil
}

func getLastStateName(mdb mdbv1.MongoDBCommunity) (string, error) {
	allStates, err := getAllStates(mdb)
	if err != nil {
		return "", err
	}
	return allStates.NextState, nil
}
