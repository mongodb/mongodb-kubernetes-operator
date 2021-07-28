package tls

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/mongodb/mongodb-kubernetes-operator/controllers/construct"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers/watch"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/configmap"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/podtemplatespec"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"

	kubernetesClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"go.uber.org/zap"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TLSCAMountPath             = "/var/lib/tls/ca/"
	TLSCACertName              = "ca.crt"
	TLSOperatorSecretMountPath = "/var/lib/tls/server/" //nolint
)

type TLSResource interface {
	IsTLSEnabled() bool
	IsTLSOptional() bool
	TLSConfigMapNamespacedName() types.NamespacedName
	TLSSecretNamespacedName() types.NamespacedName
	NamespacedName() types.NamespacedName
	TLSOperatorSecretNamespacedName() types.NamespacedName
	GetOwnerReferences() []metav1.OwnerReference
	TLSSecretKeyName() string
	TLSSecretCertName() string
	TLSSecretPEMName() string
	TLSConfigMapRequiredEntries() []string
}

// validateTLSConfig will check that the configured ConfigMap and Secret exist and that they have the correct fields.
func ValidateTLSConfig(mdb TLSResource, client kubernetesClient.Client, log *zap.SugaredLogger, secretWatcher *watch.ResourceWatcher) (bool, error) {
	if !mdb.IsTLSEnabled() {
		return true, nil
	}

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

	// Ensure ConfigMap has all the required fields
	for _, key := range mdb.TLSConfigMapRequiredEntries() {
		if cert, ok := caData[key]; !ok || cert == "" {
			log.Warnf(`ConfigMap "%s" should have a CA certificate in field "%s"`, mdb.TLSConfigMapNamespacedName(), TLSCACertName)
			return false, nil
		}
	}

	// Ensure Secret exists
	_, err = secret.ReadStringData(client, mdb.TLSSecretNamespacedName())
	if err != nil {
		if apiErrors.IsNotFound(err) {
			log.Warnf(`Secret "%s" not found`, mdb.TLSSecretNamespacedName())
			return false, nil
		}

		return false, err
	}

	// validate whether the secret contains "tls.crt" and "tls.key", or it contains "tls.pem"
	// if it contains all three, then the pem entry should be equal to the concatenation of crt and key
	_, err = GetPemOrConcatenatedCrtAndKey(client, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		log.Warnf(err.Error())
		return false, nil
	}

	// Watch certificate-key secret to handle rotations
	secretWatcher.Watch(mdb.TLSSecretNamespacedName(), mdb.NamespacedName())

	log.Infof("Successfully validated TLS config")
	return true, nil
}

// getTLSConfigModification creates a modification function which enables TLS in the automation config.
// It will also ensure that the combined cert-key secret is created.
func GetTLSConfigModification(getUpdateCreator secret.GetUpdateCreator, mdb TLSResource) (automationconfig.Modification, error) {
	if !mdb.IsTLSEnabled() {
		return automationconfig.NOOP(), nil
	}

	certKey, err := GetPemOrConcatenatedCrtAndKey(getUpdateCreator, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		return automationconfig.NOOP(), err
	}

	return tlsConfigModification(mdb, certKey), nil
}

// getCertAndKey will fetch the certificate and key from the user-provided Secret.
func getCertAndKey(getter secret.Getter, mdb TLSResource, secretName types.NamespacedName) string {
	cert, err := secret.ReadKey(getter, mdb.TLSSecretCertName(), secretName)
	if err != nil {
		return ""
	}

	key, err := secret.ReadKey(getter, mdb.TLSSecretKeyName(), secretName)
	if err != nil {
		return ""
	}

	return CombineCertificateAndKey(cert, key)
}

// getPem will fetch the pem from the user-provided secret
func getPem(getter secret.Getter, mdb TLSResource, secretName types.NamespacedName) string {
	pem, err := secret.ReadKey(getter, mdb.TLSSecretPEMName(), secretName)
	if err != nil {
		return ""
	}
	return pem
}

func CombineCertificateAndKey(cert, key string) string {
	trimmedCert := strings.TrimRight(cert, "\n")
	trimmedKey := strings.TrimRight(key, "\n")
	return fmt.Sprintf("%s\n%s", trimmedCert, trimmedKey)
}

