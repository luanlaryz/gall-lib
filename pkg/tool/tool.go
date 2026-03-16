// Package tool defines the public tool contracts consumed by agents.
package tool

import (
	"context"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Tool is the public contract for an invocable tool.
type Tool interface {
	Name() string
	Description() string
	InputSchema() Schema
	OutputSchema() Schema
	Call(ctx context.Context, call Call) (Result, error)
}

// Toolkit groups related tools under a common identity and optional namespace.
type Toolkit interface {
	Name() string
	Description() string
	Namespace() string
	Tools() []Tool
}

// Schema is the validated subset of JSON Schema supported by gaal-lib v1.
type Schema struct {
	Type                 string
	Description          string
	Properties           map[string]Schema
	Items                *Schema
	Required             []string
	Enum                 []string
	AdditionalProperties *bool
}

// Call is the input envelope for a tool invocation.
type Call struct {
	ID        string
	ToolName  string
	RunID     string
	SessionID string
	AgentID   string
	Input     map[string]any
	Metadata  types.Metadata
}

// Result is the observable output returned by a tool call.
type Result struct {
	Value    any
	Content  string
	Metadata types.Metadata
}

// Descriptor describes a registered tool using its effective name.
type Descriptor struct {
	Name         string
	LocalName    string
	Description  string
	Toolkit      string
	Namespace    string
	InputSchema  Schema
	OutputSchema Schema
}

// ToolkitDescriptor describes a registered toolkit.
type ToolkitDescriptor struct {
	Name        string
	Description string
	Namespace   string
	ToolCount   int
}

// Registry registers, resolves and lists tools and toolkits deterministically.
type Registry interface {
	Register(tools ...Tool) error
	RegisterToolkits(toolkits ...Toolkit) error
	Resolve(name string) (Tool, error)
	List() []Descriptor
	ListToolkits() []ToolkitDescriptor
}

type describedTool interface {
	Tool
	Descriptor() Descriptor
}

// NewRegistry returns a concurrency-safe in-memory registry.
func NewRegistry() Registry {
	return &registry{
		tools:    make(map[string]registryEntry),
		toolkits: make(map[string]ToolkitDescriptor),
	}
}

// DescriptorOf returns the effective descriptor for a tool.
func DescriptorOf(t Tool) Descriptor {
	if t == nil {
		return Descriptor{}
	}

	if described, ok := t.(describedTool); ok {
		desc := cloneDescriptor(described.Descriptor())
		if desc.LocalName == "" {
			desc.LocalName = strings.TrimSpace(t.Name())
		}
		if desc.Description == "" {
			desc.Description = strings.TrimSpace(t.Description())
		}
		if desc.Name == "" {
			desc.Name = effectiveName(desc.Namespace, desc.LocalName)
		}
		if isZeroSchema(desc.InputSchema) {
			desc.InputSchema = cloneSchema(t.InputSchema())
		}
		if isZeroSchema(desc.OutputSchema) {
			desc.OutputSchema = cloneSchema(t.OutputSchema())
		}
		return desc
	}

	localName := strings.TrimSpace(t.Name())
	return Descriptor{
		Name:         localName,
		LocalName:    localName,
		Description:  strings.TrimSpace(t.Description()),
		InputSchema:  cloneSchema(t.InputSchema()),
		OutputSchema: cloneSchema(t.OutputSchema()),
	}
}
