package automationconfig

import (
	"bytes"
	"encoding/json"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"go.uber.org/zap"
)

const (
	Mongod                              ProcessType = "mongod"
	DefaultMongoDBDataDir               string      = "/data"
	DefaultDBPort                       int         = 27017
	DefaultAgentLogPath                 string      = "/var/log/mongodb-mms-automation"
	DefaultAgentLogFile                 string      = "/var/log/mongodb-mms-automation/automation-agent.log"
	DefaultAgentMaxLogFileDurationHours int         = 24
)

// +kubebuilder:object:generate=true
type MemberOptions struct {
	Votes    *int              `json:"votes,omitempty"`
	Priority *string           `json:"priority,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
}

func (o *MemberOptions) GetVotes() int {
	if o.Votes != nil {
		return cast.ToInt(o.Votes)
	}
	return 1
}

func (o *MemberOptions) GetPriority() float32 {
	if o.Priority != nil {
		return cast.ToFloat32(o.Priority)
	}
	return 1.0
}

func (o *MemberOptions) GetTags() map[string]string {
	return o.Tags
}

type AutomationConfig struct {
	Version     int          `json:"version"`
	Processes   []Process    `json:"processes"`
	ReplicaSets []ReplicaSet `json:"replicaSets"`
	Auth        Auth         `json:"auth"`
	Prometheus  *Prometheus  `json:"prometheus,omitempty"`

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

func (ac *AutomationConfig) GetProcessByName(name string) *Process {
	for i := 0; i < len(ac.Processes); i++ {
		if ac.Processes[i].Name == name {
			return &ac.Processes[i]
		}
	}

	return nil
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

// CrdLogRotate is the crd definition of LogRotate including fields in strings while the agent supports them as float64
type CrdLogRotate struct {
	LogRotate `json:",inline"`
	// Maximum size for an individual log file before rotation.
	// The string needs to be able to be converted to float64.
	// Fractional values of MB are supported.
	SizeThresholdMB string `json:"sizeThresholdMB"`
	// Maximum percentage of the total disk space these log files should take up.
	// The string needs to be able to be converted to float64
	// +optional
	PercentOfDiskspace string `json:"percentOfDiskspace,omitempty"`
}

// AcLogRotate is the internal agent representation of LogRotate
type AcLogRotate struct {
	LogRotate `json:",inline"`
	// Maximum size for an individual log file before rotation.
	SizeThresholdMB float64 `json:"sizeThresholdMB"`
	// Maximum percentage of the total disk space these log files should take up.
	// +optional
	PercentOfDiskspace float64 `json:"percentOfDiskspace,omitempty"`
}

// LogRotate matches the setting defined here:
// https://www.mongodb.com/docs/ops-manager/current/reference/cluster-configuration/#mongodb-instances
// and https://www.mongodb.com/docs/rapid/reference/command/logRotate/#mongodb-dbcommand-dbcmd.logRotate
// +kubebuilder:object:generate=true
type LogRotate struct {
	// maximum hours for an individual log file before rotation
	TimeThresholdHrs int `json:"timeThresholdHrs"`
	// maximum number of log files to leave uncompressed
	// +optional
	NumUncompressed int `json:"numUncompressed,omitempty"`
	// maximum number of log files to have total
	// +optional
	NumTotal int `json:"numTotal,omitempty"`
	// set to 'true' to have the Automation Agent rotate the audit files along
	// with mongodb log files
	// +optional
	IncludeAuditLogsWithMongoDBLogs bool `json:"includeAuditLogsWithMongoDBLogs,omitempty"`
}

type Process struct {
	Name                        string       `json:"name"`
	Disabled                    bool         `json:"disabled"`
	HostName                    string       `json:"hostname"`
	Args26                      objx.Map     `json:"args2_6"`
	FeatureCompatibilityVersion string       `json:"featureCompatibilityVersion"`
	ProcessType                 ProcessType  `json:"processType"`
	Version                     string       `json:"version"`
	AuthSchemaVersion           int          `json:"authSchemaVersion"`
	LogRotate                   *AcLogRotate `json:"logRotate,omitempty"`
	AuditLogRotate              *AcLogRotate `json:"auditLogRotate,omitempty"`
}

func (p *Process) SetPort(port int) *Process {
	return p.SetArgs26Field("net.port", port)
}

func (p *Process) GetPort() int {
	if p.Args26 == nil {
		return 0
	}

	// Args26 map could be manipulated from the code, e.g. via SetPort (e.g. in unit tests) - then it will be as int,
	// or it could be deserialized from JSON and then integer in an untyped map will be deserialized as float64.
	// It's behavior of https://pkg.go.dev/encoding/json#Unmarshal that is converting JSON integers as float64.
	netPortValue := p.Args26.Get("net.port")
	if netPortValue.IsFloat64() {
		return int(netPortValue.Float64())
	}

	return netPortValue.Int()
}

func (p *Process) SetStoragePath(storagePath string) *Process {
	return p.SetArgs26Field("storage.dbPath", storagePath)
}

func (p *Process) SetReplicaSetName(replSetName string) *Process {
	return p.SetArgs26Field("replication.replSetName", replSetName)
}

func (p *Process) SetSystemLog(systemLog SystemLog) *Process {
	return p.SetArgs26Field("systemLog.path", systemLog.Path).
		// since Destination is a go type wrapper around string, we will need to force it back to string otherwise
		// SetArgs value boxing takes the upper (Destination) type instead of string.
		SetArgs26Field("systemLog.destination", string(systemLog.Destination)).
		SetArgs26Field("systemLog.logAppend", systemLog.LogAppend)
}

// SetLogRotate sets the acLogRotate by converting the CrdLogRotate to an acLogRotate.
func (p *Process) SetLogRotate(lr *CrdLogRotate) *Process {
	p.LogRotate = ConvertCrdLogRotateToAC(lr)
	return p
}

// SetAuditLogRotate sets the acLogRotate by converting the CrdLogRotate to an acLogRotate.
func (p *Process) SetAuditLogRotate(lr *CrdLogRotate) *Process {
	p.AuditLogRotate = ConvertCrdLogRotateToAC(lr)
	return p
}

// ConvertCrdLogRotateToAC converts a CrdLogRotate to an AcLogRotate representation.
func ConvertCrdLogRotateToAC(lr *CrdLogRotate) *AcLogRotate {
	if lr == nil {
		return &AcLogRotate{}
	}

	return &AcLogRotate{
		LogRotate: LogRotate{
			TimeThresholdHrs:                lr.TimeThresholdHrs,
			NumUncompressed:                 lr.NumUncompressed,
			NumTotal:                        lr.NumTotal,
			IncludeAuditLogsWithMongoDBLogs: lr.IncludeAuditLogsWithMongoDBLogs,
		},
		SizeThresholdMB:    cast.ToFloat64(lr.SizeThresholdMB),
		PercentOfDiskspace: cast.ToFloat64(lr.PercentOfDiskspace),
	}
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

type Destination string

const (
	File   Destination = "file"
	Syslog Destination = "syslog"
)

type SystemLog struct {
	Destination Destination `json:"destination"`
	Path        string      `json:"path"`
	LogAppend   bool        `json:"logAppend"`
}

type WiredTiger struct {
	EngineConfig EngineConfig `json:"engineConfig"`
}

type EngineConfig struct {
	CacheSizeGB float32 `json:"cacheSizeGB"`
}

// ReplSetForceConfig setting enables us to force reconfigure automation agent when the MongoDB deployment
// is in a broken state - for ex: doesn't have a primary.
// More info: https://www.mongodb.com/docs/ops-manager/current/reference/api/automation-config/automation-config-parameters/#replica-sets
type ReplSetForceConfig struct {
	CurrentVersion int64 `json:"currentVersion"`
}

type ReplicaSet struct {
	Id              string                 `json:"_id"`
	Members         []ReplicaSetMember     `json:"members"`
	ProtocolVersion string                 `json:"protocolVersion"`
	NumberArbiters  int                    `json:"numberArbiters"`
	Force           *ReplSetForceConfig    `json:"force,omitempty"`
	Settings        map[string]interface{} `json:"settings,omitempty"`
}

type ReplicaSetMember struct {
	Id          int                `json:"_id"`
	Host        string             `json:"host"`
	ArbiterOnly bool               `json:"arbiterOnly"`
	Horizons    ReplicaSetHorizons `json:"horizons,omitempty"`
	// this is duplicated here instead of using MemberOptions because type of priority
	// is different in AC from the CR(CR don't support float) - hence all the members are declared
	// separately
	Votes    *int              `json:"votes,omitempty"`
	Priority *float32          `json:"priority,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
}

