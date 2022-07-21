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
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"go.uber.org/zap"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

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

	mdb := mongodbcommunityv1alpha1.SimpleMongoDBCommunity{}
	err := r.Get(context.TODO(), req.NamespacedName, &mdb)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return result.OK()
		}
		logger.Error(err, "Error reconciling MongoDB resource: %s", req.Name)
		return result.Failed()
	}

	numberOfReplicas, err := calculateNumberOfReplicas(r.Client)
	r.Log.Infof("Number of replicas %v", numberOfReplicas)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SimpleMongoDBCommunityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mongodbcommunityv1alpha1.SimpleMongoDBCommunity{}).
		Complete(r)
}
