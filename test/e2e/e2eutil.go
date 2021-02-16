package e2eutil

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const TestdataDir = "/workspace/testdata/tls"

// UpdateMongoDBResource applies the provided function to the most recent version of the MongoDB resource
// and retries when there are conflicts
func UpdateMongoDBResource(original *mdbv1.MongoDBCommunity, updateFunc func(*mdbv1.MongoDBCommunity)) error {
	err := TestClient.Get(context.TODO(), types.NamespacedName{Name: original.Name, Namespace: original.Namespace}, original)
	if err != nil {
		return err
	}

	updateFunc(original)

	return TestClient.Update(context.TODO(), original)
}

// WaitForConfigMapToExist waits until a ConfigMap of the given name exists
// using the provided retryInterval and timeout
func WaitForConfigMapToExist(cmName string, retryInterval, timeout time.Duration) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	return cm, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &cm, OperatorNamespace)
}

// WaitForSecretToExist waits until a Secret of the given name exists
// using the provided retryInterval and timeout
func WaitForSecretToExist(cmName string, retryInterval, timeout time.Duration, namespace string) (corev1.Secret, error) {
	s := corev1.Secret{}
	return s, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &s, namespace)
}

// WaitForMongoDBToReachPhase waits until the given MongoDB resource reaches the expected phase
func WaitForMongoDBToReachPhase(t *testing.T, mdb *mdbv1.MongoDBCommunity, phase mdbv1.Phase, retryInterval, timeout time.Duration) error {
	return waitForMongoDBCondition(mdb, retryInterval, timeout, func(db mdbv1.MongoDBCommunity) bool {
		t.Logf("current phase: %s, waiting for phase: %s", db.Status.Phase, phase)
		return db.Status.Phase == phase
	})
}

// waitForMongoDBCondition polls and waits for a given condition to be true
func waitForMongoDBCondition(mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration, condition func(mdbv1.MongoDBCommunity) bool) error {
	mdbNew := mdbv1.MongoDBCommunity{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = TestClient.Get(context.TODO(), mdb.NamespacedName(), &mdbNew)
		if err != nil {
			return false, err
		}
		ready := condition(mdbNew)
		return ready, nil
	})
}

// WaitForStatefulSetToExist waits until a StatefulSet of the given name exists
// using the provided retryInterval and timeout
func WaitForStatefulSetToExist(stsName string, retryInterval, timeout time.Duration, namespace string) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	return sts, waitForRuntimeObjectToExist(stsName, retryInterval, timeout, &sts, namespace)
}

// WaitForStatefulSetToHaveUpdateStrategy waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func WaitForStatefulSetToHaveUpdateStrategy(t *testing.T, mdb *mdbv1.MongoDBCommunity, strategy appsv1.StatefulSetUpdateStrategyType, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, mdb, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return sts.Spec.UpdateStrategy.Type == strategy
	})
}

// WaitForStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func WaitForStatefulSetToBeReady(t *testing.T, mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, mdb, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return statefulset.IsReady(sts, mdb.Spec.Members)
	})
}

// WaitForStatefulSetToBeReadyAfterScaleDown waits for just the ready replicas to be correct
// and does not account for the updated replicas
func WaitForStatefulSetToBeReadyAfterScaleDown(t *testing.T, mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration) error {
	return waitForStatefulSetCondition(t, mdb, retryInterval, timeout, func(sts appsv1.StatefulSet) bool {
		return int32(mdb.Spec.Members) == sts.Status.ReadyReplicas
	})
}

func waitForStatefulSetCondition(t *testing.T, mdb *mdbv1.MongoDBCommunity, retryInterval, timeout time.Duration, condition func(set appsv1.StatefulSet) bool) error {
	_, err := WaitForStatefulSetToExist(mdb.Name, retryInterval, timeout, mdb.Namespace)
	if err != nil {
		return errors.Errorf("error waiting for stateful set to be created: %s", err)
	}

	sts := appsv1.StatefulSet{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = TestClient.Get(context.TODO(), mdb.NamespacedName(), &sts)
		if err != nil {
			return false, err
		}
		t.Logf("Waiting for %s to have %d replicas. Current ready replicas: %d, Current updated replicas: %d\n",
			mdb.Name, mdb.Spec.Members, sts.Status.ReadyReplicas, sts.Status.UpdatedReplicas)
		ready := condition(sts)
		return ready, nil
	})
}

