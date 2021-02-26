package automationconfig

import (
	"path"

	"github.com/stretchr/objx"
)

type processBuilder struct {
	name              string
	hostname          string
	fcv               *string
	processType       ProcessType
	version           string
	systemLog         SystemLog
	authSchemaVersion int
	args26            objx.Map
}

func newProcessBuilder() *processBuilder {
	return &processBuilder{
		processType: Mongod,
		systemLog: SystemLog{
			Destination: "file",
			Path:        path.Join(DefaultAgentLogPath, "/mongodb.log"),
		},
		authSchemaVersion: 5,
	}
}

func (b *processBuilder) Build() Process {
	fcv := "4.0"
	if b.fcv != nil {
		fcv = *b.fcv
	}

	return Process{
		Name:                        b.name,
		HostName:                    b.hostname,
		ProcessType:                 b.processType,
		FeatureCompatibilityVersion: fcv,
		Version:                     b.version,
		SystemLog:                   b.systemLog,
		AuthSchemaVersion:           b.authSchemaVersion,
		Args26:                      b.args26,
	}
}

func (b *processBuilder) SetName(name string) *processBuilder {
	b.name = name
	return b
}

func (b *processBuilder) SetHostName(name string) *processBuilder {
	b.hostname = name
	return b
}

func (b *processBuilder) SetSystemLog(sysLog SystemLog) *processBuilder {
	b.systemLog = sysLog
	return b
}

func (b *processBuilder) SetVersion(version string) *processBuilder {
	b.version = version
	return b
}

func (b *processBuilder) SetFCV(fcv *string) *processBuilder {
	b.fcv = fcv
	return b
}

func (b *processBuilder) SetReplicaSetName(replicaSetName string) *processBuilder {
	return b.SetArgs26Field("replication.replSetName", replicaSetName)
}

func (b *processBuilder) SetPort(port int) *processBuilder {
	return b.SetArgs26Field("net.port", port)
}

func (b *processBuilder) SetDbPath(dbPath string) *processBuilder {
	return b.SetArgs26Field("storage.dbPath", dbPath)
}

func (b *processBuilder) SetWiredTigerCache(cacheSizeGb *float32) *processBuilder {
	if cacheSizeGb == nil {
		return b
	}
	return b.SetArgs26Field("storage.wiredTiger.engineConfig.cacheSizeGB", cacheSizeGb)
}

func (b *processBuilder) SetArgs26Field(fieldName string, value interface{}) *processBuilder {
	b.ensureArgs26()
	b.args26.Set(fieldName, value)
	return b
}

func (b *processBuilder) ensureArgs26() {
	if b.args26 == nil {
		b.args26 = objx.New(map[string]interface{}{})
	}
}

func (b *processBuilder) SetLogPath(logPath string) *processBuilder {

	return b
}
