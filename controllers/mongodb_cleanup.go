package controllers

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
		m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}] = mongoDBUser.GetScramCredentialsSecretName()
	}

	for _, mongoDBUser := range lastAppliedMDBSpec.Users {
		currentScramSecretName, ok := m[user{db: mongoDBUser.DB, name: mongoDBUser.Name}]
		if !ok { // not used anymore
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetScramCredentialsSecretName())
		} else if currentScramSecretName != mongoDBUser.GetScramCredentialsSecretName() { // have changed
			secretsToDelete = append(secretsToDelete, mongoDBUser.GetScramCredentialsSecretName())
		}
	}
	return secretsToDelete
}
