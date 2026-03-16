// Package app defines the public application composition root for gaal-lib.
package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	coreruntime "github.com/luanlima/gaal-lib/internal/runtime"
	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

const defaultShutdownTimeout = 30 * time.Second

// App is the concurrency-safe public composition root.
type App struct {
	name     string
	logger   logger.Logger
	defaults Defaults

	mu                sync.RWMutex
	state             State
	hooks             []Hook
	servers           []Server
	serverlessHooks   []ServerlessHook
	readyAgents       map[string]*agent.Agent
	agentFactories    map[string]AgentFactory
	readyWorkflows    map[string]workflow.Workflow
	workflowFactories map[string]WorkflowFactory
	startedAgents     map[string]*agent.Agent
	startedWorkflows  map[string]workflow.Workflow
	startedServers    []Server
	startCh           chan struct{}
	startErr          error
	coldStarted       bool

	runtime   Runtime
	agentsReg AgentRegistry
	flowsReg  WorkflowRegistry
}

// Config configures a new App instance.
type Config struct {
	Name     string
	Defaults Defaults
}

// Defaults groups global runtime defaults frozen at construction time.
type Defaults struct {
	Metadata        types.Metadata
	Logger          logger.Logger
	ShutdownTimeout time.Duration
	Agent           AgentDefaults
	Workflow        WorkflowDefaults
}

// AgentDefaults carries default settings applied to agent factories.
type AgentDefaults struct {
	MaxSteps         int
	Metadata         types.Metadata
	Engine           agent.Engine
	Memory           memory.Store
	WorkingMemory    memory.WorkingMemoryFactory
	InputGuardrails  []guardrail.Input
	OutputGuardrails []guardrail.Output
	Hooks            []agent.Hook
}

// WorkflowDefaults carries default settings applied to workflow factories.
type WorkflowDefaults struct {
	Metadata types.Metadata
	Hooks    []workflow.Hook
	History  workflow.HistorySink
	Retry    workflow.RetryPolicy
}

type options struct {
	agents            []*agent.Agent
	agentFactories    []AgentFactory
	workflows         []workflow.Workflow
	workflowFactories []WorkflowFactory
	logger            logger.Logger
	defaults          Defaults
	hasDefaults       bool
	hooks             []Hook
	servers           []Server
	serverlessHooks   []ServerlessHook
}

// Option mutates app construction settings before validation.
type Option func(*options) error

// New constructs an App in StateCreated without starting the runtime.
func New(cfg Config, opts ...Option) (*App, error) {
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidConfig)
	}

	var resolved options
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&resolved); err != nil {
			return nil, err
		}
	}

	defaults := mergeDefaults(builtinDefaults(), cfg.Defaults)
	if resolved.hasDefaults {
		defaults = mergeDefaults(defaults, resolved.defaults)
	}
	if resolved.logger != nil {
		defaults.Logger = resolved.logger
	}
	defaults = finalizeDefaults(defaults)

	app := &App{
		name:              name,
		logger:            defaults.Logger,
		defaults:          defaults,
		state:             StateCreated,
		hooks:             append([]Hook(nil), resolved.hooks...),
		servers:           append([]Server(nil), resolved.servers...),
		serverlessHooks:   append([]ServerlessHook(nil), resolved.serverlessHooks...),
		readyAgents:       make(map[string]*agent.Agent),
		agentFactories:    make(map[string]AgentFactory),
		readyWorkflows:    make(map[string]workflow.Workflow),
		workflowFactories: make(map[string]WorkflowFactory),
		runtime:           &runtimeView{},
		agentsReg:         &agentRegistryView{},
		flowsReg:          &workflowRegistryView{},
	}

	app.runtime.(*runtimeView).app = app
	app.agentsReg.(*agentRegistryView).app = app
	app.flowsReg.(*workflowRegistryView).app = app

	for _, registeredAgent := range resolved.agents {
		if err := app.registerReadyAgent(registeredAgent); err != nil {
			return nil, err
		}
	}
	for _, factory := range resolved.agentFactories {
		if err := app.registerAgentFactory(factory); err != nil {
			return nil, err
		}
	}
	for _, registeredWorkflow := range resolved.workflows {
		if err := app.registerReadyWorkflow(registeredWorkflow); err != nil {
			return nil, err
		}
	}
	for _, factory := range resolved.workflowFactories {
		if err := app.registerWorkflowFactory(factory); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// Name returns the logical application name.
func (a *App) Name() string {
	if a == nil {
		return ""
	}
	return a.name
}

// State returns the current lifecycle state.
func (a *App) State() State {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.state
}

// Start is the explicit lifecycle entrypoint and is idempotent.
func (a *App) Start(ctx context.Context) error {
	return a.EnsureStarted(ctx)
}

// EnsureStarted starts the app once and is safe for concurrent callers.
func (a *App) EnsureStarted(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidConfig)
	}

	for {
		a.mu.Lock()
		switch a.state {
		case StateRunning:
			a.mu.Unlock()
			return nil
		case StateStarting:
			waitCh := a.startCh
			a.mu.Unlock()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-waitCh:
			}
			continue
		case StateCreated:
			a.state = StateStarting
			a.startCh = make(chan struct{})
			waitCh := a.startCh
			a.mu.Unlock()

			err := a.start(ctx)

			a.mu.Lock()
			a.startErr = err
			close(waitCh)
			a.startCh = nil
			a.mu.Unlock()

			return err
		case StateStopping:
			a.mu.Unlock()
			return fmt.Errorf("%w: app is stopping", ErrAppStopped)
		case StateStopped:
			err := a.startErr
			a.mu.Unlock()
			if err != nil {
				return err
			}
			return ErrAppStopped
		default:
			a.mu.Unlock()
			return fmt.Errorf("%w: unknown state %q", ErrInvalidConfig, a.state)
		}
	}
}

