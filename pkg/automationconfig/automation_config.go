package automationconfig

import (
	"bytes"
	"encoding/json"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"github.com/stretchr/objx"
)

const (
	Mongod                ProcessType = "mongod"
	DefaultMongoDBDataDir string      = "/data"
	DefaultAgentLogPath   string      = "/var/log/mongodb-mms-automation"
)

type AutomationConfig struct {
	Version     int          `json:"version"`
	Processes   []Process    `json:"processes"`
	ReplicaSets []ReplicaSet `json:"replicaSets"`
	Auth        Auth         `json:"auth"`

	// TLSConfig and SSLConfig exist to allow configuration of older agents which accept the "ssl" field rather or "tls"
	// only one of these should be set.
	TLSConfig *TLS `json:"tls,omitempty"`
	SSLConfig *TLS `json:"ssl,omitempty"`

	Versions           []MongoDbVersionConfig `json:"mongoDbVersions"`
	BackupVersions     []BackupVersion        `json:"backupVersions"`
	MonitoringVersions []MonitoringVersion    `json:"monitoringVersions"`
	Options            Options                `json:"options"`
	Roles              []CustomRole           `json:"roles,omitempty"`
}

type BackupVersion struct {
	BaseUrl string `json:"baseUrl"`
}

type MonitoringVersion struct {
	Hostname         string            `json:"hostname"`
	Name             string            `json:"name"`
	BaseUrl          string            `json:"baseUrl"`
	AdditionalParams map[string]string `json:"additionalParams,omitempty"`
}

type Process struct {
	Name                        string      `json:"name"`
	HostName                    string      `json:"hostname"`
	Args26                      objx.Map    `json:"args2_6"`
	FeatureCompatibilityVersion string      `json:"featureCompatibilityVersion"`
	ProcessType                 ProcessType `json:"processType"`
	Version                     string      `json:"version"`
	AuthSchemaVersion           int         `json:"authSchemaVersion"`
}

func (p *Process) SetPort(port int) *Process {
	return p.SetArgs26Field("net.port", port)
}

func (p *Process) SetStoragePath(storagePath string) *Process {
	return p.SetArgs26Field("storage.dbPath", storagePath)
}

func (p *Process) SetReplicaSetName(replSetName string) *Process {
	return p.SetArgs26Field("replication.replSetName", replSetName)
}

func (p *Process) SetSystemLog(systemLog SystemLog) *Process {
	return p.SetArgs26Field("systemLog.path", systemLog.Path).
		SetArgs26Field("systemLog.destination", systemLog.Destination).
		SetArgs26Field("systemLog.logAppend", systemLog.LogAppend)
}

func (p *Process) SetWiredTigerCache(cacheSizeGb *float32) *Process {
	if cacheSizeGb == nil {
		return p
	}
	return p.SetArgs26Field("storage.wiredTiger.engineConfig.cacheSizeGB", cacheSizeGb)
}

// SetArgs26Field should be used whenever any args26 field needs to be set. It ensures
// that the args26 map is non nil and assigns the given value.
func (p *Process) SetArgs26Field(fieldName string, value interface{}) *Process {
	p.ensureArgs26()
	p.Args26.Set(fieldName, value)
	return p
}

func (p *Process) ensureArgs26() {
	if p.Args26 == nil {
		p.Args26 = objx.New(map[string]interface{}{})
	}
}

type TLSMode string

const (
	TLSModeDisabled  TLSMode = "disabled"
	TLSModeAllowed   TLSMode = "allowTLS"
	TLSModePreferred TLSMode = "preferTLS"
	TLSModeRequired  TLSMode = "requireTLS"
)

type ProcessType string

type SystemLog struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
	LogAppend   bool   `json:"logAppend"`
}

type WiredTiger struct {
	EngineConfig EngineConfig `json:"engineConfig"`
}

type EngineConfig struct {
	CacheSizeGB float32 `json:"cacheSizeGB"`
}

type ReplicaSet struct {
	Id              string             `json:"_id"`
	Members         []ReplicaSetMember `json:"members"`
	ProtocolVersion string             `json:"protocolVersion"`
	NumberArbiters  int                `json:"numberArbiters"`
}

type ReplicaSetMember struct {
	Id          int                `json:"_id"`
	Host        string             `json:"host"`
	Priority    int                `json:"priority"`
	ArbiterOnly bool               `json:"arbiterOnly"`
	Votes       int                `json:"votes"`
	Horizons    ReplicaSetHorizons `json:"horizons,omitempty"`
}

type ReplicaSetHorizons map[string]string

func newReplicaSetMember(p Process, id int, horizons ReplicaSetHorizons, totalVotesSoFar int, numberArbiters int) ReplicaSetMember {
	// ensure that the number of voting members in the replica set is not more than 7
	// as this is the maximum number of voting members.
	votes := 1
	priority := 1

	isArbiter := totalVotesSoFar < numberArbiters

	if totalVotesSoFar > maxVotingMembers {
		votes = 0
		priority = 0
	}

	return ReplicaSetMember{
		Id:          id,
		Host:        p.Name,
		Priority:    priority,
		ArbiterOnly: isArbiter,
		Votes:       votes,
		Horizons:    horizons,
	}
}

