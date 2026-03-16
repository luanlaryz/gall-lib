// Package memory defines the public persistence and working-memory contracts.
package memory

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Record captures a non-message artifact produced during a run.
type Record struct {
	Kind string
	Name string
	Data map[string]any
}

// Snapshot is the observable memory view loaded into a run.
type Snapshot struct {
	Messages []types.Message
	Records  []Record
	Metadata types.Metadata
}

// Delta is the data persisted after a successful run.
type Delta struct {
	Messages []types.Message
	Records  []Record
	Response *types.Message
	Metadata types.Metadata
}

// Store persists session-scoped memory across runs.
type Store interface {
	Load(ctx context.Context, sessionID string) (Snapshot, error)
	Save(ctx context.Context, sessionID string, delta Delta) error
}

// WorkingMemoryFactory creates isolated run-local state.
type WorkingMemoryFactory interface {
	NewRunState(ctx context.Context, agentID, runID string) (WorkingSet, error)
}

// WorkingSet captures ephemeral state for a single run.
type WorkingSet interface {
	AddMessage(msg types.Message)
	AddRecord(record Record)
	Snapshot() Snapshot
}
