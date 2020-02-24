package automationconfig

import (
	"fmt"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildAutomationConfig(t *testing.T) {

	ac := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		Build()

	assert.Len(t, ac.Processes, 3)

	for i, p := range ac.Processes {
		assert.Equal(t, Mongod, p.ProcessType)
		assert.Equal(t, fmt.Sprintf("my-rs-%d.my-ns.svc.cluster.local", i), p.HostName)
		assert.Equal(t, DefaultMongoDBDataDir, p.Storage.DBPath)
		assert.Equal(t, "my-rs", p.Replication.ReplicaSetName, "replication should be configured based on the replica set name provided")
		assert.Equal(t, fmt.Sprintf("my-rs-%d", i), p.Name)
		assert.Equal(t, "4.2.0", p.Version)
	}

	assert.Len(t, ac.ReplicaSets, 1)
	rs := ac.ReplicaSets[0]
	assert.Equal(t, rs.Id, "my-rs", "The name provided should be configured to be the rs id")
	assert.Len(t, rs.Members, 3, "there should be the number of replicas provided")

	for i, member := range rs.Members {
		assert.Equal(t, 1, member.Votes)
		assert.False(t, member.ArbiterOnly)
		assert.Equal(t, cast.ToString(i), member.Id)
		assert.Equal(t, ac.Processes[i].Name, member.Host)
	}
}
