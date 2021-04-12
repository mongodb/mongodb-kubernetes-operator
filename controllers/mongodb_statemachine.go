package controllers

import (
	"context"
	"fmt"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/agent"
	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
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

	startFresh := NewStartFreshState(mdb, log)
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

func NewStartFreshState(mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: startFreshStateName,
		Reconcile: func() (reconcile.Result, error) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status, "MongoDB.Annotations", mdb.ObjectMeta.Annotations)
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
