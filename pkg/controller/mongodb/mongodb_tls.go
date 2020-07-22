package mongodb

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"

	"k8s.io/apimachinery/pkg/api/errors"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
)

const (
	tlsCAMountPath             = "/var/lib/tls/ca/"
	tlsCACertName              = "ca.crt"
	tlsOperatorSecretMountPath = "/var/lib/tls/server/" //nolint
	tlsOperatorSecretFileName  = "server.pem"
	tlsSecretCertName          = "tls.crt" //nolint
	tlsSecretKeyName           = "tls.key"
)

// validateTLSConfig will check that the configured ConfigMap and Secret exist and that they have the correct fields.
func (r *ReplicaSetReconciler) validateTLSConfig(mdb mdbv1.MongoDB) (bool, error) {
	if !mdb.Spec.Security.TLS.Enabled {
		return true, nil
	}

	r.log.Info("Ensuring TLS is correctly configured")

	// Ensure CA ConfigMap exists
	caData, err := configmap.ReadData(r.client, mdb.TLSConfigMapNamespacedName())
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Warnf(`CA ConfigMap "%s" not found`, mdb.TLSConfigMapNamespacedName())
			return false, nil
		}

		return false, err
	}

	// Ensure ConfigMap has a "ca.crt" field
	if cert, ok := caData[tlsCACertName]; !ok || cert == "" {
		r.log.Warnf(`ConfigMap "%s" should have a CA certificate in field "%s"`, mdb.TLSConfigMapNamespacedName(), tlsCACertName)
		return false, nil
	}

	// Ensure Secret exists
	secretData, err := secret.ReadStringData(r.client, mdb.TLSSecretNamespacedName())
	if err != nil {
		if errors.IsNotFound(err) {
			r.log.Warnf(`Secret "%s" not found`, mdb.TLSSecretNamespacedName())
			return false, nil
		}

		return false, err
	}

	// Ensure Secret has "tls.crt" and "tls.key" fields
	if key, ok := secretData[tlsSecretKeyName]; !ok || key == "" {
		r.log.Warnf(`Secret "%s" should have a key in field "%s"`, mdb.TLSSecretNamespacedName(), tlsSecretKeyName)
		return false, nil
	}
	if cert, ok := secretData[tlsSecretCertName]; !ok || cert == "" {
		r.log.Warnf(`Secret "%s" should have a certificate in field "%s"`, mdb.TLSSecretNamespacedName(), tlsSecretKeyName)
		return false, nil
	}

	return true, nil
}

// getTLSConfigModification creates a modification function which enables TLS in the automation config.
// It will also ensure that the combined cert-key secret is created.
func getTLSConfigModification(getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDB) (automationconfig.Modification, error) {
	if !mdb.Spec.Security.TLS.Enabled {
		return automationconfig.NOOP(), nil
	}

	if err := ensureTLSSecret(getUpdateCreator, mdb); err != nil {
		return automationconfig.NOOP(), err
	}

	// The config is only updated after the certs and keys have been rolled out to all pods.
	// The agent needs these to be in place before the config is updated.
	// Once the config is updated, the agents will gradually enable TLS in accordance with: https://docs.mongodb.com/manual/tutorial/upgrade-cluster-to-ssl/
	if hasRolledOutTLS(mdb) {
		return tlsConfigModification(mdb), nil
	}

	return automationconfig.NOOP(), nil
}

// ensureTLSSecret will create or update the operator-managed Secret containing
// the concatenated certificate and key from the user-provided Secret.
func ensureTLSSecret(getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDB) error {
	cert, err := secret.ReadKey(getUpdateCreator, tlsSecretCertName, mdb.TLSSecretNamespacedName())
	if err != nil {
		return err
	}

	key, err := secret.ReadKey(getUpdateCreator, tlsSecretKeyName, mdb.TLSSecretNamespacedName())
	if err != nil {
		return err
	}

	operatorSecret := secret.Builder().
		SetName(mdb.TLSOperatorSecretNamespacedName().Name).
		SetNamespace(mdb.TLSOperatorSecretNamespacedName().Namespace).
		SetField(tlsOperatorSecretFileName, cert+key).
		SetOwnerReferences([]metav1.OwnerReference{getOwnerReference(mdb)}).
		Build()

	return secret.CreateOrUpdate(getUpdateCreator, operatorSecret)
}

