package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/types"
)

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
	content := responseTextFromRequest(req)
	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: content,
		},
	}, nil
}

func (demoModel) Stream(_ context.Context, req agent.ModelRequest) (agent.ModelStream, error) {
	content := responseTextFromRequest(req)
	return &demoModelStream{
		events: streamEventsFromContent(content),
	}, nil
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

func responseTextFromRequest(req agent.ModelRequest) string {
	message := lastUserMessage(req.Messages)
	if len(req.Memory.Messages) > 0 {
		return fmt.Sprintf("welcome back, %s", message)
	}
	return fmt.Sprintf("hello, %s", message)
}

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
