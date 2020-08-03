package automationconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultMongoDbVersion(version string) MongoDbVersionConfig {
	return MongoDbVersionConfig{
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
		Name: version,
	}
}

func TestBuildAutomationConfig(t *testing.T) {
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		SetFCV("4.0").
		Build()

	assert.NoError(t, err)
	assert.Len(t, ac.Processes, 3)
	assert.Equal(t, 1, ac.Version)

	for i, p := range ac.Processes {
		assert.Equal(t, Mongod, p.ProcessType)
		assert.Equal(t, fmt.Sprintf("my-rs-%d.my-ns.svc.cluster.local", i), p.HostName)
		assert.Equal(t, DefaultMongoDBDataDir, p.Args26.Get("storage.dbPath").MustStr())
		assert.Equal(t, "my-rs", p.Args26.Get("replication.replSetName").MustStr())
		assert.Equal(t, toHostName("my-rs", i), p.Name)
		assert.Equal(t, "4.2.0", p.Version)
		assert.Equal(t, "4.0", p.FeatureCompatibilityVersion)
	}

	assert.Empty(t, ac.TLS.CAFilePath, "the config shouldn't have a trusted CA")

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
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		AddVersion(defaultMongoDbVersion("4.2.0")).
		Build()

	assert.NoError(t, err)
	assert.Len(t, ac.Processes, 3)
	assert.Len(t, ac.Versions, 1)
	assert.Len(t, ac.Versions[0].Builds, 1)

	// TODO: be able to pass amount of builds
	version2 := defaultMongoDbVersion("4.2.3")
	version2.Builds = append(version2.Builds,
		BuildConfig{
			Architecture: "amd64",
			GitVersion:   "some-git-version",
			Platform:     "linux",
			Url:          "some-url",
			Flavor:       "rhel",
			MaxOsVersion: "8.0",
			MinOsVersion: "7.0",
		},
	)

	ac, err = NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		AddVersion(defaultMongoDbVersion("4.2.0")).
		AddVersion(version2).
		Build()

	assert.NoError(t, err)
	assert.Len(t, ac.Processes, 3)
	assert.Len(t, ac.Versions, 2)
	assert.Len(t, ac.Versions[0].Builds, 1)
	assert.Len(t, ac.Versions[1].Builds, 2)
}

func TestHasOptions(t *testing.T) {
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, ac.Options.DownloadBase, "/var/lib/mongodb-mms-automation")
}

func TestModulesNotNil(t *testing.T) {
	// We make sure the .Modules is initialized as an empty list of strings
	// or it will dumped as null attribute in json.
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		AddVersion(defaultMongoDbVersion("4.3.2")).
		Build()

	assert.NoError(t, err)
	assert.NotNil(t, ac.Versions[0].Builds[0].Modules)
}

func TestProcessHasPortSetToDefault(t *testing.T) {
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		AddVersion(defaultMongoDbVersion("4.3.2")).
		Build()

	assert.NoError(t, err)
	assert.Len(t, ac.Processes, 3)
	for _, process := range ac.Processes {
		assert.Equal(t, 27017, process.Args26.Get("net.port").MustInt())
	}
}

func TestVersionManifest_BuildsForVersion(t *testing.T) {
	vm := VersionManifest{
		Updated: 0,
		Versions: []MongoDbVersionConfig{
			defaultMongoDbVersion("4.2.0"),
			defaultMongoDbVersion("4.2.3"),
			defaultMongoDbVersion("4.2.4"),
		},
	}

	version := vm.BuildsForVersion("4.2.0")
	assert.Len(t, version.Builds, 1)
	assert.Equal(t, defaultMongoDbVersion("4.2.0"), version)

	version = vm.BuildsForVersion("4.2.3")
	assert.Len(t, version.Builds, 1)
	assert.Equal(t, defaultMongoDbVersion("4.2.3"), version)

	version = vm.BuildsForVersion("4.2.4")
	assert.Len(t, version.Builds, 1)
	assert.Equal(t, defaultMongoDbVersion("4.2.4"), version)

	version = vm.BuildsForVersion("4.2.1")
	assert.Empty(t, version.Builds)
}

func TestModifications(t *testing.T) {
	incrementVersion := func(config *AutomationConfig) {
		config.Version += 1
	}

	ac, err := NewBuilder().
		AddModifications(incrementVersion, incrementVersion, incrementVersion).
		AddModifications(NOOP()).
		Build()

	assert.NoError(t, err)
	assert.Equal(t, 4, ac.Version)
}
