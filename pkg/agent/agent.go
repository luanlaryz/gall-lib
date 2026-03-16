// Package agent defines the public agent API and immutable agent configuration.
package agent

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/tool"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// DefaultMaxSteps is the built-in model step budget used by agents.
const DefaultMaxSteps = 8

var (
	agentIDSanitizer = regexp.MustCompile(`[^a-z0-9]+`)
	runSequence      atomic.Uint64
)

const reservedReasoningPrefix = "reasoning."

// Agent is an immutable, concurrency-safe public agent handle.
type Agent struct {
	id               string
	name             string
	instructions     string
	model            Model
	maxSteps         int
	tools            []tool.Tool
	memory           memory.Store
	workingMemory    memory.WorkingMemoryFactory
	inputGuardrails  []guardrail.Input
	outputGuardrails []guardrail.Output
	hooks            []Hook
	metadata         types.Metadata
	engine           Engine
}

// Config carries the mandatory configuration required to construct an agent.
type Config struct {
	Name         string
	Instructions string
	Model        Model
}

// Descriptor identifies an agent in registries and runtime accessors.
type Descriptor struct {
	Name string
	ID   string
}

// Definition is a read-only snapshot of an agent configuration.
type Definition struct {
	Descriptor       Descriptor
	Instructions     string
	Model            Model
	MaxSteps         int
	Tools            []tool.Tool
	Memory           memory.Store
	WorkingMemory    memory.WorkingMemoryFactory
	InputGuardrails  []guardrail.Input
	OutputGuardrails []guardrail.Output
	Hooks            []Hook
	Metadata         types.Metadata
}

type options struct {
	id               string
	hasID            bool
	tools            []tool.Tool
	memory           memory.Store
	workingMemory    memory.WorkingMemoryFactory
	inputGuardrails  []guardrail.Input
	outputGuardrails []guardrail.Output
	hooks            []Hook
	maxSteps         int
	hasMaxSteps      bool
	metadata         types.Metadata
	engine           Engine
}

// Option mutates agent construction settings before validation.
type Option func(*options) error

// New constructs an immutable agent from a validated public configuration.
func New(cfg Config, opts ...Option) (*Agent, error) {
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, newError(ErrorKindInvalidConfig, "new", "", "", fmt.Errorf("%w: name is required", ErrInvalidConfig))
	}
	if cfg.Model == nil {
		return nil, newError(ErrorKindInvalidConfig, "new", "", "", fmt.Errorf("%w: model is required", ErrInvalidConfig))
	}

	var resolved options
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&resolved); err != nil {
			return nil, newError(ErrorKindInvalidConfig, "new", "", "", err)
		}
	}

	maxSteps := DefaultMaxSteps
	if resolved.hasMaxSteps {
		maxSteps = resolved.maxSteps
	}

	toolRegistry := tool.NewRegistry()
	if err := toolRegistry.Register(resolved.tools...); err != nil {
		return nil, newError(ErrorKindInvalidConfig, "new", "", "", err)
	}

	clonedTools := make([]tool.Tool, 0, len(resolved.tools))
	for _, registeredTool := range resolved.tools {
		descriptor := tool.DescriptorOf(registeredTool)
		if isReservedToolName(descriptor.Name) {
			return nil, newError(ErrorKindInvalidConfig, "new", "", "", fmt.Errorf("%w: tool %q uses reserved internal namespace", ErrInvalidConfig, descriptor.Name))
		}

		frozenTool, err := toolRegistry.Resolve(descriptor.Name)
		if err != nil {
			return nil, newError(ErrorKindInvalidConfig, "new", "", "", err)
		}
		clonedTools = append(clonedTools, frozenTool)
	}

	agentID := normalizeID(cfg.Name)
	if resolved.hasID {
		agentID = resolved.id
	}

	workingFactory := resolved.workingMemory
	if workingFactory == nil {
		workingFactory = defaultWorkingMemoryFactory{}
	}

	return &Agent{
		id:               agentID,
		name:             strings.TrimSpace(cfg.Name),
		instructions:     cfg.Instructions,
		model:            cfg.Model,
		maxSteps:         maxSteps,
		tools:            clonedTools,
		memory:           resolved.memory,
		workingMemory:    workingFactory,
		inputGuardrails:  append([]guardrail.Input(nil), resolved.inputGuardrails...),
		outputGuardrails: append([]guardrail.Output(nil), resolved.outputGuardrails...),
		hooks:            append([]Hook(nil), resolved.hooks...),
		metadata:         types.CloneMetadata(resolved.metadata),
		engine:           resolved.engine,
	}, nil
}

