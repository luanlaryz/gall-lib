package workflow

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/logger"
)

type loggingHook struct {
	logger logger.Logger
}

// NewLoggingHook returns a workflow hook that records lifecycle events through
// the shared public logger facade.
func NewLoggingHook(log logger.Logger) Hook {
	if log == nil {
		log = logger.Nop()
	}
	return loggingHook{logger: log}
}

func (h loggingHook) OnEvent(ctx context.Context, event Event) {
	args := append(workflowEventArgs(event), workflowMetadataArgs(event.Metadata)...)

	switch event.Type {
	case EventStepStarted:
		h.logger.DebugContext(ctx, string(event.Type), args...)
	case EventWorkflowError:
		h.logger.ErrorContext(ctx, string(event.Type), args...)
	case EventWorkflowEnded:
		if event.Status == StatusCanceled {
			h.logger.InfoContext(ctx, string(event.Type), args...)
			return
		}
		if event.Status == StatusFailed {
			h.logger.ErrorContext(ctx, string(event.Type), args...)
			return
		}
		h.logger.InfoContext(ctx, string(event.Type), args...)
	default:
		h.logger.InfoContext(ctx, string(event.Type), args...)
	}
}

func workflowEventArgs(event Event) []any {
	args := []any{
		"component", "workflow",
		"event_type", string(event.Type),
		"workflow_id", event.WorkflowID,
		"workflow_name", event.WorkflowName,
		"run_id", event.RunID,
		"session_id", event.SessionID,
		"step_name", event.StepName,
		"attempt", event.Attempt,
		"status", string(event.Status),
	}
	if event.Err != nil {
		args = append(args, "error", event.Err)
	}
	return args
}

func workflowMetadataArgs(md map[string]string) []any {
	if len(md) == 0 {
		return nil
	}
	args := make([]any, 0, len(md)*2)
	for key, value := range md {
		args = append(args, key, value)
	}
	return args
}
