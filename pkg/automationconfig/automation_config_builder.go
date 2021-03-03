package automationconfig

import (
	"fmt"
	"path"
)

type Topology string

const (
	ReplicaSetTopology Topology = "ReplicaSet"
	maxVotingMembers   int      = 7
)

type Modification func(*AutomationConfig)

func NOOP() Modification {
	return func(config *AutomationConfig) {}
}

type Builder struct {
	processes          []Process
	replicaSets        []ReplicaSet
	replicaSetHorizons []ReplicaSetHorizons
	members            int
	domain             string
	name               string
	fcv                *string
	topology           Topology
	mongodbVersion     string
	previousAC         AutomationConfig
	// MongoDB installable versions
	versions             []MongoDbVersionConfig
	backupVersions       []BackupVersion
	monitoringVersions   []MonitoringVersion
	options              Options
	modifications        []Modification
	auth                 *Auth
	processModifications []func(int, *Process)
}

func NewBuilder() *Builder {
	return &Builder{
		processes:            []Process{},
		replicaSets:          []ReplicaSet{},
		versions:             []MongoDbVersionConfig{},
		modifications:        []Modification{},
		backupVersions:       []BackupVersion{},
		monitoringVersions:   []MonitoringVersion{},
		processModifications: []func(int, *Process){},
	}
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

func (b *Builder) SetFCV(fcv *string) *Builder {
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

func (b *Builder) SetBackupVersions(versions []BackupVersion) *Builder {
	b.backupVersions = versions
	return b
}

func (b *Builder) SetMonitoringVersions(versions []MonitoringVersion) *Builder {
	b.monitoringVersions = versions
	return b
}

func (b *Builder) SetPreviousAutomationConfig(previousAC AutomationConfig) *Builder {
	b.previousAC = previousAC
	return b
}

func (b *Builder) SetAuth(auth Auth) *Builder {
	b.auth = &auth
	return b
}

func (b *Builder) AddProcessModification(f func(int, *Process)) *Builder {
	b.processModifications = append(b.processModifications, f)
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

		fcv := "4.0"
		if b.fcv != nil {
			fcv = *b.fcv
		}

		process := &Process{
			Name:                        toProcessName(b.name, i),
			HostName:                    h,
			FeatureCompatibilityVersion: fcv,
			ProcessType:                 Mongod,
			Version:                     b.mongodbVersion,
			AuthSchemaVersion:           5,
		}
		process.SetSystemLog(SystemLog{
			Destination: "file",
			Path:        path.Join(DefaultAgentLogPath, "/mongodb.log"),
		})

		process.SetPort(27017)
		process.SetStoragePath(DefaultMongoDBDataDir)
		process.SetReplicaSetName(b.name)

		for _, mod := range b.processModifications {
			mod(i, process)
		}

		processes[i] = *process

		totalVotes := 0
		if b.replicaSetHorizons != nil {
			members[i] = newReplicaSetMember(*process, i, b.replicaSetHorizons[i], totalVotes)
		} else {
			members[i] = newReplicaSetMember(*process, i, nil, totalVotes)
		}
		totalVotes += members[i].Votes
	}

	if b.auth == nil {
		disabled := disabledAuth()
		b.auth = &disabled
	}

	if len(b.versions) == 0 {
		b.versions = append(b.versions, buildDummyMongoDbVersionConfig(b.mongodbVersion))
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
		MonitoringVersions: b.monitoringVersions,
		BackupVersions:     b.backupVersions,
		Versions:           b.versions,
		Options:            b.options,
		Auth:               *b.auth,
		TLS: TLS{
			ClientCertificateMode: ClientCertificateModeOptional,
		},
	}

	// Apply all modifications
	for _, modification := range b.modifications {
		modification(&currentAc)
	}

	areEqual, err := AreEqual(b.previousAC, currentAc)
	if err != nil {
		return AutomationConfig{}, err
	}

	if !areEqual {
		currentAc.Version++
	}
	return currentAc, nil
}

func toProcessName(name string, index int) string {
	return fmt.Sprintf("%s-%d", name, index)
}

// buildDummyMongoDbVersionConfig create a MongoDbVersionConfig which
// will be valid for any version of MongoDB. This is used as a default if no
// versions are manually specified.
func buildDummyMongoDbVersionConfig(version string) MongoDbVersionConfig {
	return MongoDbVersionConfig{
		Name: version,
		Builds: []BuildConfig{
			{
				Platform:     "linux",
				Architecture: "amd64",
				Flavor:       "rhel",
				Modules:      []string{},
			},
			{
				Platform:     "linux",
				Architecture: "amd64",
				Flavor:       "ubuntu",
				Modules:      []string{},
			},
		},
	}
}