// waitForRuntimeObjectToExist waits until a runtime.Object of the given name exists
// using the provided retryInterval and timeout provided.
func waitForRuntimeObjectToExist(name string, retryInterval, timeout time.Duration, obj client.Object, namespace string) error {
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = TestClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return true, nil
	})
}

func NewTestMongoDB(name string, namespace string) (mdbv1.MongoDBCommunity, mdbv1.MongoDBUser) {
	mongodbNamespace := namespace
	if mongodbNamespace == "" {
		mongodbNamespace = OperatorNamespace
	}
	mdb := mdbv1.MongoDBCommunity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: mongodbNamespace,
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members:                     3,
			Type:                        "ReplicaSet",
			Version:                     "4.4.0",
			FeatureCompatibilityVersion: "4.4",
			Security: mdbv1.Security{
				Authentication: mdbv1.Authentication{
					Modes: []mdbv1.AuthMode{"SCRAM"},
				},
			},
			AdditionalMongodConfig: mdbv1.MongodConfiguration{
				Object: map[string]interface{}{},
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
						// roles on testing db for general connectivity
						{
							DB:   "testing",
							Name: "readWrite",
						},
						{
							DB:   "testing",
							Name: "clusterAdmin",
						},
						// admin roles for reading FCV
						{
							DB:   "admin",
							Name: "readWrite",
						},
						{
							DB:   "admin",
							Name: "clusterAdmin",
						},
						{
							DB:   "admin",
							Name: "userAdmin",
						},
					},
					ScramCredentialsSecretName: fmt.Sprintf("%s-my-scram", name),
				},
			},
		},
	}
	return mdb, mdb.Spec.Users[0]
}

func NewTestTLSConfig(optional bool) mdbv1.TLS {
	return mdbv1.TLS{
		Enabled:  true,
		Optional: optional,
		CertificateKeySecret: mdbv1.LocalObjectReference{
			Name: "test-tls-secret",
		},
		CaConfigMap: mdbv1.LocalObjectReference{
			Name: "test-tls-ca",
		},
	}
}

func ensureObject(ctx *Context, obj k8sClient.Object) error {
	key := k8sClient.ObjectKeyFromObject(obj)

	err := TestClient.Get(context.TODO(), key, obj)
	if err != nil {
		if !apiErrors.IsNotFound(err) {
			return err
		}
		err = TestClient.Create(context.TODO(), obj, &CleanupOptions{TestContext: ctx})
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("%s %s/%s already exists!\n", reflect.TypeOf(obj), key.Namespace, key.Name)
		err = TestClient.Update(context.TODO(), obj)
		if err != nil {
			return err
		}
	}
	return nil
}

// EnsureNamespace checks that the given namespace exists and creates it if not.
func EnsureNamespace(ctx *Context, namespace string) error {
	return ensureObject(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
}

// EnsureServiceAccount checks that the given ServiceAccount exists and creates it if not.
func EnsureServiceAccount(ctx *Context, namespace string, svcAcctName string) error {
	return ensureObject(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcAcctName,
			Namespace: namespace,
		},
	})
}

// EnsureRole checks that the given role exists and creates it with the given rules if not.
func EnsureRole(ctx *Context, namespace string, roleName string, rules []rbacv1.PolicyRule) error {
	return ensureObject(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      roleName,
		},
		Rules: rules,
	})
}

// EnsureClusterRole checks that the given cluster role exists and creates it with the given rules if not.
func EnsureClusterRole(ctx *Context, namespace string, roleName string, rules []rbacv1.PolicyRule) error {
	return ensureObject(ctx, &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      roleName,
		},
		Rules: rules,
	})
}

// EnsureRoleBinding checks that the given role binding exists and creates it with the given subjects and roleRef if not.
func EnsureRoleBinding(ctx *Context, namespace string, roleBindingName string, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) error {
	return ensureObject(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      roleBindingName,
		},
		Subjects: subjects,
		RoleRef:  roleRef,
	})
}

// EnsureClusterRoleBinding checks that the given cluster role exists and creates it with the given subjects and roleRef if not.
func EnsureClusterRoleBinding(ctx *Context, namespace string, roleBindingName string, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef) error {
	return ensureObject(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      roleBindingName,
		},
		Subjects: subjects,
		RoleRef:  roleRef,
	})
}
