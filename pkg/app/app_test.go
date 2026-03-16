package app_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/agent"
	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

func TestNewCreatesAppInCreatedState(t *testing.T) {
	t.Parallel()

	instance, err := app.New(app.Config{Name: "test-app"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if instance.State() != app.StateCreated {
		t.Fatalf("State() = %q want %q", instance.State(), app.StateCreated)
	}
	if !instance.Defaults().Agent.HasEngine {
		t.Fatal("expected default agent engine to be configured")
	}
	if instance.Logger() == nil {
		t.Fatal("Logger() returned nil")
	}
}

func TestAgentRegistryRejectsDuplicateReadyAgent(t *testing.T) {
	t.Parallel()

	registeredAgent, err := agent.New(agent.Config{Name: "dup", Model: appStubModel{}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	instance, err := app.New(app.Config{Name: "registry-app"}, app.WithAgents(registeredAgent))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = instance.Agents().Register(registeredAgent)
	if !errors.Is(err, app.ErrDuplicateAgent) {
		t.Fatalf("expected duplicate agent error, got %v", err)
	}
}

func TestStartMaterializesFactoriesDeterministically(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var buildOrder []string

	instance, err := app.New(
		app.Config{Name: "factory-app"},
		app.WithAgentFactories(
			recordingAgentFactory{name: "zeta", buildOrder: &buildOrder, mu: &mu},
			recordingAgentFactory{name: "alpha", buildOrder: &buildOrder, mu: &mu},
		),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	mu.Lock()
	gotOrder := append([]string(nil), buildOrder...)
	mu.Unlock()

	if !reflect.DeepEqual(gotOrder, []string{"alpha", "zeta"}) {
		t.Fatalf("build order = %v", gotOrder)
	}

	listed := instance.Runtime().ListAgents()
	if !reflect.DeepEqual(listed, []agent.Descriptor{{Name: "alpha", ID: "alpha"}, {Name: "zeta", ID: "zeta"}}) {
		t.Fatalf("ListAgents() = %+v", listed)
	}
}

func TestAgentFactoryReceivesBuiltInWorkingMemoryDefault(t *testing.T) {
	t.Parallel()

	factory := &memoryAwareAgentFactory{name: "memory-default"}
	instance, err := app.New(
		app.Config{Name: "memory-default-app"},
		app.WithAgentFactories(factory),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if factory.seenDefaults.WorkingMemory == nil {
		t.Fatal("expected built-in working memory default")
	}
	if _, ok := factory.seenDefaults.WorkingMemory.(memory.InMemoryWorkingMemoryFactory); !ok {
		t.Fatalf("WorkingMemory default = %T want %T", factory.seenDefaults.WorkingMemory, memory.InMemoryWorkingMemoryFactory{})
	}
	if factory.seenDefaults.Memory != nil {
		t.Fatalf("Memory default = %T want nil", factory.seenDefaults.Memory)
	}
}

func TestAgentFactoryInheritsAppMemoryDefaults(t *testing.T) {
	t.Parallel()

	store := &countingAppMemoryStore{}
	working := &countingAppWorkingMemoryFactory{}
	factory := &memoryAwareAgentFactory{name: "inherit-memory"}

	instance, err := app.New(
		app.Config{
			Name: "inherit-memory-app",
			Defaults: app.Defaults{
				Agent: app.AgentDefaults{
					Memory:        store,
					WorkingMemory: working,
				},
			},
		},
		app.WithAgentFactories(factory),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	resolved, err := instance.Runtime().ResolveAgent("inherit-memory")
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}

	_, err = resolved.Run(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if store.loadCalls != 1 || store.saveCalls != 1 {
		t.Fatalf("store calls = load:%d save:%d want 1/1", store.loadCalls, store.saveCalls)
	}
	if working.calls != 1 {
		t.Fatalf("working factory calls = %d want 1", working.calls)
	}
}

func TestAgentFactoryLocalMemoryOverridesAppDefaults(t *testing.T) {
	t.Parallel()

	globalStore := &countingAppMemoryStore{}
	globalWorking := &countingAppWorkingMemoryFactory{}
	localStore := &countingAppMemoryStore{}
	localWorking := &countingAppWorkingMemoryFactory{}
	factory := &memoryAwareAgentFactory{
		name:                "override-memory",
		localStore:          localStore,
		localWorkingFactory: localWorking,
	}

	instance, err := app.New(
		app.Config{
			Name: "override-memory-app",
			Defaults: app.Defaults{
				Agent: app.AgentDefaults{
					Memory:        globalStore,
					WorkingMemory: globalWorking,
				},
			},
		},
		app.WithAgentFactories(factory),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	resolved, err := instance.Runtime().ResolveAgent("override-memory")
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}

	_, err = resolved.Run(context.Background(), agent.Request{
		SessionID: "session-1",
		Messages:  []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if globalStore.loadCalls != 0 || globalStore.saveCalls != 0 {
		t.Fatalf("global store calls = load:%d save:%d want 0/0", globalStore.loadCalls, globalStore.saveCalls)
	}
	if globalWorking.calls != 0 {
		t.Fatalf("global working factory calls = %d want 0", globalWorking.calls)
	}
	if localStore.loadCalls != 1 || localStore.saveCalls != 1 {
		t.Fatalf("local store calls = load:%d save:%d want 1/1", localStore.loadCalls, localStore.saveCalls)
	}
	if localWorking.calls != 1 {
		t.Fatalf("local working factory calls = %d want 1", localWorking.calls)
	}
}

func TestReadyAgentIsNotMutatedByDefaults(t *testing.T) {
	t.Parallel()

	registeredAgent, err := agent.New(agent.Config{Name: "ready", Model: appStubModel{}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	instance, err := app.New(app.Config{Name: "ready-app"}, app.WithAgents(registeredAgent))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	resolved, err := instance.Runtime().ResolveAgent("ready")
	if err != nil {
		t.Fatalf("ResolveAgent() error = %v", err)
	}

	_, err = resolved.Run(context.Background(), agent.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if !errors.Is(err, agent.ErrNoExecutionEngine) {
		t.Fatalf("expected no execution engine error, got %v", err)
	}
}

func TestAppHooksRecoverPanicAndPreserveOrder(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var events []app.EventType

	panicHook := appHookFunc(func(ctx context.Context, event app.Event) {
		if event.Type == app.EventAppStarting {
			panic("boom")
		}
	})
	recordingHook := appHookFunc(func(ctx context.Context, event app.Event) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event.Type)
	})

	instance, err := app.New(
		app.Config{Name: "hooks-app"},
		app.WithAppHooks(panicHook, recordingHook),
		app.WithAgentFactories(recordingAgentFactory{name: "alpha"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := instance.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	want := []app.EventType{
		app.EventAppStarting,
		app.EventAgentRegistered,
		app.EventAppStarted,
		app.EventAppStopping,
		app.EventAppStopped,
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("hook events = %v want %v", events, want)
	}
}

func TestShutdownCreatedMovesToStopped(t *testing.T) {
	t.Parallel()

	instance, err := app.New(app.Config{Name: "shutdown-created"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if instance.State() != app.StateStopped {
		t.Fatalf("State() = %q want %q", instance.State(), app.StateStopped)
	}
}

func TestResolveWorkflowReturnsReadyWorkflow(t *testing.T) {
	t.Parallel()

	instance, err := app.New(
		app.Config{Name: "workflow-app"},
		app.WithWorkflows(stubWorkflow{name: "flow"}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	resolved, err := instance.Runtime().ResolveWorkflow("flow")
	if err != nil {
		t.Fatalf("ResolveWorkflow() error = %v", err)
	}
	if resolved.Name() != "flow" {
		t.Fatalf("workflow name = %q want %q", resolved.Name(), "flow")
	}
}

type recordingAgentFactory struct {
	name       string
	buildOrder *[]string
	mu         *sync.Mutex
}

func (f recordingAgentFactory) Name() string {
	return f.name
}

func (f recordingAgentFactory) Build(ctx context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	if f.buildOrder != nil && f.mu != nil {
		f.mu.Lock()
		*f.buildOrder = append(*f.buildOrder, f.name)
		f.mu.Unlock()
	}

	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
	}
	if len(defaults.Metadata) > 0 {
		opts = append(opts, agent.WithMetadata(defaults.Metadata))
	}

	return agent.New(agent.Config{Name: f.name, Model: appStubModel{}}, opts...)
}

type memoryAwareAgentFactory struct {
	name                string
	localStore          memory.Store
	localWorkingFactory memory.WorkingMemoryFactory
	seenDefaults        app.AgentDefaults
}

func (f *memoryAwareAgentFactory) Name() string {
	return f.name
}

func (f *memoryAwareAgentFactory) Build(_ context.Context, defaults app.AgentDefaults) (*agent.Agent, error) {
	f.seenDefaults = defaults

	opts := []agent.Option{
		agent.WithExecutionEngine(defaults.Engine),
		agent.WithMaxSteps(defaults.MaxSteps),
	}
	if len(defaults.Metadata) > 0 {
		opts = append(opts, agent.WithMetadata(defaults.Metadata))
	}

	switch {
	case f.localStore != nil:
		opts = append(opts, agent.WithMemory(f.localStore))
	case defaults.Memory != nil:
		opts = append(opts, agent.WithMemory(defaults.Memory))
	}

	switch {
	case f.localWorkingFactory != nil:
		opts = append(opts, agent.WithWorkingMemory(f.localWorkingFactory))
	case defaults.WorkingMemory != nil:
		opts = append(opts, agent.WithWorkingMemory(defaults.WorkingMemory))
	}

	return agent.New(agent.Config{Name: f.name, Model: appStubModel{}}, opts...)
}

type appStubModel struct{}

func (appStubModel) Generate(context.Context, agent.ModelRequest) (agent.ModelResponse, error) {
	return agent.ModelResponse{
		Message: types.Message{Role: types.RoleAssistant, Content: "ok"},
	}, nil
}

func (appStubModel) Stream(context.Context, agent.ModelRequest) (agent.ModelStream, error) {
	return appClosedModelStream{}, nil
}

type appClosedModelStream struct{}

func (appClosedModelStream) Recv() (agent.ModelEvent, error) {
	return agent.ModelEvent{}, context.Canceled
}
func (appClosedModelStream) Close() error { return nil }

type appHookFunc func(context.Context, app.Event)

func (f appHookFunc) OnEvent(ctx context.Context, event app.Event) {
	f(ctx, event)
}

type stubWorkflow struct {
	name string
}

func (w stubWorkflow) Name() string {
	return w.name
}

func (stubWorkflow) Run(context.Context, workflow.Request) (workflow.Response, error) {
	return workflow.Response{}, nil
}

type countingAppMemoryStore struct {
	loadCalls int
	saveCalls int
}

func (s *countingAppMemoryStore) Load(context.Context, string) (memory.Snapshot, error) {
	s.loadCalls++
	return memory.Snapshot{}, nil
}

func (s *countingAppMemoryStore) Save(context.Context, string, memory.Delta) error {
	s.saveCalls++
	return nil
}

type countingAppWorkingMemoryFactory struct {
	calls int
}

func (f *countingAppWorkingMemoryFactory) NewRunState(ctx context.Context, agentID, runID string) (memory.WorkingSet, error) {
	f.calls++
	return memory.InMemoryWorkingMemoryFactory{}.NewRunState(ctx, agentID, runID)
}