// Run starts the app, blocks until ctx is canceled, and then performs shutdown.
func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidConfig)
	}
	if err := a.EnsureStarted(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	shutdownCtx := context.Background()
	if timeout := a.defaults.ShutdownTimeout; timeout > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(shutdownCtx, timeout)
		defer cancel()
	}

	if err := a.Shutdown(shutdownCtx); err != nil {
		return err
	}

	return ctx.Err()
}

// Shutdown performs a cooperative, idempotent shutdown.
func (a *App) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidConfig)
	}

	a.mu.Lock()
	switch a.state {
	case StateStopped:
		a.mu.Unlock()
		return nil
	case StateCreated:
		a.state = StateStopped
		a.mu.Unlock()
		return nil
	case StateStopping:
		a.mu.Unlock()
		return nil
	case StateStarting, StateRunning:
		a.state = StateStopping
		startedServers := append([]Server(nil), a.startedServers...)
		a.mu.Unlock()

		a.emitEvent(ctx, EventAppStopping, nil, nil)

		shutdownCtx, cancel := a.shutdownContext(ctx)
		defer cancel()

		var firstErr error
		for index := len(startedServers) - 1; index >= 0; index-- {
			if err := startedServers[index].Shutdown(shutdownCtx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if err := shutdownCtx.Err(); err != nil {
			firstErr = err
		}

		a.mu.Lock()
		a.state = StateStopped
		a.startedServers = nil
		a.mu.Unlock()

		a.emitEvent(ctx, EventAppStopped, firstErr, nil)
		return firstErr
	default:
		a.mu.Unlock()
		return nil
	}
}

// Logger returns the global application logger.
func (a *App) Logger() logger.Logger {
	if a == nil || a.logger == nil {
		return logger.Nop()
	}
	return a.logger
}

// Defaults returns a read-only snapshot of frozen defaults.
func (a *App) Defaults() DefaultsSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return snapshotDefaults(a.defaults)
}

// Runtime returns a read-only runtime view for adapters and composition.
func (a *App) Runtime() Runtime {
	return a.runtime
}

// Agents returns the public agent registry view.
func (a *App) Agents() AgentRegistry {
	return a.agentsReg
}

// Workflows returns the public workflow registry view.
func (a *App) Workflows() WorkflowRegistry {
	return a.flowsReg
}

// State identifies the lifecycle state of an App.
type State string

