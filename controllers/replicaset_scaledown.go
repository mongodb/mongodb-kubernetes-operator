package controllers

import (
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/statefulset"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

// checkIfStatefulSetMembersHaveBeenRemovedFromTheAutomationConfig ensures that the expected number of StatefulSet
// replicas are ready. When a member has its process removed from the Automation Config, the pod will eventually
// become unready. We use this information to determine if we are safe to continue the reconciliation process.
func checkIfStatefulSetMembersHaveBeenRemovedFromTheAutomationConfig(stsGetter statefulset.Getter, statusWriter k8sClient.StatusWriter, mdb mdbv1.MongoDBCommunity) (reconcile.Result, error) {
	isAtDesiredReplicaCount, err := hasReachedDesiredNumberOfStatefulSetReplicasReady(stsGetter, mdb)
	if err != nil {
		return status.Update(statusWriter, &mdb,
			statusOptions().
				withMessage(Error, fmt.Sprintf("Error determining state of StatefulSet: %s", err)).
				withFailedPhase(),
		)
	}

	if !isAtDesiredReplicaCount {
		return status.Update(statusWriter, &mdb,
			statusOptions().
				withMessage(Info, fmt.Sprintf("Not yet at the desired number of replicas, currentMembers=%d, desiredMembers=%d",
					mdb.CurrentReplicas(), mdb.DesiredReplicas())).
				withPendingPhase(10),
		)
	}
	return result.OK()
}

// hasReachedDesiredNumberOfStatefulSetReplicasReady checks to see if the StatefulSet corresponding
// to the given MongoDB resource has the expected number of ready replicas.
func hasReachedDesiredNumberOfStatefulSetReplicasReady(stsGetter statefulset.Getter, mdb mdbv1.MongoDBCommunity) (bool, error) {
	sts, err := stsGetter.GetStatefulSet(mdb.NamespacedName())
	if err != nil {
		return false, err
	}
	desiredReadyReplicas := int32(mdb.StatefulSetReplicasThisReconciliation())
	return sts.Status.ReadyReplicas == desiredReadyReplicas, nil
}
