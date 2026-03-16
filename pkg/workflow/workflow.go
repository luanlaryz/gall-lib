// Package workflow defines the public workflow contracts and the first
// sequential workflow implementation used by gaal-lib.
package workflow

import (
	"context"
	"time"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Request is the public input envelope for workflow execution.
type Request struct {
	RunID     string
	SessionID string
	Input     map[string]any
	State     StateSnapshot
	Metadata  types.Metadata
}

// Response is the public terminal envelope returned by a workflow run.
type Response struct {
	RunID        string
	WorkflowID   string
	WorkflowName string
	SessionID    string
	Status       Status
	CurrentStep  string
	Output       map[string]any
	State        StateSnapshot
	Checkpoint   *Checkpoint
	Metadata     types.Metadata
}

// Workflow is the minimal runnable workflow contract consumed by app.
type Workflow interface {
	Name() string
	Run(ctx context.Context, req Request) (Response, error)
}

// Runnable exposes the richer, immutable workflow definition used by callers
// that need more than the minimal app-facing interface.
type Runnable interface {
	Workflow
	ID() string
	Descriptor() Descriptor
	Definition() Definition
}

// Status identifies the terminal state of a workflow run.
type Status string

const (
	// StatusCompleted reports a successful run completion.
	StatusCompleted Status = "completed"
	// StatusFailed reports a terminal workflow failure.
	StatusFailed Status = "failed"
	// StatusCanceled reports cooperative context cancellation.
	StatusCanceled Status = "canceled"
	// StatusSuspended reports a deliberate suspension with checkpoint data.
	StatusSuspended Status = "suspended"
)

// Descriptor identifies a workflow in registries and runtime views.
type Descriptor struct {
	Name string
	ID   string
}

// Definition is the immutable public snapshot of a runnable workflow.
type Definition struct {
	Descriptor Descriptor
	Steps      []StepDescriptor
	Hooks      []Hook
	Retry      RetryPolicy
	History    HistorySink
	Metadata   types.Metadata
}

// StepDescriptor is the public summary of a registered step.
type StepDescriptor struct {
	Name string
	Kind StepKind
}

// Checkpoint is the minimal public state persisted for future resume support.
type Checkpoint struct {
	StepName string
	State    StateSnapshot
	Time     time.Time
	Metadata types.Metadata
}

// Event is the public workflow lifecycle envelope observed by hooks.
type Event struct {
	Type         EventType
	WorkflowID   string
	WorkflowName string
	RunID        string
	SessionID    string
	StepName     string
	Attempt      int
	Status       Status
	Output       map[string]any
	State        StateSnapshot
	Err          error
	Time         time.Time
	Metadata     types.Metadata
}

// EventType identifies the public workflow lifecycle event.
type EventType string

const (
	// EventWorkflowStarted maps to onStart.
	EventWorkflowStarted EventType = "workflow.started"
	// EventStepStarted maps to onStepStart.
	EventStepStarted EventType = "workflow.step_started"
	// EventStepEnded maps to onStepEnd.
	EventStepEnded EventType = "workflow.step_ended"
	// EventWorkflowError maps to onError.
	EventWorkflowError EventType = "workflow.error"
	// EventWorkflowFinished maps to onFinish.
	EventWorkflowFinished EventType = "workflow.finished"
	// EventWorkflowEnded maps to onEnd.
	EventWorkflowEnded EventType = "workflow.ended"
)

// Hook observes workflow events.
type Hook interface {
	OnEvent(ctx context.Context, event Event)
}

// HookFunc adapts a function to the Hook interface.
type HookFunc func(ctx context.Context, event Event)

// OnEvent dispatches the hook function.
func (f HookFunc) OnEvent(ctx context.Context, event Event) {
	f(ctx, event)
}

// LifecycleHooks maps public events to the named lifecycle callbacks described
// by the workflow spec.
type LifecycleHooks struct {
	OnStart     func(ctx context.Context, event Event)
	OnStepStart func(ctx context.Context, event Event)
	OnStepEnd   func(ctx context.Context, event Event)
	OnError     func(ctx context.Context, event Event)
	OnFinish    func(ctx context.Context, event Event)
	OnEnd       func(ctx context.Context, event Event)
}

// OnEvent dispatches the event to the matching lifecycle callback.
func (h LifecycleHooks) OnEvent(ctx context.Context, event Event) {
	switch event.Type {
	case EventWorkflowStarted:
		if h.OnStart != nil {
			h.OnStart(ctx, event)
		}
	case EventStepStarted:
		if h.OnStepStart != nil {
			h.OnStepStart(ctx, event)
		}
	case EventStepEnded:
		if h.OnStepEnd != nil {
			h.OnStepEnd(ctx, event)
		}
	case EventWorkflowError:
		if h.OnError != nil {
			h.OnError(ctx, event)
		}
	case EventWorkflowFinished:
		if h.OnFinish != nil {
			h.OnFinish(ctx, event)
		}
	case EventWorkflowEnded:
		if h.OnEnd != nil {
			h.OnEnd(ctx, event)
		}
	}
}

// HistoryEntry is the public history payload recorded for a workflow.
type HistoryEntry struct {
	Kind         string
	WorkflowID   string
	WorkflowName string
	RunID        string
	SessionID    string
	StepName     string
	Attempt      int
	Status       Status
	Time         time.Time
	Output       map[string]any
	Checkpoint   *Checkpoint
	Metadata     types.Metadata
}

// HistorySink persists workflow history entries.
type HistorySink interface {
	Append(ctx context.Context, entry HistoryEntry) error
}

// RetryPolicy decides whether a failed workflow step should retry.
type RetryPolicy interface {
	Next(attempt int, err error) (time.Duration, bool)
}

// RetryFunc adapts a function to the RetryPolicy interface.
type RetryFunc func(attempt int, err error) (time.Duration, bool)

// Next dispatches the retry function.
func (f RetryFunc) Next(attempt int, err error) (time.Duration, bool) {
	return f(attempt, err)
}