const (
	// StateCreated means New succeeded and startup has not begun.
	StateCreated State = "created"
	// StateStarting means startup is in progress.
	StateStarting State = "starting"
	// StateRunning means startup completed successfully.
	StateRunning State = "running"
	// StateStopping means shutdown is in progress.
	StateStopping State = "stopping"
	// StateStopped means the app has been shut down or startup failed.
	StateStopped State = "stopped"
)

// DefaultsSnapshot is the read-only public snapshot of App defaults.
type DefaultsSnapshot struct {
	Metadata        types.Metadata
	ShutdownTimeout time.Duration
	Agent           AgentDefaultsSnapshot
	Workflow        WorkflowDefaultsSnapshot
}

// AgentDefaultsSnapshot is the read-only public snapshot of agent defaults.
type AgentDefaultsSnapshot struct {
	MaxSteps  int
	Metadata  types.Metadata
	HasEngine bool
	HasMemory bool
}

// WorkflowDefaultsSnapshot is the read-only public snapshot of workflow defaults.
type WorkflowDefaultsSnapshot struct {
	Metadata   types.Metadata
	HasHistory bool
	HasRetry   bool
}

// Runtime exposes a read-only view of the materialized runtime.
type Runtime interface {
	State() State
	Logger() logger.Logger
	Defaults() DefaultsSnapshot
	ResolveAgent(name string) (*agent.Agent, error)
	ResolveWorkflow(name string) (workflow.Workflow, error)
	ListAgents() []agent.Descriptor
	ListWorkflows() []workflow.Descriptor
}

// AgentRegistry exposes the mutable bootstrap registry for agents.
type AgentRegistry interface {
	Register(agent *agent.Agent) error
	RegisterFactory(factory AgentFactory) error
	Resolve(name string) (*agent.Agent, error)
	List() []agent.Descriptor
}

// WorkflowRegistry exposes the mutable bootstrap registry for workflows.
type WorkflowRegistry interface {
	Register(workflow workflow.Workflow) error
	RegisterFactory(factory WorkflowFactory) error
	Resolve(name string) (workflow.Workflow, error)
	List() []workflow.Descriptor
}

// AgentFactory materializes an agent using frozen global defaults.
type AgentFactory interface {
	Name() string
	Build(ctx context.Context, defaults AgentDefaults) (*agent.Agent, error)
}

// WorkflowFactory materializes a workflow using frozen global defaults.
type WorkflowFactory interface {
	Name() string
	Build(ctx context.Context, defaults WorkflowDefaults) (workflow.Workflow, error)
}

// Hook observes app lifecycle and bootstrap events.
type Hook interface {
	OnEvent(ctx context.Context, event Event)
}

// Event is the public lifecycle event observed by app hooks.
type Event struct {
	Type     EventType
	AppName  string
	Time     time.Time
	Err      error
	Metadata types.Metadata
}

// EventType identifies the kind of an app event.
type EventType string

const (
	// EventAppStarting is emitted when startup begins.
	EventAppStarting EventType = "app.starting"
	// EventAppStarted is emitted when startup completes.
	EventAppStarted EventType = "app.started"
	// EventAppStopping is emitted when shutdown begins.
	EventAppStopping EventType = "app.stopping"
	// EventAppStopped is emitted when shutdown completes.
	EventAppStopped EventType = "app.stopped"
	// EventAgentRegistered is emitted for materialized agents during bootstrap.
	EventAgentRegistered EventType = "app.agent_registered"
	// EventWorkflowRegistered is emitted for materialized workflows during bootstrap.
	EventWorkflowRegistered EventType = "app.workflow_registered"
	// EventBootstrapFailed is emitted when startup fails.
	EventBootstrapFailed EventType = "app.bootstrap_failed"
)

// Server is the minimal contract for a long-lived managed server.
type Server interface {
	Name() string
	Start(ctx context.Context, rt Runtime) error
	Shutdown(ctx context.Context) error
}

// ServerlessHook observes cold starts and short-lived invocations.
type ServerlessHook interface {
	OnColdStart(ctx context.Context, rt Runtime) error
	OnInvokeStart(ctx context.Context, target Target) (context.Context, error)
	OnInvokeDone(ctx context.Context, target Target, err error)
}

// Target identifies a serverless invocation target.
type Target struct {
	Kind string
	Name string
}