// tlsConfigModification will enable TLS in the automation config.
func tlsConfigModification(mdb mdbv1.MongoDB) automationconfig.Modification {
	caCertificatePath := tlsCAMountPath + tlsCACertName
	certificateKeyPath := tlsOperatorSecretMountPath + tlsOperatorSecretFileName

	mode := automationconfig.TLSModeRequired
	if mdb.Spec.Security.TLS.Optional {
		// TLSModePreferred requires server-server connections to use TLS but makes it optional for clients.
		mode = automationconfig.TLSModePreferred
	}

	return func(config *automationconfig.AutomationConfig) {
		// Configure CA certificate for agent
		config.TLS.CAFilePath = caCertificatePath

		for i := range config.Processes {
			config.Processes[i].Args26.Net.TLS = automationconfig.MongoDBTLS{
				Mode:                               mode,
				CAFile:                             caCertificatePath,
				PEMKeyFile:                         certificateKeyPath,
				AllowConnectionsWithoutCertificate: true,
			}
		}
	}
}

// hasRolledOutTLS determines if the TLS key and certs have been mounted to all pods.
// These must be mounted before TLS can be enabled in the automation config.
func hasRolledOutTLS(mdb mdbv1.MongoDB) bool {
	_, completedRollout := mdb.Annotations[tlsRolledOutAnnotationKey]
	return completedRollout
}

// completeTLSRollout will update the automation config and set an annotation indicating that TLS has been rolled out.
// At this stage, TLS hasn't yet been enabled but the keys and certs have all been mounted.
// The automation config will be updated and the agents will continue work on gradually enabling TLS across the replica set.
func (r *ReplicaSetReconciler) completeTLSRollout(mdb mdbv1.MongoDB) error {
	if !mdb.Spec.Security.TLS.Enabled || hasRolledOutTLS(mdb) {
		return nil
	}

	r.log.Debug("Completing TLS rollout")
	if mdb.Annotations == nil {
		mdb.Annotations = map[string]string{}
	}
	mdb.Annotations[tlsRolledOutAnnotationKey] = trueAnnotation
	if err := r.ensureAutomationConfig(mdb); err != nil {
		return fmt.Errorf("error updating automation config after TLS rollout: %+v", err)
	}
	if err := r.setAnnotations(mdb.NamespacedName(), mdb.Annotations); err != nil {
		return fmt.Errorf("error setting TLS annotation: %+v", err)
	}

	return nil
}

// buildTLSPodSpecModification will add the TLS init container and volumes to the pod template if TLS is enabled.
func buildTLSPodSpecModification(mdb mdbv1.MongoDB) podtemplatespec.Modification {
	if !mdb.Spec.Security.TLS.Enabled {
		return podtemplatespec.NOOP()
	}

	// Configure a volume which mounts the CA certificate from a ConfigMap
	// The certificate is used by both mongod and the agent
	caVolume := statefulset.CreateVolumeFromConfigMap("tls-ca", mdb.Spec.Security.TLS.CaConfigMap.Name)
	caVolumeMount := statefulset.CreateVolumeMount(caVolume.Name, tlsCAMountPath, statefulset.WithReadOnly(true))

	// Configure a volume which mounts the secret holding the server key and certificate
	// The same key-certificate pair is used for all servers
	tlsSecretVolume := statefulset.CreateVolumeFromSecret("tls-secret", mdb.TLSOperatorSecretNamespacedName().Name)
	tlsSecretVolumeMount := statefulset.CreateVolumeMount(tlsSecretVolume.Name, tlsOperatorSecretMountPath, statefulset.WithReadOnly(true))

	// MongoDB expects both key and certificate to be provided in a single PEM file
	// We are using a secret format where they are stored in separate fields, tls.crt and tls.key
	// Because of this we need to use an init container which reads the two files mounted from the secret and combines them into one
	return podtemplatespec.Apply(
		podtemplatespec.WithVolume(caVolume),
		podtemplatespec.WithVolume(tlsSecretVolume),
		podtemplatespec.WithVolumeMounts(agentName, tlsSecretVolumeMount, caVolumeMount),
		podtemplatespec.WithVolumeMounts(mongodbName, tlsSecretVolumeMount, caVolumeMount),
	)
}
