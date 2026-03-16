package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func main() {
	ctx := context.Background()

	instance, err := app.New(
		app.Config{
			Name: "memory-example",
			Defaults: app.Defaults{
				Agent: app.AgentDefaults{
					Memory: &memory.InMemoryStore{},
				},
			},
		},
		app.WithAgentFactories(memoryGreeterFactory{}),
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

	greeter, err := instance.Runtime().ResolveAgent("memory-greeter")
	if err != nil {
		panic(err)
	}

	first, err := greeter.Run(ctx, agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "Ada"}},
		Metadata: types.Metadata{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	})
	if err != nil {
		panic(err)
	}

	second, err := greeter.Run(ctx, agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "Ada again"}},
		Metadata: types.Metadata{
			"user_id":         "user-1",
			"conversation_id": "conv-1",
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(first.Message.Content)
	fmt.Println(second.Message.Content)
	fmt.Println("working memory is per-run; the persisted history is what survives between runs")
}

type memoryGreeterFactory struct{}

func (memoryGreeterFactory) Name() string {
	return "memory-greeter"
}

func (memoryGreeterFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
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
			Name:         "memory-greeter",
			Instructions: "Greet briefly and acknowledge persisted context when present.",
			Model:        memoryGreeterModel{},
		},
		opts...,
	)
}

type memoryGreeterModel struct{}

func (memoryGreeterModel) Generate(ctx context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
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

	content := fmt.Sprintf("hello, %s", name)
	if len(req.Memory.Messages) > 0 {
		content = fmt.Sprintf("welcome back, %s", name)
	}

	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
	}, nil
}

func (memoryGreeterModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedModelStream{}, nil
}

type closedModelStream struct{}

func (closedModelStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedModelStream) Close() error                    { return nil }