// WithAgents registers ready-made agents for bootstrap.
func WithAgents(agents ...*agent.Agent) Option {
	return func(opts *options) error {
		opts.agents = append(opts.agents, agents...)
		return nil
	}
}

// WithAgentFactories registers agent factories for startup materialization.
func WithAgentFactories(factories ...AgentFactory) Option {
	return func(opts *options) error {
		opts.agentFactories = append(opts.agentFactories, factories...)
		return nil
	}
}

// WithWorkflows registers ready-made workflows for bootstrap.
func WithWorkflows(workflows ...workflow.Workflow) Option {
	return func(opts *options) error {
		opts.workflows = append(opts.workflows, workflows...)
		return nil
	}
}

// WithWorkflowFactories registers workflow factories for startup materialization.
func WithWorkflowFactories(factories ...WorkflowFactory) Option {
	return func(opts *options) error {
		opts.workflowFactories = append(opts.workflowFactories, factories...)
		return nil
	}
}

// WithLogger overrides the global application logger.
func WithLogger(log logger.Logger) Option {
	return func(opts *options) error {
		if log == nil {
			return fmt.Errorf("%w: logger cannot be nil", ErrInvalidConfig)
		}
		opts.logger = log
		return nil
	}
}

// WithDefaults merges additional global defaults into the app configuration.
func WithDefaults(defaults Defaults) Option {
	return func(opts *options) error {
		opts.defaults = mergeDefaults(opts.defaults, defaults)
		opts.hasDefaults = true
		return nil
	}
}

// WithAppHooks registers app lifecycle hooks in order.
func WithAppHooks(hooks ...Hook) Option {
	return func(opts *options) error {
		opts.hooks = append(opts.hooks, hooks...)
		return nil
	}
}

// WithServers registers managed long-lived servers.
func WithServers(servers ...Server) Option {
	return func(opts *options) error {
		opts.servers = append(opts.servers, servers...)
		return nil
	}
}

// WithServerlessHooks registers serverless lifecycle hooks.
func WithServerlessHooks(hooks ...ServerlessHook) Option {
	return func(opts *options) error {
		opts.serverlessHooks = append(opts.serverlessHooks, hooks...)
		return nil
	}
}

type runtimeView struct {
	app *App
}

func (r *runtimeView) State() State {
	return r.app.State()
}

func (r *runtimeView) Logger() logger.Logger {
	return r.app.Logger()
}

func (r *runtimeView) Defaults() DefaultsSnapshot {
	return r.app.Defaults()
}

func (r *runtimeView) ResolveAgent(name string) (*agent.Agent, error) {
	return r.app.Agents().Resolve(name)
}

func (r *runtimeView) ResolveWorkflow(name string) (workflow.Workflow, error) {
	return r.app.Workflows().Resolve(name)
}

func (r *runtimeView) ListAgents() []agent.Descriptor {
	return r.app.Agents().List()
}

func (r *runtimeView) ListWorkflows() []workflow.Descriptor {
	return r.app.Workflows().List()
}

type agentRegistryView struct {
	app *App
}

func (r *agentRegistryView) Register(registeredAgent *agent.Agent) error {
	return r.app.registerReadyAgent(registeredAgent)
}

func (r *agentRegistryView) RegisterFactory(factory AgentFactory) error {
	return r.app.registerAgentFactory(factory)
}

func (r *agentRegistryView) Resolve(name string) (*agent.Agent, error) {
	return r.app.resolveAgent(name)
}

func (r *agentRegistryView) List() []agent.Descriptor {
	return r.app.listAgents()
}

type workflowRegistryView struct {
	app *App
}

func (r *workflowRegistryView) Register(registeredWorkflow workflow.Workflow) error {
	return r.app.registerReadyWorkflow(registeredWorkflow)
}

func (r *workflowRegistryView) RegisterFactory(factory WorkflowFactory) error {
	return r.app.registerWorkflowFactory(factory)
}

func (r *workflowRegistryView) Resolve(name string) (workflow.Workflow, error) {
	return r.app.resolveWorkflow(name)
}

func (r *workflowRegistryView) List() []workflow.Descriptor {
	return r.app.listWorkflows()
}

