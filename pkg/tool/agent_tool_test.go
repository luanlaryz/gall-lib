package tool_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// --- Construction tests (spec 12.1–12.5) ---

func TestNewAgentToolStaticValid(t *testing.T) {
	t.Parallel()

	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Delegates research tasks",
		RunFunc:     stubRunFunc("ok", "agent-1", "run-1"),
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}
	if at.Name() != "specialist" {
		t.Fatalf("Name() = %q want %q", at.Name(), "specialist")
	}
	if at.Description() != "Delegates research tasks" {
		t.Fatalf("Description() = %q want %q", at.Description(), "Delegates research tasks")
	}
	if at.InputSchema().Type != "object" {
		t.Fatalf("InputSchema().Type = %q want %q", at.InputSchema().Type, "object")
	}
	if at.OutputSchema().Type != "object" {
		t.Fatalf("OutputSchema().Type = %q want %q", at.OutputSchema().Type, "object")
	}
}

func TestNewAgentToolBothNil(t *testing.T) {
	t.Parallel()

	_, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Delegates research tasks",
	})
	if !errors.Is(err, tool.ErrInvalidTool) {
		t.Fatalf("NewAgentTool() error = %v want ErrInvalidTool", err)
	}
}

func TestNewAgentToolInvalidName(t *testing.T) {
	t.Parallel()

	tests := []string{"", "Bad Name", "123starts-with-digit", "UPPER"}
	for _, name := range tests {
		t.Run(fmt.Sprintf("name=%q", name), func(t *testing.T) {
			t.Parallel()

			_, err := tool.NewAgentTool(tool.AgentToolConfig{
				Name:        name,
				Description: "desc",
				RunFunc:     stubRunFunc("ok", "", ""),
			})
			if !errors.Is(err, tool.ErrInvalidTool) {
				t.Fatalf("NewAgentTool(name=%q) error = %v want ErrInvalidTool", name, err)
			}
		})
	}
}

func TestNewAgentToolEmptyDescription(t *testing.T) {
	t.Parallel()

	_, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:    "specialist",
		RunFunc: stubRunFunc("ok", "", ""),
	})
	if !errors.Is(err, tool.ErrInvalidTool) {
		t.Fatalf("NewAgentTool() error = %v want ErrInvalidTool", err)
	}
}

