package status

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Option interface {
	ApplyOption()
	GetResult() (reconcile.Result, error)
}

func Update(statusWriter client.StatusWriter, obj runtime.Object, options ...Option) (reconcile.Result, error) {
	for _, opt := range options {
		opt.ApplyOption()
	}

	if err := statusWriter.Update(context.TODO(), obj); err != nil {
		return reconcile.Result{}, err
	}

	return DetermineReconciliationResult(options)
}

func DetermineReconciliationResult(options []Option) (reconcile.Result, error) {
	// if there are any errors in any of our options, we return those first
	for _, opt := range options {
		res, err := opt.GetResult()
		if err != nil {
			return res, err
		}
	}
	// otherwise we might need to re-queue
	for _, opt := range options {
		res, _ := opt.GetResult()
		if res.Requeue || res.RequeueAfter > 0 {
			return res, nil
		}
	}
	// it was a successful reconciliation, nothing to do
	return reconcile.Result{}, nil
}
