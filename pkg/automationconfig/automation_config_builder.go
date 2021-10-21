package automationconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/versions"
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
	arbiters           int
	domain             string
	name               string
	fcv                string
	topology           Topology
	mongodbVersion     string
	previousAC         AutomationConfig
	// MongoDB installable versions
	versions             []MongoDbVersionConfig
	backupVersions       []BackupVersion
	monitoringVersions   []MonitoringVersion
	options              Options
	processModifications []func(int, *Process)
	modifications        []Modification
	auth                 *Auth
	cafilePath           string
	sslConfig            *TLS
	tlsConfig            *TLS
	dataDir              string
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
		tlsConfig:            nil,
		sslConfig:            nil,
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

func (b *Builder) SetTLSConfig(tlsConfig TLS) *Builder {
	b.tlsConfig = &tlsConfig
	return b
}

func (b *Builder) SetSSLConfig(sslConfig TLS) *Builder {
	b.sslConfig = &sslConfig
	return b
}

func (b *Builder) SetMembers(members int) *Builder {
	b.members = members
	return b
}

func (b *Builder) SetArbiters(arbiters int) *Builder {
	b.arbiters = arbiters
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

func (b *Builder) SetDataDir(dataDir string) *Builder {
	b.dataDir = dataDir
	return b
}

func (b *Builder) SetFCV(fcv string) *Builder {
	b.fcv = fcv
	return b
}

func (b *Builder) SetCAFilePath(caFilePath string) *Builder {
	b.cafilePath = caFilePath
	return b
}

func (b *Builder) AddVersions(versions []MongoDbVersionConfig) *Builder {
	for _, v := range versions {
		b.AddVersion(v)
	}
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

func (b *Builder) setFeatureCompatibilityVersionIfUpgradeIsHappening() error {
	// If we are upgrading, we can't increase featureCompatibilityVersion
	// as that will make the agent never reach goal state
	if len(b.previousAC.Processes) > 0 && b.fcv == "" {

		// Create a x.y.0 version from FCV x.y
		previousFCV := b.previousAC.Processes[0].FeatureCompatibilityVersion
		previousFCVsemver, err := semver.Make(fmt.Sprintf("%s.0", previousFCV))
		if err != nil {
			return errors.Errorf("can't compute semver version from previous FeatureCompatibilityVersion %s", previousFCV)
		}

		currentVersionSemver, err := semver.Make(b.mongodbVersion)
		if err != nil {
			return errors.Errorf("current MongoDB version is not a valid semver version: %s", b.mongodbVersion)
		}

		// We would increase FCV here.
		// Note: in theory this will also catch upgrade like 4.2.0 -> 4.2.1 but we don't care about those
		// as they would not change the FCV
		if currentVersionSemver.GT(previousFCVsemver) {
			b.fcv = previousFCV
		}
	}
	return nil
}

func (b *Builder) Build() (AutomationConfig, error) {
	hostnames := make([]string, b.members)
	for i := 0; i < b.members; i++ {
		hostnames[i] = fmt.Sprintf("%s-%d.%s", b.name, i, b.domain)
	}

	members := make([]ReplicaSetMember, b.members)
	processes := make([]Process, b.members)

	if err := b.setFeatureCompatibilityVersionIfUpgradeIsHappening(); err != nil {
		return AutomationConfig{}, errors.Errorf("can't build the automation config: %s", err)
	}

	dataDir := DefaultMongoDBDataDir
	if b.dataDir != "" {
		dataDir = b.dataDir
	}

	totalVotes := 0
	for i, h := range hostnames {

		process := &Process{
			Name:                        toProcessName(b.name, i),
			HostName:                    h,
			FeatureCompatibilityVersion: versions.CalculateFeatureCompatibilityVersion(b.mongodbVersion),
			ProcessType:                 Mongod,
			Version:                     b.mongodbVersion,
			AuthSchemaVersion:           5,
		}

		if b.fcv != "" {
			process.FeatureCompatibilityVersion = b.fcv
		}

		process.SetPort(27017)
		process.SetStoragePath(dataDir)
		process.SetReplicaSetName(b.name)

		for _, mod := range b.processModifications {
			mod(i, process)
		}

		processes[i] = *process

		if b.replicaSetHorizons != nil {
			members[i] = newReplicaSetMember(*process, i, b.replicaSetHorizons[i], totalVotes, b.arbiters)
		} else {
			members[i] = newReplicaSetMember(*process, i, nil, totalVotes, b.arbiters)
		}
		totalVotes += members[i].Votes

	}

	if b.auth == nil {
		disabled := disabledAuth()
		b.auth = &disabled
	}

	dummyConfig := buildDummyMongoDbVersionConfig(b.mongodbVersion)
	if !versionsContain(b.versions, dummyConfig) {
		b.versions = append(b.versions, dummyConfig)
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
		TLSConfig: &TLS{
			ClientCertificateMode: ClientCertificateModeOptional,
			CAFilePath:            b.cafilePath,
		},
	}

	if b.tlsConfig != nil {
		currentAc.TLSConfig = b.tlsConfig
	}

	if b.sslConfig != nil {
		currentAc.SSLConfig = b.sslConfig
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

func versionsContain(versions []MongoDbVersionConfig, version MongoDbVersionConfig) bool {
	for _, v := range versions {
		if reflect.DeepEqual(v, version) {
			return true
		}
	}
	return false
}

// buildDummyMongoDbVersionConfig create a MongoDbVersionConfig which
// will be valid for any version of MongoDB. This is used as a default if no
// versions are manually specified.
func buildDummyMongoDbVersionConfig(version string) MongoDbVersionConfig {
	versionConfig := MongoDbVersionConfig{
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

	// if we are using an enterprise version of MongoDB, we need to add the enterprise string to the modules array.
	if strings.HasSuffix(version, "-ent") {
		for i := range versionConfig.Builds {
			versionConfig.Builds[i].Modules = append(versionConfig.Builds[i].Modules, "enterprise")
		}
	}
	return versionConfig
}
