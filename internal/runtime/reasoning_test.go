package runtime

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestNewEngineRegistersPrivateReasoningToolkitByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       ReasoningConfig
		wantTools []string
		wantKit   bool
	}{
		{
			name: "disabled",
			cfg:  DefaultReasoningConfig(),
		},
		{
			name: "think only",
			cfg: func() ReasoningConfig {
				cfg := DefaultReasoningConfig()
				cfg.Enabled = true
				cfg.Mode = ReasoningModeThinkOnly
				return cfg
			}(),
			wantTools: []string{"reasoning.think"},
			wantKit:   true,
		},
		{
			name: "think and analyze",
			cfg: func() ReasoningConfig {
				cfg := DefaultReasoningConfig()
				cfg.Enabled = true
				cfg.Mode = ReasoningModeThinkAndAnalyze
				return cfg
			}(),
			wantTools: []string{"reasoning.analyze", "reasoning.think"},
			wantKit:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			eng := NewEngine(WithReasoningConfig(tt.cfg)).(*engine)
			if eng.reasoningErr != nil {
				t.Fatalf("reasoningErr = %v", eng.reasoningErr)
			}

			if !tt.wantKit {
				if eng.reasoning != nil {
					t.Fatalf("reasoning runtime = %+v want nil", eng.reasoning)
				}
				return
			}

			if eng.reasoning == nil {
				t.Fatal("reasoning runtime is nil")
			}

			toolkits := eng.reasoning.registry.ListToolkits()
			if len(toolkits) != 1 || toolkits[0].Name != reasoningToolkitName {
				t.Fatalf("toolkits = %+v", toolkits)
			}

			descriptors := eng.reasoning.registry.List()
			gotNames := make([]string, len(descriptors))
			for index, descriptor := range descriptors {
				gotNames[index] = descriptor.Name
			}
			if !slices.Equal(gotNames, tt.wantTools) {
				t.Fatalf("tools = %v want %v", gotNames, tt.wantTools)
			}
		})
	}
}

