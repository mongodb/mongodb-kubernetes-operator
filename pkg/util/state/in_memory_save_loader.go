package state

type InMemorySaveLoader struct {
	StateHistory []string
	NextState    string
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

func NewInMemorySaveLoader(startingState string) *InMemorySaveLoader {
	s := &InMemorySaveLoader{}
	s.SaveNextState(startingState)
	return s
}
