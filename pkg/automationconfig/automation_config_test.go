package automationconfig

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBuildAutomationConfig(t *testing.T) {

	s := "500m"
	opts := Options{
		Name:          "my-rs",
		Namespace:     "my-ns",
		ClusterDomain: "cluster.local",
		ServiceName:   "my-svc",
		Version:       "4.2.0",
		Replicas:      3,
		Memory:        &s,
	}

	ac := New(opts)

	assert.Len(t, ac.Processes, 3)

	for i, p := range ac.Processes {
		assert.Equal(t, fmt.Sprintf("%s.%s.%s.svc.%s", p.Name, opts.ServiceName, opts.Namespace, opts.ClusterDomain), p.HostName)
		assert.Equal(t, DefaultMongoDBDataDir, p.Storage.DBPath)
		assert.Equal(t, opts.Name, p.Replication.ReplicaSetName, "replication should be configured based on the replica set name provided")
		assert.Equal(t, fmt.Sprintf("%s-%d", opts.Name, i), p.Name)
	}

	fmt.Printf("%+v", ac)

}
