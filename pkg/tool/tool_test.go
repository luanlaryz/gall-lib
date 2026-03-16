package tool_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

func TestRegistryRegisterStandaloneAndResolve(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	if err := registry.Register(newEchoTool()); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	descriptors := registry.List()
	if len(descriptors) != 1 {
		t.Fatalf("List() len = %d want 1", len(descriptors))
	}
	if descriptors[0].Name != "echo" {
		t.Fatalf("List()[0].Name = %q want %q", descriptors[0].Name, "echo")
	}

	resolved, err := registry.Resolve("echo")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := tool.DescriptorOf(resolved); got.Name != "echo" || got.LocalName != "echo" {
		t.Fatalf("DescriptorOf() = %+v", got)
	}
}

func TestRegistryRejectsInvalidStandaloneTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tool    tool.Tool
		target  error
		message string
	}{
		{
			name:    "empty name",
			tool:    testTool{description: "echo text", inputSchema: objectSchema("text", false), outputSchema: tool.Schema{Type: "string"}},
			target:  tool.ErrInvalidTool,
			message: "tool name is required",
		},
		{
			name:    "bad name",
			tool:    testTool{name: "EchoTool", description: "echo text", inputSchema: objectSchema("text", false), outputSchema: tool.Schema{Type: "string"}},
			target:  tool.ErrInvalidTool,
			message: "must match",
		},
		{
			name:    "missing description",
			tool:    testTool{name: "echo", inputSchema: objectSchema("text", false), outputSchema: tool.Schema{Type: "string"}},
			target:  tool.ErrInvalidTool,
			message: "description is required",
		},
		{
			name:    "bad input schema",
			tool:    testTool{name: "echo", description: "echo text", inputSchema: tool.Schema{Type: "string"}, outputSchema: tool.Schema{Type: "string"}},
			target:  tool.ErrInvalidSchema,
			message: "input schema type must be object",
		},
		{
			name:    "bad output schema",
			tool:    testTool{name: "echo", description: "echo text", inputSchema: objectSchema("text", false), outputSchema: tool.Schema{}},
			target:  tool.ErrInvalidSchema,
			message: "output schema type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := tool.NewRegistry()
			err := registry.Register(tt.tool)
			if !errors.Is(err, tt.target) {
				t.Fatalf("Register() error = %v want target %v", err, tt.target)
			}
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Register() error = %v want substring %q", err, tt.message)
			}
		})
	}
}

func TestRegistryRejectsInvalidToolkits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		toolkit tool.Toolkit
		target  error
	}{
		{
			name:    "invalid name",
			toolkit: testToolkit{name: "MathKit", description: "math helpers"},
			target:  tool.ErrInvalidToolkit,
		},
		{
			name:    "nil tool",
			toolkit: testToolkit{name: "math", description: "math helpers", tools: []tool.Tool{nil}},
			target:  tool.ErrInvalidToolkit,
		},
		{
			name: "duplicate local names",
			toolkit: testToolkit{
				name:        "math",
				description: "math helpers",
				tools:       []tool.Tool{newMathSumTool(), newMathSumTool()},
			},
			target: tool.ErrInvalidToolkit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := tool.NewRegistry()
			err := registry.RegisterToolkits(tt.toolkit)
			if !errors.Is(err, tt.target) {
				t.Fatalf("RegisterToolkits() error = %v want target %v", err, tt.target)
			}
		})
	}
}

func TestRegistryRejectsNameConflictsAndKeepsToolkitRegistrationAtomic(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	if err := registry.Register(newEchoTool()); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := registry.RegisterToolkits(testToolkit{
		name:        "echoes",
		description: "echo tools",
		tools: []tool.Tool{
			newEchoTool(),
		},
	})
	if !errors.Is(err, tool.ErrNameConflict) {
		t.Fatalf("RegisterToolkits() error = %v want ErrNameConflict", err)
	}

	atomicErr := registry.RegisterToolkits(testToolkit{
		name:        "math",
		description: "math helpers",
		namespace:   "math",
		tools: []tool.Tool{
			newMathSumTool(),
			testTool{
				name:         "BadName",
				description:  "broken",
				inputSchema:  objectSchema("value", false),
				outputSchema: tool.Schema{Type: "integer"},
			},
		},
	})
	if !errors.Is(atomicErr, tool.ErrInvalidToolkit) {
		t.Fatalf("RegisterToolkits() atomic error = %v want ErrInvalidToolkit", atomicErr)
	}
	if _, err := registry.Resolve("math.sum"); !errors.Is(err, tool.ErrToolNotFound) {
		t.Fatalf("Resolve(math.sum) error = %v want ErrToolNotFound", err)
	}
	if len(registry.ListToolkits()) != 0 {
		t.Fatalf("ListToolkits() len = %d want 0", len(registry.ListToolkits()))
	}
}

