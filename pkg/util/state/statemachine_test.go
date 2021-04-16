package state

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/mongodb/mongodb-kubernetes-operator/pkg/util/result"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		os.Exit(1)
	}
	zap.ReplaceGlobals(logger)
}

// inMemorySaveLoader stores and loads states to member fields
// and maintains a history of all the fields saved.
type inMemorySaveLoader struct {
	stateHistory  []string
	nextState     string
	startingState string
}

func (s *inMemorySaveLoader) SaveNextState(_ types.NamespacedName, stateName string) error {
	if stateName == "" {
		return nil
	}
	s.stateHistory = append(s.stateHistory, stateName)
	s.nextState = stateName
	return nil
}

func (s *inMemorySaveLoader) LoadNextState(_ types.NamespacedName) (string, error) {
	return s.nextState, nil
}

func newInMemorySaveLoader(startingState string) *inMemorySaveLoader {
	s := &inMemorySaveLoader{}
	s.startingState = startingState
	_ = s.SaveNextState(types.NamespacedName{}, startingState)
	return s
}

func TestOrderOfStatesIsCorrect(t *testing.T) {
	in := newInMemorySaveLoader("State0")
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	state0 := newAlwaysCompletingState("State0")
	state1 := newAlwaysCompletingState("State1")
	state2 := newAlwaysCompletingState("State2")

	s.AddDirectTransition(state0, state1)
	s.AddDirectTransition(state1, state2)

	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()

	assert.Equal(t, []string{"State0", "State1", "State2"}, in.stateHistory)
}

func TestOrderOfStatesIsCorrectIfAddedInDifferentOrder(t *testing.T) {
	in := newInMemorySaveLoader("State0")
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	state0 := newAlwaysCompletingState("State0")
	state1 := newAlwaysCompletingState("State1")
	state2 := newAlwaysCompletingState("State2")

	s.AddDirectTransition(state1, state2)
	s.AddDirectTransition(state0, state1)

	_, _ = s.Reconcile()
	assert.Equal(t, "State1", in.nextState)

	_, _ = s.Reconcile()
	assert.Equal(t, "State2", in.nextState)

	_, _ = s.Reconcile()

	assert.Equal(t, []string{"State0", "State1", "State2"}, in.stateHistory)
}

func TestPredicateReturningFalse_PreventsStateTransition(t *testing.T) {
	in := newInMemorySaveLoader("State0")
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	state0 := newAlwaysCompletingState("State0")
	state1 := newAlwaysCompletingState("State1")
	state2 := newAlwaysCompletingState("State2")
	state3 := newAlwaysCompletingState("State3")

	s.AddDirectTransition(state0, state1)

	// there is no transition from state1 to state2
	s.AddTransition(state1, state2, func() bool {
		return false
	})
	s.AddDirectTransition(state1, state3)

	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()

	assert.Equal(t, []string{"State0", "State1", "State3"}, in.stateHistory)
}

func TestAddTransition(t *testing.T) {
	in := newInMemorySaveLoader("State0")
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	state0 := newAlwaysCompletingState("State0")
	state1 := newAlwaysCompletingState("State1")

	s.AddDirectTransition(state0, state1)

	t.Run("Adds both states to internal map", func(t *testing.T) {
		assert.Contains(t, s.states, "State0")
		assert.Contains(t, s.states, "State1")
	})

	t.Run("Creates transition for first state", func(t *testing.T) {
		assert.Len(t, s.allTransitions["State0"], 1)
		assert.Equal(t, s.allTransitions["State0"][0].from.Name, "State0")
		assert.Equal(t, s.allTransitions["State0"][0].to.Name, "State1")
	})

	t.Run("Does not create transition for second state", func(t *testing.T) {
		assert.Len(t, s.allTransitions["State1"], 0)
	})
}

func TestIfStateFails_ItIsRunAgain(t *testing.T) {
	fails := newAlwaysFailsState("FailsState")
	succeeds := newAlwaysCompletingState("SucceedsState")

	in := newInMemorySaveLoader(fails.Name)
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	s.AddDirectTransition(fails, succeeds)

	t.Run("Any number of runs will not change the next state to be run", func(t *testing.T) {
		_, _ = s.Reconcile()
		assert.Equal(t, fails.Name, in.nextState)

		_, _ = s.Reconcile()
		assert.Equal(t, fails.Name, in.nextState)

		_, _ = s.Reconcile()
		assert.Equal(t, fails.Name, in.nextState)
	})

	t.Run("When the state passes, the next one will run", func(t *testing.T) {

		// the state will now succeed
		s.states["FailsState"] = newAlwaysCompletingState(fails.Name)

		_, _ = s.Reconcile()
		assert.Equal(t, succeeds.Name, in.nextState)
	})
}

