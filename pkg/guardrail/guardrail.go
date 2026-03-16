// Package guardrail defines the public contracts for input and output checks.
package guardrail

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Action represents the observable outcome of a guardrail decision.
type Action string

const (
	// ActionAllow lets the run continue without changes.
	ActionAllow Action = "allow"
	// ActionBlock aborts the run with a classified error.
	ActionBlock Action = "block"
	// ActionTransform changes the effective input or output observed by the runtime.
	ActionTransform Action = "transform"
)

// Decision is the observable result returned by a guardrail.
type Decision struct {
	Name     string
	Action   Action
	Reason   string
	Messages []types.Message
	Message  *types.Message
	Metadata types.Metadata
}

// InputRequest is the public input envelope for input guardrails.
type InputRequest struct {
	AgentID   string
	AgentName string
	RunID     string
	SessionID string
	Messages  []types.Message
	Metadata  types.Metadata
}

// OutputRequest is the public input envelope for output guardrails.
type OutputRequest struct {
	AgentID   string
	AgentName string
	RunID     string
	SessionID string
	Message   types.Message
	Metadata  types.Metadata
}

// Input validates or transforms request input before model execution.
type Input interface {
	CheckInput(ctx context.Context, req InputRequest) (Decision, error)
}

// Output validates or transforms final output before it is returned.
type Output interface {
	CheckOutput(ctx context.Context, req OutputRequest) (Decision, error)
}
