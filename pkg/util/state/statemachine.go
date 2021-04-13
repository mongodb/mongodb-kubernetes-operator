package state

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AllStates struct {
	NextState             string            `json:"nextState"`
	StateCompletionStatus map[string]string `json:"stateCompletion"`
}

type State struct {
	Name       string
	Reconcile  func() (reconcile.Result, error)
	IsComplete func() (bool, error)
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

		transition, err := m.getTransition()
		if err != nil {
			return reconcile.Result{}, err
		}
		nextState := ""
		if transition != nil {
			nextState = transition.to.Name
		}

		m.logger.Debugf("preparing transition [%s] -> [%s]", m.currentState.Name, nextState)
		if err := m.completer.Complete(nextState); err != nil {
			m.logger.Debugf("Error marking state: [%s] as complete: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
		return res, err
	}

	m.logger.Debugf("State [%s] is not yet complete", m.currentState.Name)

	return res, err
}

func (m *Machine) SetState(state State) {
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

// getTransition returns the first transition it finds that is available
// from the current state.
func (m *Machine) getTransition() (*transition, error) {
	for _, t := range m.currentTransitions {
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
