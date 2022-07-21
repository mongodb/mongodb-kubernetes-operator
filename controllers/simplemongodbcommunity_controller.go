/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	v1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"go.uber.org/zap"
	v12 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mongodbcommunityv1alpha1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1alpha1"
)

// SimpleMongoDBCommunityReconciler reconciles a SimpleMongoDBCommunity object
type SimpleMongoDBCommunityReconciler struct {
	client.Client
	Log    *zap.SugaredLogger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=simplemongodbcommunities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=simplemongodbcommunities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mongodbcommunity.mongodb.com,resources=simplemongodbcommunities/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SimpleMongoDBCommunity object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *SimpleMongoDBCommunityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	simpleMongoDBCommunity := mongodbcommunityv1alpha1.SimpleMongoDBCommunity{}
	err := r.Get(context.TODO(), req.NamespacedName, &simpleMongoDBCommunity)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return result.OK()
		}
		logger.Error(err, "Error reconciling MongoDB resource: %s", req.Name)
		return result.Failed()
	}

	numberOfReplicas, err := calculateNumberOfReplicas(r.Client, simpleMongoDBCommunity.Spec.Expectations.Data.Size)
	if err != nil {
		r.Log.Error(err)
		return result.Failed()
	}
	r.Log.Infof("Number of replicas %v", numberOfReplicas)

	passwordSecretName := simpleMongoDBCommunity.Name + "-password"
	passwordKeyName := "password"
	userPassword := v12.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      passwordSecretName,
			Namespace: req.Namespace,
		},
		StringData: map[string]string{
			//TODO: Secure Random!!!
			passwordKeyName: "password",
		},
	}
	err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      passwordSecretName,
	}, &userPassword)
	if apiErrors.IsNotFound(err) {
		err = r.Create(context.TODO(), &userPassword)
		if err != nil {
			r.Log.Error(err)
			return result.Failed()
		}
	}

	mdb := v1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      simpleMongoDBCommunity.Name,
			Namespace: req.Namespace,
		},
		Spec: v1.MongoDBCommunitySpec{
			Members: numberOfReplicas,
			Version: "5.0.9",
			Type:    v1.ReplicaSet,
			Users: []v1.MongoDBUser{
				{
					Name: simpleMongoDBCommunity.Name,
					DB:   "admin",
					PasswordSecretRef: v1.SecretKeyReference{
						Name: passwordSecretName,
						Key:  passwordKeyName,
					},
					Roles: []v1.Role{
						{
							DB:   "admin",
							Name: "clusterAdmin",
						},
						{
							DB:   "admin",
							Name: "userAdminAnyDatabase",
						},
					},
					ScramCredentialsSecretName: simpleMongoDBCommunity.Name,
				},
			},
			Security: v1.Security{
				Authentication: v1.Authentication{
					Modes: []v1.AuthMode{"SCRAM"},
				},
			},
		},
	}
	err = r.Get(context.TODO(), types.NamespacedName{
		Namespace: req.Namespace,
		Name:      mdb.ObjectMeta.Name,
	}, &mdb)
	if apiErrors.IsNotFound(err) {
		err = r.Create(context.TODO(), &mdb)
		if err != nil {
			r.Log.Error(err)
			return result.Failed()
		}
	} else {
		err = r.Update(context.TODO(), &mdb)
		if err != nil {
			r.Log.Error(err)
			return result.Failed()
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleMongoDBCommunityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mongodbcommunityv1alpha1.SimpleMongoDBCommunity{}).
		Complete(r)
}
