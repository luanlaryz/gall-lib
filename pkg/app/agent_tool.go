package app

import (
	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/tool"
)

// AgentResolver returns a tool.AgentResolverFunc backed by the runtime
// agent registry. The returned resolver can be passed to tool.NewAgentTool
// for lazy sub-agent resolution at call time.
func AgentResolver(rt Runtime) tool.AgentResolverFunc {
	return func(name string) (tool.AgentRunFunc, error) {
		a, err := rt.ResolveAgent(name)
		if err != nil {
			return nil, err
		}
		return agent.AsRunFunc(a), nil
	}
}
