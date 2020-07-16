package mongodbtests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/util/wait"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/controller/mongodb"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func StatefulSetHasOwnerReference(mdb *mdbv1.MongoDB, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{}
		err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: f.Global.OperatorNamespace}, &sts)
		if err != nil {
			t.Fatal(err)
		}
		ownerReferences := sts.GetOwnerReferences()

		assert.Len(t, ownerReferences, 1, "StatefulSet doesn't have OwnerReferences")

		assert.Equal(t, expectedOwnerReference.APIVersion, ownerReferences[0].APIVersion)
		assert.Equal(t, "MongoDB", ownerReferences[0].Kind)
		assert.Equal(t, expectedOwnerReference.Name, ownerReferences[0].Name)
		assert.Equal(t, expectedOwnerReference.UID, ownerReferences[0].UID)

		t.Logf("StatefulSet %s/%s has the correct OwnerReference!", mdb.Namespace, mdb.Name)
	}
}

// StatefulSetHasUpdateStrategy verifies that the StatefulSet holding this MongoDB
// resource has the correct Update Strategy
func StatefulSetHasUpdateStrategy(mdb *mdbv1.MongoDB, strategy appsv1.StatefulSetUpdateStrategyType) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForStatefulSetToHaveUpdateStrategy(t, mdb, strategy, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// MongoDBReachesRunningPhase ensure the MongoDB resource reaches the Running phase
func MongoDBReachesRunningPhase(mdb *mdbv1.MongoDB) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForMongoDBToReachPhase(t, mdb, mdbv1.Running, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is Running!", mdb.Namespace, mdb.Name)
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

func AutomationConfigVersionHasTheExpectedVersion(mdb *mdbv1.MongoDB, expectedVersion int) func(t *testing.T) {
	return func(t *testing.T) {
		currentCm := corev1.ConfigMap{}
		currentAc := automationconfig.AutomationConfig{}
		err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &currentCm)
		assert.NoError(t, err)
		err = json.Unmarshal([]byte(currentCm.Data[mongodb.AutomationConfigKey]), &currentAc)
		assert.NoError(t, err)
		assert.Equal(t, expectedVersion, currentAc.Version)
	}
}

// CreateMongoDBResource creates the MongoDB resource
func CreateMongoDBResource(mdb *mdbv1.MongoDB, ctx *f.Context) func(*testing.T) {
	return func(t *testing.T) {
		if err := f.Global.Client.Create(context.TODO(), mdb, &f.CleanupOptions{TestContext: ctx}); err != nil {
			t.Fatal(err)
		}
		t.Logf("Created MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
	}
}

func BasicFunctionality(mdb *mdbv1.MongoDB) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Config Map Was Correctly Created", AutomationConfigConfigMapExists(mdb))
		t.Run("Stateful Set Reaches Ready State", StatefulSetIsReady(mdb))
		t.Run("MongoDB Reaches Running Phase", MongoDBReachesRunningPhase(mdb))
		t.Run("Stateful Set has OwnerReference", StatefulSetHasOwnerReference(mdb,
			*metav1.NewControllerRef(mdb, schema.GroupVersionKind{
				Group:   mdbv1.SchemeGroupVersion.Group,
				Version: mdbv1.SchemeGroupVersion.Version,
				Kind:    mdb.Kind,
			})))
		t.Run("Test Status Was Updated", Status(mdb,
			mdbv1.MongoDBStatus{
				MongoURI: mdb.MongoURI(),
				Phase:    mdbv1.Running,
			}))
	}
}

// DeletePod will delete a pod that belongs to this MongoDB resource's StatefulSet
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

// Connectivity returns a test function which performs
// a basic MongoDB connectivity test
func Connectivity(mdb *mdbv1.MongoDB, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		if err := Connect(mdb, options.Client().SetAuth(options.Credential{
			AuthMechanism: "SCRAM-SHA-256",
			Username:      username,
			Password:      password,
		})); err != nil {
			t.Fatal(fmt.Sprintf("Error connecting to MongoDB deployment: %+v", err))
		}
	}
}

// Status compares the given status to the actual status of the MongoDB resource
func Status(mdb *mdbv1.MongoDB, expectedStatus mdbv1.MongoDBStatus) func(t *testing.T) {
	return func(t *testing.T) {
		if err := f.Global.Client.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, mdb); err != nil {
			t.Fatal(fmt.Errorf("error getting MongoDB resource: %+v", err))
		}
		assert.Equal(t, expectedStatus, mdb.Status)
	}
}

// Scale update the MongoDB with a new number of members and updates the resource
func Scale(mdb *mdbv1.MongoDB, newMembers int) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Scaling Mongodb %s, to %d members", mdb.Name, newMembers)
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDB) {
			db.Spec.Members = newMembers
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ChangeVersion(mdb *mdbv1.MongoDB, newVersion string) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Changing versions from: %s to %s", mdb.Spec.Version, newVersion)
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDB) {
			db.Spec.Version = newVersion
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Connect performs a connectivity check by initializing a mongo client
// and inserting a document into the MongoDB resource. Custom client
// options can be passed, for example to configure TLS.
func Connect(mdb *mdbv1.MongoDB, opts *options.ClientOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	mongoClient, err := mongo.Connect(ctx, opts.ApplyURI(mdb.MongoURI()))
	if err != nil {
		return err
	}

	return wait.Poll(time.Second*1, time.Second*30, func() (done bool, err error) {
		collection := mongoClient.Database("testing").Collection("numbers")
		_, err = collection.InsertOne(ctx, bson.M{"name": "pi", "value": 3.14159})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
}

// IsReachableDuring periodically tests connectivity to the provided MongoDB resource
// during execution of the provided functions. This function can be used to ensure
// The MongoDB is up throughout the test.
func IsReachableDuring(mdb *mdbv1.MongoDB, interval time.Duration, username, password string, testFunc func()) func(*testing.T) {
	return IsReachableDuringWithConnection(mdb, interval, testFunc, func() error {
		return Connect(mdb, options.Client().SetAuth(options.Credential{
			AuthMechanism: "SCRAM-SHA-256",
			Username:      username,
			Password:      password,
		}))
	})
}

func IsReachableDuringWithConnection(mdb *mdbv1.MongoDB, interval time.Duration, testFunc func(), connectFunc func() error) func(*testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background()) // start a go routine which will periodically check basic MongoDB connectivity
		defer cancel()

		// once all the test functions have been executed, the go routine will be cancelled
		go func() { //nolint
			for {
				select {
				case <-ctx.Done():
					t.Log("context cancelled, no longer checking connectivity") //nolint
					return
				case <-time.After(interval):
					if err := connectFunc(); err != nil {
						t.Fatal(fmt.Sprintf("error reaching MongoDB deployment: %+v", err))
					} else {
						t.Logf("Successfully connected to %s", mdb.Name)
					}
				}
			}
		}()
		testFunc()
	}
}
