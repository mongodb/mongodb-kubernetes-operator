package e2eutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RegisterTypesWithFramework(newTypes ...runtime.Object) error {
	for _, newType := range newTypes {
		if err := f.AddToFrameworkScheme(apis.AddToScheme, newType); err != nil {
			return fmt.Errorf("failed to add custom resource type %s to framework scheme: %v", newType.GetObjectKind(), err)
		}
	}
	return nil
}

func CreateRuntimeObject(obj runtime.Object, ctx *f.TestCtx) error {
	return f.Global.Client.Create(context.TODO(), obj, &f.CleanupOptions{TestContext: ctx})
}

// waitForConfigMapToExist waits until a ConfigMap of the given name exists
// using the provided retryInterval and timeout
func WaitForConfigMapToExist(cmName string, retryInterval, timeout time.Duration) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	return cm, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &cm)
}

// waitForStatefulSetToExist waits until a StatefulSet of the given name exists
// using the provided retryInterval and timeout
func WaitForStatefulSetToExist(stsName string, retryInterval, timeout time.Duration) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	return sts, waitForRuntimeObjectToExist(stsName, retryInterval, timeout, &sts)
}

// waitForStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func WaitForStatefulSetToBeReady(t *testing.T, stsName string, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, stsName, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return *sts.Spec.Replicas == sts.Status.ReadyReplicas
	})
}

func WaitForStatefulSetToNotBeReady(t *testing.T, stsName string, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, stsName, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return *sts.Spec.Replicas != sts.Status.ReadyReplicas
	})
}

func waitForStatefulSetCondition(t *testing.T, stsName string, retryInterval, timeout time.Duration, condition func(set appsv1.StatefulSet) bool) error {
	_, err := WaitForStatefulSetToExist(stsName, retryInterval, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for stateful set to be created: %s", err)
	}

	sts := appsv1.StatefulSet{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: stsName, Namespace: f.Global.Namespace}, &sts)
		if err != nil {
			return false, err
		}
		t.Logf("Waiting for %s to have %d replicas. Current ready replicas: %d\n", stsName, *sts.Spec.Replicas, sts.Status.ReadyReplicas)
		ready := condition(sts)
		return ready, nil
	})
}

// waitForRuntimeObjectToExist waits until a runtime.Object of the given name exists
// using the provided retryInterval and timeout provided.
func waitForRuntimeObjectToExist(name string, retryInterval, timeout time.Duration, obj runtime.Object) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: f.Global.Namespace}, obj)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return true, nil
	})
}

func NewTestMongoDB() mdbv1.MongoDB {
	return mdbv1.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-mongodb",
			Namespace: f.Global.Namespace,
		},
		Spec: mdbv1.MongoDBSpec{
			Members: 3,
			Type:    "ReplicaSet",
			Version: "4.0.6",
		},
	}
}