func (a *App) registerReadyAgent(registeredAgent *agent.Agent) error {
	if registeredAgent == nil {
		return fmt.Errorf("%w: agent cannot be nil", ErrInvalidConfig)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateCreated {
		return ErrRegistrySealed
	}

	name := strings.TrimSpace(registeredAgent.Name())
	if name == "" {
		return fmt.Errorf("%w: agent name is required", ErrInvalidConfig)
	}
	if _, exists := a.readyAgents[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateAgent, name)
	}

	a.readyAgents[name] = registeredAgent
	return nil
}

func (a *App) registerAgentFactory(factory AgentFactory) error {
	if factory == nil {
		return fmt.Errorf("%w: agent factory cannot be nil", ErrInvalidConfig)
	}

	name := strings.TrimSpace(factory.Name())
	if name == "" {
		return fmt.Errorf("%w: agent factory name is required", ErrInvalidConfig)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateCreated {
		return ErrRegistrySealed
	}
	if _, exists := a.agentFactories[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateAgent, name)
	}

	a.agentFactories[name] = factory
	return nil
}

func (a *App) registerReadyWorkflow(registeredWorkflow workflow.Workflow) error {
	if registeredWorkflow == nil {
		return fmt.Errorf("%w: workflow cannot be nil", ErrInvalidConfig)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateCreated {
		return ErrRegistrySealed
	}

	name := strings.TrimSpace(registeredWorkflow.Name())
	if name == "" {
		return fmt.Errorf("%w: workflow name is required", ErrInvalidConfig)
	}
	if _, exists := a.readyWorkflows[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateWorkflow, name)
	}

	a.readyWorkflows[name] = registeredWorkflow
	return nil
}

func (a *App) registerWorkflowFactory(factory WorkflowFactory) error {
	if factory == nil {
		return fmt.Errorf("%w: workflow factory cannot be nil", ErrInvalidConfig)
	}

	name := strings.TrimSpace(factory.Name())
	if name == "" {
		return fmt.Errorf("%w: workflow factory name is required", ErrInvalidConfig)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state != StateCreated {
		return ErrRegistrySealed
	}
	if _, exists := a.workflowFactories[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateWorkflow, name)
	}

	a.workflowFactories[name] = factory
	return nil
}

func (a *App) resolveAgent(name string) (*agent.Agent, error) {
	name = strings.TrimSpace(name)

	a.mu.RLock()
	defer a.mu.RUnlock()

	if started := a.startedAgents; len(started) > 0 {
		if registeredAgent, ok := started[name]; ok {
			return registeredAgent, nil
		}
	}
	if registeredAgent, ok := a.readyAgents[name]; ok {
		return registeredAgent, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
}

func (a *App) resolveWorkflow(name string) (workflow.Workflow, error) {
	name = strings.TrimSpace(name)

	a.mu.RLock()
	defer a.mu.RUnlock()

	if started := a.startedWorkflows; len(started) > 0 {
		if registeredWorkflow, ok := started[name]; ok {
			return registeredWorkflow, nil
		}
	}
	if registeredWorkflow, ok := a.readyWorkflows[name]; ok {
		return registeredWorkflow, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrWorkflowNotFound, name)
}

func (a *App) listAgents() []agent.Descriptor {
	a.mu.RLock()
	defer a.mu.RUnlock()

	source := a.readyAgents
	if len(a.startedAgents) > 0 {
		source = a.startedAgents
	}

	descriptors := make([]agent.Descriptor, 0, len(source))
	for _, registeredAgent := range source {
		descriptors = append(descriptors, registeredAgent.Descriptor())
	}
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Name < descriptors[j].Name
	})
	return descriptors
}

func (a *App) listWorkflows() []workflow.Descriptor {
	a.mu.RLock()
	defer a.mu.RUnlock()

	source := a.readyWorkflows
	if len(a.startedWorkflows) > 0 {
		source = a.startedWorkflows
	}

	descriptors := make([]workflow.Descriptor, 0, len(source))
	for name := range source {
		descriptors = append(descriptors, workflow.Descriptor{
			Name: name,
			ID:   name,
		})
	}
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Name < descriptors[j].Name
	})
	return descriptors
}

