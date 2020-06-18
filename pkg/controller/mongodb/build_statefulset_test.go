package mongodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
)

func TestExtraContainersAreNotAdded_OnSubsequentCalls(t *testing.T) {
	mdb := newTestReplicaSet()
	stsFunc := buildStatefulSetModificationFunction(mdb)
	sts := &appsv1.StatefulSet{}
	stsFunc(sts)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	stsFunc(sts)

	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
}
