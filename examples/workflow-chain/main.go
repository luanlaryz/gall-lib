package main

import (
	"context"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

func main() {
	ctx := context.Background()
	history := &workflow.InMemoryHistory{}

	instance, err := app.New(
		app.Config{
			Name: "workflow-chain",
			Defaults: app.Defaults{
				Workflow: app.WorkflowDefaults{
					Metadata: types.Metadata{"example": "workflow-chain"},
					History:  history,
					Retry:    workflow.FixedRetryPolicy{MaxRetries: 1},
				},
			},
		},
		app.WithWorkflowFactories(greetingWorkflowFactory{}),
	)
	if err != nil {
		panic(err)
	}

	if err := instance.Start(ctx); err != nil {
		panic(err)
	}
	defer func() {
		_ = instance.Shutdown(ctx)
	}()

	registered, err := instance.Runtime().ResolveWorkflow("greeting")
	if err != nil {
		panic(err)
	}

	resp, err := registered.Run(ctx, workflow.Request{
		Input: map[string]any{"name": "Ada"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Output["message"])
	fmt.Println(len(history.Entries()) > 0)
}

type greetingWorkflowFactory struct{}

func (greetingWorkflowFactory) Name() string {
	return "greeting"
}

func (greetingWorkflowFactory) Build(ctx context.Context, defaults app.WorkflowDefaults) (workflow.Workflow, error) {
	builder := workflow.NewBuilder("greeting").WithMetadata(defaults.Metadata)
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
		Step(workflow.Action("load", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			name, _ := stepCtx.Input["name"].(string)
			if name == "" {
				name = "friend"
			}
			stepCtx.State.Set("name", name)
			return workflow.StepResult{Output: map[string]any{"name": name}}, nil
		})).
		Step(workflow.Action("compose", func(ctx context.Context, stepCtx workflow.StepContext) (workflow.StepResult, error) {
			name, _ := stepCtx.State.Get("name")
			return workflow.StepResult{Output: map[string]any{"message": fmt.Sprintf("hello, %v", name)}}, nil
		})).
		Build()
}
