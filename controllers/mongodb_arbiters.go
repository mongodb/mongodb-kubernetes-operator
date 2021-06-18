package controllers

import (
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
)

func (r ReplicaSetReconciler) ensureArbiterResources(mdb mdbv1.MongoDBCommunity) error {
	if mdb.Spec.Arbiters < 0 {
		return fmt.Errorf(`number of arbiters must be greater or equal than 0`)
	}
	if mdb.Spec.Arbiters >= mdb.Spec.Members {
		return fmt.Errorf(`number of arbiters specified (%v) is greater or equal than the number of members in the replicaset (%v). At least one member must not be an arbiter`, mdb.Spec.Arbiters, mdb.Spec.Members)
	}

	return nil
}