func (a *App) start(ctx context.Context) error {
	a.emitEvent(ctx, EventAppStarting, nil, nil)

	startedAgents, err := a.materializeAgents(ctx)
	if err != nil {
		a.failStart(ctx, err)
		return err
	}

	startedWorkflows, err := a.materializeWorkflows(ctx)
	if err != nil {
		a.failStart(ctx, err)
		return err
	}

	a.mu.Lock()
	a.startedAgents = startedAgents
	a.startedWorkflows = startedWorkflows
	a.mu.Unlock()

	for _, descriptor := range a.listAgents() {
		a.emitEvent(ctx, EventAgentRegistered, nil, types.Metadata{"agent_name": descriptor.Name})
	}
	for _, descriptor := range a.listWorkflows() {
		a.emitEvent(ctx, EventWorkflowRegistered, nil, types.Metadata{"workflow_name": descriptor.Name})
	}

	startedServers := make([]Server, 0, len(a.servers))
	for _, server := range a.servers {
		if server == nil {
			continue
		}
		if err := server.Start(ctx, a.Runtime()); err != nil {
			rollbackErr := a.rollbackStartedServers(ctx, startedServers)
			a.mu.Lock()
			a.startedAgents = nil
			a.startedWorkflows = nil
			a.state = StateStopped
			a.mu.Unlock()

			if rollbackErr != nil {
				err = fmt.Errorf("%w; rollback: %v", err, rollbackErr)
			}
			a.failStart(ctx, err)
			return err
		}
		startedServers = append(startedServers, server)
	}

	if err := a.runColdStartHooks(ctx); err != nil {
		rollbackErr := a.rollbackStartedServers(ctx, startedServers)
		a.mu.Lock()
		a.startedAgents = nil
		a.startedWorkflows = nil
		a.state = StateStopped
		a.mu.Unlock()

		if rollbackErr != nil {
			err = fmt.Errorf("%w; rollback: %v", err, rollbackErr)
		}
		a.failStart(ctx, err)
		return err
	}

	a.mu.Lock()
	a.startedServers = startedServers
	a.state = StateRunning
	a.mu.Unlock()

	a.emitEvent(ctx, EventAppStarted, nil, nil)
	return nil
}

func (a *App) materializeAgents(ctx context.Context) (map[string]*agent.Agent, error) {
	a.mu.RLock()
	ready := cloneAgentMap(a.readyAgents)
	factories := cloneAgentFactoryMap(a.agentFactories)
	defaults := cloneAgentDefaults(a.defaults.Agent)
	a.mu.RUnlock()

	for name := range ready {
		if _, exists := factories[name]; exists {
			return nil, fmt.Errorf("%w: ready agent and factory share name %s", ErrDuplicateAgent, name)
		}
	}

	out := cloneAgentMap(ready)
	names := sortedAgentFactoryNames(factories)
	for _, name := range names {
		built, err := factories[name].Build(ctx, cloneAgentDefaults(defaults))
		if err != nil {
			return nil, err
		}
		if built == nil {
			return nil, fmt.Errorf("%w: agent factory %s returned nil", ErrInvalidConfig, name)
		}
		if built.Name() != name {
			return nil, fmt.Errorf("%w: agent factory %s built agent %s", ErrInvalidConfig, name, built.Name())
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateAgent, name)
		}
		out[name] = built
	}

	return out, nil
}

func (a *App) materializeWorkflows(ctx context.Context) (map[string]workflow.Workflow, error) {
	a.mu.RLock()
	ready := cloneWorkflowMap(a.readyWorkflows)
	factories := cloneWorkflowFactoryMap(a.workflowFactories)
	defaults := cloneWorkflowDefaults(a.defaults.Workflow)
	a.mu.RUnlock()

	for name := range ready {
		if _, exists := factories[name]; exists {
			return nil, fmt.Errorf("%w: ready workflow and factory share name %s", ErrDuplicateWorkflow, name)
		}
	}

	out := cloneWorkflowMap(ready)
	names := sortedWorkflowFactoryNames(factories)
	for _, name := range names {
		built, err := factories[name].Build(ctx, cloneWorkflowDefaults(defaults))
		if err != nil {
			return nil, err
		}
		if built == nil {
			return nil, fmt.Errorf("%w: workflow factory %s returned nil", ErrInvalidConfig, name)
		}
		if built.Name() != name {
			return nil, fmt.Errorf("%w: workflow factory %s built workflow %s", ErrInvalidConfig, name, built.Name())
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateWorkflow, name)
		}
		out[name] = built
	}

	return out, nil
}