// ID returns the stable logical identifier of the agent.
func (a *Agent) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}

// Name returns the logical name of the agent.
func (a *Agent) Name() string {
	if a == nil {
		return ""
	}
	return a.name
}

// Descriptor returns the registry descriptor for the agent.
func (a *Agent) Descriptor() Descriptor {
	return Descriptor{
		Name: a.Name(),
		ID:   a.ID(),
	}
}

// Definition returns a read-only snapshot used by the execution runtime.
func (a *Agent) Definition() Definition {
	if a == nil {
		return Definition{}
	}

	return Definition{
		Descriptor:       a.Descriptor(),
		Instructions:     a.instructions,
		Model:            a.model,
		MaxSteps:         a.maxSteps,
		Tools:            append([]tool.Tool(nil), a.tools...),
		Memory:           a.memory,
		WorkingMemory:    a.workingMemory,
		InputGuardrails:  append([]guardrail.Input(nil), a.inputGuardrails...),
		OutputGuardrails: append([]guardrail.Output(nil), a.outputGuardrails...),
		Hooks:            append([]Hook(nil), a.hooks...),
		Metadata:         types.CloneMetadata(a.metadata),
	}
}

// Run executes a complete synchronous agent run.
func (a *Agent) Run(ctx context.Context, req Request) (Response, error) {
	if a == nil {
		return Response{}, newError(ErrorKindInvalidConfig, "run", "", "", fmt.Errorf("%w: agent is nil", ErrInvalidConfig))
	}
	if a.engine == nil {
		return Response{}, newError(ErrorKindNoEngine, "run", a.id, req.RunID, ErrNoExecutionEngine)
	}

	normalized, err := a.normalizeRequest("run", ctx, req)
	if err != nil {
		return Response{}, err
	}
	return a.engine.Run(ctx, a, normalized)
}

// Stream executes a run and exposes ordered public events.
func (a *Agent) Stream(ctx context.Context, req Request) (Stream, error) {
	if a == nil {
		return nil, newError(ErrorKindInvalidConfig, "stream", "", "", fmt.Errorf("%w: agent is nil", ErrInvalidConfig))
	}
	if a.engine == nil {
		return nil, newError(ErrorKindNoEngine, "stream", a.id, req.RunID, ErrNoExecutionEngine)
	}

	normalized, err := a.normalizeRequest("stream", ctx, req)
	if err != nil {
		return nil, err
	}
	return a.engine.Stream(ctx, a, normalized)
}

// Request is the public input envelope for agent execution.
type Request struct {
	RunID        string
	SessionID    string
	Messages     []types.Message
	Metadata     types.Metadata
	MaxSteps     int
	ToolChoice   ToolChoice
	AllowedTools []string
}

// Response is the public output envelope for agent execution.
type Response struct {
	RunID              string
	AgentID            string
	SessionID          string
	Message            types.Message
	Usage              types.Usage
	ToolCalls          []ToolCallRecord
	GuardrailDecisions []GuardrailDecision
	Metadata           types.Metadata
}

// Stream exposes ordered agent execution events.
type Stream interface {
	Recv() (Event, error)
	Close() error
}

// Event is the stable local observability event emitted by a run.
type Event struct {
	Sequence  int64
	Type      EventType
	RunID     string
	AgentID   string
	SessionID string
	Time      time.Time
	Delta     *types.MessageDelta
	ToolCall  *ToolCallEvent
	Guardrail *GuardrailEvent
	Response  *Response
	Err       error
	Metadata  types.Metadata
}

