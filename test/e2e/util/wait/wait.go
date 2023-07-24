package wait

import (
	"context"
	"fmt"
	"testing"
	"time"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatefulSetType int

const (
	MembersStatefulSet StatefulSetType = iota
	ArbitersStatefulSet
)

// ForConfigMapToExist waits until a ConfigMap of the given name exists
// using the provided retryInterval and timeout
func ForConfigMapToExist(cmName string, retryInterval, timeout time.Duration) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	return cm, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &cm, e2eutil.OperatorNamespace)
}

// ForSecretToExist waits until a Secret of the given name exists
// using the provided retryInterval and timeout
func ForSecretToExist(cmName string, retryInterval, timeout time.Duration, namespace string) (corev1.Secret, error) {
	s := corev1.Secret{}
	return s, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &s, namespace)
}

// ForMongoDBToReachPhase waits until the given MongoDB resource reaches the expected phase
func ForMongoDBToReachPhase(t *testing.T, mdb *mdbv1.MongoDBCommunity, phase mdbv1.Phase, retryInterval, timeout time.Duration) error {
	return waitForMongoDBCondition(mdb, retryInterval, timeout, func(db mdbv1.MongoDBCommunity) bool {
		t.Logf("current phase: %s, waiting for phase: %s", db.Status.Phase, phase)
		return db.Status.Phase == phase
	})
}

// ForMongoDBMessageStatus waits until the given MongoDB resource gets the expected message status
func ForMongoDBMessageStatus(t *testing.T, mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration, message string) error {
	return waitForMongoDBCondition(mdb, retryInterval, timeout, func(db mdbv1.MongoDBCommunity) bool {
		t.Logf("current message: %s, waiting for message: %s", db.Status.Message, message)
		return db.Status.Message == message
	})
}

// waitForMongoDBCondition polls and waits for a given condition to be true
func waitForMongoDBCondition(mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration, condition func(mdbv1.MongoDBCommunity) bool) error {
	mdbNew := mdbv1.MongoDBCommunity{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = e2eutil.TestClient.Get(context.TODO(), mdb.NamespacedName(), &mdbNew)
		if err != nil {
			return false, err
		}
		ready := condition(mdbNew)
		return ready, nil
	})
}

// ForDeploymentToExist waits until a Deployment of the given name exists
// using the provided retryInterval and timeout
func ForDeploymentToExist(deployName string, retryInterval, timeout time.Duration, namespace string) (appsv1.Deployment, error) {
	deploy := appsv1.Deployment{}
	return deploy, waitForRuntimeObjectToExist(deployName, retryInterval, timeout, &deploy, namespace)
}

// ForStatefulSetToExist waits until a StatefulSet of the given name exists
// using the provided retryInterval and timeout
func ForStatefulSetToExist(stsName string, retryInterval, timeout time.Duration, namespace string) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	return sts, waitForRuntimeObjectToExist(stsName, retryInterval, timeout, &sts, namespace)
}

// ForStatefulSetToBeDeleted waits until a StatefulSet of the given name is deleted
// using the provided retryInterval and timeout
func ForStatefulSetToBeDeleted(stsName string, retryInterval, timeout time.Duration, namespace string) error {
	sts := appsv1.StatefulSet{}
	return waitForRuntimeObjectToBeDeleted(stsName, retryInterval, timeout, &sts, namespace)
}

// ForStatefulSetToHaveUpdateStrategy waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func ForStatefulSetToHaveUpdateStrategy(t *testing.T, mdb *mdbv1.MongoDBCommunity, strategy appsv1.StatefulSetUpdateStrategyType, opts ...Configuration) error {
	options := newOptions(opts...)
	return waitForStatefulSetCondition(t, mdb, options, func(sts appsv1.StatefulSet) bool {
		return sts.Spec.UpdateStrategy.Type == strategy
	})
}

// ForStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func ForStatefulSetToBeReady(t *testing.T, mdb *mdbv1.MongoDBCommunity, opts ...Configuration) error {
	options := newOptions(opts...)
	return waitForStatefulSetCondition(t, mdb, options, func(sts appsv1.StatefulSet) bool {
		return statefulset.IsReady(sts, mdb.Spec.Members)
	})
}

// ForStatefulSetToBeUnready waits until all replicas of the StatefulSet with the given name
// is not ready.
func ForStatefulSetToBeUnready(t *testing.T, mdb *mdbv1.MongoDBCommunity, opts ...Configuration) error {
	options := newOptions(opts...)
	return waitForStatefulSetCondition(t, mdb, options, func(sts appsv1.StatefulSet) bool {
		return !statefulset.IsReady(sts, mdb.Spec.Members)
	})
}

// ForArbitersStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status.
func ForArbitersStatefulSetToBeReady(t *testing.T, mdb *mdbv1.MongoDBCommunity, opts ...Configuration) error {
	options := newOptions(opts...)
	return waitForStatefulSetConditionWithSpecificSts(t, mdb, ArbitersStatefulSet, options, func(sts appsv1.StatefulSet) bool {
		return statefulset.IsReady(sts, mdb.Spec.Arbiters)
	})
}

// ForStatefulSetToBeReadyAfterScaleDown waits for just the ready replicas to be correct
// and does not account for the updated replicas
func ForStatefulSetToBeReadyAfterScaleDown(t *testing.T, mdb *mdbv1.MongoDBCommunity, opts ...Configuration) error {
	options := newOptions(opts...)
	return waitForStatefulSetCondition(t, mdb, options, func(sts appsv1.StatefulSet) bool {
		return int32(mdb.Spec.Members) == sts.Status.ReadyReplicas
	})
}

func waitForStatefulSetConditionWithSpecificSts(t *testing.T, mdb *mdbv1.MongoDBCommunity, statefulSetType StatefulSetType, waitOpts Options, condition func(set appsv1.StatefulSet) bool) error {
	_, err := ForStatefulSetToExist(mdb.Name, waitOpts.RetryInterval, waitOpts.Timeout, mdb.Namespace)
	if err != nil {
		return fmt.Errorf("error waiting for stateful set to be created: %s", err)
	}

	sts := appsv1.StatefulSet{}
	name := mdb.NamespacedName()
	if statefulSetType == ArbitersStatefulSet {
		name = mdb.ArbiterNamespacedName()
	}
	return wait.Poll(waitOpts.RetryInterval, waitOpts.Timeout, func() (done bool, err error) {
		err = e2eutil.TestClient.Get(context.TODO(), name, &sts)
		if err != nil {
			return false, err
		}
		t.Logf("Waiting for %s to have %d replicas. Current ready replicas: %d, Current updated replicas: %d, Current generation: %d, Observed Generation: %d\n",
			name, *sts.Spec.Replicas, sts.Status.ReadyReplicas, sts.Status.UpdatedReplicas, sts.Generation, sts.Status.ObservedGeneration)
		ready := condition(sts)
		return ready, nil
	})
}

func waitForStatefulSetCondition(t *testing.T, mdb *mdbv1.MongoDBCommunity, waitOpts Options, condition func(set appsv1.StatefulSet) bool) error {
	// uses members statefulset
	return waitForStatefulSetConditionWithSpecificSts(t, mdb, MembersStatefulSet, waitOpts, condition)
}

func waitForStatefulSetConditionArbiters(t *testing.T, mdb *mdbv1.MongoDBCommunity, waitOpts Options, condition func(set appsv1.StatefulSet) bool) error {
	// uses members statefulset
	return waitForStatefulSetConditionWithSpecificSts(t, mdb, ArbitersStatefulSet, waitOpts, condition)
}

func ForPodReadiness(t *testing.T, isReady bool, containerName string, timeout time.Duration, pod corev1.Pod) error {
	return wait.Poll(time.Second*3, timeout, func() (done bool, err error) {
		err = e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod)
		if err != nil {
			return false, err
		}
		for _, status := range pod.Status.ContainerStatuses {
			t.Logf("%s (%s), ready: %v\n", pod.Name, status.Name, status.Ready)
			if status.Name == containerName && status.Ready == isReady {
				return true, nil
			}
		}
		return false, nil
	})
}

func ForPodPhase(t *testing.T, timeout time.Duration, pod corev1.Pod, podPhase corev1.PodPhase) error {
	return wait.Poll(time.Second*3, timeout, func() (done bool, err error) {
		err = e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &pod)
		if err != nil {
			return false, err
		}
		t.Logf("Current phase %s, expected phase %s", pod.Status.Phase, podPhase)
		return pod.Status.Phase == podPhase, nil
	})
}

// waitForRuntimeObjectToExist waits until a runtime.Object of the given name exists
// using the provided retryInterval and timeout provided.
func waitForRuntimeObjectToExist(name string, retryInterval, timeout time.Duration, obj client.Object, namespace string) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		return runtimeObjectExists(name, obj, namespace)
	})
}

// waitForRuntimeObjectToBeDeleted waits until a runtime.Object of the given name is deleted
// using the provided retryInterval and timeout provided.
func waitForRuntimeObjectToBeDeleted(name string, retryInterval, timeout time.Duration, obj client.Object, namespace string) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		exists, err := runtimeObjectExists(name, obj, namespace)
		return !exists, err
	})
}

// runtimeObjectExists checks if a runtime.Object of the given name exists
func runtimeObjectExists(name string, obj client.Object, namespace string) (bool, error) {
	err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}
