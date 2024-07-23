package controllers

import (
	"context"
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/constants"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// ensureUserResources will check that the configured user password secrets can be found
// and will start monitor them so that the reconcile process is triggered every time these secrets are updated
func (r ReplicaSetReconciler) ensureUserResources(ctx context.Context, mdb mdbv1.MongoDBCommunity) error {
	for _, user := range mdb.GetAuthUsers() {
		if user.Database != constants.ExternalDB {
			secretNamespacedName := types.NamespacedName{Name: user.PasswordSecretName, Namespace: mdb.Namespace}
			if _, err := secret.ReadKey(ctx, r.client, user.PasswordSecretKey, secretNamespacedName); err != nil {
				if apiErrors.IsNotFound(err) {
					// check for SCRAM secret as well
					scramSecretName := types.NamespacedName{Name: user.ScramCredentialsSecretName, Namespace: mdb.Namespace}
					_, err = r.client.GetSecret(ctx, scramSecretName)
					if apiErrors.IsNotFound(err) {
						return fmt.Errorf(`user password secret: %s and scram secret: %s not found`, secretNamespacedName, scramSecretName)
					}
					r.log.Errorf(`user password secret "%s" not found: %s`, secretNamespacedName, err)
					continue
				}
				return err
			}
			r.secretWatcher.Watch(ctx, secretNamespacedName, mdb.NamespacedName())
		}
	}

	return nil
}

// updateConnectionStringSecrets updates secrets where user specific connection strings are stored.
// The client applications can mount these secrets and connect to the mongodb cluster
func (r ReplicaSetReconciler) updateConnectionStringSecrets(ctx context.Context, mdb mdbv1.MongoDBCommunity, clusterDomain string) error {
	for _, user := range mdb.GetAuthUsers() {
		secretName := user.ConnectionStringSecretName

		secretNamespace := mdb.Namespace
		if user.ConnectionStringSecretNamespace != "" {
			secretNamespace = user.ConnectionStringSecretNamespace
		}

		existingSecret, err := r.client.GetSecret(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: secretNamespace,
		})
		if err != nil && !apiErrors.IsNotFound(err) {
			return err
		}
		if err == nil && !secret.HasOwnerReferences(existingSecret, mdb.GetOwnerReferences()) {
			return fmt.Errorf("connection string secret %s already exists and is not managed by the operator", secretName)
		}

		pwd := ""

		if user.Database != constants.ExternalDB {
			secretNamespacedName := types.NamespacedName{Name: user.PasswordSecretName, Namespace: mdb.Namespace}
			pwd, err = secret.ReadKey(ctx, r.client, user.PasswordSecretKey, secretNamespacedName)
			if err != nil {
				return err
			}
		}

		connectionStringSecret := secret.Builder().
			SetName(secretName).
			SetNamespace(secretNamespace).
			SetField("connectionString.standard", mdb.MongoAuthUserURI(user, pwd, clusterDomain)).
			SetField("connectionString.standardSrv", mdb.MongoAuthUserSRVURI(user, pwd, clusterDomain)).
			SetField("username", user.Username).
			SetField("password", pwd).
			SetOwnerReferences(mdb.GetOwnerReferences()).
			Build()

		if err := secret.CreateOrUpdate(ctx, r.client, connectionStringSecret); err != nil {
			return err
		}

		secretNamespacedName := types.NamespacedName{Name: connectionStringSecret.Name, Namespace: connectionStringSecret.Namespace}
		r.secretWatcher.Watch(ctx, secretNamespacedName, mdb.NamespacedName())
	}

	return nil
}
