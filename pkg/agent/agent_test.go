package agent_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	coreruntime "github.com/luanlima/gaal-lib/internal/runtime"
	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestNewRejectsDuplicateTools(t *testing.T) {
	t.Parallel()

	_, err := agent.New(
		agent.Config{Name: "dupe-tools", Model: stubModel{}},
		agent.WithTools(stubTool{name: "search"}, stubTool{name: "search"}),
	)
	if !errors.Is(err, agent.ErrInvalidConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestNewRejectsReservedInternalToolNamespace(t *testing.T) {
	t.Parallel()

	_, err := agent.New(
		agent.Config{Name: "reserved-tools", Model: stubModel{}},
		agent.WithTools(reservedTool{}),
	)
	if !errors.Is(err, agent.ErrInvalidConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestRunSuccess(t *testing.T) {
	t.Parallel()

	ag, err := agent.New(
		agent.Config{
			Name:  "Greeter Agent",
			Model: staticModel{response: agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "hello"}}},
		},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if response.Message.Content != "hello" {
		t.Fatalf("Run() content = %q want %q", response.Message.Content, "hello")
	}
	if response.AgentID != ag.ID() {
		t.Fatalf("Run() agent id = %q want %q", response.AgentID, ag.ID())
	}
	if ag.Definition().MaxSteps != agent.DefaultMaxSteps {
		t.Fatalf("Definition().MaxSteps = %d want %d", ag.Definition().MaxSteps, agent.DefaultMaxSteps)
	}
}

func TestRunWithToolCallFeedsNextModelStep(t *testing.T) {
	t.Parallel()

	var calls int
	model := sequenceModel{
		generate: func(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
			calls++
			if calls == 1 {
				return agent.ModelResponse{
					ToolCalls: []agent.ModelToolCall{{
						ID:    "call-1",
						Name:  "search",
						Input: map[string]any{"query": "golang"},
					}},
				}, nil
			}

			if got := req.Messages[len(req.Messages)-1]; got.Role != types.RoleTool || got.Content != "result: golang" {
				t.Fatalf("last model message = %+v", got)
			}

			return agent.ModelResponse{
				Message: types.Message{Role: types.RoleAssistant, Content: "done"},
			}, nil
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "tool-agent", Model: model},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithTools(stubTool{name: "search", call: func(ctx context.Context, call tool.Call) (tool.Result, error) {
			if call.ToolName != "search" {
				t.Fatalf("tool name = %q want %q", call.ToolName, "search")
			}
			if call.Input["query"] != "golang" {
				t.Fatalf("tool input = %v", call.Input)
			}
			return tool.Result{Value: "result: golang", Content: "result: golang"}, nil
		}}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "search golang"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if response.Message.Content != "done" {
		t.Fatalf("Run() content = %q want %q", response.Message.Content, "done")
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "search" {
		t.Fatalf("Run() tool calls = %+v", response.ToolCalls)
	}
}

func TestRunGuardrailBlocked(t *testing.T) {
	t.Parallel()

	ag, err := agent.New(
		agent.Config{Name: "guarded", Model: staticModel{response: agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "never"}}}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithInputGuardrails(blockingInputGuardrail{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "blocked"}},
	})
	if !errors.Is(err, agent.ErrGuardrailBlocked) {
		t.Fatalf("expected guardrail blocked error, got %v", err)
	}
}

func TestRunCanceledByContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ag, err := agent.New(
		agent.Config{Name: "cancelable", Model: staticModel{response: agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "unused"}}}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = ag.Run(ctx, agent.Request{Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRunRequiresSessionWhenMemoryConfigured(t *testing.T) {
	t.Parallel()

	ag, err := agent.New(
		agent.Config{Name: "memory-agent", Model: stubModel{}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithMemory(memoryStoreStub{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, agent.ErrInvalidRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}

func TestStreamCloseCancelsRunAndIsIdempotent(t *testing.T) {
	t.Parallel()

	model := staticModel{
		generate: func(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
			<-ctx.Done()
			return agent.ModelResponse{}, ctx.Err()
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "streaming", Model: model},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	stream, err := ag.Stream(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	started, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv() started error = %v", err)
	}
	if started.Type != agent.EventAgentStarted {
		t.Fatalf("first event = %q want %q", started.Type, agent.EventAgentStarted)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("Close() first error = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}

	for {
		_, err = stream.Recv()
		if err == nil {
			continue
		}
		if errors.Is(err, agent.ErrStreamAborted) || errors.Is(err, io.EOF) {
			return
		}
		t.Fatalf("Recv() terminal error = %v", err)
	}
}

type stubModel struct{}

func (stubModel) Generate(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
	return agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "ok"}}, nil
}

func (stubModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type staticModel struct {
	generate func(context.Context, agent.ModelRequest) (agent.ModelResponse, error)
	response agent.ModelResponse
}

func (m staticModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	if m.generate != nil {
		return m.generate(ctx, req)
	}
	return m.response, nil
}

func (m staticModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type sequenceModel struct {
	generate func(context.Context, agent.ModelRequest) (agent.ModelResponse, error)
}

func (m sequenceModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	return m.generate(ctx, req)
}

func (m sequenceModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type closedModelStream struct{}

func (closedModelStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedModelStream) Close() error                    { return nil }

type stubTool struct {
	name string
	call func(context.Context, tool.Call) (tool.Result, error)
}

func (t stubTool) Name() string        { return t.name }
func (t stubTool) Description() string { return t.name }
func (t stubTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"query": {Type: "string"},
		},
	}
}
func (t stubTool) OutputSchema() tool.Schema { return tool.Schema{Type: "string"} }
func (t stubTool) Call(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.call != nil {
		return t.call(ctx, call)
	}
	return tool.Result{Value: "ok", Content: "ok"}, nil
}

type reservedTool struct{}

func (reservedTool) Name() string        { return "think" }
func (reservedTool) Description() string { return "reserved" }
func (reservedTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"query": {Type: "string"},
		},
	}
}
func (reservedTool) OutputSchema() tool.Schema { return tool.Schema{Type: "string"} }
func (reservedTool) Call(context.Context, tool.Call) (tool.Result, error) {
	return tool.Result{Value: "ok", Content: "ok"}, nil
}
func (reservedTool) Descriptor() tool.Descriptor {
	return tool.Descriptor{
		Name:      "reasoning.think",
		LocalName: "think",
		Namespace: "reasoning",
	}
}

type blockingInputGuardrail struct{}

func (blockingInputGuardrail) CheckInput(context.Context, guardrail.InputRequest) (guardrail.Decision, error) {
	return guardrail.Decision{
		Name:   "blocker",
		Action: guardrail.ActionBlock,
		Reason: "blocked for test",
	}, nil
}

type memoryStoreStub struct {
	load func(context.Context, string) (memory.Snapshot, error)
	save func(context.Context, string, memory.Delta) error
}

func (m memoryStoreStub) Load(ctx context.Context, sessionID string) (memory.Snapshot, error) {
	if m.load != nil {
		return m.load(ctx, sessionID)
	}
	return memory.Snapshot{}, nil
}

func (m memoryStoreStub) Save(ctx context.Context, sessionID string, delta memory.Delta) error {
	if m.save != nil {
		return m.save(ctx, sessionID, delta)
	}
	return nil
}

func TestOutputGuardrailTransform(t *testing.T) {
	t.Parallel()

	ag, err := agent.New(
		agent.Config{Name: "transformer", Model: staticModel{response: agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "raw"}}}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithOutputGuardrails(transformingOutputGuardrail{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if response.Message.Content != "safe" {
		t.Fatalf("Run() content = %q want %q", response.Message.Content, "safe")
	}
	if len(response.GuardrailDecisions) != 1 || response.GuardrailDecisions[0].Name != "transformer" {
		t.Fatalf("Run() guardrail decisions = %+v", response.GuardrailDecisions)
	}
}

type transformingOutputGuardrail struct{}

func (transformingOutputGuardrail) CheckOutput(context.Context, guardrail.OutputRequest) (guardrail.Decision, error) {
	return guardrail.Decision{
		Name:   "transformer",
		Action: guardrail.ActionTransform,
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: "safe",
		},
	}, nil
}

func TestConcurrentRunIsSafe(t *testing.T) {
	t.Parallel()

	ag, err := agent.New(
		agent.Config{Name: "concurrent", Model: staticModel{response: agent.ModelResponse{Message: types.Message{Role: types.RoleAssistant, Content: "ok"}}}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	var wg sync.WaitGroup
	results := make([]string, 4)
	for index := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			response, runErr := ag.Run(context.Background(), agent.Request{
				RunID:    time.Now().Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano),
				Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
			})
			if runErr != nil {
				t.Errorf("Run() error = %v", runErr)
				return
			}
			results[i] = response.Message.Content
		}(index)
	}
	wg.Wait()

	if !reflect.DeepEqual(results, []string{"ok", "ok", "ok", "ok"}) {
		t.Fatalf("results = %v", results)
	}
	if !slices.IsSortedFunc([]string{ag.Definition().Descriptor.Name}, func(a, b string) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	}) {
		t.Fatal("definition name unexpectedly unsorted")
	}
}
