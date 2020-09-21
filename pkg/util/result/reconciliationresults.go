package result

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func ShouldRequeue(result reconcile.Result, err error) bool {
	return err != nil || result.Requeue || result.RequeueAfter > 0
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
