package agent_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	coreruntime "github.com/luanlima/gaal-lib/internal/runtime"
	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestRunPreservesCorrelationMetadataInResponseAndEvents(t *testing.T) {
	t.Parallel()

	var completed agent.Event
	ag, err := agent.New(
		agent.Config{
			Name: "observable-agent",
			Model: staticModel{response: agent.ModelResponse{
				Message: types.Message{Role: types.RoleAssistant, Content: "ok"},
				Metadata: types.Metadata{
					"trace_id":       "model-trace",
					"span_id":        "model-span",
					"parent_span_id": "model-parent",
					"model_field":    "yes",
				},
			}},
		},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithHooks(agentHookFunc(func(ctx context.Context, event agent.Event) {
			if event.Type == agent.EventAgentCompleted {
				completed = event
			}
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resp, err := ag.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
		Metadata: types.Metadata{
			"trace_id":       "req-trace",
			"span_id":        "req-span",
			"parent_span_id": "req-parent",
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for key, want := range map[string]string{
		"trace_id":       "req-trace",
		"span_id":        "req-span",
		"parent_span_id": "req-parent",
	} {
		if got := resp.Metadata[key]; got != want {
			t.Fatalf("Response.Metadata[%s] = %q want %q", key, got, want)
		}
		if got := completed.Metadata[key]; got != want {
			t.Fatalf("Event.Metadata[%s] = %q want %q", key, got, want)
		}
	}
	if got := completed.Metadata["component"]; got != "agent" {
		t.Fatalf("Event.Metadata[component] = %q want agent", got)
	}
	if got := completed.Metadata["event_type"]; got != string(agent.EventAgentCompleted) {
		t.Fatalf("Event.Metadata[event_type] = %q want %q", got, agent.EventAgentCompleted)
	}
	if got := completed.Metadata["agent_id"]; got != ag.ID() {
		t.Fatalf("Event.Metadata[agent_id] = %q want %q", got, ag.ID())
	}
	if got := completed.Metadata["run_id"]; got != resp.RunID {
		t.Fatalf("Event.Metadata[run_id] = %q want %q", got, resp.RunID)
	}
}

func TestRunLogsRecoveredHookPanicFromContextLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ag, err := agent.New(
		agent.Config{Name: "panic-agent", Model: stubModel{}},
		agent.WithExecutionEngine(coreruntime.NewEngine()),
		agent.WithHooks(agentHookFunc(func(ctx context.Context, event agent.Event) {
			if event.Type == agent.EventAgentStarted {
				panic("boom")
			}
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := logger.WithContext(context.Background(), logger.NewSimple(logger.SimpleOptions{
		Writer: &buf,
		Level:  logger.LevelDebug,
	}))
	if _, err := ag.Run(ctx, agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "agent.hook_panic") {
		t.Fatalf("output = %q want agent.hook_panic", output)
	}
	if !strings.Contains(output, "event_type=agent.started") {
		t.Fatalf("output = %q want agent.started", output)
	}
}

type agentHookFunc func(context.Context, agent.Event)

func (f agentHookFunc) OnEvent(ctx context.Context, event agent.Event) {
	f(ctx, event)
}
