package x509

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
)

// Enable will configure all of the required Kubernetes resources for X509 to be enabled.
// The agent password and keyfile contents will be configured and stored in a secret.
// the user credentials will be generated if not present, or existing credentials will be read.
func Enable(ctx context.Context, auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable, agentCertSecret types.NamespacedName) error {
	opts := mdb.GetAuthOptions()

	desiredUsers := convertMongoDBResourceUsersToAutomationConfigUsers(mdb)

	if opts.AutoAuthMechanism == constants.X509 {
		if err := ensureAgent(ctx, auth, secretGetUpdateCreateDeleter, mdb, agentCertSecret); err != nil {
			return err
		}
	}

	return enableClientAuthentication(auth, opts, desiredUsers)
}

func ensureAgent(ctx context.Context, auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable, agentCertSecret types.NamespacedName) error {
	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return fmt.Errorf("could not generate keyfile contents: %s", err)
	}

	// ensure that the agent keyfile secret exists or read existing keyfile.
	agentKeyFile, err := secret.EnsureSecretWithKey(ctx, secretGetUpdateCreateDeleter, mdb.GetAgentKeyfileSecretNamespacedName(), mdb.GetOwnerReferences(), constants.AgentKeyfileKey, generatedContents)
	if err != nil {
		return err
	}

	agentCert, err := secret.ReadKey(ctx, secretGetUpdateCreateDeleter, "tls.crt", agentCertSecret)
	if err != nil {
		return err
	}

	agentSubject, err := readAgentSubjectsFromCert(agentCert)
	if err != nil {
		return err
	}

	if !isValidX509Subject(agentSubject) {
		return fmt.Errorf("Agent subject: %s is not a valid subject", agentSubject)
	}

	return enableAgentAuthentication(auth, agentKeyFile, agentSubject, mdb.GetAuthOptions())
}

// convertMongoDBResourceUsersToAutomationConfigUsers returns a list of users that are able to be set in the AutomationConfig
func convertMongoDBResourceUsersToAutomationConfigUsers(mdb authtypes.Configurable) []automationconfig.MongoDBUser {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.GetAuthUsers() {
		if u.Database == constants.ExternalDB {
			acUser := convertMongoDBUserToAutomationConfigUser(u)
			usersWanted = append(usersWanted, acUser)
		}
	}
	return usersWanted
}

// convertMongoDBUserToAutomationConfigUser converts a single user configured in the MongoDB resource and converts it to a user
// that can be added directly to the AutomationConfig.
func convertMongoDBUserToAutomationConfigUser(user authtypes.User) automationconfig.MongoDBUser {
	acUser := automationconfig.MongoDBUser{
		Username: user.Username,
		Database: user.Database,
	}
	for _, role := range user.Roles {
		acUser.Roles = append(acUser.Roles, automationconfig.Role{
			Role:     role.Name,
			Database: role.Database,
		})
	}
	acUser.AuthenticationRestrictions = []string{}
	acUser.Mechanisms = []string{}
	return acUser
}

func readAgentSubjectsFromCert(agentCert string) (string, error) {
	var rdns pkix.RDNSequence

	block, rest := pem.Decode([]byte(agentCert))

	if block != nil && block.Type == "CERTIFICATE" {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", err
		}

		if _, err := asn1.Unmarshal(cert.RawSubject, &rdns); err != nil {
			return "", err
		}
	} else if len(rest) > 0 {
		cert, err := x509.ParseCertificate(rest)
		if err != nil {
			return "", err
		}

		if _, err := asn1.Unmarshal(cert.RawSubject, &rdns); err != nil {
			return "", err
		}
	}

	return rdns.String(), nil
}

func isValidX509Subject(subject string) bool {
	expected := []string{"CN", "C", "OU"}
	for _, name := range expected {
		matched, err := regexp.MatchString(name+`=\w+`, subject)
		if err != nil {
			continue
		}
		if !matched {
			return false
		}
	}
	return true
}

func CreateAgentCertificateSecret(key string, invalid bool, agentCertSecret types.NamespacedName) v1.Secret {
	agentCert, _, _ := CreateAgentCertificate()
	if invalid {
		agentCert = "INVALID CERT"
	}

	return secret.Builder().
		SetName(agentCertSecret.Name).
		SetNamespace(agentCertSecret.Namespace).
		SetField(key, agentCert).
		Build()
}

func CreateAgentCertificate() (string, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"US"},
			Organization:       []string{"MongoDB"},
			OrganizationalUnit: []string{"ENG"},
			CommonName:         "mms-automation-agent",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // cert expires in 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	caPEM := new(bytes.Buffer)
	_ = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	_ = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	return caPEM.String(), caPrivKeyPEM.String(), nil
}