// Hook observes agent events without controlling the run.
type Hook interface {
	OnEvent(ctx context.Context, event Event)
}

// Engine executes agent runs and streams.
type Engine interface {
	Run(ctx context.Context, agent *Agent, req Request) (Response, error)
	Stream(ctx context.Context, agent *Agent, req Request) (Stream, error)
}

// ToolChoice controls whether tools are available during a run.
type ToolChoice string

const (
	// ToolChoiceAuto allows the runtime to use registered tools normally.
	ToolChoiceAuto ToolChoice = "auto"
	// ToolChoiceNone disables tool calls for the run.
	ToolChoiceNone ToolChoice = "none"
	// ToolChoiceRequired requires at least one successful tool call before completion.
	ToolChoiceRequired ToolChoice = "required"
)

// EventType identifies the public kind of an execution event.
type EventType string

const (
	// EventAgentStarted is emitted when a run begins.
	EventAgentStarted EventType = "agent.started"
	// EventAgentDelta is emitted for partial model deltas.
	EventAgentDelta EventType = "agent.delta"
	// EventToolCall is emitted when a tool call starts.
	EventToolCall EventType = "agent.tool_call"
	// EventToolResult is emitted when a tool call completes.
	EventToolResult EventType = "agent.tool_result"
	// EventGuardrail is emitted for an observable guardrail decision.
	EventGuardrail EventType = "agent.guardrail"
	// EventAgentCompleted is emitted on successful completion.
	EventAgentCompleted EventType = "agent.completed"
	// EventAgentFailed is emitted on non-cancellation failure.
	EventAgentFailed EventType = "agent.failed"
	// EventAgentCanceled is emitted when a run is canceled.
	EventAgentCanceled EventType = "agent.canceled"
)

// ToolCallRecord is the observable record of a tool execution.
type ToolCallRecord struct {
	ID       string
	Name     string
	Input    map[string]any
	Output   tool.Result
	Duration time.Duration
}

// ToolCallEvent carries tool call progress information.
type ToolCallEvent struct {
	Call   ToolCallRecord
	Status ToolCallStatus
}

// ToolCallStatus identifies the observable state of a tool call.
type ToolCallStatus string

const (
	// ToolCallStarted indicates that the runtime is about to invoke the tool.
	ToolCallStarted ToolCallStatus = "started"
	// ToolCallSucceeded indicates that the tool returned successfully.
	ToolCallSucceeded ToolCallStatus = "succeeded"
	// ToolCallFailed indicates that the tool returned an error.
	ToolCallFailed ToolCallStatus = "failed"
)

// GuardrailDecision captures an observable guardrail outcome.
type GuardrailDecision struct {
	Phase    GuardrailPhase
	Name     string
	Action   guardrail.Action
	Reason   string
	Metadata types.Metadata
}

// GuardrailEvent wraps a guardrail decision for streaming and hooks.
type GuardrailEvent struct {
	Decision GuardrailDecision
}

// GuardrailPhase identifies whether a decision happened on input or output.
type GuardrailPhase string

const (
	// GuardrailPhaseInput identifies input-stage guardrail decisions.
	GuardrailPhaseInput GuardrailPhase = "input"
	// GuardrailPhaseOutput identifies output-stage guardrail decisions.
	GuardrailPhaseOutput GuardrailPhase = "output"
)

// ToolSpec is the lightweight tool metadata exposed to models.
type ToolSpec struct {
	Name        string
	Description string
}

// ModelToolCall is a tool call requested by the model.
type ModelToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ModelRequest is the public request envelope passed to models.
type ModelRequest struct {
	AgentID      string
	RunID        string
	SessionID    string
	Instructions string
	Messages     []types.Message
	Memory       memory.Snapshot
	Metadata     types.Metadata
	MaxSteps     int
	ToolChoice   ToolChoice
	AllowedTools []string
	Tools        []ToolSpec
}

// ModelResponse is the public response envelope returned by models.
type ModelResponse struct {
	Message   types.Message
	Usage     types.Usage
	ToolCalls []ModelToolCall
	Metadata  types.Metadata
}

