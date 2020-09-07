package v1

import (
	"testing"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/status"
	"github.com/stretchr/testify/assert"
)

func TestMongoUriOption_ApplyOption(t *testing.T) {

	mdb := newReplicaSet(3, "my-rs", "my-ns")

	opt := mongoUriOption{
		mdb:      &mdb,
		mongoUri: "my-uri",
	}

	opt.ApplyOption()

	assert.Equal(t, "my-uri", mdb.Status.MongoURI, "Status should be updated")
}

func TestMembersOption_ApplyOption(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	opt := membersOption{
		mdb:     &mdb,
		members: 5,
	}

	opt.ApplyOption()

	assert.Equal(t, 3, mdb.Spec.Members, "Spec should remain unchanged")
	assert.Equal(t, 5, mdb.Status.Members, "Status should be updated")
}

func TestMultiOption_ApplyOption(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	opt := multiOption{
		options: []status.Option{
			membersOption{
				mdb:     &mdb,
				members: 4,
			},
			mongoUriOption{
				mdb:      &mdb,
				mongoUri: "my-uri",
			},
		},
	}

	opt.ApplyOption()

	assert.Equal(t, 4, mdb.Status.Members)
	assert.Equal(t, "my-uri", mdb.Status.MongoURI)
}

func TestOptionBuilder_RunningPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	NewOptionBuilder(&mdb).RunningPhase().ApplyOption()

	assert.Equal(t, Running, mdb.Status.Phase)
}

func TestOptionBuilder_PendingPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	NewOptionBuilder(&mdb).PendingPhase("pending").ApplyOption()

	assert.Equal(t, Pending, mdb.Status.Phase)
}

func TestOptionBuilder_FailedPhase(t *testing.T) {
	mdb := newReplicaSet(3, "my-rs", "my-ns")

	NewOptionBuilder(&mdb).Failed("failed").ApplyOption()

	assert.Equal(t, Failed, mdb.Status.Phase)
}
