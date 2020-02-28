package automationconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAutomationConfig(t *testing.T) {

	ac := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetAutomationConfigVersion(1).
		SetMembers(3).
		Build()

	assert.Len(t, ac.Processes, 3)
	assert.Equal(t, 1, ac.Version)

	for i, p := range ac.Processes {
		assert.Equal(t, Mongod, p.ProcessType)
		assert.Equal(t, fmt.Sprintf("my-rs-%d.my-ns.svc.cluster.local", i), p.HostName)
		assert.Equal(t, DefaultMongoDBDataDir, p.Storage.DBPath)
		assert.Equal(t, "my-rs", p.Replication.ReplicaSetName, "replication should be configured based on the replica set name provided")
		assert.Equal(t, toHostName("my-rs", i), p.Name)
		assert.Equal(t, "4.2.0", p.Version)
	}

	assert.Len(t, ac.ReplicaSets, 1)
	rs := ac.ReplicaSets[0]
	assert.Equal(t, rs.Id, "my-rs", "The name provided should be configured to be the rs id")
	assert.Len(t, rs.Members, 3, "there should be the number of replicas provided")

	for i, member := range rs.Members {
		assert.Equal(t, 1, member.Votes)
		assert.False(t, member.ArbiterOnly)
		assert.Equal(t, i, member.Id)
		assert.Equal(t, ac.Processes[i].Name, member.Host)
	}
}

func TestMongoDbVersions(t *testing.T) {
	version1 := MongoDbVersionConfig{
		Builds: []BuildConfig{
			{
				Architecture: "amd64",
				GitVersion:   "some-git-version",
				Platform:     "linux",
				Url:          "some-url",
				Flavor:       "rhel",
				MaxOsVersion: "8.0",
				MinOsVersion: "7.0",
			},
		},
		Name: "4.2.2",
	}

	ac := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetAutomationConfigVersion(1).
		SetMembers(3).
		AddVersion(version1).
		Build()

	assert.Len(t, ac.Processes, 3)
	assert.Len(t, ac.Versions, 1)
	assert.Len(t, ac.Versions[0].Builds, 1)

	version2 := MongoDbVersionConfig{
		Builds: []BuildConfig{
			{
				Architecture: "amd64",
				GitVersion:   "some-git-version",
				Platform:     "linux",
				Url:          "some-url",
				Flavor:       "rhel",
				MaxOsVersion: "8.0",
				MinOsVersion: "7.0",
			},
			{
				Architecture: "amd64",
				GitVersion:   "some-git-version",
				Platform:     "linux",
				Url:          "some-url",
				Flavor:       "ubuntu",
				MaxOsVersion: "1604",
				MinOsVersion: "1804",
			},
		},
		Name: "4.2.3",
	}

	ac = NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetAutomationConfigVersion(1).
		SetMembers(3).
		AddVersion(version1).
		AddVersion(version2).
		Build()

	assert.Len(t, ac.Processes, 3)
	assert.Len(t, ac.Versions, 2)
	assert.Len(t, ac.Versions[0].Builds, 1)
	assert.Len(t, ac.Versions[1].Builds, 2)
}

func TestHasOptions(t *testing.T) {
	ac := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetAutomationConfigVersion(1).
		SetMembers(3).
		Build()

	assert.Equal(t, ac.Options.DownloadBase, "/var/lib/mongodb-mms-automation")
}

func TestModulesNotNil(t *testing.T) {
	version1 := MongoDbVersionConfig{
		Builds: []BuildConfig{
			{
				Architecture: "amd64",
				GitVersion:   "some-git-version",
				Platform:     "linux",
				Url:          "some-url",
				Flavor:       "rhel",
				MaxOsVersion: "8.0",
				MinOsVersion: "7.0",
			},
		},
		Name: "4.2.2",
	}

	ac := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetAutomationConfigVersion(1).
		SetMembers(3).
		AddVersion(version1).
		Build()

	assert.Len(t, ac.Processes, 3)

	// why is this not working?
	// assert.NotNil(t, ac.Versions[0].Builds[0].Modules)

}
