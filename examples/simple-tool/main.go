package main

import (
	"context"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/tool"
)

func main() {
	registry := tool.NewRegistry()
	if err := registry.Register(echoTool{}); err != nil {
		panic(err)
	}

	echo, err := registry.Resolve("echo")
	if err != nil {
		panic(err)
	}

	result, err := tool.Invoke(context.Background(), echo, tool.Call{
		ID:    "call-1",
		Input: map[string]any{"text": "hello"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Content)
}

type echoTool struct{}

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "Echoes text back to the caller." }
func (echoTool) InputSchema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			"text": {Type: "string"},
		},
		Required: []string{"text"},
	}
}
func (echoTool) OutputSchema() tool.Schema {
	return tool.Schema{Type: "string"}
}
func (echoTool) Call(_ context.Context, call tool.Call) (tool.Result, error) {
	text := call.Input["text"].(string)
	return tool.Result{Value: text, Content: text}, nil
}