func (a *App) runColdStartHooks(ctx context.Context) error {
	a.mu.Lock()
	if a.coldStarted {
		a.mu.Unlock()
		return nil
	}
	a.coldStarted = true
	hooks := append([]ServerlessHook(nil), a.serverlessHooks...)
	a.mu.Unlock()

	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		if err := hook.OnColdStart(ctx, a.Runtime()); err != nil {
			a.mu.Lock()
			a.coldStarted = false
			a.mu.Unlock()
			return err
		}
	}

	return nil
}

func (a *App) rollbackStartedServers(ctx context.Context, startedServers []Server) error {
	shutdownCtx, cancel := a.shutdownContext(ctx)
	defer cancel()

	var firstErr error
	for index := len(startedServers) - 1; index >= 0; index-- {
		if err := startedServers[index].Shutdown(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := shutdownCtx.Err(); err != nil {
		firstErr = err
	}
	return firstErr
}

func (a *App) failStart(ctx context.Context, err error) {
	a.mu.Lock()
	a.state = StateStopped
	a.mu.Unlock()
	a.emitEvent(ctx, EventBootstrapFailed, err, nil)
}

func (a *App) emitEvent(ctx context.Context, eventType EventType, err error, extra types.Metadata) {
	a.mu.RLock()
	hooks := append([]Hook(nil), a.hooks...)
	appName := a.name
	metadata := types.MergeMetadata(a.defaults.Metadata, extra)
	a.mu.RUnlock()

	event := Event{
		Type:     eventType,
		AppName:  appName,
		Time:     time.Now(),
		Err:      err,
		Metadata: metadata,
	}

	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		func(h Hook) {
			defer func() {
				_ = recover()
			}()
			h.OnEvent(ctx, event)
		}(hook)
	}
}

func (a *App) shutdownContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := a.defaults.ShutdownTimeout
	if timeout <= 0 {
		return ctx, func() {}
	}

	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= timeout {
			return ctx, func() {}
		}
	}

	return context.WithTimeout(ctx, timeout)
}

func builtinDefaults() Defaults {
	return Defaults{
		Logger:          logger.Nop(),
		ShutdownTimeout: defaultShutdownTimeout,
		Agent: AgentDefaults{
			MaxSteps:      agent.DefaultMaxSteps,
			Engine:        coreruntime.NewEngine(),
			WorkingMemory: memory.InMemoryWorkingMemoryFactory{},
		},
	}
}

func finalizeDefaults(defaults Defaults) Defaults {
	out := cloneDefaults(defaults)
	if out.Logger == nil {
		out.Logger = logger.Nop()
	}
	if out.ShutdownTimeout <= 0 {
		out.ShutdownTimeout = defaultShutdownTimeout
	}
	if out.Agent.MaxSteps <= 0 {
		out.Agent.MaxSteps = agent.DefaultMaxSteps
	}
	if out.Agent.Engine == nil {
		out.Agent.Engine = coreruntime.NewEngine()
	}
	return out
}

func mergeDefaults(base, override Defaults) Defaults {
	out := cloneDefaults(base)
	out.Metadata = types.MergeMetadata(out.Metadata, override.Metadata)
	if override.Logger != nil {
		out.Logger = override.Logger
	}
	if override.ShutdownTimeout != 0 {
		out.ShutdownTimeout = override.ShutdownTimeout
	}
	out.Agent = mergeAgentDefaults(out.Agent, override.Agent)
	out.Workflow = mergeWorkflowDefaults(out.Workflow, override.Workflow)
	return out
}

