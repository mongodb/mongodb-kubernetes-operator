package controllers

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

const (
	tlsCAMountPath               = "/var/lib/tls/ca/"
	tlsCACertName                = "ca.crt"
	tlsOperatorSecretMountPath   = "/var/lib/tls/server/"     //nolint
	tlsPrometheusSecretMountPath = "/var/lib/tls/prometheus/" //nolint
	tlsSecretCertName            = "tls.crt"
	tlsSecretKeyName             = "tls.key"
	tlsSecretPemName             = "tls.pem"
)

// validateTLSConfig will check that the configured ConfigMap and Secret exist and that they have the correct fields.
func (r *ReplicaSetReconciler) validateTLSConfig(mdb mdbv1.MongoDBCommunity) (bool, error) {
	if !mdb.Spec.Security.TLS.Enabled {
		return true, nil
	}

	r.log.Info("Ensuring TLS is correctly configured")

	// Ensure CA cert is configured
	_, err := getCaCrt(r.client, r.client, mdb)

	if err != nil {
		if apiErrors.IsNotFound(err) {
			r.log.Warnf("CA resource not found: %s", err)
			return false, nil
		}

		return false, err
	}

	// Ensure Secret exists
	_, err = secret.ReadStringData(r.client, mdb.TLSSecretNamespacedName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			r.log.Warnf(`Secret "%s" not found`, mdb.TLSSecretNamespacedName())
			return false, nil
		}

		return false, err
	}

	// validate whether the secret contains "tls.crt" and "tls.key", or it contains "tls.pem"
	// if it contains all three, then the pem entry should be equal to the concatenation of crt and key
	_, err = getPemOrConcatenatedCrtAndKey(r.client, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		r.log.Warnf(err.Error())
		return false, nil
	}

	// Watch certificate-key secret to handle rotations
	r.secretWatcher.Watch(mdb.TLSSecretNamespacedName(), mdb.NamespacedName())

	// Watch CA certificate changes
	if mdb.Spec.Security.TLS.CaCertificateSecret != nil {
		r.secretWatcher.Watch(mdb.TLSCaCertificateSecretNamespacedName(), mdb.NamespacedName())
	} else {
		r.configMapWatcher.Watch(mdb.TLSConfigMapNamespacedName(), mdb.NamespacedName())
	}

	r.log.Infof("Successfully validated TLS config")
	return true, nil
}

// getTLSConfigModification creates a modification function which enables TLS in the automation config.
// It will also ensure that the combined cert-key secret is created.
func getTLSConfigModification(cmGetter configmap.Getter, secretGetter secret.Getter, mdb mdbv1.MongoDBCommunity) (automationconfig.Modification, error) {
	if !mdb.Spec.Security.TLS.Enabled {
		return automationconfig.NOOP(), nil
	}

	caCert, err := getCaCrt(cmGetter, secretGetter, mdb)
	if err != nil {
		return automationconfig.NOOP(), err
	}

	certKey, err := getPemOrConcatenatedCrtAndKey(secretGetter, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		return automationconfig.NOOP(), err
	}

	return tlsConfigModification(mdb, certKey, caCert), nil
}

// getCertAndKey will fetch the certificate and key from the user-provided Secret.
func getCertAndKey(getter secret.Getter, mdb mdbv1.MongoDBCommunity, secretName types.NamespacedName) string {
	cert, err := secret.ReadKey(getter, tlsSecretCertName, secretName)
	if err != nil {
		return ""
	}

	key, err := secret.ReadKey(getter, tlsSecretKeyName, secretName)
	if err != nil {
		return ""
	}

	return combineCertificateAndKey(cert, key)
}

// getPem will fetch the pem from the user-provided secret
func getPem(getter secret.Getter, mdb mdbv1.MongoDBCommunity, secretName types.NamespacedName) string {
	pem, err := secret.ReadKey(getter, tlsSecretPemName, secretName)
	if err != nil {
		return ""
	}
	return pem
}

func combineCertificateAndKey(cert, key string) string {
	trimmedCert := strings.TrimRight(cert, "\n")
	trimmedKey := strings.TrimRight(key, "\n")
	return fmt.Sprintf("%s\n%s", trimmedCert, trimmedKey)
}

