package mongodb

import (
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/container"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
)

// validateTLSConfig will check that the configured ConfigMap and Secret exist and that they have the correct fields.
// The possible return values are:
// - (true, nil) if the config is valid
// - (false, nil) if the config is not valid
// - (_, err) if an error occured when validating the config
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

// completeTLSRollout will update the automation config and set an annotation indicating that TLS has been rolled out.
// At this stage, TLS hasn't yet been enabled but the keys and certs have all been mounted.
// The automation config will be updated and the agents will continue work on gradually enabling TLS across the replica set.
func (r *ReplicaSetReconciler) completeTLSRollout(mdb mdbv1.MongoDB) error {
	if !mdb.Spec.Security.TLS.Enabled || mdb.HasRolledOutTLS() {
		return nil
	}

	r.log.Debug("Completing TLS rollout")

	mdb.Annotations[mdbv1.TLSRolledOutKey] = "true"
	if err := r.ensureAutomationConfig(mdb); err != nil {
		return fmt.Errorf("error updating automation config after TLS rollout: %+v", err)
	}

	if err := r.setAnnotation(mdb.NamespacedName(), mdbv1.TLSRolledOutKey, "true"); err != nil {
		return fmt.Errorf("error setting TLS annotation: %+v", err)
	}

	return nil
}

// buildTLSPodSpecModification will add the TLS init container and volumes to the pod template if TLS is enabled.
func buildTLSPodSpecModification(mdb mdbv1.MongoDB) podtemplatespec.Modification {
	if !mdb.Spec.Security.TLS.Enabled {
		return podtemplatespec.NOOP()
	}

	// Configure an empty volume into which the TLS init container will write the certificate and key file
	tlsVolume := statefulset.CreateVolumeFromEmptyDir("tls")
	tlsVolumeMount := statefulset.CreateVolumeMount(tlsVolume.Name, tlsServerMountPath, statefulset.WithReadOnly(false))

	// Configure a volume which mounts the CA certificate from a ConfigMap
	// The certificate is used by both mongod and the agent
	caVolume := statefulset.CreateVolumeFromConfigMap("tls-ca", mdb.Spec.Security.TLS.CAConfigMapName)
	caVolumeMount := statefulset.CreateVolumeMount(caVolume.Name, tlsCAMountPath, statefulset.WithReadOnly(true))

	// Configure a volume which mounts the secret holding the server key and certificate
	// The same key-certificate pair is used for all servers
	tlsSecretVolume := statefulset.CreateVolumeFromSecret("tls-secret", mdb.Spec.Security.TLS.ServerSecretName)
	tlsSecretVolumeMount := statefulset.CreateVolumeMount(tlsSecretVolume.Name, tlsSecretMountPath, statefulset.WithReadOnly(true))

	// MongoDB expects both key and certificate to be provided in a single PEM file
	// We are using a secret format where they are stored in separate fields, tls.crt and tls.key
	// Because of this we need to use an init container which reads the two files mounted from the secret and combines them into one
	return podtemplatespec.Apply(
		podtemplatespec.WithInitContainer("tls-init", tlsInit(tlsVolumeMount, tlsSecretVolumeMount)),
		podtemplatespec.WithVolume(tlsVolume),
		podtemplatespec.WithVolume(caVolume),
		podtemplatespec.WithVolume(tlsSecretVolume),
		podtemplatespec.WithVolumeMounts(agentName, tlsVolumeMount, caVolumeMount),
		podtemplatespec.WithVolumeMounts(mongodbName, tlsVolumeMount, caVolumeMount),
	)
}

// tlsInit creates an init container which combines the mounted tls.key and tls.crt into a single PEM file
func tlsInit(tlsMount, tlsSecretMount corev1.VolumeMount) container.Modification {
	command := fmt.Sprintf(
		"cat %s %s > %s",
		tlsSecretMountPath+tlsSecretCertName,
		tlsSecretMountPath+tlsSecretKeyName,
		tlsServerMountPath+tlsServerFileName)

	return container.Apply(
		container.WithName("tls-init"),
		container.WithImage("busybox"),
		container.WithCommand([]string{"sh", "-c", command}),
		container.WithVolumeMounts([]corev1.VolumeMount{tlsMount, tlsSecretMount}),
	)
}