type Auth struct {
	// Users is a list which contains the desired users at the project level.
	Users    []MongoDBUser `json:"usersWanted,omitempty"`
	Disabled bool          `json:"disabled"`
	// AuthoritativeSet indicates if the MongoDBUsers should be synced with the current list of Users
	AuthoritativeSet bool `json:"authoritativeSet"`
	// AutoAuthMechanisms is a list of auth mechanisms the Automation Agent is able to use
	AutoAuthMechanisms []string `json:"autoAuthMechanisms,omitempty"`

	// AutoAuthMechanism is the currently active agent authentication mechanism. This is a read only
	// field
	AutoAuthMechanism string `json:"autoAuthMechanism"`
	// DeploymentAuthMechanisms is a list of possible auth mechanisms that can be used within deployments
	DeploymentAuthMechanisms []string `json:"deploymentAuthMechanisms,omitempty"`
	// AutoUser is the MongoDB Automation Agent user, when x509 is enabled, it should be set to the subject of the AA's certificate
	AutoUser string `json:"autoUser,omitempty"`
	// Key is the contents of the KeyFile, the automation agent will ensure this a KeyFile with these contents exists at the `KeyFile` path
	Key string `json:"key,omitempty"`
	// KeyFile is the path to a keyfile with read & write permissions. It is a required field if `Disabled=false`
	KeyFile string `json:"keyfile,omitempty"`
	// KeyFileWindows is required if `Disabled=false` even if the value is not used
	KeyFileWindows string `json:"keyfileWindows,omitempty"`
	// AutoPwd is a required field when going from `Disabled=false` to `Disabled=true`
	AutoPwd string `json:"autoPwd,omitempty"`
}

type CustomRole struct {
	Role                       string                      `json:"role"`
	DB                         string                      `json:"db"`
	Privileges                 []Privilege                 `json:"privileges"`
	Roles                      []Role                      `json:"roles"`
	AuthenticationRestrictions []AuthenticationRestriction `json:"authenticationRestrictions,omitempty"`
}

type Privilege struct {
	Resource Resource `json:"resource"`
	Actions  []string `json:"actions"`
}

type Resource struct {
	DB          *string `json:"db,omitempty"`
	Collection  *string `json:"collection,omitempty"`
	AnyResource bool    `json:"anyResource,omitempty"`
	Cluster     bool    `json:"cluster,omitempty"`
}

type AuthenticationRestriction struct {
	ClientSource  []string `json:"clientSource"`
	ServerAddress []string `json:"serverAddress"`
}

type MongoDBUser struct {
	Mechanisms                 []string `json:"mechanisms"`
	Roles                      []Role   `json:"roles"`
	Username                   string   `json:"user"`
	Database                   string   `json:"db"`
	AuthenticationRestrictions []string `json:"authenticationRestrictions"`

	// ScramShaCreds are generated by the operator.
	ScramSha256Creds *scramcredentials.ScramCreds `json:"scramSha256Creds"`
	ScramSha1Creds   *scramcredentials.ScramCreds `json:"scramSha1Creds"`
}

type Role struct {
	Role     string `json:"role"`
	Database string `json:"db"`
}

func disabledAuth() Auth {
	return Auth{
		Users:                    make([]MongoDBUser, 0),
		AutoAuthMechanisms:       make([]string, 0),
		DeploymentAuthMechanisms: make([]string, 0),
		AutoAuthMechanism:        "MONGODB-CR",
		Disabled:                 true,
	}
}

type ClientCertificateMode string

const (
	ClientCertificateModeOptional ClientCertificateMode = "OPTIONAL"
	ClientCertificateModeRequired ClientCertificateMode = "REQUIRED"
)

type TLS struct {
	CAFilePath            string                `json:"CAFilePath"`
	ClientCertificateMode ClientCertificateMode `json:"clientCertificateMode"`
}

type LogRotate struct {
	SizeThresholdMB  int `json:"sizeThresholdMB"`
	TimeThresholdHrs int `json:"timeThresholdHrs"`
}

type ToolsVersion struct {
	Version string                       `json:"version"`
	URLs    map[string]map[string]string `json:"urls"`
}

type Options struct {
	DownloadBase string `json:"downloadBase"`
}

type VersionManifest struct {
	Updated  int                    `json:"updated"`
	Versions []MongoDbVersionConfig `json:"versions"`
}

type BuildConfig struct {
	Platform     string   `json:"platform"`
	Url          string   `json:"url"`
	GitVersion   string   `json:"gitVersion"`
	Architecture string   `json:"architecture"`
	Flavor       string   `json:"flavor"`
	MinOsVersion string   `json:"minOsVersion"`
	MaxOsVersion string   `json:"maxOsVersion"`
	Modules      []string `json:"modules"`
	// Note, that we are not including all "windows" parameters like "Win2008plus" as such distros won't be used
}

type MongoDbVersionConfig struct {
	Name   string        `json:"name"`
	Builds []BuildConfig `json:"builds"`
}

// AreEqual returns whether or not the given AutomationConfigs have the same contents.
// the comparison does not take version into account.
func AreEqual(ac0, ac1 AutomationConfig) (bool, error) {
	// Here we compare the bytes of the two automationconfigs,
	// we can't use reflect.DeepEqual() as it treats nil entries as different from empty ones,
	// and in the AutomationConfig Struct we use omitempty to set empty field to nil
	// The agent requires the nil value we provide, otherwise the agent attempts to configure authentication.
	ac0.Version = ac1.Version
	ac0Bytes, err := json.Marshal(ac0)
	if err != nil {
		return false, err
	}

	ac1Bytes, err := json.Marshal(ac1)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ac0Bytes, ac1Bytes), nil
}

func FromBytes(acBytes []byte) (AutomationConfig, error) {
	ac := AutomationConfig{}
	if err := json.Unmarshal(acBytes, &ac); err != nil {
		return AutomationConfig{}, err
	}
	return ac, nil
}
