package state

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func SuccessfulRetry(after int) (reconcile.Result, error, bool) {
	res, err := result.Retry(after)
	return res, err, true
}


func FailedRetry(after int) (reconcile.Result, error, bool) {
	res, err := result.Retry(after)
	return res, err, false
}


func EndReconciliation() (reconcile.Result, error, bool) {
	res, err := result.OK()
	return res, err, true
}
