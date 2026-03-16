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

func TestRunLoadsMemoryAfterInputGuardrailsAndPersistsBeforeCompleted(t *testing.T) {
	t.Parallel()

	var order []string
	appendOrder := func(step string) {
		order = append(order, step)
	}

	store := orderedMemoryStore{
		load: func(context.Context, string) (memory.Snapshot, error) {
			appendOrder("load")
			return memory.Snapshot{
				Messages: []types.Message{{Role: types.RoleAssistant, Content: "persisted context"}},
			}, nil
		},
		save: func(_ context.Context, _ string, delta memory.Delta) error {
			appendOrder("save")
			if got := delta.Messages[len(delta.Messages)-1].Content; got != "safe answer" {
				t.Fatalf("persisted final message = %q want %q", got, "safe answer")
			}
			return nil
		},
	}

	model := callbackModel{
		generate: func(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
			appendOrder("model")
			if want := []string{"input", "load", "working", "model"}; !reflect.DeepEqual(order, want) {
				t.Fatalf("order before model = %v want %v", order, want)
			}
			if len(req.Memory.Messages) != 1 || req.Memory.Messages[0].Content != "persisted context" {
				t.Fatalf("req.Memory = %+v", req.Memory)
			}
			return agent.ModelResponse{
				Message: types.Message{Role: types.RoleAssistant, Content: "draft answer"},
			}, nil
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "memory-order", Model: model},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithMemory(store),
		agent.WithWorkingMemory(orderedWorkingMemoryFactory{
			onCreate: func() { appendOrder("working") },
			delegate: memory.InMemoryWorkingMemoryFactory{},
		}),
		agent.WithInputGuardrails(orderedInputGuardrail(func() { appendOrder("input") })),
		agent.WithOutputGuardrails(orderedOutputGuardrail(func() {
			appendOrder("output")
		})),
		agent.WithHooks(agentHookFunc(func(ctx context.Context, event agent.Event) {
			if event.Type == agent.EventAgentCompleted {
				appendOrder("completed")
			}
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	response, err := ag.Run(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if response.Message.Content != "safe answer" {
		t.Fatalf("response content = %q want %q", response.Message.Content, "safe answer")
	}

	wantOrder := []string{"input", "load", "working", "model", "output", "save", "completed"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("order = %v want %v", order, wantOrder)
	}
}

func TestRunWorkingMemoryFactoryFailureStopsBeforeModel(t *testing.T) {
	t.Parallel()

	var modelCalled bool
	ag, err := agent.New(
		agent.Config{
			Name: "working-memory-failure",
			Model: callbackModel{
				generate: func(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
					modelCalled = true
					return agent.ModelResponse{}, nil
				},
			},
		},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithWorkingMemory(failingWorkingMemoryFactory{err: errors.New("boom")}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected run error")
	}
	var agentErr *agent.Error
	if !errors.As(err, &agentErr) || agentErr.Kind != agent.ErrorKindInternal {
		t.Fatalf("error = %v want internal agent error", err)
	}
	if modelCalled {
		t.Fatal("model was called after working memory failure")
	}
}

func TestStreamAbortDoesNotPersistMemory(t *testing.T) {
	t.Parallel()

	store := &countingMemoryStore{}
	model := callbackModel{
		generate: func(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
			<-ctx.Done()
			return agent.ModelResponse{}, ctx.Err()
		},
	}

	ag, err := agent.New(
		agent.Config{Name: "stream-abort-memory", Model: model},
		agent.WithExecutionEngine(NewEngine()),
		agent.WithMemory(store),
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

	started, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv() start error = %v", err)
	}
	if started.Type != agent.EventAgentStarted {
		t.Fatalf("first event = %q want %q", started.Type, agent.EventAgentStarted)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	for {
		_, err = stream.Recv()
		if err == nil {
			continue
		}
		if errors.Is(err, agent.ErrStreamAborted) || errors.Is(err, io.EOF) {
			break
		}
		t.Fatalf("Recv() terminal error = %v", err)
	}

	if got := store.saveCalls; got != 0 {
		t.Fatalf("save calls = %d want 0", got)
	}
}

type orderedMemoryStore struct {
	load func(context.Context, string) (memory.Snapshot, error)
	save func(context.Context, string, memory.Delta) error
}

func (s orderedMemoryStore) Load(ctx context.Context, sessionID string) (memory.Snapshot, error) {
	if s.load != nil {
		return s.load(ctx, sessionID)
	}
	return memory.Snapshot{}, nil
}

func (s orderedMemoryStore) Save(ctx context.Context, sessionID string, delta memory.Delta) error {
	if s.save != nil {
		return s.save(ctx, sessionID, delta)
	}
	return nil
}

type orderedWorkingMemoryFactory struct {
	onCreate func()
	delegate memory.WorkingMemoryFactory
}

func (f orderedWorkingMemoryFactory) NewRunState(ctx context.Context, agentID, runID string) (memory.WorkingSet, error) {
	if f.onCreate != nil {
		f.onCreate()
	}
	return f.delegate.NewRunState(ctx, agentID, runID)
}

type failingWorkingMemoryFactory struct {
	err error
}

func (f failingWorkingMemoryFactory) NewRunState(context.Context, string, string) (memory.WorkingSet, error) {
	return nil, f.err
}

type orderedInputGuardrail func()

func (g orderedInputGuardrail) CheckInput(context.Context, guardrail.InputRequest) (guardrail.Decision, error) {
	g()
	return guardrail.Decision{Name: "input", Action: guardrail.ActionAllow}, nil
}

type orderedOutputGuardrail func()

func (g orderedOutputGuardrail) CheckOutput(context.Context, guardrail.OutputRequest) (guardrail.Decision, error) {
	g()
	return guardrail.Decision{
		Name:   "output",
		Action: guardrail.ActionTransform,
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: "safe answer",
		},
	}, nil
}

type agentHookFunc func(context.Context, agent.Event)

func (f agentHookFunc) OnEvent(ctx context.Context, event agent.Event) {
	f(ctx, event)
}

type countingMemoryStore struct {
	saveCalls int
}

func (*countingMemoryStore) Load(context.Context, string) (memory.Snapshot, error) {
	return memory.Snapshot{}, nil
}

func (s *countingMemoryStore) Save(context.Context, string, memory.Delta) error {
	s.saveCalls++
	return nil
}

type callbackModel struct {
	generate func(context.Context, agent.ModelRequest) (agent.ModelResponse, error)
}

func (m callbackModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	return m.generate(ctx, req)
}

func (callbackModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}
