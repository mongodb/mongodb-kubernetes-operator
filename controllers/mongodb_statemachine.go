package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"time"

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
	stateMachineAnnotation          = "mongodb.com/v1.stateMachine"
	completeAnnotation              = "complete"
	startFreshStateName             = "StartFresh"
	validateSpecStateName           = "ValidateSpec"
	createServiceStateName          = "CreateService"
	tlsValidationStateName          = "TLSValidation"
	tlsResourcesStateName           = "CreateTLSResources"
	deployAutomationConfigStateName = "DeployAutomationConfig"
	deployStatefulSetStateName      = "DeployStatefulSet"

	noCondition = func() (bool, error) { return true, nil }
)

//nolint
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) *state.Machine {
	sm := state.NewStateMachine(&MongoDBCommunityCompleter{
		nsName: mdb.NamespacedName(),
		client: client,
	}, log)

	startFresh := NewStartFreshState(client, mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)
	tlsValidationState := NewTLSValidationState(client, mdb, secretWatcher, log)
	tlsResourcesState := NewEnsureTLSResourcesState(client, mdb, log)
	deployAutomationConfigState := NewDeployAutomationConfigState(client, mdb, log)
	deployStatefulSetState := NewDeployStatefulSetState(client, mdb, log)

	sm.AddTransition(startFresh, validateSpec, noCondition)
	sm.AddTransition(validateSpec, serviceState, noCondition)

	sm.AddTransition(serviceState, tlsValidationState, func() (bool, error) {
		// we only need to validate TLS if it is enabled in the resource
		return mdb.Spec.Security.TLS.Enabled, nil
	})

	sm.AddTransition(tlsValidationState, tlsResourcesState, noCondition)

	sm.AddTransition(tlsResourcesState, deployAutomationConfigState, func() (bool, error) {
		return needToPublishStateFirst(client, mdb, log), nil
	})
	sm.AddTransition(tlsResourcesState, deployStatefulSetState, func() (bool, error) {
		return !needToPublishStateFirst(client, mdb, log), nil
	})

	sm.AddTransition(serviceState, deployAutomationConfigState, func() (bool, error) {
		return needToPublishStateFirst(client, mdb, log), nil
	})
	sm.AddTransition(serviceState, deployStatefulSetState, func() (bool, error) {
		return !needToPublishStateFirst(client, mdb, log), nil
	})

	sm.AddTransition(deployStatefulSetState, deployAutomationConfigState, noCondition)
	sm.AddTransition(deployAutomationConfigState, deployStatefulSetState, noCondition)

	return sm
}

func NewStartFreshState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: startFreshStateName,
		Reconcile: func() (reconcile.Result, error) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status, "MongoDB.Annotations", mdb.ObjectMeta.Annotations)
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, startFreshStateName),
	}
}

func NewValidateSpecState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: validateSpecStateName,
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
		OnCompletion: updateCompletionAnnotation(client, mdb, validateSpecStateName),
	}
}

func NewCreateServiceState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: createServiceStateName,
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
		OnCompletion: updateCompletionAnnotation(client, mdb, createServiceStateName),
	}
}

func NewEnsureTLSResourcesState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsResourcesStateName,
		Reconcile: func() (reconcile.Result, error) {
			if err := ensureTLSResources(client, mdb, log); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring TLS resources: %s", err)).
						withFailedPhase(),
				)
			}
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, tlsResourcesStateName),
	}
}
func NewDeployAutomationConfigState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployAutomationConfigStateName,
		Reconcile: func() (reconcile.Result, error) {
			ready, err := deployAutomationConfig(client, mdb, log)
			if err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error deploying Automation Config: %s", err)).
						withFailedPhase(),
				)
			}
			if !ready {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "MongoDB agents are not yet ready, retrying in 10 seconds").
						withPendingPhase(10),
				)
			}
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, deployAutomationConfigStateName),
	}
}

func NewDeployStatefulSetState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployStatefulSetStateName,
		Reconcile: func() (reconcile.Result, error) {
			ready, err := deployStatefulSet(client, mdb, log)
			if err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error deploying MongoDB StatefulSet: %s", err)).
						withFailedPhase(),
				)
			}

			if !ready {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "StatefulSet is not yet ready, retrying in 10 seconds").
						withPendingPhase(10),
				)
			}
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, deployStatefulSetStateName),
	}
}

func NewTLSValidationState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsValidationStateName,
		Reconcile: func() (reconcile.Result, error) {
			isTLSValid, err := validateTLSConfig(client, mdb, secretWatcher, log)
			if err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error validating TLS config: %s", err)).
						withFailedPhase(),
				)
			}

			if !isTLSValid {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "TLS config is not yet valid, retrying in 10 seconds").
						withPendingPhase(10),
				)
			}
			log.Debug("Successfully validated TLS configuration.")
			return result.OK()
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, tlsValidationStateName),
	}
}

func ensureService(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	svc := buildService(mdb)
	err := client.Create(context.TODO(), &svc)

	if err == nil {
		log.Infof("Created service %s/%s", svc.Namespace, svc.Name)
		return nil
	}

	if err != nil && apiErrors.IsAlreadyExists(err) {
		log.Infof("The service already exists... moving forward: %s", err)
		return nil
	}

	return err
}

func newAllStates() state.AllStates {
	return state.AllStates{
		CurrentState: startFreshStateName,
	}
}

func updateCompletionAnnotation(client kubernetesClient.Client, m mdbv1.MongoDBCommunity, stateName string) func() error {
	return func() error {

		time.Sleep(3 * time.Second)

		mdb := mdbv1.MongoDBCommunity{}
		if err := client.Get(context.TODO(), m.NamespacedName(), &mdb); err != nil {
			return err
		}

		allStates, err := getAllStates(mdb)
		if err != nil {
			return err
		}
		allStates.CurrentState = stateName

		if allStates.StateCompletionStatus == nil {
			allStates.StateCompletionStatus = map[string]string{}
		}

		allStates.StateCompletionStatus[stateName] = completeAnnotation

		bytes, err := json.Marshal(allStates)
		if err != nil {
			return err
		}
		if mdb.Annotations == nil {
			mdb.Annotations = map[string]string{}
		}
		mdb.Annotations[stateMachineAnnotation] = string(bytes)

		return client.Update(context.TODO(), &mdb)
	}
}
