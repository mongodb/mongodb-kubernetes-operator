package mongodbtests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller/mongodb"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetIsReady ensures that the underlying stateful set
// reaches the running state
func StatefulSetIsReady(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForStatefulSetToBeReady(t, mdb, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

func AutomationConfigConfigMapExists(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		cm, err := e2eutil.WaitForConfigMapToExist(mdb.ConfigMapName(), time.Second*5, time.Minute*1)
		assert.NoError(t, err)

		t.Logf("ConfigMap %s/%s was successfully created", mdb.ConfigMapName(), mdb.Namespace)
		assert.Contains(t, cm.Data, mongodb.AutomationConfigKey)

		t.Log("The ConfigMap contained the automation config")
	}
}

func CreateOrUpdateResource(mdb *mdbv1.MongoDB, ctx *f.TestCtx) func(*testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.CreateOrUpdateMongoDB(mdb, ctx); err != nil {
			t.Fatal(err)
		}
		t.Logf("Created MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
	}
}

func DeletePod(mdb *mdbv1.MongoDB, podNum int) func(*testing.T) {
	return func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", mdb.Name, podNum),
				Namespace: mdb.Namespace,
			},
		}
		if err := f.Global.Client.Delete(context.TODO(), &pod); err != nil {
			t.Fatal(err)
		}

		t.Logf("pod %s/%s deleted", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	}
}

// BasicConnectivity performs a check by initializing a mongo client
// and inserting a document into the MongoDB resource
func BasicConnectivity(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Minute)
		mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mdb.MongoURI()))
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Created mongo client!")

		var res *mongo.InsertOneResult
		err = wait.Poll(time.Second*5, time.Minute*1, func() (done bool, err error) {
			collection := mongoClient.Database("testing").Collection("numbers")
			res, err = collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
			if err != nil {
				t.Logf("error inserting document: %+v", err)
				return false, err
			}
			return true, nil
		})

		if err != nil {
			t.Fatal(err)
		}
		t.Logf("inserted ID: %+v", res.InsertedID)
	}
}

func Scale(mdb *mdbv1.MongoDB, newMembers int, ctx *f.TestCtx) func(*testing.T) {
	return func(t *testing.T) {
		mdb.Spec.Members = newMembers
		t.Logf("Scaling Mongodb %s, to %d members", mdb.Name, mdb.Spec.Members)
		if err := e2eutil.CreateOrUpdateMongoDB(mdb, ctx); err != nil {
			t.Fatal(err)
		}
	}
}

// IsReachableDuring periodically tests connectivity to the provided MongoDB resource
// during execution of the provided functions. This function can be used to ensure
// The MongoDB is up throughout the test.
func IsReachableDuring(mdb *mdbv1.MongoDB, testFunc func()) func(*testing.T) {
	return func(t *testing.T) {
		quit := make(chan struct{})
		go func() {
			for {
				select {
				case <-quit:
					return
				default:
					BasicConnectivity(mdb)(t)
					time.Sleep(time.Second * 10)
				}
			}
		}()

		testFunc()
		close(quit)
	}
}
