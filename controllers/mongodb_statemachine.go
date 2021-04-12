package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/pkg/errors"
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
	stateMachineAnnotation       = "mongodb.com/v1.stateMachine"
	completeAnnotation           = "complete"
	startFreshStateAnnotation    = "StartFresh"
	validateSpecStateAnnotation  = "ValidateSpec"
	createServiceStateAnnotation = "CreateService"
	tlsValidationState           = "TLSValidation"
	tlsResourcesState            = "CreateTLSResources"

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

	sm.AddTransition(startFresh, validateSpec, noCondition)
	sm.AddTransition(validateSpec, serviceState, noCondition)

	sm.AddTransition(serviceState, tlsValidationState, func() (bool, error) {
		// we only need to validate TLS if it is enabled in the resource
		return mdb.Spec.Security.TLS.Enabled, nil
	})

	sm.AddTransition(tlsValidationState, tlsResourcesState, noCondition)

	// TODO: add transition for serviceState -> after TLS states

	return sm
}

func NewStartFreshState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: "StartFresh",
		Reconcile: func() (reconcile.Result, error) {
			log.Infow("Reconciling MongoDB", "MongoDB.Spec", mdb.Spec, "MongoDB.Status", mdb.Status, "MongoDB.Annotations", mdb.ObjectMeta.Annotations)
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, startFreshStateAnnotation),
	}
}

func NewValidateSpecState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: validateSpecStateAnnotation,
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
			return result.Retry(0)
		},
		OnCompletion: updateCompletionAnnotation(client, mdb, createServiceStateAnnotation),
	}
}

func NewEnsureTLSResourcesState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsResourcesState,
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
		OnCompletion: updateCompletionAnnotation(client, mdb, tlsResourcesState),
	}
}

// ensureTLSResources creates any required TLS resources that the MongoDBCommunity
// requires for TLS configuration.
func ensureTLSResources(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, log *zap.SugaredLogger) error {
	// the TLS secret needs to be created beforehand, as both the StatefulSet and AutomationConfig
	// require the contents.
	log.Infof("TLS is enabled, creating/updating TLS secret")
	if err := ensureTLSSecret(client, mdb); err != nil {
		return errors.Errorf("could not ensure TLS secret: %s", err)
	}
	return nil
}

func NewTLSValidationState(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) state.State {
	return state.State{
		Name: tlsValidationState,
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
		OnCompletion: updateCompletionAnnotation(client, mdb, tlsValidationState),
	}
}

// validateTLSConfig will check that the configured ConfigMap and Secret exist and that they have the correct fields.
func validateTLSConfig(client kubernetesClient.Client, mdb mdbv1.MongoDBCommunity, secretWatcher *watch.ResourceWatcher, log *zap.SugaredLogger) (bool, error) {
	log.Info("Ensuring TLS is correctly configured")

	// Ensure CA ConfigMap exists
	caData, err := configmap.ReadData(client, mdb.TLSConfigMapNamespacedName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Warnf(`CA ConfigMap "%s" not found`, mdb.TLSConfigMapNamespacedName())
			return false, nil
		}

		return false, err
	}

	// Ensure ConfigMap has a "ca.crt" field
	if cert, ok := caData[tlsCACertName]; !ok || cert == "" {
		log.Warnf(`ConfigMap "%s" should have a CA certificate in field "%s"`, mdb.TLSConfigMapNamespacedName(), tlsCACertName)
		return false, nil
	}

	// Ensure Secret exists
	secretData, err := secret.ReadStringData(client, mdb.TLSSecretNamespacedName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Warnf(`Secret "%s" not found`, mdb.TLSSecretNamespacedName())
			return false, nil
		}

		return false, err
	}

	// Ensure Secret has "tls.crt" and "tls.key" fields
	if key, ok := secretData[tlsSecretKeyName]; !ok || key == "" {
		log.Warnf(`Secret "%s" should have a key in field "%s"`, mdb.TLSSecretNamespacedName(), tlsSecretKeyName)
		return false, nil
	}
	if cert, ok := secretData[tlsSecretCertName]; !ok || cert == "" {
		log.Warnf(`Secret "%s" should have a certificate in field "%s"`, mdb.TLSSecretNamespacedName(), tlsSecretKeyName)
		return false, nil
	}

	// Watch certificate-key secret to handle rotations
	secretWatcher.Watch(mdb.TLSSecretNamespacedName(), mdb.NamespacedName())

	log.Infof("Successfully validated TLS config")
	return true, nil
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
		CurrentState: "StartFresh",
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
