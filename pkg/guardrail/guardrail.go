// Package guardrail defines the public contracts for input, stream, and output checks.
package guardrail

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Phase identifies the point in the agent lifecycle where a guardrail runs.
type Phase string

const (
	// PhaseInput identifies guardrails that run before memory and model execution.
	PhaseInput Phase = "input"
	// PhaseStream identifies guardrails that run for each candidate assistant delta.
	PhaseStream Phase = "stream"
	// PhaseOutput identifies guardrails that run on the final assistant message.
	PhaseOutput Phase = "output"
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
	// ActionDrop suppresses a single stream chunk without aborting the run.
	ActionDrop Action = "drop"
	// ActionAbort aborts the run while processing a stream chunk.
	ActionAbort Action = "abort"
)

// Context is the immutable public context shared by all guardrail phases.
type Context struct {
	Phase     Phase
	AgentID   string
	AgentName string
	RunID     string
	SessionID string
	Metadata  types.Metadata
}

// InputRequest is the public input envelope for input guardrails.
type InputRequest struct {
	Context
	Messages []types.Message
}

// StreamRequest is the public input envelope for stream guardrails.
type StreamRequest struct {
	Context
	ChunkIndex      int64
	Delta           types.MessageDelta
	BufferedContent string
}

// OutputRequest is the public input envelope for output guardrails.
type OutputRequest struct {
	Context
	Message types.Message
}

// Decision is the observable result returned by a guardrail.
type Decision struct {
	Name     string
	Action   Action
	Reason   string
	Messages []types.Message
	Delta    *types.MessageDelta
	Message  *types.Message
	Metadata types.Metadata
}

// Input validates or transforms request input before model execution.
type Input interface {
	CheckInput(ctx context.Context, req InputRequest) (Decision, error)
}

// Stream validates or transforms a candidate assistant delta before it is emitted.
type Stream interface {
	CheckStream(ctx context.Context, req StreamRequest) (Decision, error)
}

// Output validates or transforms final output before it is returned.
type Output interface {
	CheckOutput(ctx context.Context, req OutputRequest) (Decision, error)
}
