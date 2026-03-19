package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

const orderWorkflowName = "order-processing"

type orderWorkflowFactory struct{}

func (orderWorkflowFactory) Name() string { return orderWorkflowName }

func (orderWorkflowFactory) Build(_ context.Context, defaults app.WorkflowDefaults) (workflow.Workflow, error) {
	builder := workflow.NewBuilder(orderWorkflowName).WithMetadata(defaults.Metadata)
	if len(defaults.Hooks) > 0 {
		builder = builder.WithHooks(defaults.Hooks...)
	}
	if defaults.History != nil {
		builder = builder.WithHistory(defaults.History)
	}
	if defaults.Retry != nil {
		builder = builder.WithRetry(defaults.Retry)
	}

	return builder.
		Step(workflow.Action("validate_order", validateOrder)).
		Step(workflow.Branch("route_order", routeOrder)).
		Step(workflow.Action("auto_approve", autoApprove)).
		Step(workflow.Action("manual_review", manualReview)).
		Step(workflow.Action("confirm", confirmOrder)).
		Build()
}

func validateOrder(_ context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
	item, _ := stepCtx.Input["item"].(string)
	if item == "" {
		return workflow.StepResult{}, errors.New("item is required")
	}

	amount, err := extractAmount(stepCtx.Input["amount"])
	if err != nil {
		return workflow.StepResult{}, fmt.Errorf("invalid amount: %w", err)
	}

	stepCtx.State.Set("item", item)
	stepCtx.State.Set("amount", amount)

	return workflow.StepResult{
		Output: map[string]any{"item": item, "amount": amount},
	}, nil
}

func routeOrder(_ context.Context, stepCtx workflow.StepContext) (workflow.Decision, error) {
	raw, _ := stepCtx.State.Get("amount")
	amount, _ := raw.(float64)

	if amount > 100 {
		return workflow.Decision{
			Step:   "manual_review",
			Reason: fmt.Sprintf("amount %.2f exceeds auto-approve threshold", amount),
		}, nil
	}

	return workflow.Decision{
		Step:   "auto_approve",
		Reason: fmt.Sprintf("amount %.2f within auto-approve threshold", amount),
	}, nil
}

func autoApprove(_ context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
	stepCtx.State.Set("status", "approved")
	return workflow.StepResult{
		Output: map[string]any{"status": "approved"},
		Next:   workflow.Next{Step: "confirm"},
	}, nil
}

func manualReview(_ context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
	stepCtx.State.Set("status", "pending_review")
	return workflow.StepResult{
		Output: map[string]any{"status": "pending_review"},
		Next:   workflow.Next{Step: "confirm"},
	}, nil
}

func confirmOrder(_ context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
	itemRaw, _ := stepCtx.State.Get("item")
	item, _ := itemRaw.(string)

	amountRaw, _ := stepCtx.State.Get("amount")
	amount, _ := amountRaw.(float64)

	statusRaw, _ := stepCtx.State.Get("status")
	status, _ := statusRaw.(string)

	message := fmt.Sprintf("order for %q (amount: %.2f) — status: %s", item, amount, status)

	return workflow.StepResult{
		Output: map[string]any{
			"message": message,
			"item":    item,
			"amount":  amount,
			"status":  status,
		},
		Next: workflow.Next{End: true},
	}, nil
}

func extractAmount(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
