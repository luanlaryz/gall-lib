package agent

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// AsRunFunc adapts an Agent into a tool.AgentRunFunc suitable for use with
// tool.NewAgentTool. The returned function delegates to a.Run with a single
// user message built from the prompt argument.
func AsRunFunc(a *Agent) tool.AgentRunFunc {
	return func(ctx context.Context, prompt, sessionID string, metadata types.Metadata) (tool.AgentToolResult, error) {
		resp, err := a.Run(ctx, Request{
			SessionID: sessionID,
			Messages:  []types.Message{{Role: types.RoleUser, Content: prompt}},
			Metadata:  metadata,
		})
		if err != nil {
			return tool.AgentToolResult{}, err
		}
		return tool.AgentToolResult{
			Content: resp.Message.Content,
			AgentID: resp.AgentID,
			RunID:   resp.RunID,
		}, nil
	}
}
