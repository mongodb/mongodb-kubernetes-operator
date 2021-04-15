package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	stateMachineAnnotation = "mongodb.com/v1.stateMachine"

	reconciliationStartStateName            = "ReconciliationStart"
	validateSpecStateName                   = "ValidateSpec"
	createServiceStateName                  = "CreateService"
	tlsValidationStateName                  = "TLSValidation"
	tlsResourcesStateName                   = "CreateTLSResources"
	deployMongoDBReplicaSetStartName        = "DeployMongoDBReplicaSetStart"
	deployMongoDBReplicaSetEndName          = "DeployMongoDBReplicaSetEnd"
	deployAutomationConfigStateName         = "DeployAutomationConfig"
	deployStatefulSetStateName              = "DeployStatefulSet"
	resetStatefulSetUpdateStrategyStateName = "ResetStatefulSetUpdateStrategy"
	reconciliationEndStateName              = "ReconciliationEnd"
	updateStatusStateName                   = "UpdateStatus"
	secondsBetweenStates                    = 1
)

type MongoDBStates struct {
	NextState    string   `json:"nextState"`
	StateHistory []string `json:"stateHistory"`
}

// BuildStateMachine
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, savLoader state.SaveLoader, log *zap.SugaredLogger) (*state.Machine, error) {
	sm := state.NewStateMachine(savLoader, mdb.NamespacedName(), log)

	needsToPublishStateFirst, reason := needToPublishStateFirst(client, mdb)

	startFresh := NewReconciliationStartState(client, mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)
	tlsValidationState := NewTLSValidationState(client, mdb, secretWatcher, log)
	tlsResourcesState := NewEnsureTLSResourcesState(client, mdb, log)
	deployMongoDBReplicaSetStart := NewDeployMongoDBReplicaSetStartState(mdb, log)
	deployMongoDBReplicaSetEnd := NewDeployMongoDBReplicaSetEndState(mdb, log)
	deployAutomationConfigState := NewDeployAutomationConfigState(client, reason, mdb, log)
	deployStatefulSetState := NewDeployStatefulSetState(client, reason, mdb, log)
	resetUpdateStrategyState := NewResetStatefulSetUpdateStrategyState(client, mdb)
	updateStatusState := NewUpdateStatusState(client, mdb, log)
	endState := NewReconciliationEndState(client, mdb, log)

	sm.AddTransition(startFresh, validateSpec, state.DirectTransition)

	sm.AddTransition(validateSpec, serviceState, state.DirectTransition)
	sm.AddTransition(validateSpec, tlsValidationState, state.FromBool(mdb.Spec.Security.TLS.Enabled))
	sm.AddTransition(validateSpec, deployMongoDBReplicaSetStart, state.DirectTransition)

	sm.AddTransition(serviceState, tlsValidationState, state.FromBool(mdb.Spec.Security.TLS.Enabled))
	sm.AddTransition(serviceState, deployMongoDBReplicaSetStart, state.DirectTransition)

	sm.AddTransition(tlsValidationState, tlsResourcesState, state.DirectTransition)

	sm.AddTransition(tlsResourcesState, deployMongoDBReplicaSetStart, state.DirectTransition)

	sm.AddTransition(deployMongoDBReplicaSetStart, deployAutomationConfigState, state.FromBool(needsToPublishStateFirst))
	sm.AddTransition(deployMongoDBReplicaSetStart, deployStatefulSetState, state.FromBool(!needsToPublishStateFirst))

	sm.AddTransition(deployStatefulSetState, deployAutomationConfigState, state.FromBool(!needsToPublishStateFirst))
	sm.AddTransition(deployStatefulSetState, deployMongoDBReplicaSetEnd, state.DirectTransition)

	sm.AddTransition(deployMongoDBReplicaSetEnd, resetUpdateStrategyState, mdb.IsChangingVersion)
	sm.AddTransition(deployMongoDBReplicaSetEnd, updateStatusState, state.DirectTransition)

	sm.AddTransition(deployAutomationConfigState, deployStatefulSetState, state.FromBool(needsToPublishStateFirst))
	sm.AddTransition(deployAutomationConfigState, deployMongoDBReplicaSetEnd, state.DirectTransition)

	sm.AddTransition(resetUpdateStrategyState, updateStatusState, state.DirectTransition)

	// if we're scaling, we should go back and update the StatefulSet/AutomationConfig again.
	sm.AddTransition(updateStatusState, deployMongoDBReplicaSetStart, state.FromBool(scale.IsStillScaling(&mdb)))
	sm.AddTransition(updateStatusState, endState, state.DirectTransition)
	return sm, nil
}

