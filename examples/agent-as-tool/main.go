// Package main demonstrates agents-as-tools: a coordinator agent delegates a
// task to a specialist sub-agent via tool.NewAgentTool.
//
// The specialist's logic is wrapped in an AgentRunFunc. In production code
// you would use agent.AsRunFunc(realAgent) to wrap a real *agent.Agent, or
// app.AgentResolver(rt) for lazy resolution from the runtime registry.
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func main() {
	ctx := context.Background()

	specialistTool, err := tool.NewAgentTool(tool.AgentToolConfig{
		Name:        "specialist",
		Description: "Delegates Go language questions to a specialist agent",
		RunFunc: func(_ context.Context, prompt, _ string, _ types.Metadata) (tool.AgentToolResult, error) {
			return tool.AgentToolResult{
				Content: fmt.Sprintf("[specialist] Go offers simplicity, goroutines, fast compilation, strong stdlib. (question: %s)", prompt),
				AgentID: "specialist",
				RunID:   "specialist-run-1",
			}, nil
		},
	})
	if err != nil {
		panic(err)
	}

	instance, err := app.New(
		app.Config{
			Name:     "agent-as-tool-demo",
			Defaults: app.Defaults{Logger: logger.Default()},
		},
		app.WithAgentFactories(&coordinatorFactory{specialistTool: specialistTool}),
	)
	if err != nil {
		panic(err)
	}

	if err := instance.Start(ctx); err != nil {
		panic(err)
	}
	ctx = instance.Context(ctx)
	defer func() { _ = instance.Shutdown(ctx) }()

	coordinator, err := instance.Runtime().ResolveAgent("coordinator")
	if err != nil {
		panic(err)
	}

	response, err := coordinator.Run(ctx, agent.Request{
		SessionID: "demo-session",
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "What are the benefits of Go?"},
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("=== Coordinator response ===")
	fmt.Println(response.Message.Content)
}

// --- coordinator factory ---

type coordinatorFactory struct {
	specialistTool tool.Tool
}

func (c *coordinatorFactory) Name() string { return "coordinator" }

func (c *coordinatorFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
		agent.WithTools(c.specialistTool),
	}
	if len(defaults.Hooks) > 0 {
		opts = append(opts, agent.WithHooks(defaults.Hooks...))
	}

	return agent.New(agent.Config{
		Name:         "coordinator",
		Instructions: "You are a coordinator. Delegate Go questions to the specialist tool.",
		Model:        coordinatorModel{},
	}, opts...)
}

// coordinatorModel is a mock model that issues a tool call on the first turn
// and produces a final response once the tool result arrives.
type coordinatorModel struct{}

func (coordinatorModel) Generate(_ context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	for _, msg := range req.Messages {
		if msg.Role == types.RoleTool {
			return agent.ModelResponse{
				Message: types.Message{
					Role:    types.RoleAssistant,
					Content: fmt.Sprintf("Based on the specialist: %s", msg.Content),
				},
			}, nil
		}
	}

	return agent.ModelResponse{
		Message: types.Message{Role: types.RoleAssistant},
		ToolCalls: []agent.ModelToolCall{
			{ID: "tc-1", Name: "specialist", Input: map[string]any{"prompt": lastUserContent(req.Messages)}},
		},
	}, nil
}

func (coordinatorModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return closedStream{}, nil
}

// --- helpers ---

type closedStream struct{}

func (closedStream) Recv() (agent.ModelEvent, error) { return agent.ModelEvent{}, io.EOF }
func (closedStream) Close() error                    { return nil }

func lastUserContent(msgs []types.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == types.RoleUser {
			return strings.TrimSpace(msgs[i].Content)
		}
	}
	return ""
}
