package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func main() {
	ctx := context.Background()

	dataDir := filepath.Join(".", "data", "conversations")

	fileStore := memory.MustNewFileStore(dataDir)

	instance, err := app.New(
		app.Config{
			Name: "file-persistence-example",
			Defaults: app.Defaults{
				Agent: app.AgentDefaults{
					Memory: fileStore,
				},
			},
		},
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

	fmt.Println("Run 1:", first.Message.Content)
	fmt.Println("Run 2:", second.Message.Content)
	fmt.Println()

	absDir, _ := filepath.Abs(dataDir)
	fmt.Println("Conversation files stored in:", absDir)

	entries, _ := os.ReadDir(dataDir)
	for _, e := range entries {
		if !e.IsDir() {
			fmt.Println("  -", e.Name())
		}
	}

	fmt.Println()
	fmt.Println("Run this program again to see that the history persists across restarts.")
}

type greeterFactory struct{}

func (greeterFactory) Name() string { return "greeter" }

func (greeterFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
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
			Instructions: "Greet briefly and acknowledge persisted context when present.",
			Model:        greeterModel{},
		},
		opts...,
	)
}

type greeterModel struct{}

func (greeterModel) Generate(_ context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	name := "friend"
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == types.RoleUser {
			name = strings.TrimSpace(req.Messages[i].Content)
			if name == "" {
				name = "friend"
			}
			break
		}
	}

	content := fmt.Sprintf("hello, %s", name)
	if len(req.Memory.Messages) > 0 {
		content = fmt.Sprintf("welcome back, %s (history: %d messages)", name, len(req.Memory.Messages))
	}

	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
	}, nil
}

func (greeterModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedStream{}, nil
}

type closedStream struct{}

func (closedStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedStream) Close() error                    { return nil }