// getPemOrConcatenatedCrtAndKey will get the final PEM to write to the secret.
// This is either the tls.pem entry in the given secret, or the concatenation
// of tls.crt and tls.key
// It performs a basic validation on the entries.
func getPemOrConcatenatedCrtAndKey(getter secret.Getter, mdb mdbv1.MongoDBCommunity, secretName types.NamespacedName) (string, error) {
	certKey := getCertAndKey(getter, mdb, secretName)
	pem := getPem(getter, mdb, secretName)
	if certKey == "" && pem == "" {
		return "", fmt.Errorf(`neither "%s" nor the pair "%s"/"%s" were present in the TLS secret`, tlsSecretPemName, tlsSecretCertName, tlsSecretKeyName)
	}
	if certKey == "" {
		return pem, nil
	}
	if pem == "" {
		return certKey, nil
	}
	if certKey != pem {
		return "", fmt.Errorf(`if all of "%s", "%s" and "%s" are present in the secret, the entry for "%s" must be equal to the concatenation of "%s" with "%s"`, tlsSecretCertName, tlsSecretKeyName, tlsSecretPemName, tlsSecretPemName, tlsSecretCertName, tlsSecretKeyName)
	}
	return certKey, nil
}

func getCaCrt(cmGetter configmap.Getter, secretGetter secret.Getter, mdb mdbv1.MongoDBCommunity) (string, error) {
	var caResourceName types.NamespacedName
	var caData map[string]string
	var err error
	if mdb.Spec.Security.TLS.CaCertificateSecret != nil {
		caResourceName = mdb.TLSCaCertificateSecretNamespacedName()
		caData, err = secret.ReadStringData(secretGetter, caResourceName)
	} else if mdb.Spec.Security.TLS.CaConfigMap != nil {
		caResourceName = mdb.TLSConfigMapNamespacedName()
		caData, err = configmap.ReadData(cmGetter, caResourceName)
	}

	if err != nil {
		return "", err
	}

	if caData == nil {
		return "", fmt.Errorf("TLS field requires a reference to the CA certificate which signed the server certificates. Neither secret (field caCertificateSecretRef) not configMap (field CaConfigMap) reference present")
	}

	if cert, ok := caData[tlsCACertName]; !ok || cert == "" {
		return "", fmt.Errorf(`CA certificate resource "%s" should have a CA certificate in field "%s"`, caResourceName, tlsCACertName)
	} else {
		return cert, nil
	}
}

// ensureCASecret will create or update the operator managed Secret containing
// the CA certficate from the user provided Secret or ConfigMap.
func ensureCASecret(cmGetter configmap.Getter, secretGetter secret.Getter, getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDBCommunity) error {
	cert, err := getCaCrt(cmGetter, secretGetter, mdb)
	if err != nil {
		return err
	}

	caFileName := tlsOperatorSecretFileName(cert)

	operatorSecret := secret.Builder().
		SetName(mdb.TLSOperatorCASecretNamespacedName().Name).
		SetNamespace(mdb.TLSOperatorCASecretNamespacedName().Namespace).
		SetField(caFileName, cert).
		SetOwnerReferences(mdb.GetOwnerReferences()).
		Build()

	return secret.CreateOrUpdate(getUpdateCreator, operatorSecret)
}

// ensureTLSSecret will create or update the operator-managed Secret containing
// the concatenated certificate and key from the user-provided Secret.
func ensureTLSSecret(getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDBCommunity) error {
	certKey, err := getPemOrConcatenatedCrtAndKey(getUpdateCreator, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		return err
	}
	// Calculate file name from certificate and key
	fileName := tlsOperatorSecretFileName(certKey)

	operatorSecret := secret.Builder().
		SetName(mdb.TLSOperatorSecretNamespacedName().Name).
		SetNamespace(mdb.TLSOperatorSecretNamespacedName().Namespace).
		SetField(fileName, certKey).
		SetOwnerReferences(mdb.GetOwnerReferences()).
		Build()

	return secret.CreateOrUpdate(getUpdateCreator, operatorSecret)
}

// ensurePrometheusTLSSecret will create or update the operator-managed Secret containing
// the concatenated certificate and key from the user-provided Secret.
func ensurePrometheusTLSSecret(getUpdateCreator secret.GetUpdateCreator, mdb mdbv1.MongoDBCommunity) error {
	certKey, err := getPemOrConcatenatedCrtAndKey(getUpdateCreator, mdb, mdb.DeepCopy().PrometheusTLSSecretNamespacedName())
	if err != nil {
		return err
	}
	// Calculate file name from certificate and key
	fileName := tlsOperatorSecretFileName(certKey)

	operatorSecret := secret.Builder().
		SetName(mdb.PrometheusTLSOperatorSecretNamespacedName().Name).
		SetNamespace(mdb.PrometheusTLSOperatorSecretNamespacedName().Namespace).
		SetField(fileName, certKey).
		SetOwnerReferences(mdb.GetOwnerReferences()).
		Build()

	return secret.CreateOrUpdate(getUpdateCreator, operatorSecret)
}

