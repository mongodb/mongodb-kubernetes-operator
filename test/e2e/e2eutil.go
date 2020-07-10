package e2eutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateMongoDBResource applies the provided function to the most recent version of the MongoDB resource
// and retries when there are conflicts
func UpdateMongoDBResource(original *mdbv1.MongoDB, updateFunc func(*mdbv1.MongoDB)) error {
	err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: original.Name, Namespace: original.Namespace}, original)
	if err != nil {
		return err
	}

	updateFunc(original)

	return f.Global.Client.Update(context.TODO(), original)
}

// WaitForConfigMapToExist waits until a ConfigMap of the given name exists
// using the provided retryInterval and timeout
func WaitForConfigMapToExist(cmName string, retryInterval, timeout time.Duration) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	return cm, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &cm)
}

// WaitForMongoDBToReachPhase waits until the given MongoDB resource reaches the expected phase
func WaitForMongoDBToReachPhase(t *testing.T, mdb *mdbv1.MongoDB, phase mdbv1.Phase, retryInterval, timeout time.Duration) error {
	return waitForMongoDBCondition(mdb, retryInterval, timeout, func(db mdbv1.MongoDB) bool {
		t.Logf("current phase: %s, waiting for phase: %s", db.Status.Phase, phase)
		return db.Status.Phase == phase
	})
}

// waitForMongoDBCondition polls and waits for a given condition to be true
func waitForMongoDBCondition(mdb *mdbv1.MongoDB, retryInterval, timeout time.Duration, condition func(mdbv1.MongoDB) bool) error {
	mdbNew := mdbv1.MongoDB{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: f.Global.OperatorNamespace}, &mdbNew)
		if err != nil {
			return false, err
		}
		ready := condition(mdbNew)
		return ready, nil
	})
}

// WaitForStatefulSetToExist waits until a StatefulSet of the given name exists
// using the provided retryInterval and timeout
func WaitForStatefulSetToExist(stsName string, retryInterval, timeout time.Duration) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	return sts, waitForRuntimeObjectToExist(stsName, retryInterval, timeout, &sts)
}

// WaitForStatefulSetToHaveUpdateStrategy waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func WaitForStatefulSetToHaveUpdateStrategy(t *testing.T, mdb *mdbv1.MongoDB, strategy appsv1.StatefulSetUpdateStrategyType, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, mdb, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return sts.Spec.UpdateStrategy.Type == strategy
	})
}

// WaitForStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func WaitForStatefulSetToBeReady(t *testing.T, mdb *mdbv1.MongoDB, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, mdb, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return statefulset.IsReady(sts, mdb.Spec.Members)
	})
}

func waitForStatefulSetCondition(t *testing.T, mdb *mdbv1.MongoDB, retryInterval, timeout time.Duration, condition func(set appsv1.StatefulSet) bool) error {
	_, err := WaitForStatefulSetToExist(mdb.Name, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for stateful set to be created: %s", err)
	}

	sts := appsv1.StatefulSet{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: f.Global.OperatorNamespace}, &sts)
		if err != nil {
			return false, err
		}
		t.Logf("Waiting for %s to have %d replicas. Current ready replicas: %d\n", mdb.Name, mdb.Spec.Members, sts.Status.ReadyReplicas)
		ready := condition(sts)
		return ready, nil
	})
}

// waitForRuntimeObjectToExist waits until a runtime.Object of the given name exists
// using the provided retryInterval and timeout provided.
func waitForRuntimeObjectToExist(name string, retryInterval, timeout time.Duration, obj runtime.Object) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: f.Global.OperatorNamespace}, obj)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return true, nil
	})
}

func NewTestMongoDB(name string) (mdbv1.MongoDB, mdbv1.MongoDBUser) {
	mdb := mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.Global.OperatorNamespace,
		},
		Spec: mdbv1.MongoDBSpec{
			Members:                     3,
			Type:                        "ReplicaSet",
			Version:                     "4.0.6",
			FeatureCompatibilityVersion: "4.0",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			Users: []mdbv1.MongoDBUser{
				{
					Name: fmt.Sprintf("%s-user", name),
					DB:   "admin",
					PasswordSecretRef: mdbv1.SecretKeyReference{
						Key:  fmt.Sprintf("%s-password", name),
						Name: fmt.Sprintf("%s-password-secret", name),
					},
					Roles: []mdbv1.Role{
						{
							DB:   "testing",
							Name: "readWrite",
						},
						{
							DB:   "testing",
							Name: "clusterAdmin",
						},
					},
				},
			},
		},
	}
	return mdb, mdb.Spec.Users[0]
}
