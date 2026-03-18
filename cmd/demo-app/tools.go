package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/luanlima/gaal-lib/pkg/tool"
)

// getTimeTool returns the current UTC time as an RFC3339 string.
type getTimeTool struct{}

func (getTimeTool) Name() string        { return "get_time" }
func (getTimeTool) Description() string { return "Returns the current UTC time." }
func (getTimeTool) InputSchema() tool.Schema {
	return tool.Schema{Type: "object"}
}
func (getTimeTool) OutputSchema() tool.Schema {
	return tool.Schema{Type: "string"}
}
func (getTimeTool) Call(_ context.Context, _ tool.Call) (tool.Result, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	return tool.Result{Value: now, Content: now}, nil
}

// calculateSumTool sums two numbers.
type calculateSumTool struct{}

func (calculateSumTool) Name() string        { return "calculate_sum" }
func (calculateSumTool) Description() string { return "Sums two numbers a and b." }
func (calculateSumTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"a": {Type: "number"},
			"b": {Type: "number"},
		},
		Required: []string{"a", "b"},
	}
}
func (calculateSumTool) OutputSchema() tool.Schema {
	return tool.Schema{Type: "number"}
}
func (calculateSumTool) Call(_ context.Context, call tool.Call) (tool.Result, error) {
	a, err := toFloat64(call.Input["a"])
	if err != nil {
		return tool.Result{}, fmt.Errorf("invalid input a: %w", err)
	}
	b, err := toFloat64(call.Input["b"])
	if err != nil {
		return tool.Result{}, fmt.Errorf("invalid input b: %w", err)
	}
	sum := a + b
	return tool.Result{
		Value:   sum,
		Content: formatNumber(sum),
	}, nil
}

func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}

func formatNumber(n float64) string {
	if n == float64(int64(n)) {
		return fmt.Sprintf("%d", int64(n))
	}
	return fmt.Sprintf("%g", n)
}

// demoTools returns the tools registered in the demo agent.
func demoTools() []tool.Tool {
	return []tool.Tool{getTimeTool{}, calculateSumTool{}}
}
