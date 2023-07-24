package state

import (
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// State should provide a unique name, and a Reconcile function.
// This function gets called by the Machine. The first two returned values
// are returned to the caller, while the 3rd value is used to indicate if the
// State completed successfully. A value of true will move onto the next State,
// a value of false will repeat this State until true is returned.
type State struct {
	// Name should be a unique identifier of the State
	Name string

	// Reconcile should perform the actual reconciliation of the State.
	// The reconcile.Result and error should be returned from the controller.
	// the boolean value indicates that the State has been successfully completed.
	Reconcile func() (reconcile.Result, error, bool)

	// OnEnter executes before the Reconcile function is called.
	OnEnter func() error
}

// transition represents a transition between two states.
type transition struct {
	from, to  State
	predicate TransitionPredicate
}

// Saver saves the next state name that should be reconciled.
// If a transition is A -> B, after A finishes reconciling `SaveNextState("B")` will be called.
type Saver interface {
	SaveNextState(nsName types.NamespacedName, stateName string) error
}

// Loader should return the value saved by Saver.
type Loader interface {
	LoadNextState(nsName types.NamespacedName) (string, error)
}

// SaveLoader can both load and save the name of a state.
type SaveLoader interface {
	Saver
	Loader
}

// TransitionPredicate is used to indicate if two States should be connected.
type TransitionPredicate func() bool

var FromBool = func(b bool) TransitionPredicate {
	return func() bool {
		return b
	}
}

// directTransition can be used to ensure two states are directly linked.
var directTransition = FromBool(true)

// Machine allows for several States to be registered via "AddTransition"
// When calling Reconcile, the corresponding State will be used based on the values
// stored/loaded from the SaveLoader. A Machine corresponds to a single Kubernetes resource.
type Machine struct {
	allTransitions map[string][]transition
	currentState   *State
	logger         *zap.SugaredLogger
	saveLoader     SaveLoader
	states         map[string]State
	nsName         types.NamespacedName
}

// NewStateMachine returns a Machine, it must be set up with calls to "AddTransition(s1, s2, predicate)"
// before Reconcile is called.
func NewStateMachine(saver SaveLoader, nsName types.NamespacedName, logger *zap.SugaredLogger) *Machine {
	return &Machine{
		allTransitions: map[string][]transition{},
		logger:         logger,
		saveLoader:     saver,
		states:         map[string]State{},
		nsName:         nsName,
	}
}

// Reconcile will reconcile the currently active State. This method should be called
// from the controllers.
func (m *Machine) Reconcile() (reconcile.Result, error) {

	if err := m.determineState(); err != nil {
		m.logger.Errorf("error initializing starting state: %s", err)
		return reconcile.Result{}, err
	}

	m.logger.Infof("Reconciling state: [%s]", m.currentState.Name)

	if m.currentState.OnEnter != nil {
		if err := m.currentState.OnEnter(); err != nil {
			m.logger.Debugf("Error reconciling state [%s]: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
	}

	res, err, isComplete := m.currentState.Reconcile()

	if err != nil {
		m.logger.Debugf("Error reconciling state [%s]: %s", m.currentState.Name, err)
		return res, err
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

		if err := m.saveLoader.SaveNextState(m.nsName, nextState); err != nil {
			m.logger.Debugf("Error marking state: [%s] as complete: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
		return res, err
	}

	m.logger.Debugf("State [%s] is not yet complete", m.currentState.Name)

	return res, err
}

// determineState ensures that "currentState" has a valid value.
// the state that is loaded comes from the Loader.
func (m *Machine) determineState() error {
	currentStateName, err := m.saveLoader.LoadNextState(m.nsName)
	if err != nil {
		return fmt.Errorf("could not load starting state: %s", err)
	}
	nextState, ok := m.states[currentStateName]
	if !ok {
		return fmt.Errorf("could not determine state %s as it was not added to the State Machine", currentStateName)
	}
	m.currentState = &nextState
	return nil
}

// AddDirectTransition creates a transition between the two
// provided states which will always be valid.
func (m *Machine) AddDirectTransition(from, to State) {
	m.AddTransition(from, to, directTransition)
}

// AddTransition creates a transition between the two states if the given
// predicate returns true.
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