func TestRegistryResolveNamespacedToolAndListOrdering(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	if err := registry.Register(newEchoTool(), testTool{
		name:         "alpha",
		description:  "alpha",
		inputSchema:  objectSchema("text", true),
		outputSchema: tool.Schema{Type: "string"},
		call: func(context.Context, tool.Call) (tool.Result, error) {
			return tool.Result{Value: "alpha"}, nil
		},
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.RegisterToolkits(testToolkit{
		name:        "math",
		description: "math helpers",
		namespace:   "math",
		tools:       []tool.Tool{newMathSumTool()},
	}); err != nil {
		t.Fatalf("RegisterToolkits() error = %v", err)
	}

	resolved, err := registry.Resolve("math.sum")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := tool.DescriptorOf(resolved); got.Name != "math.sum" || got.Toolkit != "math" || got.Namespace != "math" {
		t.Fatalf("DescriptorOf() = %+v", got)
	}

	descriptors := registry.List()
	if len(descriptors) != 3 {
		t.Fatalf("List() len = %d want 3", len(descriptors))
	}
	if descriptors[0].Name != "alpha" || descriptors[1].Name != "echo" || descriptors[2].Name != "math.sum" {
		t.Fatalf("List() = %+v", descriptors)
	}
	if toolkits := registry.ListToolkits(); len(toolkits) != 1 || toolkits[0].Name != "math" {
		t.Fatalf("ListToolkits() = %+v", toolkits)
	}
}

func TestRegistryResolveMissingTool(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry()
	_, err := registry.Resolve("missing")
	if !errors.Is(err, tool.ErrToolNotFound) {
		t.Fatalf("Resolve() error = %v want ErrToolNotFound", err)
	}
}

func TestInvokeRejectsInvalidInputWithoutExecuting(t *testing.T) {
	t.Parallel()

	var called atomic.Bool
	echo := testTool{
		name:         "echo",
		description:  "echo text",
		inputSchema:  objectSchema("text", false),
		outputSchema: tool.Schema{Type: "string"},
		call: func(context.Context, tool.Call) (tool.Result, error) {
			called.Store(true)
			return tool.Result{Value: "should not run"}, nil
		},
	}

	_, err := tool.Invoke(context.Background(), echo, tool.Call{
		ID:    "call-1",
		Input: map[string]any{},
	})
	if !errors.Is(err, tool.ErrInvalidInput) {
		t.Fatalf("Invoke() error = %v want ErrInvalidInput", err)
	}
	if called.Load() {
		t.Fatalf("Invoke() executed tool on invalid input")
	}
}

func TestInvokePropagatesOperationalContextAndClonesSnapshots(t *testing.T) {
	t.Parallel()

	producedValue := map[string]any{
		"total": 3,
		"items": []any{1, 2},
	}

	registry := tool.NewRegistry()
	if err := registry.RegisterToolkits(testToolkit{
		name:        "math",
		description: "math helpers",
		namespace:   "math",
		tools: []tool.Tool{
			testTool{
				name:        "sum",
				description: "sum numbers",
				inputSchema: tool.Schema{
					Type: "object",
					Properties: map[string]tool.Schema{
						"values": {
							Type:  "array",
							Items: &tool.Schema{Type: "integer"},
						},
					},
					Required: []string{"values"},
				},
				outputSchema: tool.Schema{
					Type: "object",
					Properties: map[string]tool.Schema{
						"total": {Type: "integer"},
						"items": {
							Type:  "array",
							Items: &tool.Schema{Type: "integer"},
						},
					},
					Required: []string{"total", "items"},
				},
				call: func(ctx context.Context, call tool.Call) (tool.Result, error) {
					if call.ID != "call-7" || call.ToolName != "math.sum" || call.RunID != "run-1" || call.SessionID != "session-1" || call.AgentID != "agent-1" {
						t.Fatalf("Call() envelope = %+v", call)
					}
					if call.Metadata["trace_id"] != "trace-1" {
						t.Fatalf("Call() metadata = %+v", call.Metadata)
					}

					values := call.Input["values"].([]any)
					values[0] = 99
					call.Metadata["trace_id"] = "mutated"

					return tool.Result{
						Value: producedValue,
					}, nil
				},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterToolkits() error = %v", err)
	}

	resolved, err := registry.Resolve("math.sum")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	originalInput := map[string]any{"values": []any{1, 2}}
	originalMetadata := types.Metadata{"trace_id": "trace-1"}

	result, err := tool.Invoke(context.Background(), resolved, tool.Call{
		ID:        "call-7",
		RunID:     "run-1",
		SessionID: "session-1",
		AgentID:   "agent-1",
		Input:     originalInput,
		Metadata:  originalMetadata,
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if originalInput["values"].([]any)[0] != 1 {
		t.Fatalf("Invoke() mutated original input = %+v", originalInput)
	}
	if originalMetadata["trace_id"] != "trace-1" {
		t.Fatalf("Invoke() mutated original metadata = %+v", originalMetadata)
	}

	producedValue["total"] = 99
	producedValue["items"].([]any)[0] = 88

	value := result.Value.(map[string]any)
	if value["total"] != 3 {
		t.Fatalf("Invoke() mutated returned result total = %+v", value)
	}
	if value["items"].([]any)[0] != 1 {
		t.Fatalf("Invoke() mutated returned result items = %+v", value["items"])
	}
}

func TestInvokeRespectsCancellationAndWrapsExecutionErrors(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	echo := newEchoTool()
	_, err := tool.Invoke(ctx, echo, tool.Call{
		ID:    "call-1",
		Input: map[string]any{"text": "hello"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Invoke() canceled error = %v want context.Canceled", err)
	}

	boom := errors.New("boom")
	_, err = tool.Invoke(context.Background(), testTool{
		name:         "echo",
		description:  "echo text",
		inputSchema:  objectSchema("text", false),
		outputSchema: tool.Schema{Type: "string"},
		call: func(context.Context, tool.Call) (tool.Result, error) {
			return tool.Result{}, boom
		},
	}, tool.Call{
		ID:    "call-2",
		Input: map[string]any{"text": "hello"},
	})
	var toolErr *tool.Error
	if !errors.As(err, &toolErr) || toolErr.Kind != tool.ErrorKindExecution {
		t.Fatalf("Invoke() execution error = %v want tool.ErrorKindExecution", err)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("Invoke() execution error = %v want wrapped boom", err)
	}
}

func TestInvokeRejectsInvalidOutput(t *testing.T) {
	t.Parallel()

	_, err := tool.Invoke(context.Background(), testTool{
		name:         "echo",
		description:  "echo text",
		inputSchema:  objectSchema("text", false),
		outputSchema: tool.Schema{Type: "string"},
		call: func(context.Context, tool.Call) (tool.Result, error) {
			return tool.Result{Value: 42}, nil
		},
	}, tool.Call{
		ID:    "call-3",
		Input: map[string]any{"text": "hello"},
	})
	if !errors.Is(err, tool.ErrInvalidOutput) {
		t.Fatalf("Invoke() error = %v want ErrInvalidOutput", err)
	}
}

func TestInvokeSupportsConcurrentUse(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	target := testTool{
		name:         "echo",
		description:  "echo text",
		inputSchema:  objectSchema("text", false),
		outputSchema: tool.Schema{Type: "string"},
		call: func(context.Context, tool.Call) (tool.Result, error) {
			calls.Add(1)
			return tool.Result{Value: "ok"}, nil
		},
	}

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)
	for index := 0; index < workers; index++ {
		index := index
		go func() {
			defer wg.Done()
			_, err := tool.Invoke(context.Background(), target, tool.Call{
				ID:    callID(index),
				Input: map[string]any{"text": "hello"},
			})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("Invoke() concurrent error = %v", err)
		}
	}
	if calls.Load() != workers {
		t.Fatalf("calls = %d want %d", calls.Load(), workers)
	}
}

type testTool struct {
	name         string
	description  string
	inputSchema  tool.Schema
	outputSchema tool.Schema
	call         func(context.Context, tool.Call) (tool.Result, error)
}

func (t testTool) Name() string        { return t.name }
func (t testTool) Description() string { return t.description }
func (t testTool) InputSchema() tool.Schema {
	return t.inputSchema
}
func (t testTool) OutputSchema() tool.Schema {
	return t.outputSchema
}
func (t testTool) Call(ctx context.Context, call tool.Call) (tool.Result, error) {
	if t.call != nil {
		return t.call(ctx, call)
	}
	return tool.Result{Value: nil}, io.EOF
}

type testToolkit struct {
	name        string
	description string
	namespace   string
	tools       []tool.Tool
}

func (t testToolkit) Name() string        { return t.name }
func (t testToolkit) Description() string { return t.description }
func (t testToolkit) Namespace() string   { return t.namespace }
func (t testToolkit) Tools() []tool.Tool  { return append([]tool.Tool(nil), t.tools...) }

func newEchoTool() tool.Tool {
	return testTool{
		name:         "echo",
		description:  "echo text",
		inputSchema:  objectSchema("text", false),
		outputSchema: tool.Schema{Type: "string"},
		call: func(_ context.Context, call tool.Call) (tool.Result, error) {
			return tool.Result{Value: call.Input["text"], Content: call.Input["text"].(string)}, nil
		},
	}
}

func newMathSumTool() tool.Tool {
	return testTool{
		name:        "sum",
		description: "sum two numbers",
		inputSchema: tool.Schema{
			Type: "object",
			Properties: map[string]tool.Schema{
				"a": {Type: "integer"},
				"b": {Type: "integer"},
			},
			Required: []string{"a", "b"},
		},
		outputSchema: tool.Schema{Type: "integer"},
		call: func(_ context.Context, call tool.Call) (tool.Result, error) {
			return tool.Result{
				Value: call.Input["a"].(int) + call.Input["b"].(int),
			}, nil
		},
	}
}

func objectSchema(required string, allowAdditional bool) tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Schema{
			required: {Type: "string"},
		},
		Required:             []string{required},
		AdditionalProperties: boolPointer(allowAdditional),
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func callID(index int) string {
	return fmt.Sprintf("call-%d", index)
}
