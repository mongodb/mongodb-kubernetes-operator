package automationconfig

import (
	"fmt"
)

type Topology string

const (
	ReplicaSetTopology Topology = "ReplicaSet"
)

type Builder struct {
	processes      []Process
	replicaSets    []ReplicaSet
	version        int
	auth           Auth
	members        int
	domain         string
	name           string
	fcv            string
	topology       Topology
	mongodbVersion string

	// MongoDB installable versions
	versions []MongoDbVersionConfig
}

func NewBuilder() *Builder {
	return &Builder{
		processes:   []Process{},
		replicaSets: []ReplicaSet{},
		versions:    []MongoDbVersionConfig{},
	}
}

func (b *Builder) SetTopology(topology Topology) *Builder {
	b.topology = topology
	return b
}

func (b *Builder) SetMembers(members int) *Builder {
	b.members = members
	return b
}

func (b *Builder) SetDomain(domain string) *Builder {
	b.domain = domain
	return b
}

func (b *Builder) SetName(name string) *Builder {
	b.name = name
	return b
}

func (b *Builder) SetFCV(fcv string) *Builder {
	b.fcv = fcv
	return b
}

func (b *Builder) AddVersion(version MongoDbVersionConfig) *Builder {
	for idx := range version.Builds {
		if version.Builds[idx].Modules == nil {
			version.Builds[idx].Modules = make([]string, 0)
		}
	}
	b.versions = append(b.versions, version)
	return b
}

func (b *Builder) SetMongoDBVersion(version string) *Builder {
	b.mongodbVersion = version
	return b
}

func (b *Builder) SetAutomationConfigVersion(version int) *Builder {
	b.version = version
	return b
}

func (b *Builder) Build() AutomationConfig {
	hostnames := make([]string, b.members)
	for i := 0; i < b.members; i++ {
		hostnames[i] = fmt.Sprintf("%s-%d.%s", b.name, i, b.domain)
	}

	members := make([]ReplicaSetMember, b.members)
	processes := make([]Process, b.members)
	for i, h := range hostnames {
		process := newProcess(toHostName(b.name, i), h, b.mongodbVersion, b.name, withFCV(b.fcv))
		processes[i] = process
		members[i] = newReplicaSetMember(process, i)
	}

	return AutomationConfig{
		Version:   b.version,
		Processes: processes,
		ReplicaSets: []ReplicaSet{
			{
				Id:              b.name,
				Members:         members,
				ProtocolVersion: "1",
			},
		},
		Versions: b.versions,
		Options:  Options{DownloadBase: "/var/lib/mongodb-mms-automation"},
		Auth:     DisabledAuth(),
	}
}

func toHostName(name string, index int) string {
	return fmt.Sprintf("%s-%d", name, index)
}

// Process functional options
func withFCV(fcv string) func(*Process) {
	return func(process *Process) {
		process.FeatureCompatibilityVersion = fcv
	}
}
