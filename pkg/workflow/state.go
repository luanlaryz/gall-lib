package workflow

import (
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// StateSnapshot is the public immutable view of shared workflow state.
type StateSnapshot map[string]any

// State is the shared run-scoped mutable context passed across steps.
type State struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewState creates a new isolated mutable state from initial.
func NewState(initial StateSnapshot) *State {
	return &State{data: cloneMap(initial)}
}

// Get returns the value stored at key.
func (s *State) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.data[key]
	return cloneValue(value), ok
}

// Set stores value at key.
func (s *State) Set(key string, value any) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data == nil {
		s.data = make(map[string]any)
	}
	s.data[key] = cloneValue(value)
}

// Delete removes key from state.
func (s *State) Delete(key string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
}

// Snapshot returns a defensive copy of the current state.
func (s *State) Snapshot() StateSnapshot {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return StateSnapshot(cloneMap(s.data))
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneSlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}

	out := make([]any, len(in))
	for index, value := range in {
		out[index] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	case StateSnapshot:
		return StateSnapshot(cloneMap(typed))
	default:
		return typed
	}
}

func cloneCheckpoint(in *Checkpoint) *Checkpoint {
	if in == nil {
		return nil
	}
	return &Checkpoint{
		StepName: in.StepName,
		State:    StateSnapshot(cloneMap(in.State)),
		Time:     in.Time,
		Metadata: types.CloneMetadata(in.Metadata),
	}
}
