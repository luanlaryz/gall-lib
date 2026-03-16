package workflow_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

func TestWorkflowEventsCarryStandardMetadata(t *testing.T) {
	t.Parallel()

	var started workflow.Event
	wf := mustBuildWorkflow(t, workflow.NewBuilder("metadata").
		WithMetadata(types.Metadata{"scope": "workflow"}).
		WithHooks(workflow.HookFunc(func(ctx context.Context, event workflow.Event) {
			if event.Type == workflow.EventWorkflowStarted {
				started = event
			}
		})).
		Step(workflow.Action("one", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{Output: map[string]any{"ok": true}}, nil
		})))

	resp, err := wf.Run(context.Background(), workflow.Request{
		Metadata: types.Metadata{"trace_id": "trace-1"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := started.Metadata["component"]; got != "workflow" {
		t.Fatalf("Metadata[component] = %q want workflow", got)
	}
	if got := started.Metadata["event_type"]; got != string(workflow.EventWorkflowStarted) {
		t.Fatalf("Metadata[event_type] = %q want %q", got, workflow.EventWorkflowStarted)
	}
	if got := started.Metadata["workflow_id"]; got != wf.ID() {
		t.Fatalf("Metadata[workflow_id] = %q want %q", got, wf.ID())
	}
	if got := started.Metadata["run_id"]; got != resp.RunID {
		t.Fatalf("Metadata[run_id] = %q want %q", got, resp.RunID)
	}
	if got := started.Metadata["trace_id"]; got != "trace-1" {
		t.Fatalf("Metadata[trace_id] = %q want trace-1", got)
	}
}

func TestWorkflowLogsRecoveredHookPanicFromContextLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	wf := mustBuildWorkflow(t, workflow.NewBuilder("panic-workflow").
		WithHooks(workflow.HookFunc(func(ctx context.Context, event workflow.Event) {
			if event.Type == workflow.EventWorkflowStarted {
				panic("boom")
			}
		})).
		Step(workflow.Action("one", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{Output: map[string]any{"ok": true}}, nil
		})))

	ctx := logger.WithContext(context.Background(), logger.NewSimple(logger.SimpleOptions{
		Writer: &buf,
		Level:  logger.LevelDebug,
	}))
	if _, err := wf.Run(ctx, workflow.Request{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "workflow.hook_panic") {
		t.Fatalf("output = %q want workflow.hook_panic", output)
	}
	if !strings.Contains(output, "event_type=workflow.started") {
		t.Fatalf("output = %q want workflow.started", output)
	}
}
