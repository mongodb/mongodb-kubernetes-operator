package agenthealth

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
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

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func (m MmsDirectorStatus) IsChangingVersion(logger *zap.SugaredLogger) bool {
	logger.Infof("Plans: %+v", prettyPrint(m))
	if len(m.Plans) == 0 {
		return false
	}
	lastPlan := m.Plans[len(m.Plans)-1]
	for _, m := range lastPlan.Moves {
		if changingVersionMove := m.Move == "ChangeVersion"; !changingVersionMove {
			continue
		}

		for _, s := range m.Steps {
			if stopStepSuccess := s.Step == "Stop" && s.Completed != nil && s.Result == "success"; !stopStepSuccess {
				continue
			}
			return true
		}
	}
	return false
}

// FindCurrentStep returns the step which seems to be run by the Agent now. The step is always in the last plan so we iterate over all the steps
// there and find the last step which has "Started" non nil
// (indeed this is not the perfect logic as sometimes the agent doesn't update the 'Started', but seems it works for finding deadlocks still
func FindCurrentStep(processStatuses map[string]MmsDirectorStatus, logger *zap.SugaredLogger) (*StepStatus, *MoveStatus, error) {
	var currentPlan *PlanStatus
	if len(processStatuses) == 0 {
		// Seems shouldn't happen but let's check anyway - may be needs to be changed to Info if this happens
		return nil, nil, fmt.Errorf("there is no information about Agent process plans")
	}
	if len(processStatuses) > 1 {
		return nil, nil, fmt.Errorf("only one process status is expected but got %d", len(processStatuses))
	}
	logger.Infof("processStatuses: %+v", prettyPrint(processStatuses))
	// There is always only one process managed by the Agent - so there will be only one loop
	for k, v := range processStatuses {
		if len(v.Plans) == 0 {
			return nil, nil, fmt.Errorf("the process %s doesn't contain any plans", k)
		}
		currentPlan = v.Plans[len(v.Plans)-1]
	}

	if currentPlan == nil {
		return nil, nil, fmt.Errorf("the current plan was nil")
	}

	if currentPlan.Completed != nil {
		return nil, nil, fmt.Errorf("the Agent hasn't reported working on the new config yet, the last plan finished at %s",
			currentPlan.Completed.Format(time.RFC3339))
	}

	var lastStartedStep *StepStatus
	var lastStartedMove *MoveStatus
	for _, m := range currentPlan.Moves {
		for _, s := range m.Steps {
			if s.Started != nil {
				lastStartedMove = m
				lastStartedStep = s
			}
		}
	}

	if lastStartedStep != nil || lastStartedMove != nil {
		return lastStartedStep, lastStartedMove, nil
	}

	return nil, nil, fmt.Errorf("there were no plans found")
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
