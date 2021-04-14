package state

import (
	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type AllStates struct {
	NextState             string            `json:"NextState"`
	StateCompletionStatus map[string]string `json:"stateCompletion"`
}

type State struct {
	Name       string
	Reconcile  func() (reconcile.Result, error)
	IsComplete func() (bool, error)
}

type transition struct {
	from, to  State
	predicate TransitionPredicate
}

type Saver interface {
	SaveNextState(stateName string) error
}

type Loader interface {
	LoadNextState() (string, error)
}

type SaveLoader interface {
	Saver
	Loader
}

type TransitionPredicate func() bool

var FromBool = func(b bool) TransitionPredicate {
	return func() bool {
		return b
	}
}

var DirectTransition = FromBool(true)

type Machine struct {
	allTransitions map[string][]transition
	currentState   *State
	logger         *zap.SugaredLogger
	saveLoader     SaveLoader
	states         map[string]State
}

func NewStateMachine(saver SaveLoader, logger *zap.SugaredLogger) *Machine {
	return &Machine{
		allTransitions: map[string][]transition{},
		logger:         logger,
		saveLoader:     saver,
		states:         map[string]State{},
	}
}

func (m *Machine) Reconcile() (reconcile.Result, error) {
	if m.currentState == nil {
		if err := m.initStartingState(); err != nil {
			m.logger.Errorf("error initializing starting state: %s", err)
			return result.Failed()
		}
	}

	m.logger.Infof("Reconciling state: [%s]", m.currentState.Name)
	//time.Sleep(2 * time.Second)
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

		transition := m.getTransitionForState(*m.currentState)
		nextState := ""
		if transition != nil {
			nextState = transition.to.Name
		}

		if nextState != "" {
			m.logger.Debugf("preparing transition [%s] -> [%s]", m.currentState.Name, nextState)
		}

		//time.Sleep(3 * time.Second)
		if err := m.saveLoader.SaveNextState(nextState); err != nil {
			m.logger.Debugf("Error marking state: [%s] as complete: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
		return res, err
	}

	m.logger.Debugf("State [%s] is not yet complete", m.currentState.Name)

	return res, err
}

func (m *Machine) initStartingState() error {
	currentStateName, err := m.saveLoader.LoadNextState()
	if err != nil {
		return errors.Errorf("could not load starting state: %s", err)
	}
	nextState := m.states[currentStateName]
	m.currentState = &nextState
	return nil
}

func (m *Machine) AddTransition(from, to State, predicate TransitionPredicate) {
	_, ok := m.allTransitions[from.Name]
	if !ok {
		m.allTransitions[from.Name] = []transition{}
	}
	m.allTransitions[from.Name] = append(m.allTransitions[from.Name], transition{
		from:      from,
		to:        to,
		predicate: predicate,
	})

	m.states[from.Name] = from
	m.states[to.Name] = to

}

// getTransitionForState returns the first transition it finds that is available
// from the current state.
func (m *Machine) getTransitionForState(s State) *transition {
	transitions := m.allTransitions[s.Name]
	for _, t := range transitions {
		if t.predicate() {
			return &t
		}
	}
	return nil
}
