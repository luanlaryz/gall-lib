package agent

import (
	"errors"
	"fmt"
)

// ErrorKind classifies observable agent failures.
type ErrorKind string

const (
	// ErrorKindInvalidConfig reports invalid agent construction options.
	ErrorKindInvalidConfig ErrorKind = "invalid_config"
	// ErrorKindInvalidRequest reports invalid request input.
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	// ErrorKindNoEngine reports a missing execution engine.
	ErrorKindNoEngine ErrorKind = "no_engine"
	// ErrorKindGuardrailBlocked reports a guardrail block decision.
	ErrorKindGuardrailBlocked ErrorKind = "guardrail_blocked"
	// ErrorKindTool reports tool lookup or execution failures.
	ErrorKindTool ErrorKind = "tool"
	// ErrorKindMemory reports memory load/save failures.
	ErrorKindMemory ErrorKind = "memory"
	// ErrorKindModel reports model execution failures.
	ErrorKindModel ErrorKind = "model"
	// ErrorKindMaxSteps reports that the run exhausted its step budget.
	ErrorKindMaxSteps ErrorKind = "max_steps"
	// ErrorKindCanceled reports cancellation by context or stream abort.
	ErrorKindCanceled ErrorKind = "canceled"
	// ErrorKindStreamAborted reports a local consumer stream abort.
	ErrorKindStreamAborted ErrorKind = "stream_aborted"
	// ErrorKindInternal reports unexpected internal failures.
	ErrorKindInternal ErrorKind = "internal"
)

var (
	// ErrInvalidConfig is the sentinel used for invalid agent construction.
	ErrInvalidConfig = errors.New("agent: invalid config")
	// ErrInvalidRequest is the sentinel used for invalid requests.
	ErrInvalidRequest = errors.New("agent: invalid request")
	// ErrNoExecutionEngine is the sentinel used when an agent has no execution engine.
	ErrNoExecutionEngine = errors.New("agent: no execution engine")
	// ErrGuardrailBlocked is the sentinel used when a guardrail blocks a run.
	ErrGuardrailBlocked = errors.New("agent: guardrail blocked")
	// ErrMaxStepsExceeded is the sentinel used when a run exhausts its step budget.
	ErrMaxStepsExceeded = errors.New("agent: max steps exceeded")
	// ErrStreamAborted is the sentinel used when a stream is locally aborted.
	ErrStreamAborted = errors.New("agent: stream aborted")
)

// Error is the structured public error returned by agent execution APIs.
type Error struct {
	Kind    ErrorKind
	Op      string
	AgentID string
	RunID   string
	Cause   error
}

// Error returns a short, actionable error message.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	base := fmt.Sprintf("agent %s", e.Op)
	if e.AgentID != "" {
		base = fmt.Sprintf("%s agent=%s", base, e.AgentID)
	}
	if e.RunID != "" {
		base = fmt.Sprintf("%s run=%s", base, e.RunID)
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", base, e.Cause)
	}
	return base
}

// Unwrap exposes the wrapped cause for errors.Is and errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is matches public sentinels by error kind.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrInvalidConfig:
		return e.Kind == ErrorKindInvalidConfig
	case ErrInvalidRequest:
		return e.Kind == ErrorKindInvalidRequest
	case ErrNoExecutionEngine:
		return e.Kind == ErrorKindNoEngine
	case ErrGuardrailBlocked:
		return e.Kind == ErrorKindGuardrailBlocked
	case ErrMaxStepsExceeded:
		return e.Kind == ErrorKindMaxSteps
	case ErrStreamAborted:
		return e.Kind == ErrorKindStreamAborted
	default:
		return false
	}
}