func TestRunReasoningContinueAfterToolResultStaysInternal(t *testing.T) {
	t.Parallel()

	factory := &capturingWorkingMemoryFactory{}
	var toolCatalog [][]string
	model := &sequenceModel{
		generate: func(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
			toolCatalog = append(toolCatalog, toolSpecNames(req.Tools))
			if len(toolCatalog) == 1 {
				return agent.ModelResponse{
					ToolCalls: []agent.ModelToolCall{{
						ID:    "search-1",
						Name:  "search",
						Input: map[string]any{"query": "golang"},
					}},
				}, nil
			}

			if toolCatalog[1] == nil || slices.Contains(toolCatalog[1], "reasoning.think") || slices.Contains(toolCatalog[1], "reasoning.analyze") {
				t.Fatalf("model tools unexpectedly exposed reasoning toolkit: %v", toolCatalog[1])
			}
			if !containsSystemNote(req.Messages) {
				t.Fatalf("expected internal reasoning note in model context, got %+v", req.Messages)
			}

			return agent.ModelResponse{
				Message: types.Message{Role: types.RoleAssistant, Content: "final answer"},
			}, nil
		},
	}

	cfg := DefaultReasoningConfig()
	cfg.Enabled = true
	cfg.Mode = ReasoningModeThinkOnly

	ag, err := agent.New(
		agent.Config{Name: "reasoning-continue", Model: model},
		agent.WithExecutionEngine(NewEngine(WithReasoningConfig(cfg))),
		agent.WithWorkingMemory(factory),
		agent.WithTools(testTool{name: "search", result: tool.Result{Value: "result: golang", Content: "result: golang"}}),
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

	if response.Message.Content != "final answer" {
		t.Fatalf("response content = %q want %q", response.Message.Content, "final answer")
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "search" {
		t.Fatalf("tool calls = %+v", response.ToolCalls)
	}
	if hasPublicReasoningArtifacts(response.ToolCalls) {
		t.Fatalf("reasoning leaked into public tool calls: %+v", response.ToolCalls)
	}

	records := factory.records()
	if !hasRecord(records, reasoningKindArtifact, "reasoning.think") {
		t.Fatalf("working memory records = %+v want reasoning think artifact", records)
	}
}

func TestRunReasoningValidateFinalAnswerWithoutPersistingArtifacts(t *testing.T) {
	t.Parallel()

	factory := &capturingWorkingMemoryFactory{}
	store := &capturingMemoryStore{}
	cfg := DefaultReasoningConfig()
	cfg.Enabled = true
	cfg.Mode = ReasoningModeThinkAndAnalyze
	cfg.RequireAnalyzeBeforeResponse = true

	ag, err := agent.New(
		agent.Config{
			Name: "reasoning-validate",
			Model: staticModel{
				response: agent.ModelResponse{
					Message: types.Message{Role: types.RoleAssistant, Content: "draft answer"},
				},
			},
		},
		agent.WithExecutionEngine(NewEngine(WithReasoningConfig(cfg))),
		agent.WithWorkingMemory(factory),
		agent.WithMemory(store),
		agent.WithOutputGuardrails(testOutputGuardrail{content: "final answer"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "answer carefully"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if response.Message.Content != "final answer" {
		t.Fatalf("response content = %q want %q", response.Message.Content, "final answer")
	}
	records := factory.records()
	if !hasRecord(records, reasoningKindArtifact, "reasoning.think") || !hasRecord(records, reasoningKindArtifact, "reasoning.analyze") {
		t.Fatalf("working memory records = %+v want reasoning think/analyze", records)
	}

	saved := store.delta()
	if saved == nil {
		t.Fatal("expected persisted delta")
	}
	if len(saved.Records) != 0 {
		t.Fatalf("persisted records = %+v want no reasoning artifacts", saved.Records)
	}
	if saved.Response == nil || saved.Response.Content != "final answer" {
		t.Fatalf("persisted response = %+v", saved.Response)
	}
}

func TestRunRejectsReservedReasoningToolCallsFromModel(t *testing.T) {
	t.Parallel()

	cfg := DefaultReasoningConfig()
	cfg.Enabled = true
	cfg.Mode = ReasoningModeThinkAndAnalyze

	ag, err := agent.New(
		agent.Config{
			Name: "reasoning-reserved",
			Model: staticModel{
				response: agent.ModelResponse{
					ToolCalls: []agent.ModelToolCall{{
						ID:    "internal-1",
						Name:  "reasoning.think",
						Input: map[string]any{},
					}},
				},
			},
		},
		agent.WithExecutionEngine(NewEngine(WithReasoningConfig(cfg))),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hack"}},
	})
	if err == nil {
		t.Fatal("expected run error")
	}
	var agentErr *agent.Error
	if !errors.As(err, &agentErr) || agentErr.Kind != agent.ErrorKindTool {
		t.Fatalf("error = %v want tool error", err)
	}
}

type sequenceModel struct {
	mu       sync.Mutex
	generate func(context.Context, agent.ModelRequest) (agent.ModelResponse, error)
}

func (m *sequenceModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generate(ctx, req)
}

func (*sequenceModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type staticModel struct {
	response agent.ModelResponse
}

func (m staticModel) Generate(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
	return m.response, nil
}

func (m staticModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type closedModelStream struct{}

func (closedModelStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedModelStream) Close() error                    { return nil }

type testTool struct {
	name   string
	result tool.Result
}

func (t testTool) Name() string        { return t.name }
func (t testTool) Description() string { return t.name }
func (t testTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"query": {Type: "string"},
		},
	}
}
func (t testTool) OutputSchema() tool.Schema { return tool.Schema{Type: "string"} }
func (t testTool) Call(context.Context, tool.Call) (tool.Result, error) {
	return t.result, nil
}

type capturingWorkingMemoryFactory struct {
	mu  sync.Mutex
	set *capturingWorkingSet
}

func (f *capturingWorkingMemoryFactory) NewRunState(context.Context, string, string) (memory.WorkingSet, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.set = &capturingWorkingSet{}
	return f.set, nil
}

func (f *capturingWorkingMemoryFactory) records() []memory.Record {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.set == nil {
		return nil
	}
	return f.set.recordsSnapshot()
}

type capturingWorkingSet struct {
	mu       sync.Mutex
	messages []types.Message
	records  []memory.Record
}

func (w *capturingWorkingSet) AddMessage(message types.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, types.CloneMessage(message))
}

func (w *capturingWorkingSet) AddRecord(record memory.Record) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, memory.Record{
		Kind: record.Kind,
		Name: record.Name,
		Data: cloneMap(record.Data),
	})
}

func (w *capturingWorkingSet) Snapshot() memory.Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()

	return memory.Snapshot{
		Messages: types.CloneMessages(w.messages),
		Records:  w.recordsSnapshot(),
	}
}