func TestStateReconcileValue_IsReturnedFromStateMachine(t *testing.T) {
	t.Run("When State is Completed", func(t *testing.T) {
		s0 := State{
			Name: "State0",
			Reconcile: func() (reconcile.Result, error, bool) {
				return reconcile.Result{RequeueAfter: time.Duration(15000)}, errors.New("error"), true
			},
		}

		s1 := newAlwaysCompletingState("State1")

		in := newInMemorySaveLoader(s0.Name)
		s := NewStateMachine(in, types.NamespacedName{}, zap.S())

		s.AddDirectTransition(s0, s1)

		res, err := s.Reconcile()
		assert.False(t, res.Requeue)
		assert.Equal(t, time.Duration(15000), res.RequeueAfter)
		assert.Equal(t, errors.New("error"), err)
	})

	t.Run("When State is not Completed", func(t *testing.T) {
		s0 := State{
			Name: "State0",
			Reconcile: func() (reconcile.Result, error, bool) {
				return reconcile.Result{Requeue: true, RequeueAfter: time.Duration(5000)}, errors.New("error"), false
			},
		}

		s1 := newAlwaysCompletingState("State1")

		in := newInMemorySaveLoader(s0.Name)
		s := NewStateMachine(in, types.NamespacedName{}, zap.S())

		s.AddDirectTransition(s0, s1)

		res, err := s.Reconcile()
		assert.True(t, res.Requeue)
		assert.Equal(t, time.Duration(5000), res.RequeueAfter)
		assert.Equal(t, errors.New("error"), err)
	})
}

func TestCycleInStateMachine(t *testing.T) {
	s0 := newAlwaysCompletingState("State0")
	s1 := newAlwaysCompletingState("State1")
	s2 := newAlwaysCompletingState("State2")
	s3 := newAlwaysCompletingState("State3")
	s4 := newAlwaysCompletingState("State4")

	in := newInMemorySaveLoader("State0")
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	flag := true
	s.AddDirectTransition(s0, s1)
	s.AddDirectTransition(s1, s2)
	s.AddDirectTransition(s2, s3)

	// create a one time cycle back to s1
	s.AddTransition(s3, s1, func() bool {
		res := flag
		flag = !flag
		return res
	})

	s.AddDirectTransition(s3, s4)

	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()
	_, _ = s.Reconcile()

	assert.Equal(t, []string{"State0", "State1", "State2", "State3", "State1", "State2", "State3", "State4"}, in.stateHistory)
}

func TestBranchingPath(t *testing.T) {
	root := newAlwaysCompletingState("Root")
	left0 := newAlwaysCompletingState("Left0")
	left1 := newAlwaysCompletingState("Left1")
	left2 := newAlwaysCompletingState("Left2")

	right0 := newAlwaysCompletingState("Right0")
	right1 := newAlwaysCompletingState("Right1")
	right2 := newAlwaysCompletingState("Right2")

	in := newInMemorySaveLoader(root.Name)
	s := NewStateMachine(in, types.NamespacedName{}, zap.S())

	goLeft := true

	s.AddTransition(root, left0, func() bool {
		return goLeft
	})
	s.AddDirectTransition(left0, left1)
	s.AddDirectTransition(left1, left2)

	s.AddTransition(root, right0, func() bool {
		return !goLeft
	})

	s.AddDirectTransition(right0, right1)
	s.AddDirectTransition(right1, right2)

	t.Run("Left Path", func(t *testing.T) {

		_, _ = s.Reconcile()
		_, _ = s.Reconcile()
		_, _ = s.Reconcile()
		_, _ = s.Reconcile()

		assert.Equal(t, []string{"Root", "Left0", "Left1", "Left2"}, in.stateHistory)
	})

	t.Run("Right Path", func(t *testing.T) {
		goLeft = false
		// reset save loader state
		in.stateHistory = nil
		_ = in.SaveNextState(types.NamespacedName{}, root.Name)

		_, _ = s.Reconcile()
		_, _ = s.Reconcile()
		_, _ = s.Reconcile()
		_, _ = s.Reconcile()

		assert.Equal(t, []string{"Root", "Right0", "Right1", "Right2"}, in.stateHistory)
	})
}

func TestDetermineStartingState_ReadsFromLoader(t *testing.T) {
	t.Run("State Can be determined once added", func(t *testing.T) {
		s0 := newAlwaysCompletingState("State0")
		s1 := newAlwaysCompletingState("State1")

		in := newInMemorySaveLoader(s0.Name)
		s := NewStateMachine(in, types.NamespacedName{}, zap.S())

		// State must be added before it can be returned in determine state
		s.AddDirectTransition(s0, s1)

		assert.Nil(t, s.currentState)
		err := s.determineState()
		assert.NoError(t, err)
		assert.Equal(t, "State0", s.currentState.Name)
	})

	t.Run("State cannot be determined if not added", func(t *testing.T) {
		s0 := newAlwaysCompletingState("State0")

		in := newInMemorySaveLoader(s0.Name)
		s := NewStateMachine(in, types.NamespacedName{}, zap.S())

		assert.Nil(t, s.currentState)
		err := s.determineState()
		assert.Error(t, err)
		assert.Nil(t, s.currentState)
	})

}

// newAlwaysCompletingState returns a State that will always succeed.
func newAlwaysCompletingState(name string) State {
	return State{
		Name:      name,
		Reconcile: result.StateComplete,
	}
}

// newAlwaysFailsState returns a State that will always fail.
func newAlwaysFailsState(name string) State {
	return State{
		Name:      name,
		Reconcile: result.FailedState,
	}
}
