package mongodb

import (
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestMongoUriOption_ApplyOption(t *testing.T) {

	mdb := newReplicaSet(3, "my-rs", "my-ns")

	opt := mongoUriOption{
		mongoUri: "my-uri",
	}

	opt.ApplyOption(&mdb)

	assert.Equal(t, "my-uri", mdb.Status.MongoURI, "Status should be updated")
}

func TestMembersOption_ApplyOption(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	opt := membersOption{
		members: 5,
	}

	opt.ApplyOption(&mdb)

	assert.Equal(t, 3, mdb.Spec.Members, "Spec should remain unchanged")
	assert.Equal(t, 5, mdb.Status.Members, "Status should be updated")
}

func TestOptionBuilder_RunningPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	statusOptions().withRunningPhase().GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Running, mdb.Status.Phase)
}

func TestOptionBuilder_PendingPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	statusOptions().withPendingPhase(10).GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Pending, mdb.Status.Phase)
}

func TestOptionBuilder_FailedPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	statusOptions().withFailedPhase().GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Failed, mdb.Status.Phase)
}

func newReplicaSet(members int, name, namespace string) mdbv1.MongoDB {
	return mdbv1.MongoDB{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: mdbv1.MongoDBSpec{
			Members: members,
		},
	}
}
