package controllers

import (
	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// updateScalingStatus updates the status fields which are required to keep track of the current
// scaling state of the resource
func updateScalingStatus(statusWriter k8sClient.StatusWriter, mdb mdbv1.MongoDBCommunity) error {
	_, err := status.Update(statusWriter, &mdb,
		statusOptions().
			withMongoDBMembers(mdb.AutomationConfigMembersThisReconciliation()).
			withStatefulSetReplicas(mdb.StatefulSetReplicasThisReconciliation()),
	)
	return err
}
