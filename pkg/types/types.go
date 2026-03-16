// Package types contains small shared public types used across gaal-lib.
package types

// Metadata carries small string-based attributes across public APIs.
type Metadata map[string]string

// CloneMetadata returns a defensive copy of md.
func CloneMetadata(md Metadata) Metadata {
	if len(md) == 0 {
		return nil
	}

	out := make(Metadata, len(md))
	for key, value := range md {
		out[key] = value
	}
	return out
}

// MergeMetadata merges base with override, where override wins on key conflicts.
func MergeMetadata(base, override Metadata) Metadata {
	switch {
	case len(base) == 0 && len(override) == 0:
		return nil
	case len(base) == 0:
		return CloneMetadata(override)
	case len(override) == 0:
		return CloneMetadata(base)
	}

	out := CloneMetadata(base)
	for key, value := range override {
		out[key] = value
	}
	return out
}

// MessageRole identifies the semantic role of a message in a run.
type MessageRole string

const (
	// RoleSystem carries fixed instructions for a model.
	RoleSystem MessageRole = "system"
	// RoleUser carries user-provided input.
	RoleUser MessageRole = "user"
	// RoleAssistant carries assistant/model output.
	RoleAssistant MessageRole = "assistant"
	// RoleTool carries a tool result injected back into the model context.
	RoleTool MessageRole = "tool"
)

// ToolCall represents a tool invocation requested by a model.
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// CloneToolCalls returns a defensive copy of tool calls.
func CloneToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]ToolCall, len(calls))
	for index, call := range calls {
		out[index] = ToolCall{
			ID:    call.ID,
			Name:  call.Name,
			Input: cloneMap(call.Input),
		}
	}
	return out
}

// Message is the canonical public message envelope used by requests and responses.
type Message struct {
	Role       MessageRole
	Content    string
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
	Metadata   Metadata
}

// CloneMessage returns a defensive copy of msg.
func CloneMessage(msg Message) Message {
	return Message{
		Role:       msg.Role,
		Content:    msg.Content,
		Name:       msg.Name,
		ToolCallID: msg.ToolCallID,
		ToolCalls:  CloneToolCalls(msg.ToolCalls),
		Metadata:   CloneMetadata(msg.Metadata),
	}
}

// CloneMessages returns a defensive copy of msgs.
func CloneMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return nil
	}

	out := make([]Message, len(msgs))
	for index, msg := range msgs {
		out[index] = CloneMessage(msg)
	}
	return out
}

// MessageDelta carries partial message content during streaming.
type MessageDelta struct {
	RunID   string
	Role    MessageRole
	Content string
}

// Usage tracks coarse model usage counters for a run.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// Add sums two usage values.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		InputTokens:  u.InputTokens + other.InputTokens,
		OutputTokens: u.OutputTokens + other.OutputTokens,
		TotalTokens:  u.TotalTokens + other.TotalTokens,
	}
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneSlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}

	out := make([]any, len(in))
	for index, value := range in {
		out[index] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		return cloneSlice(value)
	default:
		return value
	}
}
