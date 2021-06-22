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
		assert.Equal(t, DefaultMongoDBDataDir, p.Args26.Get("storage.dbPath").Data())
		assert.Equal(t, "my-rs", p.Args26.Get("replication.replSetName").Data())
		assert.Equal(t, toProcessName("my-rs", i), p.Name)
		assert.Equal(t, "4.2.0", p.Version)
		assert.Equal(t, "4.0", p.FeatureCompatibilityVersion)
	}

	assert.Empty(t, ac.TLSConfig.CAFilePath, "the config shouldn't have a trusted CA")

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

func TestBuildAutomationConfigArbiters(t *testing.T) {
	// Test no arbiter (field specified)
	noArbiters := 0
	ac, err := NewBuilder().
		SetMembers(4).
		SetArbiters(noArbiters).
		Build()

	assert.NoError(t, err)

	rs := ac.ReplicaSets[0]
	for _, member := range rs.Members {
		assert.False(t, member.ArbiterOnly, "No member should be an arbiter")
	}

	// Test no arbiter (field NOT specified)
	ac, err = NewBuilder().
		SetMembers(4).
		Build()

	assert.NoError(t, err)

	rs = ac.ReplicaSets[0]
	for _, member := range rs.Members {
		assert.False(t, member.ArbiterOnly, "No member should be an arbiter")
	}

	// Test only one arbiter
	noArbiters = 1
	ac, err = NewBuilder().
		SetMembers(4).
		SetArbiters(noArbiters).
		Build()

	assert.NoError(t, err)

	rs = ac.ReplicaSets[0]
	for i, member := range rs.Members {
		if i < noArbiters {
			assert.True(t, member.ArbiterOnly, "The first member should be an arbiter")
		} else {
			assert.False(t, member.ArbiterOnly, "These members should not be arbiters")
		}
	}

	// Test with multiple arbiters
	noArbiters = 2
	ac, err = NewBuilder().
		SetMembers(4).
		SetArbiters(noArbiters).
		Build()

	assert.NoError(t, err)

	rs = ac.ReplicaSets[0]
	for i, member := range rs.Members {
		if i < noArbiters {
			assert.True(t, member.ArbiterOnly, "The first two members should be arbiters")
		} else {
			assert.False(t, member.ArbiterOnly, "These members should not be arbiters")
		}
	}

	//Test With only Arbiters
	// The error will be generated when the reconcile loop is called (tested in the e2e tests).
	noArbiters = 4
	ac, err = NewBuilder().
		SetMembers(noArbiters).
		SetArbiters(noArbiters).
		Build()

	assert.NoError(t, err)

	rs = ac.ReplicaSets[0]
	for i, member := range rs.Members {
		if i < noArbiters {
			assert.True(t, member.ArbiterOnly, "The first two members should be arbiters")
		} else {
			assert.False(t, member.ArbiterOnly, "These members should not be arbiters")
		}
	}

	//Test With more Arbiters than members
	// The error will be generated when the reconcile loop is called (tested in the e2e tests).
	noMembers := 3
	noArbiters = noMembers + 1
	ac, err = NewBuilder().
		SetMembers(noMembers).
		SetArbiters(noArbiters).
		Build()

	assert.NoError(t, err)

	rs = ac.ReplicaSets[0]
	for i, member := range rs.Members {
		if i < noArbiters {
			assert.True(t, member.ArbiterOnly, "The first two members should be arbiters")
		} else {
			assert.False(t, member.ArbiterOnly, "These members should not be arbiters")
		}
	}
}

