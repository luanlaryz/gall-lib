package agent

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/logger"
)

type loggingHook struct {
	logger logger.Logger
}

// NewLoggingHook returns an event hook that records observable agent events
// through the shared public logger facade.
func NewLoggingHook(log logger.Logger) Hook {
	if log == nil {
		log = logger.Nop()
	}
	return loggingHook{logger: log}
}

func (h loggingHook) OnEvent(ctx context.Context, event Event) {
	args := append(agentEventArgs(event), metadataArgs(event.Metadata)...)

	switch event.Type {
	case EventAgentDelta, EventToolCall, EventToolResult:
		h.logger.DebugContext(ctx, string(event.Type), args...)
	case EventGuardrail:
		switch {
		case event.Guardrail == nil:
			h.logger.DebugContext(ctx, string(event.Type), args...)
		case event.Guardrail.Decision.Action == guardrail.ActionAllow:
			h.logger.DebugContext(ctx, string(event.Type), args...)
		case event.Guardrail.Decision.Action == guardrail.ActionAbort || event.Guardrail.Decision.Action == guardrail.ActionBlock:
			h.logger.WarnContext(ctx, string(event.Type), args...)
		default:
			h.logger.InfoContext(ctx, string(event.Type), args...)
		}
	case EventAgentFailed:
		h.logger.ErrorContext(ctx, string(event.Type), args...)
	case EventAgentCanceled:
		h.logger.InfoContext(ctx, string(event.Type), args...)
	default:
		h.logger.InfoContext(ctx, string(event.Type), args...)
	}
}

func agentEventArgs(event Event) []any {
	args := []any{
		"component", "agent",
		"event_type", string(event.Type),
		"agent_id", event.AgentID,
		"run_id", event.RunID,
		"session_id", event.SessionID,
		"sequence", event.Sequence,
	}

	if event.Delta != nil {
		args = append(args, "delta_run_id", event.Delta.RunID)
	}
	if event.ToolCall != nil {
		args = append(args,
			"tool_name", event.ToolCall.Call.Name,
			"tool_call_id", event.ToolCall.Call.ID,
			"tool_status", string(event.ToolCall.Status),
		)
	}
	if event.Guardrail != nil {
		args = append(args,
			"phase", string(event.Guardrail.Decision.Phase),
			"guardrail_name", event.Guardrail.Decision.Name,
			"action", string(event.Guardrail.Decision.Action),
		)
		if event.Guardrail.Decision.Reason != "" {
			args = append(args, "reason", event.Guardrail.Decision.Reason)
		}
		args = append(args, metadataArgs(event.Guardrail.Decision.Metadata)...)
	}
	if event.Response != nil {
		args = append(args, "response_run_id", event.Response.RunID)
	}
	if event.Err != nil {
		args = append(args, "error", event.Err)
	}
	return args
}

func metadataArgs(md map[string]string) []any {
	if len(md) == 0 {
		return nil
	}
	args := make([]any, 0, len(md)*2)
	for key, value := range md {
		args = append(args, key, value)
	}
	return args
}
