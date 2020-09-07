package status

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type errorOption struct{}

func (e errorOption) ApplyOption() {}

func (e errorOption) GetResult() (reconcile.Result, error) {
	return reconcile.Result{}, errors.Errorf("error")
}

type successOption struct{}

func (s successOption) ApplyOption() {}

func (s successOption) GetResult() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

type retryOption struct{}

func (r retryOption) ApplyOption() {}

func (r retryOption) GetResult() (reconcile.Result, error) {
	return reconcile.Result{Requeue: true}, nil
}

func TestDetermineReconciliationResult(t *testing.T) {

	t.Run("A single error option should result in an error return", func(t *testing.T) {
		opts := []Option{
			errorOption{},
			successOption{},
			successOption{},
		}

		res, err := DetermineReconciliationResult(opts)
		assert.NotNil(t, err)
		assert.Equal(t, false, res.Requeue)
		assert.Equal(t, time.Duration(0), res.RequeueAfter)
	})

	t.Run("An error option takes precedence over a retry", func(t *testing.T) {
		opts := []Option{
			errorOption{},
			retryOption{},
			successOption{},
		}
		res, err := DetermineReconciliationResult(opts)
		assert.NotNil(t, err)
		assert.Equal(t, false, res.Requeue)
		assert.Equal(t, time.Duration(0), res.RequeueAfter)
	})

	t.Run("No errors will result in a successful reconciliation", func(t *testing.T) {
		opts := []Option{
			successOption{},
			successOption{},
			successOption{},
		}
		res, err := DetermineReconciliationResult(opts)
		assert.Nil(t, err)
		assert.Equal(t, false, res.Requeue)
		assert.Equal(t, time.Duration(0), res.RequeueAfter)
	})

	t.Run("A retry will take precedence over success", func(t *testing.T) {
		opts := []Option{
			successOption{},
			successOption{},
			retryOption{},
		}
		res, err := DetermineReconciliationResult(opts)
		assert.Nil(t, err)
		assert.Equal(t, true, res.Requeue)
	})

}
