package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func main() {
	ctx := context.Background()

	instance, err := app.New(
		app.Config{
			Name: "stream-redaction-example",
			Defaults: app.Defaults{
				Agent: app.AgentDefaults{
					StreamGuardrails: []guardrail.Stream{digitRedactionGuardrail{}},
				},
			},
		},
		app.WithAgentFactories(redactionFactory{}),
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

	redactor, err := instance.Runtime().ResolveAgent("stream-redactor")
	if err != nil {
		panic(err)
	}

	stream, err := redactor.Stream(ctx, agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "show the receipt"}},
	})
	if err != nil {
		panic(err)
	}

	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}

		switch event.Type {
		case agent.EventAgentDelta:
			fmt.Printf("delta: %s\n", event.Delta.Content)
		case agent.EventAgentCompleted:
			fmt.Printf("final: %s\n", event.Response.Message.Content)
		}
	}
}

type redactionFactory struct{}

func (redactionFactory) Name() string {
	return "stream-redactor"
}

func (redactionFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
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

	opts = append(opts, agent.WithOutputGuardrails(finalSuffixGuardrail{}))

	return agent.New(
		agent.Config{
			Name:         "stream-redactor",
			Instructions: "Stream a short receipt-like answer.",
			Model:        redactionModel{},
		},
		opts...,
	)
}

type redactionModel struct{}

func (redactionModel) Generate(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
	return agent.ModelResponse{
		Message: types.Message{
			Role:    types.RoleAssistant,
			Content: "card 4242 code 123",
		},
	}, nil
}

func (redactionModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return &receiptStream{
		events: []agent.ModelEvent{
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: "card 4242"}},
			{Delta: &types.MessageDelta{Role: types.RoleAssistant, Content: " code 123"}},
			{Message: &types.Message{Role: types.RoleAssistant, Content: "card 4242 code 123"}, Done: true},
		},
	}, nil
}

type receiptStream struct {
	events []agent.ModelEvent
	index  int
}

func (s *receiptStream) Recv() (agent.ModelEvent, error) {
	if s.index >= len(s.events) {
		return agent.ModelEvent{}, io.EOF
	}
	event := s.events[s.index]
	s.index++
	return event, nil
}

func (*receiptStream) Close() error { return nil }

type digitRedactionGuardrail struct{}

func (digitRedactionGuardrail) CheckStream(_ context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
	replaced := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return 'X'
		}
		return r
	}, req.Delta.Content)

	if replaced == req.Delta.Content {
		return guardrail.Decision{Name: "digit-redaction", Action: guardrail.ActionAllow}, nil
	}

	return guardrail.Decision{
		Name:   "digit-redaction",
		Action: guardrail.ActionTransform,
		Delta: &types.MessageDelta{
			RunID:   req.Delta.RunID,
			Role:    req.Delta.Role,
			Content: replaced,
		},
	}, nil
}

type finalSuffixGuardrail struct{}

func (finalSuffixGuardrail) CheckOutput(_ context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
	return guardrail.Decision{
		Name:   "final-suffix",
		Action: guardrail.ActionTransform,
		Message: &types.Message{
			Role:    types.RoleAssistant,
			Content: req.Message.Content + " [final]",
		},
	}, nil
}
