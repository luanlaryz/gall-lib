package memory

import (
	"context"
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// InMemoryWorkingMemoryFactory creates run-scoped, process-local working sets.
type InMemoryWorkingMemoryFactory struct{}

// NewRunState returns a fresh in-memory working set for a single run.
func (InMemoryWorkingMemoryFactory) NewRunState(ctx context.Context, agentID, runID string) (WorkingSet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &InMemoryWorkingSet{}, nil
}

// InMemoryWorkingSet is the reference ephemeral WorkingSet implementation.
type InMemoryWorkingSet struct {
	mu       sync.Mutex
	messages []types.Message
	records  []Record
}

// AddMessage appends msg to the run-local working state.
func (w *InMemoryWorkingSet) AddMessage(msg types.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.messages = append(w.messages, types.CloneMessage(msg))
}

// AddRecord appends record to the run-local working state.
func (w *InMemoryWorkingSet) AddRecord(record Record) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.records = append(w.records, cloneRecord(record))
}

// Snapshot returns a defensive copy of the current working state.
func (w *InMemoryWorkingSet) Snapshot() Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()

	return Snapshot{
		Messages: types.CloneMessages(w.messages),
		Records:  cloneRecords(w.records),
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{
		Messages: types.CloneMessages(snapshot.Messages),
		Records:  cloneRecords(snapshot.Records),
		Metadata: types.CloneMetadata(snapshot.Metadata),
	}
}

func cloneRecords(records []Record) []Record {
	if len(records) == 0 {
		return nil
	}

	out := make([]Record, len(records))
	for index, record := range records {
		out[index] = cloneRecord(record)
	}
	return out
}

func cloneRecord(record Record) Record {
	return Record{
		Kind: record.Kind,
		Name: record.Name,
		Data: cloneMap(record.Data),
	}
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
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		return cloneSlice(value)
	default:
		return value
	}
}
