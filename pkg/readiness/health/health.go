package health

import (
	"fmt"
	"time"
)

type replicationStatus int

const (
	replicationStatusPrimary   replicationStatus = 1
	replicationStatusSecondary replicationStatus = 2
)

type Status struct {
	Healthiness  map[string]processHealth     `json:"statuses"`
	ProcessPlans map[string]MmsDirectorStatus `json:"mmsStatus"`
}

type processHealth struct {
	IsInGoalState   bool               `json:"IsInGoalState"`
	LastMongoUpTime int64              `json:"LastMongoUpTime"`
	ExpectedToBeUp  bool               `json:"ExpectedToBeUp"`
	ReplicaStatus   *replicationStatus `json:"ReplicationStatus,omitempty"`
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
// accept read operations. There are no other states in which the MongoDB server could that
// would mean a Ready State.
func (h processHealth) IsReadyState() bool {
	return *h.ReplicaStatus == replicationStatusPrimary ||
		*h.ReplicaStatus == replicationStatusSecondary
}
