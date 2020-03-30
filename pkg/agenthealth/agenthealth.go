package agenthealth

import (
	"time"
)

type Health struct {
	Healthiness  map[string]ProcessHealth     `json:"statuses"`
	ProcessPlans map[string]MmsDirectorStatus `json:"mmsStatus"`
}

type ProcessHealth struct {
	IsInGoalState   bool  `json:"IsInGoalState"`
	LastMongoUpTime int64 `json:"LastMongoUpTime"`
	ExpectedToBeUp  bool  `json:"ExpectedToBeUp"`
}

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
	Move  string        `json:"move"`
	Steps []*StepStatus `json:"steps"`
}

type StepStatus struct {
	Step      string     `json:"step"`
	Started   *time.Time `json:"started"`
	Completed *time.Time `json:"completed"`
	Result    string     `json:"result"`
}
