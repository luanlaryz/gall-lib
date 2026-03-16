// Package tool defines the public tool contracts consumed by agents.
package tool

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Schema is a lightweight placeholder for tool input shape metadata.
type Schema struct {
	Type       string
	Properties map[string]any
	Required   []string
}

// Call is the input envelope for a tool invocation.
type Call struct {
	ID        string
	RunID     string
	SessionID string
	Input     map[string]any
	Metadata  types.Metadata
}

// Result is the raw observable output returned by a tool call.
type Result struct {
	Content  string
	Data     map[string]any
	Metadata types.Metadata
}

// Tool is the public contract for an invocable tool.
type Tool interface {
	Name() string
	Description() string
	Schema() Schema
	Call(ctx context.Context, call Call) (Result, error)
}
