package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func main() {
	ctx := context.Background()

	instance, err := app.New(
		app.Config{Name: "basic-agent"},
		app.WithAgentFactories(greeterFactory{}),
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

	greeter, err := instance.Runtime().ResolveAgent("greeter")
	if err != nil {
		panic(err)
	}

	response, err := greeter.Run(ctx, agent.Request{
		SessionID: "session-1",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "Ada"},
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(response.Message.Content)
}

type greeterFactory struct{}

func (greeterFactory) Name() string {
	return "greeter"
}

func (greeterFactory) Build(ctx context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
	}
	if len(defaults.Metadata) > 0 {
		opts = append(opts, agent.WithMetadata(defaults.Metadata))
	}
	if defaults.Memory != nil {
		opts = append(opts, agent.WithMemory(defaults.Memory))
	}
	if defaults.WorkingMemory != nil {
		opts = append(opts, agent.WithWorkingMemory(defaults.WorkingMemory))
	}
	if len(defaults.InputGuardrails) > 0 {
		opts = append(opts, agent.WithInputGuardrails(defaults.InputGuardrails...))
	}
	if len(defaults.StreamGuardrails) > 0 {
		opts = append(opts, agent.WithStreamGuardrails(defaults.StreamGuardrails...))
	}
	if len(defaults.OutputGuardrails) > 0 {
		opts = append(opts, agent.WithOutputGuardrails(defaults.OutputGuardrails...))
	}
	if len(defaults.Hooks) > 0 {
		opts = append(opts, agent.WithHooks(defaults.Hooks...))
	}

	return agent.New(
		agent.Config{
			Name:         "greeter",
			Instructions: "Answer briefly and greet the user by name.",
			Model:        greeterModel{},
		},
		opts...,
	)
}

type greeterModel struct{}

func (greeterModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	name := "friend"
	for index := len(req.Messages) - 1; index >= 0; index-- {
		if req.Messages[index].Role == types.RoleUser {
			name = strings.TrimSpace(req.Messages[index].Content)
			if name == "" {
				name = "friend"
			}
			break
		}
	}

	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: fmt.Sprintf("hello, %s", name),
		},
	}, nil
}

func (greeterModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type closedModelStream struct{}

func (closedModelStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedModelStream) Close() error                    { return nil }
