package scale

// ReplicaSetScaler is an interface which is able to scale up and down a replicaset
// a single member at a time
type ReplicaSetScaler interface {
	DesiredReplicas() int
	CurrentReplicas() int
	ForcedIndividualScaling() bool
}

// ReplicasThisReconciliation returns the number of replicas that should be configured
// for that reconciliation. As of MongoDB 4.4 we can only scale members up / down 1 at a time.
func ReplicasThisReconciliation(replicaSetScaler ReplicaSetScaler) int {
	// the current replica set members will be 0 when we are creating a new deployment
	// if this is the case, we want to jump straight to the desired members and not make changes incrementally

	if replicaSetScaler.CurrentReplicas() == replicaSetScaler.DesiredReplicas() {
		return replicaSetScaler.DesiredReplicas()
	}

	if !replicaSetScaler.ForcedIndividualScaling() {
		// Short-circuit to scale up all at once
		if replicaSetScaler.CurrentReplicas() == 0 {
			return replicaSetScaler.DesiredReplicas()
		}
	}

	if IsScalingDown(replicaSetScaler) {
		return replicaSetScaler.CurrentReplicas() - 1
	}

	return replicaSetScaler.CurrentReplicas() + 1

}

func IsStillScaling(replicaSetScaler ReplicaSetScaler) bool {
	return ReplicasThisReconciliation(replicaSetScaler) != replicaSetScaler.DesiredReplicas()
}

func IsScalingDown(replicaSetScaler ReplicaSetScaler) bool {
	return replicaSetScaler.DesiredReplicas() < replicaSetScaler.CurrentReplicas()
}

func IsScalingUp(replicaSetScaler ReplicaSetScaler) bool {
	return replicaSetScaler.DesiredReplicas() > replicaSetScaler.CurrentReplicas() && replicaSetScaler.CurrentReplicas() != 0
}

func HasZeroReplicas(replicaSetScaler ReplicaSetScaler) bool {
	return replicaSetScaler.CurrentReplicas() == 0
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
