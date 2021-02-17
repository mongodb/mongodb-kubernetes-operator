package mongodbtests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/util/wait"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/controllers"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/automationconfig"
	e2eutil "github.com/mongodb/mongodb-kubernetes-operator/test/e2e"
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
func StatefulSetIsReady(mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return statefulSetIsReady(mdb, time.Second*15, time.Minute*5)
}

// StatefulSetIsReadyAfterScaleDown ensures that a replica set is scaled down correctly
// note: scaling down takes considerably longer than scaling up due the readiness probe
// failure threshold being high
func StatefulSetIsReadyAfterScaleDown(mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForStatefulSetToBeReadyAfterScaleDown(t, mdb, time.Second*60, time.Minute*45)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// StatefulSetIsReady ensures that the underlying stateful set
// reaches the running state
func statefulSetIsReady(mdb *mdbv1.MongoDBCommunity, interval time.Duration, timeout time.Duration) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForStatefulSetToBeReady(t, mdb, interval, timeout)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

func StatefulSetHasOwnerReference(mdb *mdbv1.MongoDBCommunity, expectedOwnerReference metav1.OwnerReference) func(t *testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{}
		err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
		if err != nil {
			t.Fatal(err)
		}
		ownerReferences := sts.GetOwnerReferences()

		assert.Len(t, ownerReferences, 1, "StatefulSet doesn't have OwnerReferences")

		assert.Equal(t, expectedOwnerReference.APIVersion, ownerReferences[0].APIVersion)
		assert.Equal(t, "MongoDBCommunity", ownerReferences[0].Kind)
		assert.Equal(t, expectedOwnerReference.Name, ownerReferences[0].Name)
		assert.Equal(t, expectedOwnerReference.UID, ownerReferences[0].UID)

		t.Logf("StatefulSet %s/%s has the correct OwnerReference!", mdb.Namespace, mdb.Name)
	}
}

// StatefulSetHasUpdateStrategy verifies that the StatefulSet holding this MongoDB
// resource has the correct Update Strategy
func StatefulSetHasUpdateStrategy(mdb *mdbv1.MongoDBCommunity, strategy appsv1.StatefulSetUpdateStrategyType) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForStatefulSetToHaveUpdateStrategy(t, mdb, strategy, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("StatefulSet %s/%s is ready!", mdb.Namespace, mdb.Name)
	}
}

// MongoDBReachesRunningPhase ensure the MongoDB resource reaches the Running phase
func MongoDBReachesRunningPhase(mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForMongoDBToReachPhase(t, mdb, mdbv1.Running, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is Running!", mdb.Namespace, mdb.Name)
	}
}

// MongoDBReachesFailed ensure the MongoDB resource reaches the Failed phase.
func MongoDBReachesFailedPhase(mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		err := e2eutil.WaitForMongoDBToReachPhase(t, mdb, mdbv1.Failed, time.Second*15, time.Minute*5)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("MongoDB %s/%s is in Failed state!", mdb.Namespace, mdb.Name)
	}
}

func AutomationConfigSecretExists(mdb *mdbv1.MongoDBCommunity) func(t *testing.T) {
	return func(t *testing.T) {
		s, err := e2eutil.WaitForSecretToExist(mdb.AutomationConfigSecretName(), time.Second*5, time.Minute*1, mdb.Namespace)
		assert.NoError(t, err)

		t.Logf("Secret %s/%s was successfully created", mdb.AutomationConfigSecretName(), mdb.Namespace)
		assert.Contains(t, s.Data, controllers.AutomationConfigKey)

		t.Log("The Secret contained the automation config")
	}
}

func getAutomationConfig(t *testing.T, mdb *mdbv1.MongoDBCommunity) automationconfig.AutomationConfig {
	currentSecret := corev1.Secret{}
	currentAc := automationconfig.AutomationConfig{}
	err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: mdb.AutomationConfigSecretName(), Namespace: mdb.Namespace}, &currentSecret)
	assert.NoError(t, err)
	err = json.Unmarshal(currentSecret.Data[controllers.AutomationConfigKey], &currentAc)
	assert.NoError(t, err)
	return currentAc
}

// AutomationConfigVersionHasTheExpectedVersion verifies that the automation config has the expected version.
func AutomationConfigVersionHasTheExpectedVersion(mdb *mdbv1.MongoDBCommunity, expectedVersion int) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(t, mdb)
		assert.Equal(t, expectedVersion, currentAc.Version)
	}
}

// AutomationConfigHasTheExpectedCustomRoles verifies that the automation config has the expected custom roles.
func AutomationConfigHasTheExpectedCustomRoles(mdb *mdbv1.MongoDBCommunity, roles []automationconfig.CustomRole) func(t *testing.T) {
	return func(t *testing.T) {
		currentAc := getAutomationConfig(t, mdb)
		assert.ElementsMatch(t, roles, currentAc.Roles)
	}
}

