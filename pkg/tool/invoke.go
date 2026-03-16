package tool

import (
	"context"
	"errors"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Invoke validates input and output contracts around a tool call.
func Invoke(ctx context.Context, target Tool, call Call) (Result, error) {
	descriptor, err := validateToolDefinition(target, toolkitOrigin{})
	if err != nil {
		return Result{}, err
	}

	preparedCall := Call{
		ID:        call.ID,
		ToolName:  descriptor.Name,
		RunID:     call.RunID,
		SessionID: call.SessionID,
		AgentID:   call.AgentID,
		Input:     cloneMap(call.Input),
		Metadata:  types.CloneMetadata(call.Metadata),
	}
	if preparedCall.ID == "" {
		return Result{}, newError(
			ErrorKindInvalidInput,
			"invoke",
			descriptor.Name,
			descriptor.Toolkit,
			"",
			fmt.Errorf("%w: call id is required", ErrInvalidInput),
		)
	}
	if err := validateValueAgainstSchema(preparedCall.Input, descriptor.InputSchema, "input"); err != nil {
		return Result{}, newError(ErrorKindInvalidInput, "invoke", descriptor.Name, descriptor.Toolkit, preparedCall.ID, fmt.Errorf("%w: %v", ErrInvalidInput, err))
	}
	if err := checkContext(ctx, descriptor, preparedCall.ID); err != nil {
		return Result{}, err
	}

	result, err := target.Call(ctx, preparedCall)
	if err != nil {
		if ctxErr := classifyContextError(err, descriptor, preparedCall.ID); ctxErr != nil {
			return Result{}, ctxErr
		}
		return Result{}, newError(ErrorKindExecution, "invoke", descriptor.Name, descriptor.Toolkit, preparedCall.ID, err)
	}
	if err := validateValueAgainstSchema(result.Value, descriptor.OutputSchema, "output"); err != nil {
		return Result{}, newError(ErrorKindInvalidOutput, "invoke", descriptor.Name, descriptor.Toolkit, preparedCall.ID, fmt.Errorf("%w: %v", ErrInvalidOutput, err))
	}

	return Result{
		Value:    cloneValue(result.Value),
		Content:  result.Content,
		Metadata: types.CloneMetadata(result.Metadata),
	}, nil
}

func checkContext(ctx context.Context, descriptor Descriptor, callID string) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return newError(ErrorKindCanceled, "invoke", descriptor.Name, descriptor.Toolkit, callID, err)
	}
	return nil
}

func classifyContextError(err error, descriptor Descriptor, callID string) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return newError(ErrorKindCanceled, "invoke", descriptor.Name, descriptor.Toolkit, callID, err)
	default:
		return nil
	}
}
