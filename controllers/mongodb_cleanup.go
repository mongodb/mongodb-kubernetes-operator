package controllers

import (
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
)

// cleanupPemSecret cleans up the old pem secret generated for the agent certificate.
func (r *ReplicaSetReconciler) cleanupPemSecret(currentMDB mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, namespace string) {
	if currentMDB.GetAgentAuthMode() == lastAppliedMDBSpec.GetAgentAuthMode() {
		return
	}

	if !currentMDB.IsAgentX509() && lastAppliedMDBSpec.IsAgentX509() {
		agentCertSecret := lastAppliedMDBSpec.GetAgentCertificateRef()
		if err := r.client.DeleteSecret(types.NamespacedName{
			Namespace: namespace,
			Name:      agentCertSecret + "-pem",
		}); err != nil {
			if apiErrors.IsNotFound(err) {
				r.log.Debugf("Agent pem file secret %s-pem was already deleted", agentCertSecret)
			} else {
				r.log.Warnf("Could not cleanup old agent pem file %s-pem: %s", agentCertSecret, err)
			}
		}
	}
}

// cleanupScramSecrets cleans up old scram secrets based on the last successful applied mongodb spec.
func (r *ReplicaSetReconciler) cleanupScramSecrets(currentMDB mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, namespace string) {
	secretsToDelete := getScramSecretsToDelete(currentMDB, lastAppliedMDBSpec)

	for _, s := range secretsToDelete {
		if err := r.client.DeleteSecret(types.NamespacedName{
			Name:      s,
			Namespace: namespace,
		}); err != nil {
			r.log.Warnf("Could not cleanup old secret %s", s)
		} else {
			r.log.Debugf("Sucessfully cleaned up secret: %s", s)
		}
	}
}

func getScramSecretsToDelete(currentMDB mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec) []string {
	type user struct {
		db   string
		name string
	}
	m := map[user]string{}
	var secretsToDelete []string

	for _, mongoDBUser := range currentMDB.Users {
		if mongoDBUser.DB != constants.ExternalDB {
			m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}] = mongoDBUser.GetScramCredentialsSecretName()
		}
	}

	for _, mongoDBUser := range lastAppliedMDBSpec.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		currentScramSecretName, ok := m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}]
		if !ok { // not used anymore
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetScramCredentialsSecretName())
		} else if currentScramSecretName != mongoDBUser.GetScramCredentialsSecretName() { // have changed
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetScramCredentialsSecretName())
		}
	}
	return secretsToDelete
}