func (w *capturingWorkingSet) recordsSnapshot() []memory.Record {
	if len(w.records) == 0 {
		return nil
	}
	out := make([]memory.Record, len(w.records))
	for index, record := range w.records {
		out[index] = memory.Record{
			Kind: record.Kind,
			Name: record.Name,
			Data: cloneMap(record.Data),
		}
	}
	return out
}

type capturingMemoryStore struct {
	mu    sync.Mutex
	saved *memory.Delta
}

func (*capturingMemoryStore) Load(context.Context, string) (memory.Snapshot, error) {
	return memory.Snapshot{}, nil
}

func (s *capturingMemoryStore) Save(_ context.Context, _ string, delta memory.Delta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := memory.Delta{
		Messages: types.CloneMessages(delta.Messages),
		Records:  cloneRecords(delta.Records),
		Response: cloneMessagePointerValue(delta.Response),
		Metadata: types.CloneMetadata(delta.Metadata),
	}
	s.saved = &cloned
	return nil
}

func (s *capturingMemoryStore) delta() *memory.Delta {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.saved == nil {
		return nil
	}
	cloned := *s.saved
	cloned.Messages = types.CloneMessages(cloned.Messages)
	cloned.Records = cloneRecords(cloned.Records)
	cloned.Response = cloneMessagePointerValue(cloned.Response)
	cloned.Metadata = types.CloneMetadata(cloned.Metadata)
	return &cloned
}

type testOutputGuardrail struct {
	content string
}

func (g testOutputGuardrail) CheckOutput(context.Context, guardrail.OutputRequest) (guardrail.Decision, error) {
	return guardrail.Decision{
		Name:   "rewrite",
		Action: guardrail.ActionTransform,
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: g.content,
		},
	}, nil
}

func toolSpecNames(specs []agent.ToolSpec) []string {
	if len(specs) == 0 {
		return nil
	}
	out := make([]string, len(specs))
	for index, spec := range specs {
		out[index] = spec.Name
	}
	return out
}

func containsSystemNote(messages []types.Message) bool {
	for _, message := range messages {
		if message.Role == types.RoleSystem && message.Content != "" && message.Content == internalReasoningNote("continue the loop with the latest public tool result") {
			return true
		}
	}
	return false
}

func hasPublicReasoningArtifacts(records []agent.ToolCallRecord) bool {
	for _, record := range records {
		if isReservedReasoningToolName(record.Name) {
			return true
		}
	}
	return false
}

func hasRecord(records []memory.Record, kind, name string) bool {
	for _, record := range records {
		if record.Kind == kind && record.Name == name {
			return true
		}
	}
	return false
}

func cloneMessagePointerValue(message *types.Message) *types.Message {
	if message == nil {
		return nil
	}
	cloned := types.CloneMessage(*message)
	return &cloned
}
