package state

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type State struct {
	Name      string
	Reconcile func() (reconcile.Result, error)
}

type transition struct {
	to        State
	predicate func() (bool, error)
}

type Machine struct {
	allTransitions     map[string][]transition
	currentTransitions []transition
	currentState       *State
}

func NewStateMachine() *Machine {
	m := &Machine{
		allTransitions:     map[string][]transition{},
		currentTransitions: []transition{},
	}
	return m
}

func (m *Machine) Reconcile() (reconcile.Result, error) {
	transition, err := m.getTransition()
	if err != nil {
		return reconcile.Result{}, err
	}

	if transition != nil {
		if err := m.SetState(transition.to); err != nil {
			return reconcile.Result{}, err
		}
	}

	if m.currentState == nil {
		panic("no current state!")
	}
	return m.currentState.Reconcile()
}

func (m *Machine) SetState(state State) error {
	if m.currentState.Name == state.Name {
		return nil
	}
	m.currentState = &state
	m.currentTransitions = m.allTransitions[m.currentState.Name]
	return nil
}

func (m *Machine) AddTransition(from, to State, predicate func() (bool, error)) {
	_, ok := m.allTransitions[from.Name]
	if !ok {
		m.allTransitions[from.Name] = []transition{}
	}
	m.allTransitions[from.Name] = append(m.allTransitions[from.Name], transition{
		to:        to,
		predicate: predicate,
	})

}

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