type ReplicaSetHorizons map[string]string

// newReplicaSetMember returns a ReplicaSetMember.
func newReplicaSetMember(name string, id int, horizons ReplicaSetHorizons, isArbiter bool, isVotingMember bool) ReplicaSetMember {
	// ensure that the number of voting members in the replica set is not more than 7
	// as this is the maximum number of voting members.
	votes := 0
	priority := float32(0.0)

	if isVotingMember {
		votes = 1
		priority = 1
	}

	return ReplicaSetMember{
		Id:          id,
		Host:        name,
		ArbiterOnly: isArbiter,
		Horizons:    horizons,
		Votes:       &votes,
		Priority:    &priority,
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

	// AutoAuthMechanism is the currently active agent authentication mechanism. This is a read only field
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
	// UsersDeleted is an array of DeletedUser objects that define the authenticated users to be deleted from specified databases
	UsersDeleted []DeletedUser `json:"usersDeleted,omitempty"`
}

type DeletedUser struct {
	// User is the username that should be deleted
	User string `json:"user,omitempty"`
	// Dbs is the array of database names from which the authenticated user should be deleted
	Dbs []string `json:"dbs,omitempty"`
}

type Prometheus struct {
	Enabled        bool   `json:"enabled"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	PasswordHash   string `json:"passwordHash,omitempty"`
	PasswordSalt   string `json:"passwordSalt,omitempty"`
	Scheme         string `json:"scheme"`
	TLSPemPath     string `json:"tlsPemPath"`
	TLSPemPassword string `json:"tlsPemPassword"`
	Mode           string `json:"mode"`
	ListenAddress  string `json:"listenAddress"`
	MetricsPath    string `json:"metricsPath"`
}

func NewDefaultPrometheus(username string) Prometheus {
	return Prometheus{
		Enabled:       true,
		Username:      username,
		Scheme:        "http",
		Mode:          "opsManager",
		ListenAddress: "0.0.0.0:9216",
		MetricsPath:   "/metrics",
	}
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
	ScramSha256Creds *scramcredentials.ScramCreds `json:"scramSha256Creds,omitempty"`
	ScramSha1Creds   *scramcredentials.ScramCreds `json:"scramSha1Creds,omitempty"`
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
	ClientCertificateModeRequired ClientCertificateMode = "REQUIRE"
)

type TLS struct {
	CAFilePath            string                `json:"CAFilePath"`
	AutoPEMKeyFilePath    string                `json:"autoPEMKeyFilePath,omitempty"`
	ClientCertificateMode ClientCertificateMode `json:"clientCertificateMode"`
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

// AreEqual returns whether the given AutomationConfigs have the same contents.
// the comparison does not take the version into account.
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

func ConfigureAgentConfiguration(systemLog *SystemLog, logRotate *CrdLogRotate, auditLR *CrdLogRotate, p *Process) {
	if systemLog != nil {
		p.SetSystemLog(*systemLog)
	}

	if logRotate != nil {
		if systemLog == nil {
			zap.S().Warn("Configuring LogRotate without systemLog will not work")
		}
		if systemLog != nil && systemLog.Destination == Syslog {
			zap.S().Warn("Configuring LogRotate with systemLog.Destination = Syslog will not work")
		}
		p.SetLogRotate(logRotate)
		p.SetAuditLogRotate(auditLR)
	}

}
