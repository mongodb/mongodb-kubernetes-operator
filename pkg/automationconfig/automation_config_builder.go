package automationconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Topology string

const (
	ReplicaSetTopology Topology = "ReplicaSet"
)

// AuthEnabler is an interface which can configure authentication settings
type AuthEnabler interface {
	EnableAuth(auth Auth) Auth
}

type Modification func(*AutomationConfig)

func NOOP() Modification {
	return func(config *AutomationConfig) {}
}

type Builder struct {
	enabler            AuthEnabler
	processes          []Process
	replicaSets        []ReplicaSet
	replicaSetHorizons []ReplicaSetHorizons
	members            int
	domain             string
	name               string
	fcv                string
	topology           Topology
	mongodbVersion     string
	previousAC         AutomationConfig
	// MongoDB installable versions
	versions      []MongoDbVersionConfig
	modifications []Modification
	options       Options
}

func NewBuilder() *Builder {
	return &Builder{
		processes:     []Process{},
		replicaSets:   []ReplicaSet{},
		versions:      []MongoDbVersionConfig{},
		modifications: []Modification{},
	}
}

func (b *Builder) SetAuthEnabler(enabler AuthEnabler) *Builder {
	b.enabler = enabler
	return b
}

func (b *Builder) SetOptions(options Options) *Builder {
	b.options = options
	return b
}

func (b *Builder) SetTopology(topology Topology) *Builder {
	b.topology = topology
	return b
}

func (b *Builder) SetReplicaSetHorizons(horizons []ReplicaSetHorizons) *Builder {
	b.replicaSetHorizons = horizons
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

func (b *Builder) SetPreviousAutomationConfig(previousAC AutomationConfig) *Builder {
	b.previousAC = previousAC
	return b
}

func (b *Builder) AddModifications(mod ...Modification) *Builder {
	b.modifications = append(b.modifications, mod...)
	return b
}

func (b *Builder) Build() (AutomationConfig, error) {
	hostnames := make([]string, b.members)
	for i := 0; i < b.members; i++ {
		hostnames[i] = fmt.Sprintf("%s-%d.%s", b.name, i, b.domain)
	}

	members := make([]ReplicaSetMember, b.members)
	processes := make([]Process, b.members)
	for i, h := range hostnames {
		opts := []func(*Process){
			withFCV(b.fcv),
		}

		process := newProcess(toHostName(b.name, i), h, b.mongodbVersion, b.name, opts...)
		processes[i] = process

		if b.replicaSetHorizons != nil {
			members[i] = newReplicaSetMember(process, i, b.replicaSetHorizons[i])
		} else {
			members[i] = newReplicaSetMember(process, i, nil)
		}
	}

	auth := disabledAuth()
	if b.enabler != nil {
		auth = b.enabler.EnableAuth(auth)
	}

	currentAc := AutomationConfig{
		Version:   b.previousAC.Version,
		Processes: processes,
		ReplicaSets: []ReplicaSet{
			{
				Id:              b.name,
				Members:         members,
				ProtocolVersion: "1",
			},
		},
		Versions: b.versions,
		Options:  b.options,
		Auth:     auth,
		TLS: TLS{
			ClientCertificateMode: ClientCertificateModeOptional,
		},
	}

	// Apply all modifications
	for _, modification := range b.modifications {
		modification(&currentAc)
	}

	// Here we compare the bytes of the two automationconfigs,
	// we can't use reflect.DeepEqual() as it treats nil entries as different from empty ones,
	// and in the AutomationConfig Struct we use omitempty to set empty field to nil
	// The agent requires the nil value we provide, otherwise the agent attempts to configure authentication.

	newAcBytes, err := json.Marshal(b.previousAC)
	if err != nil {
		return AutomationConfig{}, err
	}

	currentAcBytes, err := json.Marshal(currentAc)
	if err != nil {
		return AutomationConfig{}, err
	}

	if !bytes.Equal(newAcBytes, currentAcBytes) {
		currentAc.Version++
	}
	return currentAc, nil
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