func TestReplicaSetHorizons(t *testing.T) {
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		SetReplicaSetHorizons([]ReplicaSetHorizons{
			{"horizon": "test-horizon-0"},
			{"horizon": "test-horizon-1"},
			{"horizon": "test-horizon-2"},
		}).
		Build()

	assert.NoError(t, err)

	for i, member := range ac.ReplicaSets[0].Members {
		assert.NotEmpty(t, member.Horizons)
		assert.Contains(t, member.Horizons, "horizon")
		assert.Equal(t, fmt.Sprintf("test-horizon-%d", i), member.Horizons["horizon"])
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
	assert.Len(t, ac.Versions, 2)
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
	assert.Len(t, ac.Versions, 3)
	assert.Len(t, ac.Versions[0].Builds, 1)
	assert.Len(t, ac.Versions[1].Builds, 2)
}

func TestHasOptions(t *testing.T) {
	ac, err := NewBuilder().
		SetName("my-rs").
		SetDomain("my-ns.svc.cluster.local").
		SetMongoDBVersion("4.2.0").
		SetMembers(3).
		SetOptions(Options{DownloadBase: "/var/lib/mongodb-mms-automation"}).
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
		assert.Equal(t, 27017, process.Args26.Get("net.port").Data())
	}
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

func TestMongoDBVersionsConfig(t *testing.T) {

	t.Run("Dummy Config is used when no versions are set", func(t *testing.T) {
		ac, err := NewBuilder().SetMongoDBVersion("4.4.2").Build()
		assert.NoError(t, err)

		versions := ac.Versions
		assert.Len(t, versions, 1)
		v := versions[0]
		dummyConfig := buildDummyMongoDbVersionConfig("4.4.2")
		assert.Equal(t, v, dummyConfig)
	})

	t.Run("Dummy Config is not used when versions are set", func(t *testing.T) {
		ac, err := NewBuilder().SetMongoDBVersion("4.4.2").AddVersion(MongoDbVersionConfig{
			Name: "4.4.2",
			Builds: []BuildConfig{
				{
					Platform:     "linux",
					Url:          "url",
					GitVersion:   "gitVersion",
					Architecture: "arch",
					Flavor:       "flavor",
					MinOsVersion: "minOs",
					MaxOsVersion: "maxOs",
				},
			},
		}).Build()

		assert.NoError(t, err)

		versions := ac.Versions
		assert.Len(t, versions, 2)
		v := versions[0]
		dummyConfig := buildDummyMongoDbVersionConfig("4.4.2")
		assert.NotEqual(t, v, dummyConfig)

		b := versions[0].Builds[0]
		assert.Equal(t, "linux", b.Platform)
		assert.Equal(t, "url", b.Url)
		assert.Equal(t, "gitVersion", b.GitVersion)
		assert.Equal(t, "arch", b.Architecture)
		assert.Equal(t, "minOs", b.MinOsVersion)
		assert.Equal(t, "maxOs", b.MaxOsVersion)

	})

}

func TestAreEqual(t *testing.T) {
	t.Run("Automation Configs with same values are equal", func(t *testing.T) {

		areEqual, err := AreEqual(
			createAutomationConfig("name0", "mdbVersion0", "domain0", Options{DownloadBase: "downloadBase0"}, Auth{Disabled: true}, 5, 2),
			createAutomationConfig("name0", "mdbVersion0", "domain0", Options{DownloadBase: "downloadBase0"}, Auth{Disabled: true}, 5, 2),
		)

		assert.NoError(t, err)
		assert.True(t, areEqual)
	})

	t.Run("Automation Configs with same values but different version are equal", func(t *testing.T) {

		areEqual, err := AreEqual(
			createAutomationConfig("name0", "mdbVersion0", "domain0", Options{DownloadBase: "downloadBase0"}, Auth{Disabled: true}, 5, 2),
			createAutomationConfig("name0", "mdbVersion0", "domain0", Options{DownloadBase: "downloadBase0"}, Auth{Disabled: true}, 5, 10),
		)

		assert.NoError(t, err)
		assert.True(t, areEqual)
	})

	t.Run("Automation Configs with different values are not equal", func(t *testing.T) {

		areEqual, err := AreEqual(
			createAutomationConfig("name0", "differentVersion", "domain0", Options{DownloadBase: "downloadBase1"}, Auth{Disabled: false}, 2, 2),
			createAutomationConfig("name0", "mdbVersion0", "domain0", Options{DownloadBase: "downloadBase0"}, Auth{Disabled: true}, 5, 2),
		)

		assert.NoError(t, err)
		assert.False(t, areEqual)
	})
}

func createAutomationConfig(name, mongodbVersion, domain string, opts Options, auth Auth, members, acVersion int) AutomationConfig {
	ac, _ := NewBuilder().
		SetName(name).
		SetMongoDBVersion(mongodbVersion).
		SetOptions(opts).
		SetDomain(domain).
		SetMembers(members).
		SetAuth(auth).
		Build()

	ac.Version = acVersion
	return ac
}
