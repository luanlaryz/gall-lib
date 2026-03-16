package tool

import (
	"errors"
	"fmt"
)

// ErrorKind classifies observable tool failures.
type ErrorKind string

const (
	ErrorKindInvalidTool    ErrorKind = "invalid_tool"
	ErrorKindInvalidToolkit ErrorKind = "invalid_toolkit"
	ErrorKindInvalidSchema  ErrorKind = "invalid_schema"
	ErrorKindInvalidInput   ErrorKind = "invalid_input"
	ErrorKindInvalidOutput  ErrorKind = "invalid_output"
	ErrorKindNameConflict   ErrorKind = "name_conflict"
	ErrorKindNotFound       ErrorKind = "not_found"
	ErrorKindExecution      ErrorKind = "execution"
	ErrorKindCanceled       ErrorKind = "canceled"
)

var (
	// ErrInvalidTool marks invalid tool definitions.
	ErrInvalidTool = errors.New("tool: invalid tool")
	// ErrInvalidToolkit marks invalid toolkit definitions.
	ErrInvalidToolkit = errors.New("tool: invalid toolkit")
	// ErrInvalidSchema marks invalid input or output schemas.
	ErrInvalidSchema = errors.New("tool: invalid schema")
	// ErrInvalidInput marks invalid tool input.
	ErrInvalidInput = errors.New("tool: invalid input")
	// ErrInvalidOutput marks invalid tool output.
	ErrInvalidOutput = errors.New("tool: invalid output")
	// ErrNameConflict marks tool or toolkit name conflicts.
	ErrNameConflict = errors.New("tool: name conflict")
	// ErrToolNotFound marks missing tool resolution.
	ErrToolNotFound = errors.New("tool: not found")
)

// Error is the structured public error returned by tool APIs.
type Error struct {
	Kind        ErrorKind
	Op          string
	ToolName    string
	ToolkitName string
	CallID      string
	Cause       error
}

// Error returns a short, explicit error message.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	base := "tool"
	if e.Op != "" {
		base = fmt.Sprintf("%s %s", base, e.Op)
	}
	if e.ToolkitName != "" {
		base = fmt.Sprintf("%s toolkit=%s", base, e.ToolkitName)
	}
	if e.ToolName != "" {
		base = fmt.Sprintf("%s tool=%s", base, e.ToolName)
	}
	if e.CallID != "" {
		base = fmt.Sprintf("%s call=%s", base, e.CallID)
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

// Is matches tool sentinels by error kind.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrInvalidTool:
		return e.Kind == ErrorKindInvalidTool
	case ErrInvalidToolkit:
		return e.Kind == ErrorKindInvalidToolkit
	case ErrInvalidSchema:
		return e.Kind == ErrorKindInvalidSchema
	case ErrInvalidInput:
		return e.Kind == ErrorKindInvalidInput
	case ErrInvalidOutput:
		return e.Kind == ErrorKindInvalidOutput
	case ErrNameConflict:
		return e.Kind == ErrorKindNameConflict
	case ErrToolNotFound:
		return e.Kind == ErrorKindNotFound
	default:
		return false
	}
}

func newError(kind ErrorKind, op, toolName, toolkitName, callID string, cause error) *Error {
	return &Error{
		Kind:        kind,
		Op:          op,
		ToolName:    toolName,
		ToolkitName: toolkitName,
		CallID:      callID,
		Cause:       cause,
	}
}
