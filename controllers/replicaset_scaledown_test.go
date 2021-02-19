package controllers

import (
	"context"
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	"github.com/stretchr/testify/assert"
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
