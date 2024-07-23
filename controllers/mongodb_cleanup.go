package controllers

import (
	"context"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
)

// cleanupPemSecret cleans up the old pem secret generated for the agent certificate.
func (r *ReplicaSetReconciler) cleanupPemSecret(ctx context.Context, currentMDBSpec mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, namespace string) {
	if currentMDBSpec.GetAgentAuthMode() == lastAppliedMDBSpec.GetAgentAuthMode() {
		return
	}

	if !currentMDBSpec.IsAgentX509() && lastAppliedMDBSpec.IsAgentX509() {
		agentCertSecret := lastAppliedMDBSpec.GetAgentCertificateRef()
		if err := r.client.DeleteSecret(ctx, types.NamespacedName{
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
func (r *ReplicaSetReconciler) cleanupScramSecrets(ctx context.Context, currentMDBSpec mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, namespace string) {
	secretsToDelete := getScramSecretsToDelete(currentMDBSpec, lastAppliedMDBSpec)

	for _, s := range secretsToDelete {
		if err := r.client.DeleteSecret(ctx, types.NamespacedName{
			Name:      s,
			Namespace: namespace,
		}); err != nil {
			r.log.Warnf("Could not cleanup old secret %s: %s", s, err)
		} else {
			r.log.Debugf("Sucessfully cleaned up secret: %s", s)
		}
	}
}

// cleanupConnectionStringSecrets cleans up old scram secrets based on the last successful applied mongodb spec.
func (r *ReplicaSetReconciler) cleanupConnectionStringSecrets(ctx context.Context, currentMDBSpec mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, namespace string, resourceName string) {
	secretsToDelete := getConnectionStringSecretsToDelete(currentMDBSpec, lastAppliedMDBSpec, resourceName)

	for _, s := range secretsToDelete {
		if err := r.client.DeleteSecret(ctx, types.NamespacedName{
			Name:      s,
			Namespace: namespace,
		}); err != nil {
			r.log.Warnf("Could not cleanup old secret %s: %s", s, err)
		} else {
			r.log.Debugf("Sucessfully cleaned up secret: %s", s)
		}
	}
}

func getScramSecretsToDelete(currentMDBSpec mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec) []string {
	type user struct {
		db   string
		name string
	}
	m := map[user]string{}
	var secretsToDelete []string

	for _, mongoDBUser := range currentMDBSpec.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}] = mongoDBUser.GetScramCredentialsSecretName()
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

func getConnectionStringSecretsToDelete(currentMDBSpec mdbv1.MongoDBCommunitySpec, lastAppliedMDBSpec mdbv1.MongoDBCommunitySpec, resourceName string) []string {
	type user struct {
		db   string
		name string
	}
	m := map[user]string{}
	var secretsToDelete []string

	for _, mongoDBUser := range currentMDBSpec.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}] = mongoDBUser.GetConnectionStringSecretName(resourceName)
	}

	for _, mongoDBUser := range lastAppliedMDBSpec.Users {
		if mongoDBUser.DB == constants.ExternalDB {
			continue
		}
		currentConnectionStringSecretName, ok := m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}]
		if !ok { // user was removed
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetConnectionStringSecretName(resourceName))
		} else if currentConnectionStringSecretName != mongoDBUser.GetConnectionStringSecretName(resourceName) {
			// this happens when a new ConnectionStringSecretName was set for the old user
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetConnectionStringSecretName(resourceName))
		}
	}
	return secretsToDelete
}
