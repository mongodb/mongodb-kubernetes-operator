package e2e

import (
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/apis"
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	f "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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
			Version: "4.2.0",
		},
	}
}

func TestReplicaSet(t *testing.T) {
	mongodb := &mdbv1.MongoDB{}
	err := f.AddToFrameworkScheme(apis.AddToScheme, mongodb)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	t.Run("Create MongoDB Replica Set", func(t *testing.T) {
		mdb := newTestMongoDB()
		err = f.Global.Client.Create(context.TODO(), &mdb, &f.CleanupOptions{})
		assert.NoError(t, err)

		err := waitForStatefulSetToHaveMembers(t, f.Global.Client, mdb.Name, mdb.Spec.Members, time.Second*5, time.Second*60)
		assert.NoError(t, err)

		t.Logf("StatefulSet %s/%s successfully created!", mdb.Namespace, mdb.Name)
	})
}

func waitForStatefulSetToHaveMembers(t *testing.T, c f.FrameworkClient, stsName string, totalMembers int, retryInterval, timeout time.Duration) error {
	sts := &appsv1.StatefulSet{}
	return wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = c.Get(context.TODO(), types.NamespacedName{Name: stsName, Namespace: f.Global.Namespace}, sts)
		if err != nil {
			return false, err
		}
		t.Logf("Waiting for %s to have %d members. Current members: %d\n", stsName, totalMembers, sts.Status.ReadyReplicas)
		ready := *sts.Spec.Replicas == sts.Status.ReadyReplicas
		return ready, nil
	})
}
