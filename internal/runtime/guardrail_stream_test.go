package runtime

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestRunSequentialInputAndOutputGuardrailsAreChained(t *testing.T) {
	t.Parallel()

	var secondInputSeen string
	var secondOutputSeen string
	model := callbackModel{
		generate: func(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
			return agent.ModelResponse{
				Message: types.Message{Role: types.RoleAssistant, Content: "draft"},
			}, nil
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "sequential-guardrails", Model: model},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithInputGuardrails(
			inputGuardrailFunc(func(context.Context, guardrail.InputRequest) (guardrail.Decision, error) {
				return guardrail.Decision{
					Name:    "input-first",
					Action:  guardrail.ActionTransform,
					Messages: []types.Message{{Role: types.RoleUser, Content: "first"}},
				}, nil
			}),
			inputGuardrailFunc(func(_ context.Context, req guardrail.InputRequest) (guardrail.Decision, error) {
				secondInputSeen = req.Messages[0].Content
				return guardrail.Decision{
					Name:    "input-second",
					Action:  guardrail.ActionTransform,
					Messages: []types.Message{{Role: types.RoleUser, Content: req.Messages[0].Content + "-second"}},
				}, nil
			}),
		),
		agent.WithOutputGuardrails(
			outputGuardrailFunc(func(_ context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
				return guardrail.Decision{
					Name:   "output-first",
					Action: guardrail.ActionTransform,
					Message: &types.Message{
						Role:    types.RoleAssistant,
						Content: "safe",
					},
				}, nil
			}),
			outputGuardrailFunc(func(_ context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
				secondOutputSeen = req.Message.Content
				return guardrail.Decision{
					Name:   "output-second",
					Action: guardrail.ActionTransform,
					Message: &types.Message{
						Role:    types.RoleAssistant,
						Content: req.Message.Content + "!",
					},
				}, nil
			}),
		),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "ignored"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if secondInputSeen != "first" {
		t.Fatalf("second input guardrail saw %q want %q", secondInputSeen, "first")
	}
	if secondOutputSeen != "safe" {
		t.Fatalf("second output guardrail saw %q want %q", secondOutputSeen, "safe")
	}
	if response.Message.Content != "safe!" {
		t.Fatalf("response content = %q want %q", response.Message.Content, "safe!")
	}
}

func TestStreamGuardrailsModifyDropAndOutputUsesApprovedBuffer(t *testing.T) {
	t.Parallel()

	store := &capturingDeltaStore{}
	var outputBase string
	var bufferedByChunk []string
	model := scriptedStreamModel{
		streamEvents: []agent.ModelEvent{
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "hello "}},
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "secret"}},
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: " world"}},
			{Message: &types.Message{Role: types.RoleAssistant, Content: "hello secret world"}, Done: true},
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "stream-guardrails", Model: model},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithMemory(store),
		agent.WithInputGuardrails(inputGuardrailFunc(func(context.Context, guardrail.InputRequest) (guardrail.Decision, error) {
			return guardrail.Decision{
				Name:    "input-pass",
				Action:  guardrail.ActionTransform,
				Messages: []types.Message{{Role: types.RoleUser, Content: "redact"}},
			}, nil
		})),
		agent.WithStreamGuardrails(
			streamGuardrailFunc(func(_ context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
				bufferedByChunk = append(bufferedByChunk, req.BufferedContent)
				if req.ChunkIndex == 2 {
					return guardrail.Decision{
						Name:   "mask-secret",
						Action: guardrail.ActionTransform,
						Delta: &types.MessageDelta{
							Role:    req.Delta.Role,
							RunID:   req.Delta.RunID,
							Content: "[secret]",
						},
					}, nil
				}
				return guardrail.Decision{Name: "mask-secret", Action: guardrail.ActionAllow}, nil
			}),
			streamGuardrailFunc(func(_ context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
				if req.ChunkIndex == 2 {
					return guardrail.Decision{
						Name:   "decorate-secret",
						Action: guardrail.ActionTransform,
						Delta: &types.MessageDelta{
							Role:    req.Delta.Role,
							RunID:   req.Delta.RunID,
							Content: "<" + req.Delta.Content + ">",
						},
					}, nil
				}
				if req.ChunkIndex == 3 {
					return guardrail.Decision{Name: "drop-world", Action: guardrail.ActionDrop, Reason: "drop trailing chunk"}, nil
				}
				return guardrail.Decision{Name: "drop-world", Action: guardrail.ActionAllow}, nil
			}),
		),
		agent.WithOutputGuardrails(outputGuardrailFunc(func(_ context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
			outputBase = req.Message.Content
			return guardrail.Decision{
				Name:   "output-suffix",
				Action: guardrail.ActionTransform,
				Message: &types.Message{
					Role:    types.RoleAssistant,
					Content: req.Message.Content + "!",
				},
			}, nil
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	stream, err := ag.Stream(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "please redact"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	events, err := drainEvents(stream)
	if err != nil {
		t.Fatalf("drainEvents() error = %v", err)
	}

	gotDeltas := collectDeltaContents(events)
	if !reflect.DeepEqual(gotDeltas, []string{"hello ", "<[secret]>"}) {
		t.Fatalf("delta contents = %v want %v", gotDeltas, []string{"hello ", "<[secret]>"})
	}
	if outputBase != "hello <[secret]>" {
		t.Fatalf("output base = %q want %q", outputBase, "hello <[secret]>")
	}
	if !reflect.DeepEqual(bufferedByChunk, []string{"", "hello ", "hello <[secret]>"}) {
		t.Fatalf("buffered content = %v", bufferedByChunk)
	}

	completed := terminalCompletedEvent(t, events)
	if completed.Response == nil {
		t.Fatal("completed response is nil")
	}
	if completed.Response.Message.Content != "hello <[secret]>!" {
		t.Fatalf("completed response content = %q want %q", completed.Response.Message.Content, "hello <[secret]>!")
	}
	if store.delta == nil || store.delta.Response == nil || store.delta.Response.Content != "hello <[secret]>!" {
		t.Fatalf("persisted response = %+v", store.delta)
	}

	gotActions := filterNonAllowDecisions(completed.Response.GuardrailDecisions)
	wantActions := []string{
		"input:transform:input-pass",
		"stream:transform:mask-secret",
		"stream:transform:decorate-secret",
		"stream:drop:drop-world",
		"output:transform:output-suffix",
	}
	if !reflect.DeepEqual(gotActions, wantActions) {
		t.Fatalf("guardrail decisions = %v want %v", gotActions, wantActions)
	}
}

func TestStreamGuardrailAbortFailsRunWithoutOutputOrPersistence(t *testing.T) {
	t.Parallel()

	store := &capturingDeltaStore{}
	var outputCalled bool
	model := scriptedStreamModel{
		streamEvents: []agent.ModelEvent{
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "safe"}},
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "forbidden"}},
			{Message: &types.Message{Role: types.RoleAssistant, Content: "should not finish"}, Done: true},
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "stream-abort", Model: model},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithMemory(store),
		agent.WithStreamGuardrails(streamGuardrailFunc(func(_ context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
			if req.Delta.Content == "forbidden" {
				return guardrail.Decision{Name: "abort-forbidden", Action: guardrail.ActionAbort, Reason: "forbidden content"}, nil
			}
			return guardrail.Decision{Name: "abort-forbidden", Action: guardrail.ActionAllow}, nil
		})),
		agent.WithOutputGuardrails(outputGuardrailFunc(func(context.Context, guardrail.OutputRequest) (guardrail.Decision, error) {
			outputCalled = true
			return guardrail.Decision{Name: "output", Action: guardrail.ActionAllow}, nil
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	stream, err := ag.Stream(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	events, err := drainEvents(stream)
	if err != nil {
		t.Fatalf("drainEvents() error = %v", err)
	}

	if got := collectDeltaContents(events); !reflect.DeepEqual(got, []string{"safe"}) {
		t.Fatalf("delta contents = %v want %v", got, []string{"safe"})
	}
	if outputCalled {
		t.Fatal("output guardrail was called after abort")
	}
	if store.saveCalls != 0 {
		t.Fatalf("save calls = %d want 0", store.saveCalls)
	}

	failed := terminalFailedEvent(t, events)
	if failed.Err == nil || !errors.Is(failed.Err, agent.ErrGuardrailBlocked) {
		t.Fatalf("failed err = %v want guardrail blocked", failed.Err)
	}
	if hasCompletedEvent(events) {
		t.Fatal("unexpected completed event after abort")
	}
}

func TestStreamGuardrailInvalidDecisionFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule guardrail.Stream
	}{
		{
			name: "transform without delta",
			rule: streamGuardrailFunc(func(context.Context, guardrail.StreamRequest) (guardrail.Decision, error) {
				return guardrail.Decision{Name: "invalid-transform", Action: guardrail.ActionTransform}, nil
			}),
		},
		{
			name: "block on stream",
			rule: streamGuardrailFunc(func(context.Context, guardrail.StreamRequest) (guardrail.Decision, error) {
				return guardrail.Decision{Name: "invalid-block", Action: guardrail.ActionBlock}, nil
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := scriptedStreamModel{
				streamEvents: []agent.ModelEvent{
					{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "hello"}},
					{Message: &types.Message{Role: types.RoleAssistant, Content: "hello"}, Done: true},
				},
			}
			ag, err := agent.New(
				agent.Config{Name: "stream-invalid", Model: model},
				agent.WithExecutionEngine(NewEngine()),
				agent.WithStreamGuardrails(tt.rule),
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

			events, err := drainEvents(stream)
			if err != nil {
				t.Fatalf("drainEvents() error = %v", err)
			}

			failed := terminalFailedEvent(t, events)
			var agentErr *agent.Error
			if !errors.As(failed.Err, &agentErr) || agentErr.Kind != agent.ErrorKindInternal || agentErr.Op != "guardrail.stream" {
				t.Fatalf("failed err = %#v want internal guardrail.stream error", failed.Err)
			}
		})
	}
}

type inputGuardrailFunc func(context.Context, guardrail.InputRequest) (guardrail.Decision, error)

func (f inputGuardrailFunc) CheckInput(ctx context.Context, req guardrail.InputRequest) (guardrail.Decision, error) {
	return f(ctx, req)
}

type streamGuardrailFunc func(context.Context, guardrail.StreamRequest) (guardrail.Decision, error)

func (f streamGuardrailFunc) CheckStream(ctx context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
	return f(ctx, req)
}

type outputGuardrailFunc func(context.Context, guardrail.OutputRequest) (guardrail.Decision, error)

func (f outputGuardrailFunc) CheckOutput(ctx context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
	return f(ctx, req)
}

type scriptedStreamModel struct {
	streamEvents []agent.ModelEvent
	generate     func(context.Context, agent.ModelRequest) (agent.ModelResponse, error)
}

func (m scriptedStreamModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	if m.generate != nil {
		return m.generate(ctx, req)
	}
	return agent.ModelResponse{
		Message: types.Message{Role: types.RoleAssistant, Content: "fallback"},
	}, nil
}

func (m scriptedStreamModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return &scriptedModelStream{events: append([]agent.ModelEvent(nil), m.streamEvents...)}, nil
}

type scriptedModelStream struct {
	events []agent.ModelEvent
	index  int
}

func (s *scriptedModelStream) Recv() (agent.ModelEvent, error) {
	if s.index >= len(s.events) {
		return agent.ModelEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (*scriptedModelStream) Close() error { return nil }

type capturingDeltaStore struct {
	saveCalls int
	delta     *memory.Delta
}

func (*capturingDeltaStore) Load(context.Context, string) (memory.Snapshot, error) {
	return memory.Snapshot{}, nil
}

func (s *capturingDeltaStore) Save(_ context.Context, _ string, delta memory.Delta) error {
	s.saveCalls++
	cloned := memory.Delta{
		Messages: types.CloneMessages(delta.Messages),
		Records:  append([]memory.Record(nil), delta.Records...),
		Metadata: types.CloneMetadata(delta.Metadata),
	}
	if delta.Response != nil {
		message := types.CloneMessage(*delta.Response)
		cloned.Response = &message
	}
	s.delta = &cloned
	return nil
}

func drainEvents(stream agent.Stream) ([]agent.Event, error) {
	var events []agent.Event
	for {
		event, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return events, nil
			}
			return nil, err
		}
		events = append(events, event)
	}
}

func collectDeltaContents(events []agent.Event) []string {
	var out []string
	for _, event := range events {
		if event.Type == agent.EventAgentDelta && event.Delta != nil {
			out = append(out, event.Delta.Content)
		}
	}
	return out
}

func filterNonAllowDecisions(decisions []agent.GuardrailDecision) []string {
	var out []string
	for _, decision := range decisions {
		if decision.Action == guardrail.ActionAllow {
			continue
		}
		out = append(out, string(decision.Phase)+":"+string(decision.Action)+":"+decision.Name)
	}
	return out
}

func terminalCompletedEvent(t *testing.T, events []agent.Event) agent.Event {
	t.Helper()
	for _, event := range events {
		if event.Type == agent.EventAgentCompleted {
			return event
		}
	}
	t.Fatal("completed event not found")
	return agent.Event{}
}

func terminalFailedEvent(t *testing.T, events []agent.Event) agent.Event {
	t.Helper()
	for _, event := range events {
		if event.Type == agent.EventAgentFailed {
			return event
		}
	}
	t.Fatal("failed event not found")
	return agent.Event{}
}

func hasCompletedEvent(events []agent.Event) bool {
	for _, event := range events {
		if event.Type == agent.EventAgentCompleted {
			return true
		}
	}
	return false
}
