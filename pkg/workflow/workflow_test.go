package workflow_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/luanlima/gaal-lib/pkg/workflow"
)

func TestBuilderRejectsMissingNameAndSteps(t *testing.T) {
	t.Parallel()

	if _, err := workflow.NewBuilder("").Build(); !errors.Is(err, workflow.ErrInvalidConfig) {
		t.Fatalf("Build() error = %v want invalid config", err)
	}

	if _, err := workflow.NewBuilder("empty").Build(); !errors.Is(err, workflow.ErrInvalidConfig) {
		t.Fatalf("Build() error = %v want invalid config", err)
	}
}

func TestBuilderRejectsDuplicateStepNames(t *testing.T) {
	t.Parallel()

	builder := workflow.NewBuilder("dup").
		Step(workflow.Action("same", func(context.Context, workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{}, nil
		})).
		Step(workflow.Action("same", func(context.Context, workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{}, nil
		}))

	if _, err := builder.Build(); !errors.Is(err, workflow.ErrInvalidConfig) {
		t.Fatalf("Build() error = %v want invalid config", err)
	}
}

func TestRunExecutesSequentialStepsWithSharedState(t *testing.T) {
	t.Parallel()

	wf := mustBuildWorkflow(t, workflow.NewBuilder("greet").
		Step(workflow.Action("collect", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			stepCtx.State.Set("name", "Ada")
			return workflow.StepResult{Output: map[string]any{"name": "Ada"}}, nil
		})).
		Step(workflow.Action("greet", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			gotInput, ok := stepCtx.Input["name"]
			if !ok || gotInput != "Ada" {
				t.Fatalf("step input = %v ok=%v want Ada/true", gotInput, ok)
			}
			name, ok := stepCtx.State.Get("name")
			if !ok || name != "Ada" {
				t.Fatalf("state name = %v ok=%v want Ada/true", name, ok)
			}
			return workflow.StepResult{Output: map[string]any{"greeting": "hello, Ada"}}, nil
		})))

	resp, err := wf.Run(context.Background(), workflow.Request{
		Input: map[string]any{"seed": "ignored"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if resp.Status != workflow.StatusCompleted {
		t.Fatalf("Status = %q want %q", resp.Status, workflow.StatusCompleted)
	}
	if got := resp.Output["greeting"]; got != "hello, Ada" {
		t.Fatalf("Output[greeting] = %v want %q", got, "hello, Ada")
	}
	if got := resp.State["name"]; got != "Ada" {
		t.Fatalf("State[name] = %v want %q", got, "Ada")
	}
}

func TestBranchRoutesToNamedStep(t *testing.T) {
	t.Parallel()

	wf := mustBuildWorkflow(t, workflow.NewBuilder("branch").
		Step(workflow.Action("prepare", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			stepCtx.State.Set("path", "yes")
			return workflow.StepResult{}, nil
		})).
		Step(workflow.Branch("decide", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.Decision, error) {
			return workflow.Decision{Step: "yes"}, nil
		})).
		Step(workflow.Action("yes", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{
				Output: map[string]any{"branch": "yes"},
				Next:   workflow.Next{End: true},
			}, nil
		})).
		Step(workflow.Action("no", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			t.Fatal("unexpected branch execution")
			return workflow.StepResult{}, nil
		})))

	resp, err := wf.Run(context.Background(), workflow.Request{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := resp.Output["branch"]; got != "yes" {
		t.Fatalf("Output[branch] = %v want yes", got)
	}
}

func TestBranchSuspendReturnsCheckpoint(t *testing.T) {
	t.Parallel()

	wf := mustBuildWorkflow(t, workflow.NewBuilder("suspend").
		Step(workflow.Action("prepare", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			stepCtx.State.Set("phase", "waiting")
			return workflow.StepResult{}, nil
		})).
		Step(workflow.Branch("pause", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.Decision, error) {
			return workflow.Decision{Suspend: true}, nil
		})).
		Step(workflow.Action("never", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			t.Fatal("unexpected step after suspension")
			return workflow.StepResult{}, nil
		})))

	resp, err := wf.Run(context.Background(), workflow.Request{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if resp.Status != workflow.StatusSuspended {
		t.Fatalf("Status = %q want %q", resp.Status, workflow.StatusSuspended)
	}
	if resp.Checkpoint == nil {
		t.Fatal("expected checkpoint")
	}
	if resp.Checkpoint.StepName != "pause" {
		t.Fatalf("Checkpoint.StepName = %q want %q", resp.Checkpoint.StepName, "pause")
	}
	if got := resp.Checkpoint.State["phase"]; got != "waiting" {
		t.Fatalf("Checkpoint.State[phase] = %v want waiting", got)
	}
}

func TestWorkflowRetryAppliesWhenStepHasNoOverride(t *testing.T) {
	t.Parallel()

	var attempts int
	wf := mustBuildWorkflow(t, workflow.NewBuilder("retry-workflow").
		WithRetry(workflow.FixedRetryPolicy{MaxRetries: 1}).
		Step(workflow.Action("unstable", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			attempts++
			if attempts == 1 {
				return workflow.StepResult{}, errors.New("boom")
			}
			return workflow.StepResult{Output: map[string]any{"attempts": attempts}}, nil
		})))

	resp, err := wf.Run(context.Background(), workflow.Request{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d want 2", attempts)
	}
	if got := resp.Output["attempts"]; got != 2 {
		t.Fatalf("Output[attempts] = %v want 2", got)
	}
}

func TestStepRetryOverridesWorkflowRetry(t *testing.T) {
	t.Parallel()

	var attempts int
	wf := mustBuildWorkflow(t, workflow.NewBuilder("retry-step").
		WithRetry(workflow.FixedRetryPolicy{MaxRetries: 1}).
		Step(workflow.Action(
			"unstable",
			func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
				attempts++
				if attempts < 3 {
					return workflow.StepResult{}, errors.New("boom")
				}
				return workflow.StepResult{Output: map[string]any{"attempts": attempts}}, nil
			},
			workflow.WithStepRetry(workflow.FixedRetryPolicy{MaxRetries: 2}),
		)))

	resp, err := wf.Run(context.Background(), workflow.Request{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d want 3", attempts)
	}
	if got := resp.Output["attempts"]; got != 3 {
		t.Fatalf("Output[attempts] = %v want 3", got)
	}
}

func TestHooksObserveLifecycleOrder(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var events []workflow.EventType

	hook := workflow.LifecycleHooks{
		OnStart: func(ctx context.Context, event workflow.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event.Type)
		},
		OnStepStart: func(ctx context.Context, event workflow.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event.Type)
		},
		OnStepEnd: func(ctx context.Context, event workflow.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event.Type)
		},
		OnFinish: func(ctx context.Context, event workflow.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event.Type)
		},
		OnEnd: func(ctx context.Context, event workflow.Event) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, event.Type)
		},
	}

	wf := mustBuildWorkflow(t, workflow.NewBuilder("hooks").
		WithHooks(hook).
		Step(workflow.Action("one", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{Output: map[string]any{"ok": true}}, nil
		})))

	if _, err := wf.Run(context.Background(), workflow.Request{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []workflow.EventType{
		workflow.EventWorkflowStarted,
		workflow.EventStepStarted,
		workflow.EventStepEnded,
		workflow.EventWorkflowFinished,
		workflow.EventWorkflowEnded,
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v want %v", events, want)
	}
}

func TestInMemoryHistoryRecordsEntries(t *testing.T) {
	t.Parallel()

	history := &workflow.InMemoryHistory{}
	wf := mustBuildWorkflow(t, workflow.NewBuilder("history").
		WithHistory(history).
		Step(workflow.Action("one", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{Output: map[string]any{"ok": true}}, nil
		})))

	if _, err := wf.Run(context.Background(), workflow.Request{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	gotKinds := make([]string, 0, len(history.Entries()))
	for _, entry := range history.Entries() {
		gotKinds = append(gotKinds, entry.Kind)
	}
	wantKinds := []string{
		"workflow.started",
		"workflow.step_started",
		"workflow.step_ended",
		"workflow.finished",
		"workflow.ended",
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("history kinds = %v want %v", gotKinds, wantKinds)
	}
}

func TestHistoryFailureInvalidatesSuccess(t *testing.T) {
	t.Parallel()

	wf := mustBuildWorkflow(t, workflow.NewBuilder("history-fail").
		WithHistory(failingHistorySink{failOn: "workflow.finished"}).
		Step(workflow.Action("one", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{Output: map[string]any{"ok": true}}, nil
		})))

	_, err := wf.Run(context.Background(), workflow.Request{})
	if !errors.Is(err, workflow.ErrHistoryWrite) {
		t.Fatalf("Run() error = %v want history write", err)
	}
}

func TestRunCancellationStopsWorkflow(t *testing.T) {
	t.Parallel()

	wf := mustBuildWorkflow(t, workflow.NewBuilder("cancel").
		WithRetry(workflow.FixedRetryPolicy{MaxRetries: 1, Delay: 50 * time.Millisecond}).
		Step(workflow.Action("block", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			return workflow.StepResult{}, errors.New("boom")
		})))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := wf.Run(ctx, workflow.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v want context canceled", err)
	}
}

func mustBuildWorkflow(t *testing.T, builder *workflow.Builder) *workflow.Chain {
	t.Helper()

	wf, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return wf
}

type failingHistorySink struct {
	failOn string
}

func (s failingHistorySink) Append(ctx context.Context, entry workflow.HistoryEntry) error {
	if entry.Kind == s.failOn {
		return errors.New("history down")
	}
	return nil
}