func TestNewAgentToolRunFuncPrecedenceOverResolver(t *testing.T) {
	t.Parallel()

	var resolverCalled atomic.Bool
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "desc",
		RunFunc:     stubRunFunc("from-static", "a1", "r1"),
		Resolver: func(name string) (tool.AgentRunFunc, error) {
			resolverCalled.Store(true)
			return stubRunFunc("from-resolver", "a2", "r2"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	result, err := at.Call(context.Background(), tool.Call{
		ID:    "c1",
		Input: map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result.Content != "from-static" {
		t.Fatalf("Content = %q want %q", result.Content, "from-static")
	}
	if resolverCalled.Load() {
		t.Fatal("Resolver was called despite RunFunc being provided")
	}
}

// --- Execution tests (spec 12.6–12.9) ---

func TestAgentToolHappyPath(t *testing.T) {
	t.Parallel()

	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc:     stubRunFunc("research result", "specialist-id", "run-42"),
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	result, err := at.Call(context.Background(), tool.Call{
		ID:        "call-1",
		AgentID:   "coordinator-id",
		RunID:     "coord-run-1",
		SessionID: "session-abc",
		Input:     map[string]any{"prompt": "What is Go?"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result.Content != "research result" {
		t.Fatalf("Content = %q want %q", result.Content, "research result")
	}
	valueMap, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatalf("Value type = %T want map[string]any", result.Value)
	}
	if valueMap["response"] != "research result" {
		t.Fatalf("Value[response] = %v want %q", valueMap["response"], "research result")
	}
}

func TestAgentToolSubAgentError(t *testing.T) {
	t.Parallel()

	subErr := errors.New("sub-agent failed")
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc: func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
			return tool.AgentToolResult{}, subErr
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	_, callErr := at.Call(context.Background(), tool.Call{
		ID:    "call-1",
		Input: map[string]any{"prompt": "hello"},
	})
	if callErr == nil {
		t.Fatal("Call() expected error")
	}

	var toolErr *tool.Error
	if !errors.As(callErr, &toolErr) {
		t.Fatalf("Call() error type = %T want *tool.Error", callErr)
	}
	if toolErr.Kind != tool.ErrorKindExecution {
		t.Fatalf("Kind = %q want %q", toolErr.Kind, tool.ErrorKindExecution)
	}
	if !errors.Is(callErr, subErr) {
		t.Fatalf("errors.Is(err, subErr) = false; want original cause accessible")
	}
}

func TestAgentToolCancellation(t *testing.T) {
	t.Parallel()

	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc: func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
			return tool.AgentToolResult{}, ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, callErr := at.Call(ctx, tool.Call{
		ID:    "call-1",
		Input: map[string]any{"prompt": "hello"},
	})
	if callErr == nil {
		t.Fatal("Call() expected error")
	}
	if !errors.Is(callErr, context.Canceled) {
		t.Fatalf("errors.Is(err, context.Canceled) = false; err = %v", callErr)
	}
}

func TestAgentToolSessionIDPropagation(t *testing.T) {
	t.Parallel()

	var capturedSessionID string
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc: func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
			capturedSessionID = sessionID
			return tool.AgentToolResult{Content: "done"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	_, err = at.Call(context.Background(), tool.Call{
		ID:        "call-1",
		SessionID: "session-xyz",
		Input:     map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if capturedSessionID != "session-xyz" {
		t.Fatalf("sessionID = %q want %q", capturedSessionID, "session-xyz")
	}
}

// --- Resolver tests (spec 12.10–12.12) ---

func TestAgentToolResolverLazyFirstCallOnly(t *testing.T) {
	t.Parallel()

	var resolveCount atomic.Int64
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		Resolver: func(name string) (tool.AgentRunFunc, error) {
			resolveCount.Add(1)
			return stubRunFunc("resolved", "a1", "r1"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	if resolveCount.Load() != 0 {
		t.Fatal("Resolver called at construction time")
	}

	_, err = at.Call(context.Background(), tool.Call{
		ID:    "call-1",
		Input: map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if resolveCount.Load() != 1 {
		t.Fatalf("resolve count = %d want 1", resolveCount.Load())
	}
}

func TestAgentToolResolverError(t *testing.T) {
	t.Parallel()

	resolveErr := errors.New("agent not found")
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		Resolver: func(name string) (tool.AgentRunFunc, error) {
			return nil, resolveErr
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	_, callErr := at.Call(context.Background(), tool.Call{
		ID:    "call-1",
		Input: map[string]any{"prompt": "hello"},
	})
	if callErr == nil {
		t.Fatal("Call() expected error")
	}
	var toolErr *tool.Error
	if !errors.As(callErr, &toolErr) {
		t.Fatalf("error type = %T want *tool.Error", callErr)
	}
	if toolErr.Kind != tool.ErrorKindExecution {
		t.Fatalf("Kind = %q want %q", toolErr.Kind, tool.ErrorKindExecution)
	}
	if !errors.Is(callErr, resolveErr) {
		t.Fatalf("errors.Is(err, resolveErr) = false; want original cause")
	}
}

func TestAgentToolResolverCachesAgent(t *testing.T) {
	t.Parallel()

	var resolveCount atomic.Int64
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		Resolver: func(name string) (tool.AgentRunFunc, error) {
			resolveCount.Add(1)
			return stubRunFunc("cached", "a1", "r1"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := at.Call(context.Background(), tool.Call{
			ID:    fmt.Sprintf("call-%d", i),
			Input: map[string]any{"prompt": "hello"},
		})
		if err != nil {
			t.Fatalf("Call(%d) error = %v", i, err)
		}
	}

	if resolveCount.Load() != 1 {
		t.Fatalf("resolve count = %d want 1 (should cache after first call)", resolveCount.Load())
	}
}

// --- Concurrency tests (spec 12.13–12.14) ---

func TestAgentToolConcurrentUsage(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc: func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
			calls.Add(1)
			return tool.AgentToolResult{Content: prompt, AgentID: "a1", RunID: "r1"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := at.Call(context.Background(), tool.Call{
				ID:    fmt.Sprintf("call-%d", i),
				Input: map[string]any{"prompt": "hello"},
			})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Call() error = %v", err)
		}
	}
	if calls.Load() != workers {
		t.Fatalf("calls = %d want %d", calls.Load(), workers)
	}
}

func TestAgentToolConcurrentResolverLazy(t *testing.T) {
	t.Parallel()

	var resolveCount atomic.Int64
	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		Resolver: func(name string) (tool.AgentRunFunc, error) {
			resolveCount.Add(1)
			return stubRunFunc("resolved", "a1", "r1"), nil
		},
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, err := at.Call(context.Background(), tool.Call{
				ID:    fmt.Sprintf("call-%d", i),
				Input: map[string]any{"prompt": "hello"},
			})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Call() error = %v", err)
		}
	}
	if resolveCount.Load() != 1 {
		t.Fatalf("resolve count = %d want 1 (sync.Once guarantees single resolution)", resolveCount.Load())
	}
}

// --- Observability test (spec 12.15) ---

func TestAgentToolResultMetadata(t *testing.T) {
	t.Parallel()

	at, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Research specialist",
		RunFunc:     stubRunFunc("answer", "specialist-id", "specialist-run-42"),
	})
	if err != nil {
		t.Fatalf("NewAgentTool() error = %v", err)
	}

	result, err := at.Call(context.Background(), tool.Call{
		ID:      "call-1",
		AgentID: "coordinator-id",
		RunID:   "coord-run-1",
		Input:   map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result.Metadata["sub_agent_id"] != "specialist-id" {
		t.Fatalf("Metadata[sub_agent_id] = %q want %q", result.Metadata["sub_agent_id"], "specialist-id")
	}
	if result.Metadata["sub_agent_run_id"] != "specialist-run-42" {
		t.Fatalf("Metadata[sub_agent_run_id] = %q want %q", result.Metadata["sub_agent_run_id"], "specialist-run-42")
	}
}

// --- helpers ---

func stubRunFunc(content, agentID, runID string) tool.AgentRunFunc {
	return func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
		return tool.AgentToolResult{
			Content: content,
			AgentID: agentID,
			RunID:   runID,
		}, nil
	}
}
