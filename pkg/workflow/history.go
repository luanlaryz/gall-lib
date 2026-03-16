package workflow

import (
	"context"
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// InMemoryHistory stores workflow history entries in process memory.
//
// It is suitable for tests, examples and local development. Future phases may
// add richer persistence or dedicated checkpoint storage without changing this
// minimal contract.
type InMemoryHistory struct {
	mu      sync.RWMutex
	entries []HistoryEntry
}

// Append stores a defensive copy of entry.
func (h *InMemoryHistory) Append(_ context.Context, entry HistoryEntry) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = append(h.entries, cloneHistoryEntry(entry))
	return nil
}

// Entries returns a defensive copy of all recorded entries.
func (h *InMemoryHistory) Entries() []HistoryEntry {
	if h == nil {
		return nil
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]HistoryEntry, len(h.entries))
	for index, entry := range h.entries {
		out[index] = cloneHistoryEntry(entry)
	}
	return out
}

func cloneHistoryEntry(in HistoryEntry) HistoryEntry {
	return HistoryEntry{
		Kind:         in.Kind,
		WorkflowID:   in.WorkflowID,
		WorkflowName: in.WorkflowName,
		RunID:        in.RunID,
		SessionID:    in.SessionID,
		StepName:     in.StepName,
		Attempt:      in.Attempt,
		Status:       in.Status,
		Time:         in.Time,
		Output:       cloneMap(in.Output),
		Checkpoint:   cloneCheckpoint(in.Checkpoint),
		Metadata:     types.CloneMetadata(in.Metadata),
	}
}
