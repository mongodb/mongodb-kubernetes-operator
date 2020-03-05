package e2e

import (
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller/mongodb"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newTestMongoDB() mdbv1.MongoDB {
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

func TestReplicaSet(t *testing.T) {
	ctx := f.NewTestCtx(t)
	defer ctx.Cleanup()

	// register our types with the testing framework
	if err := f.AddToFrameworkScheme(apis.AddToScheme, &mdbv1.MongoDB{}); err != nil {
		t.Fatal(fmt.Errorf("failed to add custom resource scheme to framework: %v", err))
	}

	t.Run("Create MongoDB Replica Set", func(t *testing.T) {
		mdb := newTestMongoDB()
		err := f.Global.Client.Create(context.TODO(), &mdb, &f.CleanupOptions{TestContext: ctx})
		assert.NoError(t, err)

		t.Logf("Created MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
		cm, err := waitForConfigMapToExist(mdb.ConfigMapName(), time.Second*5, time.Minute*1)
		assert.NoError(t, err)

		t.Logf("ConfigMap %s/%s was successfully created", mdb.ConfigMapName(), mdb.Namespace)
		assert.Contains(t, cm.Data, mongodb.AutomationConfigKey)

		t.Log("The ConfigMap contained the automation config")

		err = waitForStatefulSetToBeReady(t, mdb.Name, time.Second*5, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("StatefulSet %s/%s successfully created!", mdb.Namespace, mdb.Name)

		ctx, _ := context.WithTimeout(context.Background(), 10*time.Minute)
		mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mdb.MongoURI()))
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Created mongo client!")

		collection := mongoClient.Database("testing").Collection("numbers")
		res, err := collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("inserted ID: %+v", res.InsertedID)
	})
}

// waitForConfigMapToExist waits until a ConfigMap of the given name exists
// using the provided retryInterval and timeout
func waitForConfigMapToExist(cmName string, retryInterval, timeout time.Duration) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{}
	return cm, waitForRuntimeObjectToExist(cmName, retryInterval, timeout, &cm)
}

// waitForStatefulSetToExist waits until a StatefulSet of the given name exists
// using the provided retryInterval and timeout
func waitForStatefulSetToExist(stsName string, retryInterval, timeout time.Duration) (appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	return sts, waitForRuntimeObjectToExist(stsName, retryInterval, timeout, &sts)
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

// waitForStatefulSetToBeReady waits until all replicas of the StatefulSet with the given name
// have reached the ready status
func waitForStatefulSetToBeReady(t *testing.T, stsName string, retryInterval, timeout time.Duration) error {
	_, err := waitForStatefulSetToExist(stsName, retryInterval, timeout)
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
		ready := *sts.Spec.Replicas == sts.Status.ReadyReplicas
		return ready, nil
	})
}
