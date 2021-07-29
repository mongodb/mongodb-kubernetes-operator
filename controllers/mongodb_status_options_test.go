package controllers

import (
	"testing"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

const testVersion string = "4.2.6"

func TestMongoUriOption_ApplyOption(t *testing.T) {

	mdb := newReplicaSet(3, testVersion, "my-rs", "my-ns")

	opt := mongoUriOption{
		mongoUri: "my-uri",
	}

	opt.ApplyOption(&mdb)

	assert.Equal(t, "my-uri", mdb.Status.MongoURI, "Status should be updated")
}

func TestOptionBuilder_RunningPhase(t *testing.T) {
	mdb := newReplicaSet(3, testVersion, "my-rs", "my-ns")

	statusOptions().withRunningPhase().GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Running, mdb.Status.Phase)
}

func TestOptionBuilder_PendingPhase(t *testing.T) {
	mdb := newReplicaSet(3, testVersion, "my-rs", "my-ns")

	statusOptions().withPendingPhase(10).GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Pending, mdb.Status.Phase)
}

func TestOptionBuilder_FailedPhase(t *testing.T) {
	mdb := newReplicaSet(3, testVersion, "my-rs", "my-ns")

	statusOptions().withFailedPhase().GetOptions()[0].ApplyOption(&mdb)

	assert.Equal(t, mdbv1.Failed, mdb.Status.Phase)
}

func TestVersion_ApplyOption(t *testing.T) {
	mdb := newReplicaSet(3, testVersion, "my-rs", "my-ns")

	opt := versionOption{
		version: testVersion,
	}
	opt.ApplyOption(&mdb)

	assert.Equal(t, testVersion, mdb.Status.Version, "Status should be updated")
}

func newReplicaSet(members int, version string, name, namespace string) mdbv1.MongoDBCommunity {
	return mdbv1.MongoDBCommunity{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: mdbv1.MongoDBCommunitySpec{
			Members: members,
			Version: version,
		},
	}
}
