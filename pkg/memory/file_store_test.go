package memory_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestFileStoreLoadMissingSessionReturnsEmptySnapshot(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())

	snapshot, err := store.Load(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 0 || len(snapshot.Records) != 0 || len(snapshot.Metadata) != 0 {
		t.Fatalf("Load() snapshot = %+v want empty", snapshot)
	}
}

func TestFileStoreSaveAndLoadRoundtrip(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "hi"},
			{Role: types.RoleAssistant, Content: "hello"},
		},
		Records: []memory.Record{{
			Kind: "note",
			Name: "first",
			Data: map[string]any{"tags": []any{"a", "b"}},
		}},
		Metadata: types.Metadata{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	}

	if err := store.Save(context.Background(), "session-1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.Messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(snapshot.Messages))
	}
	if snapshot.Messages[0].Content != "hi" {
		t.Fatalf("message[0] = %q want %q", snapshot.Messages[0].Content, "hi")
	}
	if snapshot.Messages[1].Content != "hello" {
		t.Fatalf("message[1] = %q want %q", snapshot.Messages[1].Content, "hello")
	}
}

func TestFileStorePreservesChronologicalMessageOrder(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "first"},
			{Role: types.RoleAssistant, Content: "second"},
			{Role: types.RoleUser, Content: "third"},
			{Role: types.RoleAssistant, Content: "fourth"},
		},
	}

	if err := store.Save(context.Background(), "s1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for i, want := range []string{"first", "second", "third", "fourth"} {
		if snapshot.Messages[i].Content != want {
			t.Fatalf("message[%d] = %q want %q", i, snapshot.Messages[i].Content, want)
		}
	}
}

func TestFileStorePreservesRecords(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Records: []memory.Record{
			{Kind: "tool_call", Name: "search", Data: map[string]any{"q": "golang"}},
			{Kind: "note", Name: "summary", Data: map[string]any{"text": "found it"}},
		},
	}

	if err := store.Save(context.Background(), "s1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.Records) != 2 {
		t.Fatalf("got %d records, want 2", len(snapshot.Records))
	}
	if snapshot.Records[0].Kind != "tool_call" || snapshot.Records[0].Name != "search" {
		t.Fatalf("record[0] = %+v", snapshot.Records[0])
	}
	if snapshot.Records[0].Data["q"] != "golang" {
		t.Fatalf("record[0].Data[q] = %v want %q", snapshot.Records[0].Data["q"], "golang")
	}
	if snapshot.Records[1].Kind != "note" || snapshot.Records[1].Name != "summary" {
		t.Fatalf("record[1] = %+v", snapshot.Records[1])
	}
}

func TestFileStorePreservesMetadata(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
		Metadata: types.Metadata{
			"user_id":         "user-42",
			"conversation_id": "conv-99",
			"custom_key":      "custom_val",
		},
	}

	if err := store.Save(context.Background(), "s1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, key := range []string{"user_id", "conversation_id", "custom_key"} {
		if snapshot.Metadata[key] != delta.Metadata[key] {
			t.Fatalf("metadata[%s] = %q want %q", key, snapshot.Metadata[key], delta.Metadata[key])
		}
	}
}

func TestFileStoreLoadInvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := memory.MustNewFileStore(dir)

	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.Load(context.Background(), "broken")
	if err == nil {
		t.Fatal("Load() expected error for invalid JSON")
	}
}

func TestFileStoreLoadEmptyFileReturnsEmptySnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := memory.MustNewFileStore(dir)

	if err := os.WriteFile(filepath.Join(dir, "empty.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.Load(context.Background(), "empty")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 0 {
		t.Fatalf("Load() snapshot = %+v want empty", snapshot)
	}
}

func TestFileStoreLoadHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Load(ctx, "any")
	if err == nil {
		t.Fatal("Load() expected error for canceled context")
	}
}

func TestFileStoreSaveHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Save(ctx, "any", memory.Delta{})
	if err == nil {
		t.Fatal("Save() expected error for canceled context")
	}
}

func TestNewFileStoreCreatesDirectoryAutomatically(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "conversations")
	store, err := memory.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if err := store.Save(context.Background(), "s1", memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(snapshot.Messages))
	}
}

func TestNewFileStoreRejectsEmptyDir(t *testing.T) {
	t.Parallel()

	_, err := memory.NewFileStore("")
	if err == nil {
		t.Fatal("NewFileStore(\"\") expected error")
	}
}

func TestFileStoreRestartProof(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	store1 := memory.MustNewFileStore(dir)
	delta := memory.Delta{
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "hello from run 1"},
			{Role: types.RoleAssistant, Content: "acknowledged"},
		},
		Records: []memory.Record{
			{Kind: "tool_call", Name: "search", Data: map[string]any{"q": "test"}},
		},
		Metadata: types.Metadata{"user_id": "u1"},
	}

	if err := store1.Save(context.Background(), "persistent-session", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	store2 := memory.MustNewFileStore(dir)

	snapshot, err := store2.Load(context.Background(), "persistent-session")
	if err != nil {
		t.Fatalf("Load() from new instance error = %v", err)
	}

	if len(snapshot.Messages) != 2 {
		t.Fatalf("got %d messages after restart, want 2", len(snapshot.Messages))
	}
	if snapshot.Messages[0].Content != "hello from run 1" {
		t.Fatalf("message[0] = %q want %q", snapshot.Messages[0].Content, "hello from run 1")
	}
	if snapshot.Messages[1].Content != "acknowledged" {
		t.Fatalf("message[1] = %q want %q", snapshot.Messages[1].Content, "acknowledged")
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].Kind != "tool_call" {
		t.Fatalf("records after restart = %+v", snapshot.Records)
	}
	if snapshot.Metadata["user_id"] != "u1" {
		t.Fatalf("metadata after restart = %+v", snapshot.Metadata)
	}
}

func TestFileStoreConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()

			sessionID := "concurrent-session"
			delta := memory.Delta{
				Messages: []types.Message{
					{Role: types.RoleUser, Content: "msg"},
				},
			}

			if err := store.Save(context.Background(), sessionID, delta); err != nil {
				t.Errorf("goroutine %d Save() error = %v", n, err)
				return
			}

			if _, err := store.Load(context.Background(), sessionID); err != nil {
				t.Errorf("goroutine %d Load() error = %v", n, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestFileStoreOverwritesExistingSession(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())

	first := memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "original"}},
	}
	if err := store.Save(context.Background(), "s1", first); err != nil {
		t.Fatalf("Save() first error = %v", err)
	}

	second := memory.Delta{
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "original"},
			{Role: types.RoleAssistant, Content: "response"},
			{Role: types.RoleUser, Content: "followup"},
		},
	}
	if err := store.Save(context.Background(), "s1", second); err != nil {
		t.Fatalf("Save() second error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 3 {
		t.Fatalf("got %d messages, want 3", len(snapshot.Messages))
	}
	if snapshot.Messages[2].Content != "followup" {
		t.Fatalf("message[2] = %q want %q", snapshot.Messages[2].Content, "followup")
	}
}

func TestFileStoreDefensiveCopies(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "original"}},
		Metadata: types.Metadata{"key": "value"},
	}

	if err := store.Save(context.Background(), "s1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	delta.Messages[0].Content = "mutated"
	delta.Metadata["key"] = "mutated"

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if snapshot.Messages[0].Content != "original" {
		t.Fatalf("message = %q want %q (mutation leaked through Save)", snapshot.Messages[0].Content, "original")
	}
	if snapshot.Metadata["key"] != "value" {
		t.Fatalf("metadata = %q want %q (mutation leaked through Save)", snapshot.Metadata["key"], "value")
	}

	snapshot.Messages[0].Content = "tampered"
	snapshot.Metadata["key"] = "tampered"

	reloaded, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() reload error = %v", err)
	}
	if reloaded.Messages[0].Content != "original" {
		t.Fatalf("reloaded message = %q want %q (mutation leaked through Load)", reloaded.Messages[0].Content, "original")
	}
	if reloaded.Metadata["key"] != "value" {
		t.Fatalf("reloaded metadata = %q want %q (mutation leaked through Load)", reloaded.Metadata["key"], "value")
	}
}

func TestFileStoreSanitizesSessionIDForFilename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := memory.MustNewFileStore(dir)

	delta := memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "data"}},
	}

	if err := store.Save(context.Background(), "user/conv:123", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "user/conv:123")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Messages) != 1 || snapshot.Messages[0].Content != "data" {
		t.Fatalf("roundtrip failed for special-char sessionID")
	}

	expected := filepath.Join(dir, "user_conv_123.json")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected file %q not found: %v", expected, err)
	}
}

func TestFileStoreSessionIsolation(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())

	if err := store.Save(context.Background(), "a", memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "alpha"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "b", memory.Delta{
		Messages: []types.Message{{Role: types.RoleUser, Content: "beta"}},
	}); err != nil {
		t.Fatal(err)
	}

	sa, _ := store.Load(context.Background(), "a")
	sb, _ := store.Load(context.Background(), "b")

	if sa.Messages[0].Content != "alpha" {
		t.Fatalf("session a = %q want %q", sa.Messages[0].Content, "alpha")
	}
	if sb.Messages[0].Content != "beta" {
		t.Fatalf("session b = %q want %q", sb.Messages[0].Content, "beta")
	}
}

func TestFileStoreMessageWithToolCalls(t *testing.T) {
	t.Parallel()

	store := memory.MustNewFileStore(t.TempDir())
	delta := memory.Delta{
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "search for golang"},
			{
				Role:    types.RoleAssistant,
				Content: "",
				ToolCalls: []types.ToolCall{
					{ID: "tc1", Name: "search", Input: map[string]any{"q": "golang"}},
				},
			},
			{Role: types.RoleTool, Content: "found 42 results", ToolCallID: "tc1"},
			{Role: types.RoleAssistant, Content: "I found 42 results for golang"},
		},
	}

	if err := store.Save(context.Background(), "s1", delta); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snapshot, err := store.Load(context.Background(), "s1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(snapshot.Messages) != 4 {
		t.Fatalf("got %d messages, want 4", len(snapshot.Messages))
	}
	if len(snapshot.Messages[1].ToolCalls) != 1 {
		t.Fatalf("message[1] tool calls = %d want 1", len(snapshot.Messages[1].ToolCalls))
	}
	tc := snapshot.Messages[1].ToolCalls[0]
	if tc.ID != "tc1" || tc.Name != "search" {
		t.Fatalf("tool call = %+v", tc)
	}
	if tc.Input["q"] != "golang" {
		t.Fatalf("tool call input = %v", tc.Input)
	}
	if snapshot.Messages[2].ToolCallID != "tc1" {
		t.Fatalf("message[2] tool_call_id = %q want %q", snapshot.Messages[2].ToolCallID, "tc1")
	}
}
