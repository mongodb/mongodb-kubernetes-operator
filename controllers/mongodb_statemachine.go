package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"go.uber.org/zap"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	stateMachineAnnotation       = "mongodb.com/v1.states/stateMachine"
	completeAnnotation           = "complete"
	lastStateAnnotation          = "mongodb.com/v1.states/lastState"
	startFreshStateAnnotation    = "mongodb.com/v1.states/StartFresh"
	validateSpecStateAnnotation  = "mongodb.com/v1.states/ValidateSpec"
	createServiceStateAnnotation = "mongodb.com/v1.states/CreateService"
	noCondition                  = func() (bool, error) { return true, nil }
)

//nolint
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) *state.Machine {
	sm := state.NewStateMachine(&MongoDBCommunityCompleter{
		nsName: mdb.NamespacedName(),
		client: client,
	}, log)
	startFresh := NewStartFreshState(client, mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)

	sm.AddTransition(startFresh, validateSpec, noCondition)
	sm.AddTransition(validateSpec, serviceState, noCondition)
	return sm
}

func NewStartFreshState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: "StartFresh",
		Reconcile: func() (reconcile.Result, error) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, startFreshStateAnnotation),
	}
}

func NewValidateSpecState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: "ValidateSpec",
		Reconcile: func() (reconcile.Result, error) {
			log.Debug("Validating MongoDB.Spec")
			if err := validateUpdate(mdb); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("error validating new Spec: %s", err)).
						withFailedPhase(),
				)
			}
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, validateSpecStateAnnotation),
	}
}

func NewCreateServiceState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: "CreateService",
		Reconcile: func() (reconcile.Result, error) {
			log.Debug("Ensuring the service exists")
			if err := ensureService(client, mdb, log); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring the service exists: %s", err)).
						withFailedPhase(),
				)
			}
			return result.OK()
			//return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, createServiceStateAnnotation),
	}
}

func ensureService(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	svc := buildService(mdb)
	err := client.Create(context.TODO(), &svc)
	if err != nil && apiErrors.IsAlreadyExists(err) {
		log.Infof("The service already exists... moving forward: %s", err)
		return nil
	}
	return err
}

func newAllStates() state.AllStates {
	return state.AllStates{
		CurrentState: "StartFresh",
	}
}

func updateCompletionAnnotation(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, stateName string) func() error {
	return func() error {
		allStates, err := getAllStates(mdb)
		if err != nil {
			return err
		}
		allStates.CurrentState = stateName
		allStates.StateCompletionStatus[stateName] = completeAnnotation

		bytes, err := json.Marshal(allStates)
		mdb.Annotations[stateMachineAnnotation] = string(bytes)

		return client.Update(context.TODO(), &mdb)
	}
}
