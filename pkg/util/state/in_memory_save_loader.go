package state

type InMemorySaveLoader struct {
	StateHistory  []string
	NextState     string
	startingState string
}

func (s *InMemorySaveLoader) SaveNextState(stateName string) error {
	if stateName == "" {
		return nil
	}
	s.StateHistory = append(s.StateHistory, stateName)
	s.NextState = stateName
	return nil
}

func (s *InMemorySaveLoader) LoadNextState() (string, error) {
	return s.NextState, nil
}

func (s *InMemorySaveLoader) Reset() {
	s.StateHistory = nil
	s.SaveNextState(s.startingState)
}

func NewInMemorySaveLoader(startingState string) *InMemorySaveLoader {
	s := &InMemorySaveLoader{}
	s.startingState = startingState
	s.SaveNextState(startingState)
	return s
}
