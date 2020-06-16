package automationconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	mdbv1 "github.com/mongodb/mongodb-kubernetes-operator/pkg/apis/mongodb/v1"
	mdbClient "github.com/mongodb/mongodb-kubernetes-operator/pkg/kube/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	fcv            string
	topology       Topology
	mongodbVersion string
	// MongoDB installable versions
	versions []MongoDbVersionConfig
}

func NewBuilder() *Builder {
	return &Builder{
		processes:   []Process{},
		replicaSets: []ReplicaSet{},
		versions:    []MongoDbVersionConfig{},
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

func getCurrentAutomationConfig(client mdbClient.Client, mdb mdbv1.MongoDB, automationConfigKey string) (AutomationConfig, error) {
	currentCm := corev1.ConfigMap{}
	currentAc := AutomationConfig{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: mdb.ConfigMapName(), Namespace: mdb.Namespace}, &currentCm); err != nil {
		// If the AC was not found we don't surface it as an error
		return AutomationConfig{}, k8sClient.IgnoreNotFound(err)

	}
	if err := json.Unmarshal([]byte(currentCm.Data[automationConfigKey]), &currentAc); err != nil {
		return AutomationConfig{}, err
	}
	return currentAc, nil
}

func (b *Builder) Build(client mdbClient.Client, mdb mdbv1.MongoDB, automationConfigKey string) (AutomationConfig, error) {
	hostnames := make([]string, b.members)
	for i := 0; i < b.members; i++ {
		hostnames[i] = fmt.Sprintf("%s-%d.%s", b.name, i, b.domain)
	}

	members := make([]ReplicaSetMember, b.members)
	processes := make([]Process, b.members)
	for i, h := range hostnames {
		process := newProcess(toHostName(b.name, i), h, b.mongodbVersion, b.name, withFCV(b.fcv))
		processes[i] = process
		members[i] = newReplicaSetMember(process, i)
	}

	previousAC, err := getCurrentAutomationConfig(client, mdb, automationConfigKey)
	if err != nil {
		return AutomationConfig{}, err
	}

	currentAc := AutomationConfig{
		Version:   previousAC.Version,
		Processes: processes,
		ReplicaSets: []ReplicaSet{
			{
				Id:              b.name,
				Members:         members,
				ProtocolVersion: "1",
			},
		},
		Versions: b.versions,
		Options:  Options{DownloadBase: "/var/lib/mongodb-mms-automation"},
		Auth:     DisabledAuth(),
	}

	// Here we compare the bytes of the two automationconfigs,
	// we can't use reflect.DeepEqual() as it treats nil entries as different from empty ones,
	// and in the AutomationConfig Struct we use omitempty to set empty field to nil
	// The agent requires the nil value we provide, otherwise the agent attempts to configure authentication.

	newAcBytes, err := json.Marshal(previousAC)
	if err != nil {
		return AutomationConfig{}, err
	}

	currentAcBytes, err := json.Marshal(currentAc)
	if err != nil {
		return AutomationConfig{}, err
	}

	if bytes.Compare(newAcBytes, currentAcBytes) != 0 {
		currentAc.Version += 1
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
