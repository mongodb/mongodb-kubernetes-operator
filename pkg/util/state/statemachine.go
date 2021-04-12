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
	Name         string
	Reconcile    func() (reconcile.Result, error)
	OnCompletion func() error
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
		m.SetState(transition.to)
	}

	if m.currentState == nil {
		panic("no current state!")
	}

	m.logger.Infof("Reconciling state: [%s]", m.currentState.Name)
	res, err := m.currentState.Reconcile()

	// we only complete the state if if we are requeuing immediately.
	if res.Requeue && res.RequeueAfter == 0 && m.currentState.OnCompletion != nil {
		if err := m.currentState.OnCompletion(); err != nil {
			m.logger.Errorf("error running OnCompletion for state %s: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
	}

	m.logger.Debugw("Reconcile Result", "res", res, "err", err)
	return res, err
}

func (m *Machine) SetState(state State) {
	m.logger.Debugf("Transition to state: %s", state.Name)
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
