package x509

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/authtypes"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
)

// Enable will configure all of the required Kubernetes resources for X509 to be enabled.
// The agent password and keyfile contents will be configured and stored in a secret.
// the user credentials will be generated if not present, or existing credentials will be read.
func Enable(auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) error {
	opts := mdb.GetAuthOptions()

	desiredUsers, err := convertMongoDBResourceUsersToAutomationConfigUsers(mdb)
	if err != nil {
		return fmt.Errorf("could not convert users to Automation Config users: %s", err)
	}

	if opts.AutoAuthMechanism == constants.X509 {
		if err := ensureAgent(auth, secretGetUpdateCreateDeleter, mdb); err != nil {
			return err
		}
	}

	return enableClientAuthentication(auth, opts, desiredUsers)
}

func ensureAgent(auth *automationconfig.Auth, secretGetUpdateCreateDeleter secret.GetUpdateCreateDeleter, mdb authtypes.Configurable) error {
	generatedContents, err := generate.KeyFileContents()
	if err != nil {
		return fmt.Errorf("could not generate keyfile contents: %s", err)
	}

	// ensure that the agent keyfile secret exists or read existing keyfile.
	agentKeyFile, err := secret.EnsureSecretWithKey(secretGetUpdateCreateDeleter, mdb.GetAgentKeyfileSecretNamespacedName(), mdb.GetOwnerReferences(), constants.AgentKeyfileKey, generatedContents)
	if err != nil {
		return err
	}

	agentCert, err := secret.EnsureSecretWithKey(secretGetUpdateCreateDeleter, mdb.AgentCertificateSecretNamespacedName(), mdb.GetOwnerReferences(), "tls.crt", "")
	if err != nil {
		return err
	}

	agentSubject, err := readAgentSubjectsFromSecret(agentCert)
	if err != nil {
		return err
	}

	return enableAgentAuthentication(auth, agentKeyFile, agentSubject, mdb.GetAuthOptions())
}

// convertMongoDBResourceUsersToAutomationConfigUsers returns a list of users that are able to be set in the AutomationConfig
func convertMongoDBResourceUsersToAutomationConfigUsers(mdb authtypes.Configurable) ([]automationconfig.MongoDBUser, error) {
	var usersWanted []automationconfig.MongoDBUser
	for _, u := range mdb.GetAuthUsers() {
		if u.Database == constants.ExternalDB {
			acUser := convertMongoDBUserToAutomationConfigUser(u)
			usersWanted = append(usersWanted, acUser)
		}
	}
	return usersWanted, nil
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

func readAgentSubjectsFromSecret(agentCert string) (string, error) {
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
