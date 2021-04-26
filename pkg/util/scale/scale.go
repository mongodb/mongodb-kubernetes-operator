package scale

// ReplicaSetScaler is an interface which is able to scale up and down a replicaset
// a single member at a time
type ReplicaSetScaler interface {
	DesiredReplicaSetMembers() int
	CurrentReplicaSetMember() int
}

// ReplicasThisReconciliation returns the number of replicas that should be configured
// for that reconciliation. As of MongoDB 4.4 we can only scale members up / down 1 at a time.
func ReplicasThisReconciliation(replicaSetScaler ReplicaSetScaler) int {
	// the current replica set members will be 0 when we are creating a new deployment
	// if this is the case, we want to jump straight to the desired members and not make changes incrementally
	if replicaSetScaler.CurrentReplicaSetMember() == 0 || replicaSetScaler.CurrentReplicaSetMember() == replicaSetScaler.DesiredReplicaSetMembers() {
		return replicaSetScaler.DesiredReplicaSetMembers()
	}

	if IsScalingDown(replicaSetScaler) {
		return replicaSetScaler.CurrentReplicaSetMember() - 1
	}

	return replicaSetScaler.CurrentReplicaSetMember() + 1

}

func IsStillScaling(replicaSetScaler ReplicaSetScaler) bool {
	return ReplicasThisReconciliation(replicaSetScaler) != replicaSetScaler.DesiredReplicaSetMembers()
}

func IsScalingDown(replicaSetScaler ReplicaSetScaler) bool {
	return replicaSetScaler.DesiredReplicaSetMembers() < replicaSetScaler.CurrentReplicaSetMember()
}

func IsScalingUp(replicaSetScaler ReplicaSetScaler) bool {
	return replicaSetScaler.DesiredReplicaSetMembers() > replicaSetScaler.CurrentReplicaSetMember() && replicaSetScaler.CurrentReplicaSetMember() != 0
}

// AnyAreStillScaling reports true if any of one the provided members is still scaling
func AnyAreStillScaling(scalers ...ReplicaSetScaler) bool {
	for _, s := range scalers {
		if IsStillScaling(s) {
			return true
		}
	}
	return false
}
