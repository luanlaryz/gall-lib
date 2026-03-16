package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestInMemoryStoreLoadMissingSessionReturnsEmptySnapshot(t *testing.T) {
	t.Parallel()

	store := &memory.InMemoryStore{}

	snapshot, err := store.Load(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 0 || len(snapshot.Records) != 0 || len(snapshot.Metadata) != 0 {
		t.Fatalf("Load() snapshot = %+v want empty", snapshot)
	}
}

func TestInMemoryStorePersistsBySessionWithDefensiveCopies(t *testing.T) {
	t.Parallel()

	store := &memory.InMemoryStore{}
	delta := memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
		Records: []memory.Record{{
			Kind: "note",
			Name: "first",
			Data: map[string]any{"tags": []any{"a", "b"}},
		}},
		Response: &types.Message{Role: types.RoleAssistant, Content: "hello"},
		Metadata: types.Metadata{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	}

	if err := store.Save(context.Background(), "session-1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	delta.Messages[0].Content = "mutated"
	delta.Records[0].Data["tags"] = []any{"changed"}
	delta.Metadata["user_id"] = "changed"

	snapshot, err := store.Load(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got := snapshot.Messages[0].Content; got != "hi" {
		t.Fatalf("snapshot message = %q want %q", got, "hi")
	}
	if got := snapshot.Records[0].Data["tags"].([]any)[0]; got != "a" {
		t.Fatalf("snapshot record tags = %v want %v", snapshot.Records[0].Data["tags"], []any{"a", "b"})
	}
	if got := snapshot.Metadata["user_id"]; got != "user-1" {
		t.Fatalf("snapshot metadata user_id = %q want %q", got, "user-1")
	}

	snapshot.Messages[0].Content = "tampered"
	snapshot.Records[0].Data["tags"] = []any{"tampered"}
	snapshot.Metadata["user_id"] = "tampered"

	reloaded, err := store.Load(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Load() reload error = %v", err)
	}
	if got := reloaded.Messages[0].Content; got != "hi" {
		t.Fatalf("reloaded message = %q want %q", got, "hi")
	}
	if got := reloaded.Metadata["user_id"]; got != "user-1" {
		t.Fatalf("reloaded metadata user_id = %q want %q", got, "user-1")
	}
}

func TestInMemoryStoreKeepsSessionIsolationInsideSameUserConversationNamespace(t *testing.T) {
	t.Parallel()

	store := &memory.InMemoryStore{}
	sharedMetadata := types.Metadata{
		"user_id":         "user-1",
		"conversation_id": "conv-1",
	}

	if err := store.Save(context.Background(), "session-a", memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "first"}},
		Metadata: sharedMetadata,
	}); err != nil {
		t.Fatalf("Save() session-a error = %v", err)
	}
	if err := store.Save(context.Background(), "session-b", memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "second"}},
		Metadata: sharedMetadata,
	}); err != nil {
		t.Fatalf("Save() session-b error = %v", err)
	}

	first, err := store.Load(context.Background(), "session-a")
	if err != nil {
		t.Fatalf("Load() session-a error = %v", err)
	}
	second, err := store.Load(context.Background(), "session-b")
	if err != nil {
		t.Fatalf("Load() session-b error = %v", err)
	}

	if got := first.Messages[0].Content; got != "first" {
		t.Fatalf("session-a message = %q want %q", got, "first")
	}
	if got := second.Messages[0].Content; got != "second" {
		t.Fatalf("session-b message = %q want %q", got, "second")
	}
}

func TestInMemoryWorkingMemoryFactoryCreatesIsolatedWorkingSets(t *testing.T) {
	t.Parallel()

	factory := memory.InMemoryWorkingMemoryFactory{}

	first, err := factory.NewRunState(context.Background(), "agent-1", "run-1")
	if err != nil {
		t.Fatalf("NewRunState() first error = %v", err)
	}
	second, err := factory.NewRunState(context.Background(), "agent-1", "run-2")
	if err != nil {
		t.Fatalf("NewRunState() second error = %v", err)
	}

	first.AddMessage(types.Message{Role: types.RoleUser, Content: "hi"})
	first.AddRecord(memory.Record{Kind: "tool_call", Name: "search", Data: map[string]any{"q": "golang"}})

	firstSnapshot := first.Snapshot()
	secondSnapshot := second.Snapshot()

	if len(firstSnapshot.Messages) != 1 || len(firstSnapshot.Records) != 1 {
		t.Fatalf("first snapshot = %+v", firstSnapshot)
	}
	if len(secondSnapshot.Messages) != 0 || len(secondSnapshot.Records) != 0 {
		t.Fatalf("second snapshot = %+v want empty", secondSnapshot)
	}

	firstSnapshot.Messages[0].Content = "tampered"
	firstSnapshot.Records[0].Data["q"] = "tampered"

	reloaded := first.Snapshot()
	if got := reloaded.Messages[0].Content; got != "hi" {
		t.Fatalf("reloaded message = %q want %q", got, "hi")
	}
	if got := reloaded.Records[0].Data["q"]; got != "golang" {
		t.Fatalf("reloaded record q = %v want %q", got, "golang")
	}
}

func TestInMemoryWorkingMemoryFactoryHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (memory.InMemoryWorkingMemoryFactory{}).NewRunState(ctx, "agent-1", "run-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NewRunState() error = %v want %v", err, context.Canceled)
	}
}
