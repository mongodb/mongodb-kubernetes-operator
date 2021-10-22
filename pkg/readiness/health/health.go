package health

import (
	"fmt"
	"time"
)

type replicationStatus int

const (
	replicationStatusStartup    replicationStatus = 0
	replicationStatusPrimary    replicationStatus = 1
	replicationStatusSecondary  replicationStatus = 2
	replicationStatusRecovering replicationStatus = 3
	replicationStatusStartup2   replicationStatus = 5
	replicationStatusUnknown    replicationStatus = 6
	replicationStatusArbiter    replicationStatus = 7
	replicationStatusDown       replicationStatus = 8
	replicationStatusRollback   replicationStatus = 9
	replicationStatusRemoved    replicationStatus = 10
	replicationStatusUndefined  replicationStatus = -1
)

type Status struct {
	Healthiness  map[string]processHealth     `json:"statuses"`
	ProcessPlans map[string]MmsDirectorStatus `json:"mmsStatus"`
}

type processHealth struct {
	IsInGoalState   bool               `json:"IsInGoalState"`
	LastMongoUpTime int64              `json:"LastMongoUpTime"`
	ExpectedToBeUp  bool               `json:"ExpectedToBeUp"`
	ReplicaStatus   *replicationStatus `json:"ReplicationStatus"`
}

func (h processHealth) String() string {
	return fmt.Sprintf("ExpectedToBeUp: %t, IsInGoalState: %t, LastMongoUpTime: %v", h.ExpectedToBeUp,
		h.IsInGoalState, time.Unix(h.LastMongoUpTime, 0))
}

// These structs are copied from go_planner mmsdirectorstatus.go. Some fields are pruned as not used.
type MmsDirectorStatus struct {
	Name                              string        `json:"name"`
	LastGoalStateClusterConfigVersion int64         `json:"lastGoalVersionAchieved"`
	Plans                             []*PlanStatus `json:"plans"`
}

type PlanStatus struct {
	Moves     []*MoveStatus `json:"moves"`
	Started   *time.Time    `json:"started"`
	Completed *time.Time    `json:"completed"`
}

type MoveStatus struct {
	Steps []*StepStatus `json:"steps"`
}
type StepStatus struct {
	Step      string     `json:"step"`
	Started   *time.Time `json:"started"`
	Completed *time.Time `json:"completed"`
	Result    string     `json:"result"`
}

// isReadyState will return true, meaning a *ready state* in the sense that this Process can
// accept read operations.
// It returns true if the managed process is mongos or standalone (replicationStatusUndefined)
// or if the agent doesn't publish the replica status (older agents)
func (h processHealth) IsReadyState() bool {
	if h.ReplicaStatus == nil {
		return true
	}
	status := *h.ReplicaStatus
	if status == replicationStatusUndefined {
		return true
	}

	switch status {
	case
		// There are no other states in which the MongoDB
		// server could that would mean a Ready State.
		replicationStatusPrimary,
		replicationStatusSecondary,
		replicationStatusArbiter:
		return true
	}

	return false
}
