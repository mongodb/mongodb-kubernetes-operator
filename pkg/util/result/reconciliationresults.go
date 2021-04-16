package result

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func OK() (reconcile.Result, error, bool) {
	return reconcile.Result{}, nil, true
}

func StateComplete() (reconcile.Result, error, bool) {
	return retry(0, true)
}

func FailedState() (reconcile.Result, error, bool) {
	return RetryState(1)
}

func RetryState(after int) (reconcile.Result, error, bool) {
	return retry(after, false)
}

func retry(after int, isComplete bool) (reconcile.Result, error, bool) {
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * time.Duration(after)}, nil, isComplete
}
