package main

import (
	"context"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// inputBlockGuardrail rejects runs whose user message contains "BLOCK_ME".
type inputBlockGuardrail struct{}

func (inputBlockGuardrail) CheckInput(_ context.Context, req guardrail.InputRequest) (guardrail.Decision, error) {
	for _, msg := range req.Messages {
		if msg.Role == types.RoleUser && strings.Contains(msg.Content, "BLOCK_ME") {
			return guardrail.Decision{
				Name:   "input-block",
				Action: guardrail.ActionBlock,
				Reason: "input contains forbidden keyword",
			}, nil
		}
	}
	return guardrail.Decision{Name: "input-block", Action: guardrail.ActionAllow}, nil
}

// outputTagGuardrail appends " [guardrail:ok]" to every final response.
type outputTagGuardrail struct{}

func (outputTagGuardrail) CheckOutput(_ context.Context, req guardrail.OutputRequest) (guardrail.Decision, error) {
	return guardrail.Decision{
		Name:   "output-tag",
		Action: guardrail.ActionTransform,
		Message: &types.Message{
			Role:    req.Message.Role,
			Content: req.Message.Content + " [guardrail:ok]",
		},
	}, nil
}

// streamDigitGuardrail replaces digits with '*' in each stream chunk.
type streamDigitGuardrail struct{}

func (streamDigitGuardrail) CheckStream(_ context.Context, req guardrail.StreamRequest) (guardrail.Decision, error) {
	replaced := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return '*'
		}
		return r
	}, req.Delta.Content)

	if replaced == req.Delta.Content {
		return guardrail.Decision{Name: "stream-digit-redaction", Action: guardrail.ActionAllow}, nil
	}

	return guardrail.Decision{
		Name:   "stream-digit-redaction",
		Action: guardrail.ActionTransform,
		Delta: &types.MessageDelta{
			RunID:   req.Delta.RunID,
			Role:    req.Delta.Role,
			Content: replaced,
		},
	}, nil
}
