package main

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

var numberPattern = regexp.MustCompile(`\d+(?:\.\d+)?`)

type agentFactory struct {
	agentName string
}

func (f agentFactory) Name() string {
	name := strings.TrimSpace(f.agentName)
	if name == "" {
		return defaultAgentName
	}
	return name
}

func (f agentFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
		agent.WithTools(demoTools()...),
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
			Name:         f.Name(),
			Instructions: "Reply briefly using local demo logic and acknowledge previous memory when the session already exists.",
			Model:        demoModel{},
		},
		opts...,
	)
}

type demoModel struct{}

func (demoModel) Generate(_ context.Context, req agent.ModelRequest) (agent.ModelResponse, error) {
	if result, ok := lastToolResult(req.Messages); ok {
		content := responseFromToolResult(req, result)
		return agent.ModelResponse{
			Message: types.Message{Role: types.RoleAssistant, Content: content},
		}, nil
	}

	if calls := detectToolCalls(req); len(calls) > 0 {
		return agent.ModelResponse{ToolCalls: calls}, nil
	}

	content := responseTextFromRequest(req)
	return agent.ModelResponse{
		Message: types.Message{Role: types.RoleAssistant, Content: content},
	}, nil
}

func (demoModel) Stream(_ context.Context, req agent.ModelRequest) (agent.ModelStream, error) {
	if result, ok := lastToolResult(req.Messages); ok {
		content := responseFromToolResult(req, result)
		return &demoModelStream{events: streamEventsFromContent(content)}, nil
	}

	if calls := detectToolCalls(req); len(calls) > 0 {
		events := make([]agent.ModelEvent, len(calls))
		for i, call := range calls {
			c := call
			events[i] = agent.ModelEvent{ToolCall: &c, Done: i == len(calls)-1}
		}
		return &demoModelStream{events: events}, nil
	}

	content := responseTextFromRequest(req)
	return &demoModelStream{events: streamEventsFromContent(content)}, nil
}

type demoModelStream struct {
	events []agent.ModelEvent
	index  int
}

func (s *demoModelStream) Recv() (agent.ModelEvent, error) {
	if s.index >= len(s.events) {
		return agent.ModelEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (*demoModelStream) Close() error { return nil }

func detectToolCalls(req agent.ModelRequest) []agent.ModelToolCall {
	msg := strings.ToLower(lastUserMessage(req.Messages))

	if strings.Contains(msg, "use unknown_tool") {
		return []agent.ModelToolCall{{ID: "tc-unknown", Name: "unknown_tool"}}
	}

	if strings.Contains(msg, "time") || strings.Contains(msg, "hora") {
		return []agent.ModelToolCall{{ID: "tc-time", Name: "get_time"}}
	}

	if strings.Contains(msg, "sum") || strings.Contains(msg, "soma") {
		a, b := extractTwoNumbers(msg)
		return []agent.ModelToolCall{{
			ID:    "tc-sum",
			Name:  "calculate_sum",
			Input: map[string]any{"a": a, "b": b},
		}}
	}

	return nil
}

func extractTwoNumbers(text string) (float64, float64) {
	matches := numberPattern.FindAllString(text, -1)
	if len(matches) < 2 {
		return 0, 0
	}
	a, _ := strconv.ParseFloat(matches[0], 64)
	b, _ := strconv.ParseFloat(matches[1], 64)
	return a, b
}

func lastToolResult(messages []types.Message) (types.Message, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.RoleTool {
			return messages[i], true
		}
	}
	return types.Message{}, false
}

func responseFromToolResult(req agent.ModelRequest, result types.Message) string {
	toolName := result.Name
	content := strings.TrimSpace(result.Content)

	switch toolName {
	case "get_time":
		return fmt.Sprintf("the current time is %s", content)
	case "calculate_sum":
		return fmt.Sprintf("the result is %s", content)
	default:
		return fmt.Sprintf("tool %s returned: %s", toolName, content)
	}
}

func responseTextFromRequest(req agent.ModelRequest) string {
	message := lastUserMessage(req.Messages)
	if len(req.Memory.Messages) > 0 {
		return fmt.Sprintf("welcome back, %s", message)
	}
	return fmt.Sprintf("hello, %s", message)
}

// keep tool import used at compile time
var _ tool.Tool = getTimeTool{}

func lastUserMessage(messages []types.Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != types.RoleUser {
			continue
		}
		text := strings.TrimSpace(messages[index].Content)
		if text == "" {
			return "friend"
		}
		return text
	}
	return "friend"
}

func streamEventsFromContent(content string) []agent.ModelEvent {
	if content == "" {
		content = "hello, friend"
	}
	chunks := splitContent(content)
	events := make([]agent.ModelEvent, 0, len(chunks)+1)
	for _, chunk := range chunks {
		events = append(events, agent.ModelEvent{
			Delta: &types.MessageDelta{
				Role:    types.RoleAssistant,
				Content: chunk,
			},
		})
	}
	events = append(events, agent.ModelEvent{
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
		Done: true,
	})
	return events
}

func splitContent(content string) []string {
	words := strings.Fields(content)
	if len(words) <= 1 {
		return []string{content}
	}

	chunks := make([]string, 0, len(words))
	for index, word := range words {
		if index == 0 {
			chunks = append(chunks, word)
			continue
		}
		chunks = append(chunks, " "+word)
	}
	return chunks
}
