package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsReadyState checks that Primary, Secondary and Undefined always result
// in Ready State.
func TestIsReadyStateNotPrimaryNorSecondary(t *testing.T) {
	status := []replicationStatus{replicationStatusUndefined, replicationStatusPrimary, replicationStatusSecondary}

	for i := range status {
		h := processHealth{ReplicaStatus: &status[i]}
		assert.True(t, h.IsReadyState())
	}
}

// TestIsNotReady any of these states will result on a Database not being ready.
func TestIsNotReady(t *testing.T) {
	status := []replicationStatus{
		replicationStatusStartup, replicationStatusRecovering, replicationStatusStartup2,
		replicationStatusUnknown, replicationStatusArbiter, replicationStatusDown,
		replicationStatusRollback, replicationStatusRemoved,
	}

	for i := range status {
		h := processHealth{ReplicaStatus: &status[i]}
		assert.False(t, h.IsReadyState())
	}
}