func NewReconciliationStartState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: reconciliationStartStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := resetStateMachineHistory(client, mdb); err != nil {
				log.Errorf("Failed resetting StateMachine annotation: %s", err)
				return result.RetryState(secondsBetweenStates)
			}

			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewDeployMongoDBReplicaSetStartState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name:            deployMongoDBReplicaSetStartName,
		IsStateGrouping: true,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Infof("Deploying MongoDB ReplicaSet %s/%s", mdb.Namespace, mdb.Name)
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewDeployMongoDBReplicaSetEndState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name:            deployMongoDBReplicaSetEndName,
		IsStateGrouping: true,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Infof("Finished deploying MongoDB ReplicaSet %s/%s", mdb.Namespace, mdb.Name)
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewValidateSpecState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: validateSpecStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := validateUpdate(mdb); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("error validating new Spec: %s", err)).
						withFailedPhase(),
				)
			}
			log.Debug("MongoDB.Spec was successfully validated.")
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewResetStatefulSetUpdateStrategyState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity) state.State {
	return state.State{
		Name: resetStatefulSetUpdateStrategyStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := statefulset.ResetUpdateStrategy(&mdb, client); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
						withFailedPhase(),
				)
			}
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewUpdateStatusState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: updateStatusStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if scale.IsStillScaling(mdb) {

				// TODO: support pending phase also being a success
				// in terms of a state being successful.
				res, err, _ := status.Update(client.Status(), &mdb, statusOptions().
					withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
					withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
						mdb.CurrentReplicas(), mdb.DesiredReplicas())).
					withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
					withPendingPhase(10),
				)
				if err != nil {
					log.Errorf("Error updating the status of the MongoDB resource: %s", err)
					return res, err, false
				}
				return res, err, true
			}

			res, err, _ := status.Update(client.Status(), &mdb,
				statusOptions().
					withMongoURI(mdb.MongoURI()).
					withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
					withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
					withMessage(None, "").
					withRunningPhase(),
			)
			if err != nil {
				log.Errorf("Error updating the status of the MongoDB resource: %s", err)
				return res, err, false
			}

			// the last version will be duplicated in two annotations.
			// This is needed to reuse the update strategy logic in enterprise
			if err := annotations.UpdateLastAppliedMongoDBVersion(&mdb, client); err != nil {
				log.Errorf("Could not save current version as an annotation: %s", err)
			}
			if err := updateLastSuccessfulConfiguration(client, mdb); err != nil {
				log.Errorf("Could not save current spec as an annotation: %s", err)
			}

			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewCreateServiceState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: createServiceStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Debug("Ensuring the service exists")
			if err := ensureService(client, mdb, log); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring the service exists: %s", err)).
						withFailedPhase(),
				)
			}
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewEnsureTLSResourcesState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsResourcesStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := ensureTLSResources(client, mdb, log); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring TLS resources: %s", err)).
						withFailedPhase(),
				)
			}
			return result.StateComplete(secondsBetweenStates)
		},
	}
}
func NewDeployAutomationConfigState(client kubernetesClient.Client, reason string, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployAutomationConfigStateName,
		OnEnter: func() error {
			if reason != "" {
				log.Debug(reason)
			}
			return nil
		},
		Reconcile: func() (reconcile.Result, error, bool) {
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
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewDeployStatefulSetState(client kubernetesClient.Client, reason string, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployStatefulSetStateName,
		OnEnter: func() error {
			if reason != "" {
				log.Debug(reason)
			}
			return nil
		},
		Reconcile: func() (reconcile.Result, error, bool) {
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
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

func NewReconciliationEndState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: reconciliationEndStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := setNextState(client, mdb, reconciliationStartStateName); err != nil {
				log.Errorf("Failed resetting State Machine annotation: %s", err)
				return result.RetryState(1)
			}
			log.Infow("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status:", mdb.Status)
			return result.OK()
		},
	}
}

func NewTLSValidationState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsValidationStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
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
			return result.StateComplete(secondsBetweenStates)
		},
	}
}

// newStartingStates returns a MongoDBStates instance which will cause the State Machine
// to transition to the first state.
func newStartingStates() MongoDBStates {
	return MongoDBStates{
		NextState: reconciliationStartStateName,
	}
}

// setNextState updates the StateMachine annotation to indicate that it should transition to the
// given state next.
func setNextState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, stateName string) error {
	allStates, err := getExistingStateMachineStatesFromAnnotation(mdb)
	if err != nil {
		return err
	}
	allStates.NextState = stateName

	bytes, err := json.Marshal(allStates)
	if err != nil {
		return err
	}

	annotations.SetAnnotation(&mdb, stateMachineAnnotation, string(bytes))

	if err := client.Update(context.TODO(), &mdb); err != nil {
		return err
	}
	return nil
}

// resetStateMachineHistory resets the history of states that have occurred. This should happen
// at the beginning of the first reconiliation.
func resetStateMachineHistory(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity) error {
	allStates, err := getExistingStateMachineStatesFromAnnotation(mdb)
	if err != nil {
		return err
	}
	allStates.StateHistory = []string{}

	bytes, err := json.Marshal(allStates)
	if err != nil {
		return err
	}

	annotations.SetAnnotation(&mdb, stateMachineAnnotation, string(bytes))

	if err := client.Update(context.TODO(), &mdb); err != nil {
		return err
	}
	return nil
}