// CreateMongoDBResource creates the MongoDB resource
func CreateMongoDBResource(mdb *mdbv1.MongoDBCommunity, ctx *e2eutil.Context) func(*testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.TestClient.Create(context.TODO(), mdb, &e2eutil.CleanupOptions{TestContext: ctx}); err != nil {
			t.Fatal(err)
		}
		t.Logf("Created MongoDB resource %s/%s", mdb.Name, mdb.Namespace)
	}
}

func BasicFunctionality(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Config Map Was Correctly Created", AutomationConfigSecretExists(mdb))
		t.Run("Stateful Set Reaches Ready State", StatefulSetIsReady(mdb))
		t.Run("MongoDB Reaches Running Phase", MongoDBReachesRunningPhase(mdb))
		t.Run("Stateful Set has OwnerReference", StatefulSetHasOwnerReference(mdb,
			*metav1.NewControllerRef(mdb, schema.GroupVersionKind{
				Group:   mdbv1.GroupVersion.Group,
				Version: mdbv1.GroupVersion.Version,
				Kind:    mdb.Kind,
			})))
		t.Run("Test Status Was Updated", Status(mdb,
			mdbv1.MongoDBCommunityStatus{
				MongoURI:                   mdb.MongoURI(),
				Phase:                      mdbv1.Running,
				CurrentMongoDBMembers:      mdb.Spec.Members,
				CurrentStatefulSetReplicas: mdb.Spec.Members,
			}))
	}
}

// DeletePod will delete a pod that belongs to this MongoDB resource's StatefulSet
func DeletePod(mdb *mdbv1.MongoDBCommunity, podNum int) func(*testing.T) {
	return func(t *testing.T) {
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", mdb.Name, podNum),
				Namespace: mdb.Namespace,
			},
		}
		if err := e2eutil.TestClient.Delete(context.TODO(), &pod); err != nil {
			t.Fatal(err)
		}

		t.Logf("pod %s/%s deleted", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	}
}

// Connectivity returns a test function which performs
// a basic MongoDB connectivity test
func Connectivity(mdb *mdbv1.MongoDBCommunity, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		if err := Connect(mdb, options.Client().SetAuth(options.Credential{
			AuthMechanism: "SCRAM-SHA-256",
			Username:      username,
			Password:      password,
		})); err != nil {
			t.Fatalf("Error connecting to MongoDB deployment: %s", err)
		}
	}
}

// Status compares the given status to the actual status of the MongoDB resource
func Status(mdb *mdbv1.MongoDBCommunity, expectedStatus mdbv1.MongoDBCommunityStatus) func(t *testing.T) {
	return func(t *testing.T) {
		if err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, mdb); err != nil {
			t.Fatalf("error getting MongoDB resource: %s", err)
		}
		assert.Equal(t, expectedStatus, mdb.Status)
	}
}

// Scale update the MongoDB with a new number of members and updates the resource
func Scale(mdb *mdbv1.MongoDBCommunity, newMembers int) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Scaling Mongodb %s, to %d members", mdb.Name, newMembers)
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Members = newMembers
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// DisableTLS changes the tls.enabled attribute to false.
func DisableTLS(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return tls(mdb, false)
}

// EnableTLS changes the tls.enabled attribute to true.
func EnableTLS(mdb *mdbv1.MongoDBCommunity) func(*testing.T) {
	return tls(mdb, true)
}

// tls function configures the security.tls.enabled attribute.
func tls(mdb *mdbv1.MongoDBCommunity, enabled bool) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Setting security.tls.enabled to %t", enabled)
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDBCommunity) {
			db.Spec.Security.TLS.Enabled = enabled
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ChangeVersion(mdb *mdbv1.MongoDBCommunity, newVersion string) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("Changing versions from: %s to %s", mdb.Spec.Version, newVersion)
		err := e2eutil.UpdateMongoDBResource(mdb, func(db *mdbv1.MongoDBCommunity) {
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
func Connect(mdb *mdbv1.MongoDBCommunity, opts *options.ClientOptions) error {
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

func StatefulSetContainerConditionIsTrue(mdb *mdbv1.MongoDBCommunity, containerName string, condition func(container corev1.Container) bool) func(*testing.T) {
	return func(t *testing.T) {
		sts := appsv1.StatefulSet{}
		err := e2eutil.TestClient.Get(context.TODO(), types.NamespacedName{Name: mdb.Name, Namespace: mdb.Namespace}, &sts)
		if err != nil {
			t.Fatal(err)
		}

		container := findContainerByName(containerName, sts.Spec.Template.Spec.Containers)
		if container == nil {
			t.Fatalf(`No container found with name "%s" in StatefulSet pod template`, containerName)
		}

		if !condition(*container) {
			t.Fatalf(`Container "%s" does not satisfy condition`, containerName)
		}
	}
}

func findContainerByName(name string, containers []corev1.Container) *corev1.Container {
	for _, c := range containers {
		if c.Name == name {
			return &c
		}
	}

	return nil
}