// ModelEvent is the public event envelope returned by streaming models.
type ModelEvent struct {
	Delta    *types.MessageDelta
	Message  *types.Message
	ToolCall *ModelToolCall
	Usage    types.Usage
	Done     bool
}

// Model is the public model contract consumed by agents.
type Model interface {
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)
	Stream(ctx context.Context, req ModelRequest) (ModelStream, error)
}

// ModelStream exposes streaming model events.
type ModelStream interface {
	Recv() (ModelEvent, error)
	Close() error
}

// WithID sets a stable logical identifier for the agent.
func WithID(id string) Option {
	return func(opts *options) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("%w: id cannot be empty", ErrInvalidConfig)
		}
		opts.id = id
		opts.hasID = true
		return nil
	}
}

// WithTools registers tools available to the agent.
func WithTools(tools ...tool.Tool) Option {
	return func(opts *options) error {
		opts.tools = append(opts.tools, tools...)
		return nil
	}
}

// WithMemory enables persistent memory for the agent.
func WithMemory(store memory.Store) Option {
	return func(opts *options) error {
		if store == nil {
			return fmt.Errorf("%w: memory store cannot be nil", ErrInvalidConfig)
		}
		opts.memory = store
		return nil
	}
}

// WithWorkingMemory overrides the run-local working-memory factory.
func WithWorkingMemory(factory memory.WorkingMemoryFactory) Option {
	return func(opts *options) error {
		if factory == nil {
			return fmt.Errorf("%w: working memory factory cannot be nil", ErrInvalidConfig)
		}
		opts.workingMemory = factory
		return nil
	}
}

// WithInputGuardrails registers input guardrails in order.
func WithInputGuardrails(guardrails ...guardrail.Input) Option {
	return func(opts *options) error {
		opts.inputGuardrails = append(opts.inputGuardrails, guardrails...)
		return nil
	}
}

// WithOutputGuardrails registers output guardrails in order.
func WithOutputGuardrails(guardrails ...guardrail.Output) Option {
	return func(opts *options) error {
		opts.outputGuardrails = append(opts.outputGuardrails, guardrails...)
		return nil
	}
}

// WithHooks registers observable agent hooks in order.
func WithHooks(hooks ...Hook) Option {
	return func(opts *options) error {
		opts.hooks = append(opts.hooks, hooks...)
		return nil
	}
}

// WithMaxSteps overrides the default maximum number of model iterations.
func WithMaxSteps(n int) Option {
	return func(opts *options) error {
		if n <= 0 {
			return fmt.Errorf("%w: max steps must be positive", ErrInvalidConfig)
		}
		opts.maxSteps = n
		opts.hasMaxSteps = true
		return nil
	}
}

// WithMetadata sets default metadata for the agent.
func WithMetadata(md types.Metadata) Option {
	return func(opts *options) error {
		opts.metadata = types.CloneMetadata(md)
		return nil
	}
}

// WithExecutionEngine sets the execution engine used by Run and Stream.
func WithExecutionEngine(engine Engine) Option {
	return func(opts *options) error {
		if engine == nil {
			return fmt.Errorf("%w: execution engine cannot be nil", ErrInvalidConfig)
		}
		opts.engine = engine
		return nil
	}
}

