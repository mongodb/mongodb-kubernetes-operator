package automationconfig

import (
	"fmt"

	"github.com/spf13/cast"
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
	topology       Topology
	mongodbVersion string
}

func NewBuilder() *Builder {
	return &Builder{
		processes:   []Process{},
		replicaSets: []ReplicaSet{},
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

func (b *Builder) AddVersion(version Version) *Builder {
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
		process := newProcess(toHostName(b.name, i), h, b.mongodbVersion, b.name)
		processes[i] = process
		members[i] = newReplicaSetMember(process, cast.ToString(i))
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
		Auth: DisabledAuth(),
	}
}

func toHostName(name string, index int) string {
	return fmt.Sprintf("%s-%d", name, index)
}