// GetPemOrConcatenatedCrtAndKey will get the final PEM to write to the secret.
// This is either the tls.pem entry in the given secret, or the concatenation
// of tls.crt and tls.key
// It performs a basic validation on the entries.
func GetPemOrConcatenatedCrtAndKey(getter secret.Getter, mdb TLSResource, secretName types.NamespacedName) (string, error) {
	certKey := getCertAndKey(getter, mdb, secretName)
	pem := getPem(getter, mdb, secretName)
	if certKey == "" && pem == "" {
		return "", fmt.Errorf(`Neither "%s" nor the pair "%s"/"%s" were present in the TLS secret`, mdb.TLSSecretPEMName(), mdb.TLSSecretCertName(), mdb.TLSSecretKeyName())
	}
	if certKey == "" {
		return pem, nil
	}
	if pem == "" {
		return certKey, nil
	}
	if certKey != pem {
		return "", fmt.Errorf(`If all of "%s", "%s" and "%s" are present in the secret, the entry for "%s" must be equal to the concatenation of "%s" with "%s"`, mdb.TLSSecretCertName(), mdb.TLSSecretKeyName(), mdb.TLSSecretPEMName(), mdb.TLSSecretPEMName(), mdb.TLSSecretCertName(), mdb.TLSSecretKeyName())
	}
	return certKey, nil
}

// ensureTLSSecret will create or update the operator-managed Secret containing
// the concatenated certificate and key from the user-provided Secret.
func EnsureTLSSecret(getUpdateCreator secret.GetUpdateCreator, mdb TLSResource) error {
	certKey, err := GetPemOrConcatenatedCrtAndKey(getUpdateCreator, mdb, mdb.TLSSecretNamespacedName())
	if err != nil {
		return err
	}
	// Calculate file name from certificate and key
	fileName := TLSOperatorSecretFileName(certKey)

	operatorSecret := secret.Builder().
		SetName(mdb.TLSOperatorSecretNamespacedName().Name).
		SetNamespace(mdb.TLSOperatorSecretNamespacedName().Namespace).
		SetField(fileName, certKey).
		SetOwnerReferences(mdb.GetOwnerReferences()).
		Build()

	return secret.CreateOrUpdate(getUpdateCreator, operatorSecret)
}

// TLSOperatorSecretFileName calculates the file name to use for the mounted
// certificate-key file. The name is based on the hash of the combined cert and key.
// If the certificate or key changes, the file path changes as well which will trigger
// the agent to perform a restart.
// The user-provided secret is being watched and will trigger a reconciliation
// on changes. This enables the operator to automatically handle cert rotations.
func TLSOperatorSecretFileName(certKey string) string {
	hash := sha256.Sum256([]byte(certKey))
	return fmt.Sprintf("%x.pem", hash)
}

// tlsConfigModification will enable TLS in the automation config.
func tlsConfigModification(mdb TLSResource, certKey string) automationconfig.Modification {
	caCertificatePath := TLSCAMountPath + TLSCACertName
	certificateKeyPath := TLSOperatorSecretMountPath + TLSOperatorSecretFileName(certKey)

	mode := automationconfig.TLSModeRequired
	if mdb.IsTLSOptional() {
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
func BuildTLSPodSpecModification(mdb TLSResource) podtemplatespec.Modification {
	if !mdb.IsTLSEnabled() {
		return podtemplatespec.NOOP()
	}

	// Configure a volume which mounts the CA certificate from a ConfigMap
	// The certificate is used by both mongod and the agent
	caVolume := statefulset.CreateVolumeFromConfigMap("tls-ca", mdb.TLSConfigMapNamespacedName().Name)
	caVolumeMount := statefulset.CreateVolumeMount(caVolume.Name, TLSCAMountPath, statefulset.WithReadOnly(true))

	// Configure a volume which mounts the secret holding the server key and certificate
	// The same key-certificate pair is used for all servers
	tlsSecretVolume := statefulset.CreateVolumeFromSecret("tls-secret", mdb.TLSOperatorSecretNamespacedName().Name)
	tlsSecretVolumeMount := statefulset.CreateVolumeMount(tlsSecretVolume.Name, TLSOperatorSecretMountPath, statefulset.WithReadOnly(true))

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
