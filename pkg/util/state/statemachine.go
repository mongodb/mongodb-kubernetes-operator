package state

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AllStates struct {
	CurrentState          string            `json:"currentState"`
	StateCompletionStatus map[string]string `json:"stateCompletion"`
}

type State struct {
	Name            string
	Reconcile       func() (reconcile.Result, error)
	IsComplete      func() (bool, error)
}

type transition struct {
	from, to  State
	predicate func() (bool, error)
}

type Completer interface {
	IsComplete(stateName string) (bool, error)
	Complete(stateName string) error
}

type Machine struct {
	allTransitions     map[string][]transition
	currentTransitions []transition
	currentState       *State
	logger             *zap.SugaredLogger
	completer          Completer
	States             map[string]State
}

func NewStateMachine(completer Completer, logger *zap.SugaredLogger) *Machine {
	return &Machine{
		allTransitions:     map[string][]transition{},
		currentTransitions: []transition{},
		logger:             logger,
		completer:          completer,
		States:             map[string]State{},
	}
}

func (m *Machine) Reconcile() (reconcile.Result, error) {
	transition, err := m.getTransition()
	if err != nil {
		return reconcile.Result{}, err
	}

	if transition != nil {
		m.SetState(transition.to, false)
	}

	if m.currentState == nil {
		panic("no current state!")
	}

	m.logger.Infof("Reconciling state: [%s]", m.currentState.Name)
	res, err := m.currentState.Reconcile()

	if err != nil {
		m.logger.Debugf("Error reconciling state [%s]: %s", m.currentState.Name, err)
		return res, err
	}

	isComplete := true
	if m.currentState.IsComplete != nil {
		isComplete, err = m.currentState.IsComplete()
		if err != nil {
			m.logger.Debugf("Error determining if state [%s] is complete: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
	}

	if isComplete {
		m.logger.Debugf("Completed state: [%s]", m.currentState.Name)
		if err := m.completer.Complete(m.currentState.Name); err != nil {
			m.logger.Debugf("Error marking state: [%s] as complete: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
		return res, err
	}

	m.logger.Debugf("State [%s] is not yet complete", m.currentState.Name)
	return res, err
}

func (m *Machine) SetState(state State, prev bool) {
	if prev {
		m.logger.Debugf("Beginning at previous state: %s", state.Name)
	} else {
		m.logger.Debugf("Transition to state: %s", state.Name)
	}
	m.currentState = &state
	m.currentTransitions = m.allTransitions[m.currentState.Name]
}

func (m *Machine) AddTransition(from, to State, predicate func() (bool, error)) {
	_, ok := m.allTransitions[from.Name]
	if !ok {
		m.allTransitions[from.Name] = []transition{}
	}
	m.allTransitions[from.Name] = append(m.allTransitions[from.Name], transition{
		from:      from,
		to:        to,
		predicate: predicate,
	})

	m.States[from.Name] = from
	m.States[to.Name] = to

}

func (m *Machine) getTransition() (*transition, error) {
	for _, t := range m.currentTransitions {

		isComplete, err := m.completer.IsComplete(t.from.Name)
		if err != nil {
			return nil, err
		}

		// we should never transition from a state if it is not yet complete
		if !isComplete {
			m.logger.Debugf("Not transitioning from [%s] because it is not complete.", t.from.Name)
			continue
		}

		isComplete, err = m.completer.IsComplete(t.to.Name)
		if err != nil {
			return nil, err
		}

		//we should never transition to a state if it is already completed.
		if isComplete {
			m.logger.Debugf("Not transitioning from [%s] to [%s] because [%s] is already complete.", t.from.Name, t.to.Name, t.to.Name)
			continue
		}

		ok, err := t.predicate()
		if err != nil {
			return nil, err
		}
		if ok {
			return &t, nil
		}
	}
	return nil, nil
}