func mergeAgentDefaults(base, override AgentDefaults) AgentDefaults {
	out := cloneAgentDefaults(base)
	if override.MaxSteps != 0 {
		out.MaxSteps = override.MaxSteps
	}
	out.Metadata = types.MergeMetadata(out.Metadata, override.Metadata)
	if override.Engine != nil {
		out.Engine = override.Engine
	}
	if override.Memory != nil {
		out.Memory = override.Memory
	}
	if override.WorkingMemory != nil {
		out.WorkingMemory = override.WorkingMemory
	}
	out.InputGuardrails = append(out.InputGuardrails, override.InputGuardrails...)
	out.OutputGuardrails = append(out.OutputGuardrails, override.OutputGuardrails...)
	out.Hooks = append(out.Hooks, override.Hooks...)
	return out
}

func mergeWorkflowDefaults(base, override WorkflowDefaults) WorkflowDefaults {
	out := cloneWorkflowDefaults(base)
	out.Metadata = types.MergeMetadata(out.Metadata, override.Metadata)
	if override.History != nil {
		out.History = override.History
	}
	if override.Retry != nil {
		out.Retry = override.Retry
	}
	out.Hooks = append(out.Hooks, override.Hooks...)
	return out
}

func cloneDefaults(in Defaults) Defaults {
	return Defaults{
		Metadata:        types.CloneMetadata(in.Metadata),
		Logger:          in.Logger,
		ShutdownTimeout: in.ShutdownTimeout,
		Agent:           cloneAgentDefaults(in.Agent),
		Workflow:        cloneWorkflowDefaults(in.Workflow),
	}
}

func cloneAgentDefaults(in AgentDefaults) AgentDefaults {
	return AgentDefaults{
		MaxSteps:         in.MaxSteps,
		Metadata:         types.CloneMetadata(in.Metadata),
		Engine:           in.Engine,
		Memory:           in.Memory,
		WorkingMemory:    in.WorkingMemory,
		InputGuardrails:  append([]guardrail.Input(nil), in.InputGuardrails...),
		OutputGuardrails: append([]guardrail.Output(nil), in.OutputGuardrails...),
		Hooks:            append([]agent.Hook(nil), in.Hooks...),
	}
}

func cloneWorkflowDefaults(in WorkflowDefaults) WorkflowDefaults {
	return WorkflowDefaults{
		Metadata: types.CloneMetadata(in.Metadata),
		Hooks:    append([]workflow.Hook(nil), in.Hooks...),
		History:  in.History,
		Retry:    in.Retry,
	}
}

func snapshotDefaults(in Defaults) DefaultsSnapshot {
	return DefaultsSnapshot{
		Metadata:        types.CloneMetadata(in.Metadata),
		ShutdownTimeout: in.ShutdownTimeout,
		Agent: AgentDefaultsSnapshot{
			MaxSteps:  in.Agent.MaxSteps,
			Metadata:  types.CloneMetadata(in.Agent.Metadata),
			HasEngine: in.Agent.Engine != nil,
			HasMemory: in.Agent.Memory != nil,
		},
		Workflow: WorkflowDefaultsSnapshot{
			Metadata:   types.CloneMetadata(in.Workflow.Metadata),
			HasHistory: in.Workflow.History != nil,
			HasRetry:   in.Workflow.Retry != nil,
		},
	}
}

func cloneAgentMap(in map[string]*agent.Agent) map[string]*agent.Agent {
	out := make(map[string]*agent.Agent, len(in))
	for name, registeredAgent := range in {
		out[name] = registeredAgent
	}
	return out
}

func cloneWorkflowMap(in map[string]workflow.Workflow) map[string]workflow.Workflow {
	out := make(map[string]workflow.Workflow, len(in))
	for name, registeredWorkflow := range in {
		out[name] = registeredWorkflow
	}
	return out
}

func cloneAgentFactoryMap(in map[string]AgentFactory) map[string]AgentFactory {
	out := make(map[string]AgentFactory, len(in))
	for name, factory := range in {
		out[name] = factory
	}
	return out
}

func cloneWorkflowFactoryMap(in map[string]WorkflowFactory) map[string]WorkflowFactory {
	out := make(map[string]WorkflowFactory, len(in))
	for name, factory := range in {
		out[name] = factory
	}
	return out
}

func sortedAgentFactoryNames(in map[string]AgentFactory) []string {
	out := make([]string, 0, len(in))
	for name := range in {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func sortedWorkflowFactoryNames(in map[string]WorkflowFactory) []string {
	out := make([]string, 0, len(in))
	for name := range in {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
