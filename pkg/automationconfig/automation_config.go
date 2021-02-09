package automationconfig

import (
	"path"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/authentication/scramcredentials"
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/generate"
	"github.com/stretchr/objx"
)

const (
	Mongod                ProcessType = "mongod"
	DefaultMongoDBDataDir string      = "/data"
	DefaultAgentLogPath   string      = "/var/log/mongodb-mms-automation"
	SecretKey                         = "cluster-config.json"
)

type AutomationConfig struct {
	Version     int                    `json:"version"`
	Processes   []Process              `json:"processes"`
	ReplicaSets []ReplicaSet           `json:"replicaSets"`
	Auth        Auth                   `json:"auth"`
	TLS         TLS                    `json:"tls"`
	Versions    []MongoDbVersionConfig `json:"mongoDbVersions"`
	Options     Options                `json:"options"`
	Roles       []CustomRole           `json:"roles,omitempty"`
}

// EnsurePassword makes sure that there is an Automation Agent password
// that the agents will use to communicate with the deployments. The password
// is returned so it can be provided to the other agents
func (ac *AutomationConfig) EnsurePassword() (string, error) {
	if ac.Auth.AutoPwd == "" {
		generatedPassword, err := generate.KeyFileContents()
		if err != nil {
			return "", err
		}
		ac.Auth.AutoPwd = generatedPassword
	}
	return ac.Auth.AutoPwd, nil
}

// EnsureKeyFileContents makes sure a valid keyfile is generated and used for internal cluster authentication
func (ac *AutomationConfig) EnsureKeyFileContents() error {
	if ac.Auth.Key == "" {
		keyfileContents, err := generate.KeyFileContents()
		if err != nil {
			return err
		}
		ac.Auth.Key = keyfileContents
	}
	return nil
}

type Process struct {
	Name                        string      `json:"name"`
	HostName                    string      `json:"hostname"`
	Args26                      objx.Map    `json:"args2_6"`
	FeatureCompatibilityVersion string      `json:"featureCompatibilityVersion"`
	ProcessType                 ProcessType `json:"processType"`
	Version                     string      `json:"version"`
	AuthSchemaVersion           int         `json:"authSchemaVersion"`
	SystemLog                   SystemLog   `json:"systemLog"`
	WiredTiger                  WiredTiger  `json:"wiredTiger"`
}

func newProcess(name, hostName, version, replSetName string, opts ...func(process *Process)) Process {
	args26 := objx.New(map[string]interface{}{})
	args26.Set("net.port", 27017)
	args26.Set("storage.dbPath", DefaultMongoDBDataDir)
	args26.Set("replication.replSetName", replSetName)

	p := Process{
		Name:                        name,
		HostName:                    hostName,
		FeatureCompatibilityVersion: "4.0",
		ProcessType:                 Mongod,
		Version:                     version,
		SystemLog: SystemLog{
			Destination: "file",
			Path:        path.Join(DefaultAgentLogPath, "/mongodb.log"),
		},
		AuthSchemaVersion: 5,
		Args26:            args26,
	}

	for _, opt := range opts {
		opt(&p)
	}

	return p
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

func newReplicaSetMember(p Process, id int, horizons ReplicaSetHorizons) ReplicaSetMember {
	return ReplicaSetMember{
		Id:          id,
		Host:        p.Name,
		Priority:    1,
		ArbiterOnly: false,
		Votes:       1,
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

// BuildsForVersion returns the MongoDbVersionConfig containing all of the version informatioon
// for the given mongodb version provided
func (v VersionManifest) BuildsForVersion(version string) MongoDbVersionConfig {
	var builds []BuildConfig
	for _, versionConfig := range v.Versions {
		if versionConfig.Name != version {
			continue
		}
		builds = versionConfig.Builds
		break
	}
	return MongoDbVersionConfig{
		Name:   version,
		Builds: builds,
	}
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