// tlsOperatorSecretFileName calculates the file name to use for the mounted
// certificate-key file. The name is based on the hash of the combined cert and key.
// If the certificate or key changes, the file path changes as well which will trigger
// the agent to perform a restart.
// The user-provided secret is being watched and will trigger a reconciliation
// on changes. This enables the operator to automatically handle cert rotations.
func tlsOperatorSecretFileName(certKey string) string {
	hash := sha256.Sum256([]byte(certKey))
	return fmt.Sprintf("%x.pem", hash)
}

// tlsConfigModification will enable TLS in the automation config.
func tlsConfigModification(mdb mdbv1.MongoDBCommunity, certKey, caCert string) automationconfig.Modification {
	caCertificatePath := tlsCAMountPath + tlsOperatorSecretFileName(caCert)
	certificateKeyPath := tlsOperatorSecretMountPath + tlsOperatorSecretFileName(certKey)

	mode := automationconfig.TLSModeRequired
	if mdb.Spec.Security.TLS.Optional {
		// TLSModePreferred requires server-server connections to use TLS but makes it optional for clients.
		mode = automationconfig.TLSModePreferred
	}

	return func(config *automationconfig.AutomationConfig) {
		// Configure CA certificate for agent
		config.TLSConfig.CAFilePath = caCertificatePath

		for i := range config.Processes {
			args := config.Processes[i].Args26

			args.Set("net.tls.mode", mode)
			args.Set("net.tls.CAFile", caCertificatePath)
			args.Set("net.tls.certificateKeyFile", certificateKeyPath)
			args.Set("net.tls.allowConnectionsWithoutCertificates", true)
		}
	}
}

// buildTLSPodSpecModification will add the TLS init container and volumes to the pod template if TLS is enabled.
func buildTLSPodSpecModification(mdb mdbv1.MongoDBCommunity) podtemplatespec.Modification {
	if !mdb.Spec.Security.TLS.Enabled {
		return podtemplatespec.NOOP()
	}

	// Configure a volume which mounts the CA certificate from either a Secret or a ConfigMap
	// The certificate is used by both mongod and the agent
	caVolume := statefulset.CreateVolumeFromSecret("tls-ca", mdb.TLSOperatorCASecretNamespacedName().Name)
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
		podtemplatespec.WithVolumeMounts(construct.AgentName, tlsSecretVolumeMount, caVolumeMount),
		podtemplatespec.WithVolumeMounts(construct.MongodbName, tlsSecretVolumeMount, caVolumeMount),
	)
}

// buildTLSPrometheus adds the TLS mounts for Prometheus.
func buildTLSPrometheus(mdb mdbv1.MongoDBCommunity) podtemplatespec.Modification {
	if mdb.Spec.Prometheus == nil || mdb.Spec.Prometheus.TLSSecretRef.Name == "" {
		return podtemplatespec.NOOP()
	}

	// Configure a volume which mounts the secret holding the server key and certificate
	// The same key-certificate pair is used for all servers
	tlsSecretVolume := statefulset.CreateVolumeFromSecret("prom-tls-secret", mdb.PrometheusTLSOperatorSecretNamespacedName().Name)

	tlsSecretVolumeMount := statefulset.CreateVolumeMount(tlsSecretVolume.Name, tlsPrometheusSecretMountPath, statefulset.WithReadOnly(true))

	// MongoDB expects both key and certificate to be provided in a single PEM file
	// We are using a secret format where they are stored in separate fields, tls.crt and tls.key
	// Because of this we need to use an init container which reads the two files mounted from the secret and combines them into one
	return podtemplatespec.Apply(
		// podtemplatespec.WithVolume(caVolume),
		podtemplatespec.WithVolume(tlsSecretVolume),
		podtemplatespec.WithVolumeMounts(construct.AgentName, tlsSecretVolumeMount),
		podtemplatespec.WithVolumeMounts(construct.MongodbName, tlsSecretVolumeMount),
	)
}
