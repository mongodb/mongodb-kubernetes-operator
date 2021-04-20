package result

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// StateComplete returns the result required for the State Machine
// to execute the next State in the next reconciliation.
func StateComplete() (reconcile.Result, error, bool) {
	return retry(0, true)
}

// RetryState returns the result required for the State Machine to
// execute this state in the next reconciliation.
func RetryState(after int) (reconcile.Result, error, bool) {
	return retry(after, false)
}

// FailedState returns the result required for the State to retry
// the current State.
func FailedState() (reconcile.Result, error, bool) {
	return RetryState(1)
}

func retry(after int, isComplete bool) (reconcile.Result, error, bool) {
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * time.Duration(after)}, nil, isComplete
}

func OK() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func Retry(after int) (reconcile.Result, error) {
	return reconcile.Result{Requeue: true, RequeueAfter: time.Second * time.Duration(after)}, nil
}

func Failed() (reconcile.Result, error) {
	return Retry(0)
}
