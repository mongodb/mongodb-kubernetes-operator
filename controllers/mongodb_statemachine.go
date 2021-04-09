package controllers

import (
	"context"
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

//nolint
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) *state.Machine {
	sm := state.NewStateMachine()
	startFresh := NewStartFreshState(mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)

	sm.AddTransition(startFresh, validateSpec, func() (bool, error) {
		return true, nil
	})

	sm.AddTransition(validateSpec, serviceState, func() (bool, error) {
		return true, nil
	})

	return sm
}

func NewStartFreshState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: "StartFresh",
		Reconcile: func() (reconcile.Result, error) {
			log = zap.S().With("ReplicaSet", mdb.Namespace)
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)
			return result.Retry(0)
		},
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
			return result.Retry(0)
		},
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
