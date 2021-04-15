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
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"go.uber.org/zap"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	stateMachineAnnotation = "mongodb.com/v1.stateMachine"

	completeAnnotation = "complete"

	startFreshStateName                     = "StartFresh"
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
	NextState      string   `json:"nextState"`
	PreviousStates []string `json:"previousStates"`
}

//nolint
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, savLoader state.SaveLoader, log *zap.SugaredLogger) (*state.Machine, error) {
	sm := state.NewStateMachine(savLoader, log)

	startFresh := NewStartFreshState(mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)
	tlsValidationState := NewTLSValidationState(client, mdb, secretWatcher, log)
	tlsResourcesState := NewEnsureTLSResourcesState(client, mdb, log)
	deployMongoDBReplicaSetStart := NewDeployMongoDBReplicaSetStartState(mdb, log)
	deployMongoDBReplicaSetEnd := NewDeployMongoDBReplicaSetEndState(mdb, log)
	deployAutomationConfigState := NewDeployAutomationConfigState(client, mdb, log)
	deployStatefulSetState := NewDeployStatefulSetState(client, mdb, log)
	resetUpdateStrategyState := NewResetStatefulSetUpdateStrategyState(client, mdb)
	updateStatusState := NewUpdateStatusState(client, mdb, log)
	endState := NewReconciliationEndState(client, mdb, log)

	needsToPublishStateFirst := needToPublishStateFirst(client, mdb, log)

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

	sm.AddTransition(updateStatusState, startFresh, state.FromBool(scale.IsStillScaling(&mdb)))
	sm.AddTransition(updateStatusState, endState, state.DirectTransition)
	return sm, nil
}

func NewStartFreshState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: startFreshStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewDeployMongoDBReplicaSetStartState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployMongoDBReplicaSetStartName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Infof("Deploying MongoDB ReplicaSet %s/%s", mdb.Namespace, mdb.Name)
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewDeployMongoDBReplicaSetEndState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployMongoDBReplicaSetEndName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Infof("Finished deploying MongoDB ReplicaSet %s/%s", mdb.Namespace, mdb.Name)
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewValidateSpecState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: validateSpecStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Debug("Validating MongoDB.Spec")
			if err := validateUpdate(mdb); err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("error validating new Spec: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewResetStatefulSetUpdateStrategyState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity) state.State {
	return state.State{
		Name: resetStatefulSetUpdateStrategyStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := statefulset.ResetUpdateStrategy(&mdb, client); err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewUpdateStatusState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: updateStatusStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if scale.IsStillScaling(mdb) {
				res, err := status.Update(client.Status(), &mdb, statusOptions().
					withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
					withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
						mdb.CurrentReplicas(), mdb.DesiredReplicas())).
					withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
					withPendingPhase(10),
				)
				return res, err, false
			}

			res, err := status.Update(client.Status(), &mdb,
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

			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewCreateServiceState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: createServiceStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			log.Debug("Ensuring the service exists")
			if err := ensureService(client, mdb, log); err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring the service exists: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewEnsureTLSResourcesState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsResourcesStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			if err := ensureTLSResources(client, mdb, log); err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error ensuring TLS resources: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}
func NewDeployAutomationConfigState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployAutomationConfigStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			ready, err := deployAutomationConfig(client, mdb, log)
			if err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error deploying Automation Config: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}
			if !ready {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "MongoDB agents are not yet ready, retrying in 10 seconds").
						withPendingPhase(10),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewDeployStatefulSetState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: deployStatefulSetStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			ready, err := deployStatefulSet(client, mdb, log)
			if err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error deploying MongoDB StatefulSet: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}

			if !ready {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "StatefulSet is not yet ready, retrying in 10 seconds").
						withPendingPhase(10),
				)
				return res, err, false
			}
			return state.SuccessfulRetry(secondsBetweenStates)
		},
	}
}

func NewReconciliationEndState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: reconciliationEndStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			allStates := newAllStates()

			bytes, err := json.Marshal(allStates)
			if err != nil {
				log.Errorf("error marshalling states: %s", err)
				return state.FailedRetry(secondsBetweenStates)
			}
			if mdb.Annotations == nil {
				mdb.Annotations = map[string]string{}
			}
			mdb.Annotations[stateMachineAnnotation] = string(bytes)

			if err := client.Update(context.TODO(), &mdb); err != nil {
				log.Errorf("error updating annotations: %s", err)
				return state.FailedRetry(secondsBetweenStates)
			}

			log.Infow("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status:", mdb.Status)
			return state.EndReconciliation()
		},
	}
}

func NewTLSValidationState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsValidationStateName,
		Reconcile: func() (reconcile.Result, error, bool) {
			isTLSValid, err := validateTLSConfig(client, mdb, secretWatcher, log)
			if err != nil {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error validating TLS config: %s", err)).
						withFailedPhase(),
				)
				return res, err, false
			}

			if !isTLSValid {
				res, err := status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Info, "TLS config is not yet valid, retrying in 10 seconds").
						withPendingPhase(10),
				)
				return res, err, false
			}
			log.Debug("Successfully validated TLS configuration.")
			return state.SuccessfulRetry(secondsBetweenStates)
		},
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

func newAllStates() MongoDBStates {
	return MongoDBStates{
		NextState: startFreshStateName,
	}
}
