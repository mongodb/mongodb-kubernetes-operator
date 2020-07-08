package mongodb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func assertReconciliationSuccessful(t *testing.T, result reconcile.Result, err error) {
	assert.NoError(t, err)
	assert.Equal(t, false, result.Requeue)
	assert.Equal(t, time.Duration(0), result.RequeueAfter)
}

func assertReconciliationRetries(t *testing.T, result reconcile.Result, err error) {
	errorHappened := err != nil && !result.Requeue
	isExplicitlyRequeuing := result.Requeue || result.RequeueAfter > 0
	assert.True(t, errorHappened || isExplicitlyRequeuing)
}
