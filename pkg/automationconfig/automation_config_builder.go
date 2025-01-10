package automationconfig

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/blang/semver"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/versions"

	"k8s.io/utils/ptr"
)

type Topology string

const (
	ReplicaSetTopology    Topology = "ReplicaSet"
	maxVotingMembers      int      = 7
	arbitersStartingIndex int      = 100
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
	arbiterDomain      string
	name               string
	fcv                string
	topology           Topology
	isEnterprise       bool
	mongodbVersion     string
	previousAC         AutomationConfig
	// MongoDB installable versions
	versions                  []MongoDbVersionConfig
	backupVersions            []BackupVersion
	monitoringVersions        []MonitoringVersion
	options                   Options
	processModifications      []func(int, *Process)
	modifications             []Modification
	auth                      *Auth
	cafilePath                string
	sslConfig                 *TLS
	tlsConfig                 *TLS
	dataDir                   string
	port                      int
	memberOptions             []MemberOptions
	forceReconfigureToVersion *int64
	replicaSetId              *string
	settings                  map[string]interface{}
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

func (b *Builder) SetMemberOptions(memberOptions []MemberOptions) *Builder {
	b.memberOptions = memberOptions
	return b
}

func (b *Builder) SetOptions(options Options) *Builder {
	b.options = options
	return b
}

func (b *Builder) IsEnterprise(isEnterprise bool) *Builder {
	b.isEnterprise = isEnterprise
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

func (b *Builder) SetArbiterDomain(domain string) *Builder {
	b.arbiterDomain = domain
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

// Deprecated: ports should be set via ProcessModification or Modification
func (b *Builder) SetPort(port int) *Builder {
	b.port = port
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

func (b *Builder) SetReplicaSetId(id *string) *Builder {
	b.replicaSetId = id
	return b
}

func (b *Builder) SetSettings(settings map[string]interface{}) *Builder {
	b.settings = settings
	return b
}

func (b *Builder) SetForceReconfigureToVersion(version int64) *Builder {
	b.forceReconfigureToVersion = &version
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
			return fmt.Errorf("can't compute semver version from previous FeatureCompatibilityVersion %s", previousFCV)
		}

		currentVersionSemver, err := semver.Make(b.mongodbVersion)
		if err != nil {
			return fmt.Errorf("current MongoDB version is not a valid semver version: %s", b.mongodbVersion)
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
	if err := b.setFeatureCompatibilityVersionIfUpgradeIsHappening(); err != nil {
		return AutomationConfig{}, fmt.Errorf("can't build the automation config: %s", err)
	}

	hostnames := make([]string, 0, b.members+b.arbiters)

	// Create hostnames for data-bearing nodes. They start from 0
	for i := 0; i < b.members; i++ {
		hostnames = append(hostnames, fmt.Sprintf("%s-%d.%s", b.name, i, b.domain))
	}

	// Create hostnames for arbiters. They are added right after the regular members
	for i := 0; i < b.arbiters; i++ {
		// Arbiters will be in b.name-arb-svc service
		hostnames = append(hostnames, fmt.Sprintf("%s-arb-%d.%s", b.name, i, b.arbiterDomain))
	}

	members := make([]ReplicaSetMember, b.members+b.arbiters)
	processes := make([]Process, b.members+b.arbiters)

	if b.fcv != "" {
		_, err := semver.Make(fmt.Sprintf("%s.0", b.fcv))

		if err != nil {
			return AutomationConfig{}, fmt.Errorf("invalid feature compatibility version: %s", err)
		}
	}

	if err := b.setFeatureCompatibilityVersionIfUpgradeIsHappening(); err != nil {
		return AutomationConfig{}, fmt.Errorf("can't build the automation config: %s", err)
	}

	dataDir := DefaultMongoDBDataDir
	if b.dataDir != "" {
		dataDir = b.dataDir
	}

	fcv := versions.CalculateFeatureCompatibilityVersion(b.mongodbVersion)
	if len(b.fcv) > 0 {
		fcv = b.fcv
	}

	mongoDBVersion := b.mongodbVersion
	if b.isEnterprise {
		if !strings.HasSuffix(mongoDBVersion, "-ent") {
			mongoDBVersion = mongoDBVersion + "-ent"
		}
	}

	for i, h := range hostnames {
		// Arbiters start counting from b.members and up
		isArbiter := i >= b.members
		replicaSetIndex := i
		processIndex := i

		if isArbiter {
			processIndex = i - b.members
			// The arbiter's index will start on `arbitersStartingIndex` and increase
			// from there. These ids must be kept constant if the data-bearing nodes
			// change indexes, if for instance, they are scaled up and down.
			//
			replicaSetIndex = arbitersStartingIndex + processIndex
		}

		// TODO: Replace with a Builder for Process.
		process := &Process{
			Name:                        toProcessName(b.name, processIndex, isArbiter),
			HostName:                    h,
			FeatureCompatibilityVersion: fcv,
			ProcessType:                 Mongod,
			Version:                     mongoDBVersion,
			AuthSchemaVersion:           5,
		}

		// ports should be change via ProcessModification or Modification
		// left for backwards compatibility, to be removed in the future
		if b.port != 0 {
			process.SetPort(b.port)
		}
		process.SetStoragePath(dataDir)
		process.SetReplicaSetName(b.name)

		for _, mod := range b.processModifications {
			mod(i, process)
		}

		// ensure it has port set
		if process.GetPort() == 0 {
			process.SetPort(DefaultDBPort)
		}

		processes[i] = *process

		var horizon ReplicaSetHorizons
		if b.replicaSetHorizons != nil && i < len(b.replicaSetHorizons) {
			horizon = b.replicaSetHorizons[i]
		}

		// Arbiters can't be non-voting members
		// If there are more than 7 (maxVotingMembers) members on this Replica Set
		// those that lose right to vote should be the data-bearing nodes, not the
		// arbiters.
		isVotingMember := isArbiter || i < (maxVotingMembers-b.arbiters)

		// TODO: Replace with a Builder for ReplicaSetMember.
		members[i] = newReplicaSetMember(process.Name, replicaSetIndex, horizon, isArbiter, isVotingMember)

		if len(b.memberOptions) > i {
			// override the member options if explicitly specified in the spec
			members[i].Votes = b.memberOptions[i].Votes
			members[i].Priority = ptr.To(b.memberOptions[i].GetPriority())
			members[i].Tags = b.memberOptions[i].Tags
		}
	}

	if b.auth == nil {
		disabled := disabledAuth()
		b.auth = &disabled
	}

	dummyConfig := buildDummyMongoDbVersionConfig(mongoDBVersion)
	if !versionsContain(b.versions, dummyConfig) {
		b.versions = append(b.versions, dummyConfig)
	}

	var replSetForceConfig *ReplSetForceConfig
	if b.forceReconfigureToVersion != nil {
		replSetForceConfig = &ReplSetForceConfig{CurrentVersion: *b.forceReconfigureToVersion}
	}

	replicaSetId := b.name
	if b.replicaSetId != nil {
		replicaSetId = *b.replicaSetId
	}

	currentAc := AutomationConfig{
		Version:   b.previousAC.Version,
		Processes: processes,
		ReplicaSets: []ReplicaSet{
			{
				Id:              replicaSetId,
				Members:         members,
				ProtocolVersion: "1",
				NumberArbiters:  b.arbiters,
				Force:           replSetForceConfig,
				Settings:        b.settings,
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

func toProcessName(name string, index int, isArbiter bool) string {
	if isArbiter {
		return fmt.Sprintf("%s-arb-%d", name, index)
	}
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
			{
				Platform:     "linux",
				Architecture: "aarch64",
				Flavor:       "ubuntu",
				Modules:      []string{},
			},
			{
				Platform:     "linux",
				Architecture: "aarch64",
				Flavor:       "rhel",
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
