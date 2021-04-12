package controllers

import (
	"context"
	"encoding/json"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type MongoDBCommunityCompleter struct {
	nsName types.NamespacedName
	client k8sClient.Client
}

func (m *MongoDBCommunityCompleter) IsComplete(stateName string) (bool, error) {
	mdb := mdbv1.MongoDBCommunity{}
	err := m.client.Get(context.TODO(), m.nsName, &mdb)
	if err != nil {
		return false, err
	}

	if mdb.Annotations == nil {
		return false, nil
	}

	allStates, err := getAllStates(mdb)
	if err != nil {
		return false, err
	}

	completionStatus := allStates.StateCompletionStatus[stateName]
	return completionStatus == completeAnnotation, nil
}

func (m *MongoDBCommunityCompleter) Complete(stateName string) error {
	mdb := mdbv1.MongoDBCommunity{}
	err := m.client.Get(context.TODO(), m.nsName, &mdb)
	if err != nil {
		return err
	}

	allStates, err := getAllStates(mdb)
	if err != nil {
		return err
	}

	if mdb.Annotations == nil {
		mdb.Annotations = map[string]string{}
	}

	if allStates.StateCompletionStatus == nil {
		allStates.StateCompletionStatus = map[string]string{}
	}

	allStates.StateCompletionStatus[stateName] = completeAnnotation
	allStates.CurrentState = stateName

	bytes, err := json.Marshal(allStates)
	if err != nil {
		return err
	}

	mdb.Annotations[stateMachineAnnotation] = string(bytes)
	return m.client.Update(context.TODO(), &mdb)
}

func getAllStates(mdb mdbv1.MongoDBCommunity) (state.AllStates, error) {
	if mdb.Annotations == nil {
		return newAllStates(), nil
	}

	stateAnnotation, ok := mdb.Annotations[stateMachineAnnotation]
	if !ok {
		return newAllStates(), nil
	}

	allStates := state.AllStates{}
	if err := json.Unmarshal([]byte(stateAnnotation), &allStates); err != nil {
		return state.AllStates{}, err
	}
	return allStates, nil
}

func getLastStateName(mdb mdbv1.MongoDBCommunity) (string, error) {
	allStates, err := getAllStates(mdb)
	if err != nil {
		return "", err
	}
	return allStates.CurrentState, nil
}
