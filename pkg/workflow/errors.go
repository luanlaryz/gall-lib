package workflow

import (
	"errors"
	"fmt"
)

// ErrorKind classifies observable workflow failures.
type ErrorKind string

const (
	// ErrorKindInvalidConfig reports invalid workflow construction options.
	ErrorKindInvalidConfig ErrorKind = "invalid_config"
	// ErrorKindInvalidRequest reports invalid workflow requests.
	ErrorKindInvalidRequest ErrorKind = "invalid_request"
	// ErrorKindTransition reports invalid next-step transitions.
	ErrorKindTransition ErrorKind = "transition"
	// ErrorKindStep reports step execution failures.
	ErrorKindStep ErrorKind = "step"
	// ErrorKindHistory reports history persistence failures.
	ErrorKindHistory ErrorKind = "history"
	// ErrorKindCanceled reports context cancellation.
	ErrorKindCanceled ErrorKind = "canceled"
	// ErrorKindInternal reports unexpected internal failures.
	ErrorKindInternal ErrorKind = "internal"
)

var (
	// ErrInvalidConfig is the sentinel used for invalid workflow construction.
	ErrInvalidConfig = errors.New("workflow: invalid config")
	// ErrInvalidRequest is the sentinel used for invalid workflow requests.
	ErrInvalidRequest = errors.New("workflow: invalid request")
	// ErrInvalidTransition is the sentinel used for invalid next-step transitions.
	ErrInvalidTransition = errors.New("workflow: invalid transition")
	// ErrHistoryWrite is the sentinel used when writing history fails.
	ErrHistoryWrite = errors.New("workflow: history write failed")
)

// Error is the structured public error returned by workflow execution APIs.
type Error struct {
	Kind       ErrorKind
	Op         string
	WorkflowID string
	RunID      string
	StepName   string
	Cause      error
}

// Error returns a short, actionable error message.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	base := fmt.Sprintf("workflow %s", e.Op)
	if e.WorkflowID != "" {
		base = fmt.Sprintf("%s workflow=%s", base, e.WorkflowID)
	}
	if e.RunID != "" {
		base = fmt.Sprintf("%s run=%s", base, e.RunID)
	}
	if e.StepName != "" {
		base = fmt.Sprintf("%s step=%s", base, e.StepName)
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
	case ErrInvalidTransition:
		return e.Kind == ErrorKindTransition
	case ErrHistoryWrite:
		return e.Kind == ErrorKindHistory
	default:
		return false
	}
}

func newError(kind ErrorKind, op, workflowID, runID, stepName string, cause error) *Error {
	return &Error{
		Kind:       kind,
		Op:         op,
		WorkflowID: workflowID,
		RunID:      runID,
		StepName:   stepName,
		Cause:      cause,
	}
}
