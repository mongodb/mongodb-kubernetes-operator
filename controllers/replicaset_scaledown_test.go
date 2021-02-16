package controllers

import (
	"context"
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUpdateScalingStatus(t *testing.T) {
	mdb := newTestReplicaSet()
	mgr := client.NewManager(&mdb)

	assert.Equal(t, 0, mdb.Status.CurrentStatefulSetReplicas)
	assert.Equal(t, 0, mdb.Status.CurrentMongoDBMembers)

	expectedAutomationConfigMembers := mdb.AutomationConfigMembersThisReconciliation()
	expectedStatefulSetReplicas := mdb.StatefulSetReplicasThisReconciliation()

	err := updateScalingStatus(mgr.Client, mdb)
	assert.NoError(t, err)

	err = mgr.Client.Get(context.TODO(), mdb.NamespacedName(), &mdb)
	assert.NoError(t, err)

	assert.Equal(t, expectedAutomationConfigMembers, mdb.Status.CurrentStatefulSetReplicas)
	assert.Equal(t, expectedStatefulSetReplicas, mdb.Status.CurrentMongoDBMembers)
}

func TestHasReachedDesiredNumberOfStatefulSetReplicasReady(t *testing.T) {
	createStatefulSet := func(c k8sClient.Client, mdb mdbv1.MongoDBCommunity) error {
		replicas := int32(mdb.Spec.Members)
		return c.Create(context.TODO(), &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      mdb.Name,
				Namespace: mdb.Namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: &replicas,
			},
		})
	}

	t.Run("There is an error when the StatefulSet does not exist", func(t *testing.T) {
		// Arrange
		mdb := newTestReplicaSet()
		mgr := client.NewManager(&mdb)

		// Act
		_, err := hasReachedDesiredNumberOfStatefulSetReplicasReady(mgr.Client, mdb)

		// Assert
		assert.Error(t, err)
	})

	t.Run("Returns true when the StatefulSet exists and is ready", func(t *testing.T) {
		// Arrange
		mdb := newTestReplicaSet()
		mgr := client.NewManager(&mdb)
		err := createStatefulSet(mgr.Client, mdb)
		assert.NoError(t, err)
		makeStatefulSetReady(t, mgr.Client, mdb)

		// Act
		hasReached, err := hasReachedDesiredNumberOfStatefulSetReplicasReady(mgr.Client, mdb)

		// Assert
		assert.NoError(t, err, "should be no error when the StatefulSet exists")
		assert.True(t, hasReached, "Should not be ready when the stateful set is not ready")
	})

	t.Run("Returns false when the StatefulSet exists and is not ready", func(t *testing.T) {
		// Arrange
		mdb := newTestReplicaSet()
		mgr := client.NewManager(&mdb)
		err := createStatefulSet(mgr.Client, mdb)
		assert.NoError(t, err)
		makeStatefulSetUnReady(t, mgr.Client, mdb)

		// Act
		hasReached, err := hasReachedDesiredNumberOfStatefulSetReplicasReady(mgr.Client, mdb)

		// Assert
		assert.NoError(t, err, "should be no error when the StatefulSet exists")
		assert.False(t, hasReached, "Should not be ready when the stateful set is not ready")
	})

}
