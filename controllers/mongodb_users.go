package controllers

import (
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/secret"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r ReplicaSetReconciler) ensureUserResources(mdb mdbv1.MongoDBCommunity) error {
	for _, user := range mdb.GetScramUsers() {
		secretNamespacedName := types.NamespacedName{Name: user.PasswordSecretName, Namespace: mdb.NamespacedName().Namespace}
		if _, err := secret.ReadKey(r.client, user.PasswordSecretKey, secretNamespacedName); err != nil {
			if apiErrors.IsNotFound(err) {
				r.log.Errorf(`User password secret "%s" not found`, secretNamespacedName)
			}
			return err
		}

		r.secretWatcher.Watch(secretNamespacedName, mdb.NamespacedName())
	}

	return nil
}

func (r ReplicaSetReconciler) updateConnectionStringSecrets(mdb mdbv1.MongoDBCommunity) error {
	for _, user := range mdb.GetScramUsers() {
		secretNamespacedName := types.NamespacedName{Name: user.PasswordSecretName, Namespace: mdb.NamespacedName().Namespace}
		pwd, err := secret.ReadKey(r.client, user.PasswordSecretKey, secretNamespacedName)
		if err != nil {
			return err
		}

		operatorSecret := secret.Builder().
			SetName(fmt.Sprintf("mdbc-%s-%s", user.Database, user.Username)).
			SetNamespace(mdb.Namespace).
			SetField("connectionString.standard", mdb.MongoAuthUserURI(user, pwd)).
			SetField("connectionString.standardSrv", mdb.MongoAuthUserSRVURI(user, pwd)).
			SetField("username", user.Username).
			SetField("password", pwd).
			SetOwnerReferences(mdb.GetOwnerReferences()).
			Build()

		if err := secret.CreateOrUpdate(r.client, operatorSecret); err != nil {
			return err
		}
	}

	return nil
}
