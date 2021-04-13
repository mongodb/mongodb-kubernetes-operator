package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/annotations"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/scale"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/state"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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
	deployAutomationConfigStateName         = "DeployAutomationConfig"
	deployStatefulSetStateName              = "DeployStatefulSet"
	resetStatefulSetUpdateStrategyStateName = "ResetStatefulSetUpdateStrategy"
	reconciliationEndState                  = "ReconciliationEnd"
	updateStatusState                       = "UpdateStatus"
)

//nolint
func BuildStateMachine(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) (*state.Machine, error) {
	sm := state.NewStateMachine(&MongoDBCommunityStateSaver{
		nsName: mdb.NamespacedName(),
		client: client,
	}, log)

	startFresh := NewStartFreshState(mdb, log)
	validateSpec := NewValidateSpecState(client, mdb, log)
	serviceState := NewCreateServiceState(client, mdb, log)
	tlsValidationState := NewTLSValidationState(client, mdb, secretWatcher, log)
	tlsResourcesState := NewEnsureTLSResourcesState(client, mdb, log)
	deployAutomationConfigState := NewDeployAutomationConfigState(client, mdb, log)
	deployStatefulSetState := NewDeployStatefulSetState(client, mdb, log)
	resetUpdateStrategyState := NewResetStatefulSetUpdateStrategyState(client, mdb)
	updateStatusState := NewUpdateStatusState(client, mdb, log)
	endState := NewReconciliationEndState(client, mdb, log)

	sm.AddTransition(startFresh, validateSpec, state.DirectTransition)
	sm.AddTransition(validateSpec, serviceState, state.DirectTransition)
	sm.AddTransition(validateSpec, tlsValidationState, state.FromBool(mdb.Spec.Security.TLS.Enabled))
	sm.AddTransition(validateSpec, deployAutomationConfigState, func() bool {
		return needToPublishStateFirst(client, mdb, log)
	})
	sm.AddTransition(validateSpec, deployStatefulSetState, func() bool {
		return !needToPublishStateFirst(client, mdb, log)
	})

	sm.AddTransition(serviceState, tlsValidationState, state.FromBool(mdb.Spec.Security.TLS.Enabled))
	sm.AddTransition(serviceState, deployAutomationConfigState, func() bool {
		return needToPublishStateFirst(client, mdb, log)
	})
	sm.AddTransition(serviceState, deployStatefulSetState, func() bool {
		return !needToPublishStateFirst(client, mdb, log)
	})

	sm.AddTransition(tlsValidationState, tlsResourcesState, state.DirectTransition)

	sm.AddTransition(tlsResourcesState, deployAutomationConfigState, func() bool {
		return needToPublishStateFirst(client, mdb, log)
	})
	sm.AddTransition(tlsResourcesState, deployStatefulSetState, func() bool {
		return !needToPublishStateFirst(client, mdb, log)
	})

	sm.AddTransition(deployStatefulSetState, deployAutomationConfigState, func() bool {
		return !needToPublishStateFirst(client, mdb, log)
	})
	// we only need to reset the update strategy if a version change is in progress.
	sm.AddTransition(deployStatefulSetState, resetUpdateStrategyState, mdb.IsChangingVersion)

	sm.AddTransition(deployStatefulSetState, updateStatusState, state.DirectTransition)

	sm.AddTransition(deployAutomationConfigState, deployStatefulSetState, func() bool {
		return needToPublishStateFirst(client, mdb, log)
	})

	sm.AddTransition(deployAutomationConfigState, resetUpdateStrategyState, mdb.IsChangingVersion)

	sm.AddTransition(deployAutomationConfigState, updateStatusState, state.DirectTransition)

	sm.AddTransition(resetUpdateStrategyState, updateStatusState, state.DirectTransition)

	// if we're still scaling, we need to retry until we are at the desired replica count.
	sm.AddTransition(updateStatusState, startFresh, func() bool {
		return scale.IsStillScaling(&mdb)
	})

	sm.AddTransition(updateStatusState, endState, state.DirectTransition)

	startingStateName, err := getLastStateName(mdb)
	if err != nil {
		return nil, errors.Errorf("error fetching last state name from MongoDBCommunity annotations: %s", err)
	}

	if startingStateName == "" {
		startingStateName = startFreshStateName
	}

	startingState, ok := sm.States[startingStateName]
	if !ok {
		return nil, errors.Errorf("attempted to set starting state to %s, but it was not registered with the State Machine!", startingStateName)
	}

	sm.SetState(startingState)

	return sm, nil
}

