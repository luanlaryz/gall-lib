package tool

import (
	"context"
	"fmt"
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// AgentToolResult carries the output of a sub-agent invocation back to the
// AgentTool adapter without requiring pkg/tool to import pkg/agent.
type AgentToolResult struct {
	Content string
	AgentID string
	RunID   string
}

// AgentRunFunc is the callback signature that bridges an agent's Run method
// into the tool layer. pkg/agent.AsRunFunc produces a ready-to-use value.
type AgentRunFunc func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (AgentToolResult, error)

// AgentResolverFunc resolves an AgentRunFunc by agent name at call time.
// pkg/app.AgentResolver produces a ready-to-use value backed by the runtime
// registry.
type AgentResolverFunc func(name string) (AgentRunFunc, error)

// AgentToolConfig carries validated configuration for NewAgentTool.
//
// Exactly one agent source must be provided:
//   - RunFunc (static): the sub-agent is fixed at construction time.
//   - Resolver (lazy): the sub-agent is resolved by Name on the first Call.
//
// When both are provided, RunFunc takes precedence and Resolver is ignored.
type AgentToolConfig struct {
	Name        string
	Description string
	RunFunc     AgentRunFunc
	Resolver    AgentResolverFunc
}

// NewAgentTool creates a Tool that delegates Call to a sub-agent's Run.
//
// The returned Tool is safe for concurrent use after construction.
func NewAgentTool(cfg AgentToolConfig) (Tool, error) {
	if err := validateName(cfg.Name, "tool name"); err != nil {
		return nil, newError(ErrorKindInvalidTool, "new_agent_tool", cfg.Name, "", "", err)
	}
	if cfg.Description == "" {
		return nil, newError(
			ErrorKindInvalidTool,
			"new_agent_tool",
			cfg.Name,
			"",
			"",
			fmt.Errorf("%w: tool description is required", ErrInvalidTool),
		)
	}
	if cfg.RunFunc == nil && cfg.Resolver == nil {
		return nil, newError(
			ErrorKindInvalidTool,
			"new_agent_tool",
			cfg.Name,
			"",
			"",
			fmt.Errorf("%w: either RunFunc or Resolver must be provided", ErrInvalidTool),
		)
	}

	at := &agentTool{
		name:        cfg.Name,
		description: cfg.Description,
	}

	if cfg.RunFunc != nil {
		at.runFunc = cfg.RunFunc
	} else {
		at.resolver = cfg.Resolver
	}

	return at, nil
}

type agentTool struct {
	name        string
	description string

	runFunc  AgentRunFunc
	resolver AgentResolverFunc

	once       sync.Once
	resolved   AgentRunFunc
	resolveErr error
}

func (at *agentTool) Name() string        { return at.name }
func (at *agentTool) Description() string { return at.description }

func (at *agentTool) InputSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Input for the sub-agent tool",
		Properties: map[string]Schema{
			"prompt": {
				Type:        "string",
				Description: "The task or question to delegate to the sub-agent",
			},
		},
		Required: []string{"prompt"},
	}
}

func (at *agentTool) OutputSchema() Schema {
	return Schema{
		Type:        "object",
		Description: "Output from the sub-agent tool",
		Properties: map[string]Schema{
			"response": {
				Type:        "string",
				Description: "The text response produced by the sub-agent",
			},
		},
		Required: []string{"response"},
	}
}

func (at *agentTool) Call(ctx context.Context, call Call) (Result, error) {
	fn, err := at.effectiveRunFunc()
	if err != nil {
		return Result{}, newError(ErrorKindExecution, "call", at.name, "", call.ID, err)
	}

	prompt, _ := call.Input["prompt"].(string)

	md := types.CloneMetadata(call.Metadata)
	if md == nil {
		md = make(types.Metadata)
	}
	md["coordinator_agent_id"] = call.AgentID
	md["coordinator_run_id"] = call.RunID

	result, err := fn(ctx, prompt, call.SessionID, md)
	if err != nil {
		return Result{}, newError(ErrorKindExecution, "call", at.name, "", call.ID, err)
	}

	return Result{
		Content: result.Content,
		Value:   map[string]any{"response": result.Content},
		Metadata: types.Metadata{
			"sub_agent_id":     result.AgentID,
			"sub_agent_run_id": result.RunID,
		},
	}, nil
}

func (at *agentTool) effectiveRunFunc() (AgentRunFunc, error) {
	if at.runFunc != nil {
		return at.runFunc, nil
	}

	at.once.Do(func() {
		at.resolved, at.resolveErr = at.resolver(at.name)
	})
	return at.resolved, at.resolveErr
}