func (a *Agent) normalizeRequest(op string, ctx context.Context, req Request) (Request, error) {
	if ctx == nil {
		return Request{}, newError(ErrorKindInvalidRequest, op, a.id, req.RunID, fmt.Errorf("%w: context is required", ErrInvalidRequest))
	}
	if req.MaxSteps < 0 {
		return Request{}, newError(ErrorKindInvalidRequest, op, a.id, req.RunID, fmt.Errorf("%w: max steps cannot be negative", ErrInvalidRequest))
	}
	if a.memory != nil && strings.TrimSpace(req.SessionID) == "" {
		return Request{}, newError(ErrorKindInvalidRequest, op, a.id, req.RunID, fmt.Errorf("%w: session id is required when memory is configured", ErrInvalidRequest))
	}

	effective := Request{
		RunID:        strings.TrimSpace(req.RunID),
		SessionID:    strings.TrimSpace(req.SessionID),
		Messages:     types.CloneMessages(req.Messages),
		Metadata:     types.MergeMetadata(a.metadata, req.Metadata),
		MaxSteps:     a.maxSteps,
		ToolChoice:   req.ToolChoice,
		AllowedTools: append([]string(nil), req.AllowedTools...),
	}

	if effective.RunID == "" {
		effective.RunID = newRunID(a.id)
	}
	if effective.ToolChoice == "" {
		effective.ToolChoice = ToolChoiceAuto
	}
	if req.MaxSteps > 0 && req.MaxSteps < effective.MaxSteps {
		effective.MaxSteps = req.MaxSteps
	}
	if req.MaxSteps > effective.MaxSteps {
		effective.MaxSteps = a.maxSteps
	}

	allowedSet := make(map[string]struct{}, len(effective.AllowedTools))
	for _, name := range effective.AllowedTools {
		name = strings.TrimSpace(name)
		if name == "" {
			return Request{}, newError(ErrorKindInvalidRequest, op, a.id, effective.RunID, fmt.Errorf("%w: allowed tool name cannot be empty", ErrInvalidRequest))
		}
		if isReservedToolName(name) {
			return Request{}, newError(ErrorKindInvalidRequest, op, a.id, effective.RunID, fmt.Errorf("%w: allowed tool %q uses reserved internal namespace", ErrInvalidRequest, name))
		}
		if _, exists := allowedSet[name]; exists {
			continue
		}
		if !a.hasTool(name) {
			return Request{}, newError(ErrorKindInvalidRequest, op, a.id, effective.RunID, fmt.Errorf("%w: unknown allowed tool %q", ErrInvalidRequest, name))
		}
		allowedSet[name] = struct{}{}
	}

	return effective, nil
}

func (a *Agent) hasTool(name string) bool {
	for _, registeredTool := range a.tools {
		if tool.DescriptorOf(registeredTool).Name == name {
			return true
		}
	}
	return false
}

func isReservedToolName(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, reservedReasoningPrefix)
}

func normalizeID(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = agentIDSanitizer.ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	if id == "" {
		return "agent"
	}
	return id
}

func newRunID(agentID string) string {
	return fmt.Sprintf("%s-run-%d", agentID, runSequence.Add(1))
}

func newError(kind ErrorKind, op, agentID, runID string, cause error) *Error {
	return &Error{
		Kind:    kind,
		Op:      op,
		AgentID: agentID,
		RunID:   runID,
		Cause:   cause,
	}
}

type defaultWorkingMemoryFactory struct{}

func (defaultWorkingMemoryFactory) NewRunState(context.Context, string, string) (memory.WorkingSet, error) {
	return &defaultWorkingSet{}, nil
}

type defaultWorkingSet struct {
	mu       sync.Mutex
	messages []types.Message
	records  []memory.Record
}

func (w *defaultWorkingSet) AddMessage(msg types.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.messages = append(w.messages, types.CloneMessage(msg))
}

func (w *defaultWorkingSet) AddRecord(record memory.Record) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.records = append(w.records, cloneRecord(record))
}

func (w *defaultWorkingSet) Snapshot() memory.Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := memory.Snapshot{
		Messages: types.CloneMessages(w.messages),
	}
	if len(w.records) > 0 {
		out.Records = make([]memory.Record, len(w.records))
		for index, record := range w.records {
			out.Records[index] = cloneRecord(record)
		}
	}
	return out
}

func cloneRecord(record memory.Record) memory.Record {
	out := memory.Record{
		Kind: record.Kind,
		Name: record.Name,
	}
	if len(record.Data) > 0 {
		out.Data = make(map[string]any, len(record.Data))
		for key, value := range record.Data {
			out.Data[key] = value
		}
	}
	return out
}

var _ Stream = (*streamClosed)(nil)

type streamClosed struct{}

func (streamClosed) Recv() (Event, error) { return Event{}, io.EOF }
func (streamClosed) Close() error         { return nil }
