package automationconfig

type Builder struct {
	processes   []Process
	replicaSets []ReplicaSet
	version     string
	auth        Auth
}

func NewBuilder() *Builder {
	return &Builder{
		processes:   []Process{},
		replicaSets: []ReplicaSet{},
	}
}

func (b *Builder) SetVersion(version string) *Builder {
	b.version = version
	return b
}

func (b *Builder) AddProcess(p Process) *Builder {
	return b.AddProcesses([]Process{p})
}

func (b *Builder) AddProcesses(processes []Process) *Builder {
	b.processes = append(b.processes, processes...)
	return b
}

func (b *Builder) AddReplicaSet(rs ReplicaSet) *Builder {
	return b.AddReplicaSets([]ReplicaSet{rs})
}

func (b *Builder) AddReplicaSets(replicaSets []ReplicaSet) *Builder {
	b.replicaSets = append(b.replicaSets, replicaSets...)
	return b
}

func (b *Builder) SetAuth(auth Auth) *Builder {
	b.auth = auth
	return b
}

func (b *Builder) Build() AutomationConfig {
	processesCopy := make([]Process, len(b.processes))
	copy(processesCopy, b.processes)

	replicaSetsCopy := make([]ReplicaSet, len(b.replicaSets))
	copy(replicaSetsCopy, b.replicaSets)

	return AutomationConfig{
		Processes:   processesCopy,
		ReplicaSets: replicaSetsCopy,
		Version:     b.version,
		Auth:        b.auth,
	}
}
