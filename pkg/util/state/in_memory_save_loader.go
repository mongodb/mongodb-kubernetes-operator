package state

import "k8s.io/apimachinery/pkg/types"

type InMemorySaveLoader struct {
	StateHistory  []string
	NextState     string
	startingState string
	nsName        types.NamespacedName
}

func (s *InMemorySaveLoader) SaveNextState(name types.NamespacedName, stateName string) error {
	if stateName == "" {
		return nil
	}
	s.StateHistory = append(s.StateHistory, stateName)
	s.NextState = stateName
	return nil
}

func (s *InMemorySaveLoader) LoadNextState(types.NamespacedName) (string, error) {
	return s.NextState, nil
}

func (s *InMemorySaveLoader) Reset() {
	s.StateHistory = nil
	s.SaveNextState(s.nsName, s.startingState)
}

func NewInMemorySaveLoader(nsName types.NamespacedName, startingState string) *InMemorySaveLoader {
	s := &InMemorySaveLoader{}
	s.startingState = startingState
	s.SaveNextState(nsName, startingState)
	return s
}
