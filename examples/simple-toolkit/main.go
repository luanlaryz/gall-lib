package main

import (
	"context"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/tool"
)

func main() {
	registry := tool.NewRegistry()
	if err := registry.RegisterToolkits(mathToolkit{}); err != nil {
		panic(err)
	}

	sum, err := registry.Resolve("math.sum")
	if err != nil {
		panic(err)
	}

	result, err := tool.Invoke(context.Background(), sum, tool.Call{
		ID: "call-1",
		Input: map[string]any{
			"a": 2,
			"b": 3,
		},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Value)
}

type mathToolkit struct{}

func (mathToolkit) Name() string        { return "math" }
func (mathToolkit) Description() string { return "Basic arithmetic helpers." }
func (mathToolkit) Namespace() string   { return "math" }
func (mathToolkit) Tools() []tool.Tool  { return []tool.Tool{sumTool{}} }

type sumTool struct{}

func (sumTool) Name() string        { return "sum" }
func (sumTool) Description() string { return "Sums two integers." }
func (sumTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"a": {Type: "integer"},
			"b": {Type: "integer"},
		},
		Required: []string{"a", "b"},
	}
}
func (sumTool) OutputSchema() tool.Schema { return tool.Schema{Type: "integer"} }
func (sumTool) Call(_ context.Context, call tool.Call) (tool.Result, error) {
	return tool.Result{
		Value: call.Input["a"].(int) + call.Input["b"].(int),
	}, nil
}