func NewStartFreshState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: startFreshStateName,
		Reconcile: func() (reconcile.Result, error) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status)
			return result.Retry(0)
		},
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
	}
}

func NewResetStatefulSetUpdateStrategyState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity) state.State {
	return state.State{
		Name: resetStatefulSetUpdateStrategyStateName,
		Reconcile: func() (reconcile.Result, error) {
			if err := statefulset.ResetUpdateStrategy(&mdb, client); err != nil {
				return status.Update(client.Status(), &mdb,
					statusOptions().
						withMessage(Error, fmt.Sprintf("Error resetting StatefulSet UpdateStrategyType: %s", err)).
						withFailedPhase(),
				)
			}
			return result.Retry(0)
		},
		IsComplete: func() (bool, error) {
			sts, err := client.GetStatefulSet(mdb.NamespacedName())
			if err != nil {
				return false, err
			}
			return sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType, nil
		},
	}
}

func NewUpdateStatusState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: updateStatusState,
		Reconcile: func() (reconcile.Result, error) {
			if scale.IsStillScaling(mdb) {
				return status.Update(client.Status(), &mdb, statusOptions().
					withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
					withMessage(Info, fmt.Sprintf("Performing scaling operation, currentMembers=%d, desiredMembers=%d",
						mdb.CurrentReplicas(), mdb.DesiredReplicas())).
					withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()).
					withPendingPhase(10),
				)
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
				return res, err
			}

			// the last version will be duplicated in two annotations.
			// This is needed to reuse the update strategy logic in enterprise
			if err := annotations.UpdateLastAppliedMongoDBVersion(&mdb, client); err != nil {
				log.Errorf("Could not save current version as an annotation: %s", err)
			}
			if err := updateLastSuccessfulConfiguration(client, mdb); err != nil {
				log.Errorf("Could not save current spec as an annotation: %s", err)
			}

			return result.Retry(0)
		},
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
		IsComplete: func() (bool, error) {
			_, err := client.GetService(types.NamespacedName{Name: mdb.ServiceName(), Namespace: mdb.Namespace})
			return err == nil, err
		},
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
		IsComplete: func() (bool, error) {
			sts, err := client.GetStatefulSet(mdb.NamespacedName())
			if err != nil && !apiErrors.IsNotFound(err) {
				return false, fmt.Errorf("failed to get StatefulSet: %s", err)
			}
			ac, err := ensureAutomationConfig(client, mdb)
			if err != nil {
				return false, fmt.Errorf("failed to ensure AutomationConfig: %s", err)
			}
			return agent.AllReachedGoalState(sts, client, mdb.StatefulSetReplicasThisReconciliation(), ac.Version, log)
		},
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
		IsComplete: func() (bool, error) {
			currentSts, err := client.GetStatefulSet(mdb.NamespacedName())
			if err != nil {
				return false, errors.Errorf("error getting StatefulSet: %s", err)
			}

			isReady := statefulset.IsReady(currentSts, mdb.StatefulSetReplicasThisReconciliation())
			return isReady || currentSts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType, nil
		},
	}
}

func NewReconciliationEndState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: reconciliationEndState,
		Reconcile: func() (reconcile.Result, error) {
			allStates := newAllStates()

			bytes, err := json.Marshal(allStates)
			if err != nil {
				log.Errorf("error marshalling states: %s", err)
				return reconcile.Result{}, err
			}
			if mdb.Annotations == nil {
				mdb.Annotations = map[string]string{}
			}
			mdb.Annotations[stateMachineAnnotation] = string(bytes)

			if err := client.Update(context.TODO(), &mdb); err != nil {
				log.Errorf("error updating annotations: %s", err)
				return reconcile.Result{}, err
			}

			log.Infow("Successfully finished reconciliation", "MongoDB.Spec:", mdb.Spec, "MongoDB.Status:", mdb.Status)
			return result.OK()
		},
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
			return result.Retry(0)
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

func newAllStates() state.AllStates {
	return state.AllStates{
		NextState: startFreshStateName,
	}
}
