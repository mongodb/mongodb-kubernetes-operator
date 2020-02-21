package automationconfig

import (
	appsv1 "k8s.io/api/apps/v1"
	"path"
)

type ProcessType string

const (
	Mongod ProcessType = "mongod"
)

type Auth struct {
}

type Process struct {
	Name              string      `json:"name"`
	HostName          string      `json:"hostname"`
	Args26            Args26      `json:"args2_6"`
	Replication       Replication `json:"replication"`
	Storage           Storage     `json:"storage"`
	ProcessType       ProcessType `json:"processType"`
	Version           string      `json:"version"`
	AuthSchemaVersion int         `json:"authSchemaVersion"`
	SystemLog         SystemLog   `json:"systemLog"`
	WiredTiger        WiredTiger  `json:"wiredTiger"`
}

type SystemLog struct {
	Destination string `json:"destination"`
	Path        string `json:"path"`
}

func NewProcess(name, hostName, version string, cacheSizeGb *float32) Process {
	p := Process{
		Name:     name,
		HostName: hostName,
		Storage: Storage{
			DBPath: "/data",
		},
		ProcessType: Mongod,
		Version:     version,
		SystemLog: SystemLog{
			Destination: "file",
			Path:        path.Join("/var/log/mongodb-mms-automation", "/mongodb.log"),
		},
	}

	if cacheSizeGb != nil {
		p.Storage.WiredTiger.EngineConfig.CacheSizeGB = *cacheSizeGb
	}
	return p
}

type Replication struct {
	ReplicaSetName string `json:"replSetName"`
}

type Storage struct {
	DBPath     string     `json:"dbPath"`
	WiredTiger WiredTiger `json:"wiredTiger"`
}

type WiredTiger struct {
	EngineConfig EngineConfig `json:"engineConfig"`
}

type EngineConfig struct {
	CacheSizeGB float32 `json:"cacheSizeGB"`
}

type LogRotate struct {
	SizeThresholdMB  int `json:"sizeThresholdMB"`
	TimeThresholdHrs int `json:"timeThresholdHrs"`
}

type Security struct {
}

type Args26 struct {
}

type ReplicaSet struct {
	Id              string             `json:"_id"`
	Members         []ReplicaSetMember `json:"members"`
	ProtocolVersion string             `json:"protocolVersion"`
}

type ReplicaSetMember struct {
	Id          string `json:"_id"`
	Host        string `json:"host"`
	Priority    int    `json:"priority"`
	ArbiterOnly bool   `json:"arbiterOnly"`
}

//func (*ReplicaSetMember) addMember(p Process) {
//
//}

func NewReplicaSet(processes []Process, name, protocolVersion string) ReplicaSet {
	return ReplicaSet{
		Id:              name,
		Members:         make([]ReplicaSetMember, len(processes)),
		ProtocolVersion: protocolVersion,
	}
}

//func NewReplicaSet(processes []*Process) ReplicaSet {
//	rs := ReplicaSet{
//
//	}
//
//	for _, p := range processes {
//		p.Replication.ReplicaSetName = rs.Id
//	}
//
//	return rs
//}

type AutomationConfig struct {
	Version     string       `json:"version"`
	Processes   []Process    `json:"processes"`
	ReplicaSets []ReplicaSet `json:"replicaSets"`
	Auth        Auth         `json:"auth"`
}

// Writer is an interface which writes an AutomationConfig to a source
// which is consumable by the Automation Agents
type Writer interface {
	Write(AutomationConfig) error
}

//func New() AutomationConfig {
//	return AutomationConfig{
//		Processes:   []Process{},
//		ReplicaSets: []ReplicaSet{},
//	}
//}

//automationconfig.BuildConfig(automationconfig.Thing{
//	sts.Name, sts.Namespace...
//})

//type Options {
	// Service Name, Name, Namespace,
//}

func FromStatefulSet(sts appsv1.StatefulSet, version string) AutomationConfig {
	clusterDomain := "svc.cluster.local" // TODO: make configurable
	hostnames, names := getDnsForStatefulSet(sts, clusterDomain)
	processes := make([]Process, len(hostnames))
	wiredTigerCache := calculateWiredTigerCache(sts, version)
	for idx, hostname := range hostnames {
		processes[idx] = NewProcess(names[idx], hostname, version, wiredTigerCache)
	}

	// reference the replica set from this process
	for i := range processes {
		processes[i].Replication.ReplicaSetName = sts.Name

	}


	builder := NewBuilder().AddProcesses(processes)

	return builder.Build()
}


// Read MDB

// BuildSts struct

// Build AC (from sts)

// Build CM from AC

// Apply STS -> K8s