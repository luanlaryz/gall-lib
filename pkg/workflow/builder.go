package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/luanlima/gaal-lib/pkg/types"
)

var (
	workflowIDSanitizer = regexp.MustCompile(`[^a-z0-9]+`)
	runSequence         atomic.Uint64
)

// Option mutates builder settings before validation.
type Option func(*Builder) error

// Builder composes a runnable sequential workflow.
type Builder struct {
	name     string
	id       string
	steps    []Step
	hooks    []Hook
	retry    RetryPolicy
	history  HistorySink
	metadata types.Metadata
}

// NewBuilder creates a mutable workflow builder.
func NewBuilder(name string) *Builder {
	return &Builder{name: strings.TrimSpace(name)}
}

// New is a convenience helper that applies opts and builds a runnable chain.
func New(name string, opts ...Option) (*Chain, error) {
	builder := NewBuilder(name)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(builder); err != nil {
			return nil, err
		}
	}
	return builder.Build()
}

// Step appends a step to the workflow definition.
func (b *Builder) Step(step Step) *Builder {
	if b == nil {
		return nil
	}
	b.steps = append(b.steps, step)
	return b
}

// WithMetadata merges workflow metadata into the builder.
func (b *Builder) WithMetadata(md types.Metadata) *Builder {
	if b == nil {
		return nil
	}
	b.metadata = types.MergeMetadata(b.metadata, md)
	return b
}

// WithHooks appends workflow hooks in order.
func (b *Builder) WithHooks(hooks ...Hook) *Builder {
	if b == nil {
		return nil
	}
	b.hooks = append(b.hooks, hooks...)
	return b
}

// WithRetry sets the default workflow retry policy.
func (b *Builder) WithRetry(policy RetryPolicy) *Builder {
	if b == nil {
		return nil
	}
	b.retry = policy
	return b
}

// WithHistory sets the workflow history sink.
func (b *Builder) WithHistory(sink HistorySink) *Builder {
	if b == nil {
		return nil
	}
	b.history = sink
	return b
}

// Build validates the definition and returns an immutable runnable chain.
func (b *Builder) Build() (*Chain, error) {
	if b == nil {
		return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: builder is nil", ErrInvalidConfig))
	}

	name := strings.TrimSpace(b.name)
	if name == "" {
		return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: name is required", ErrInvalidConfig))
	}
	if len(b.steps) == 0 {
		return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: at least one step is required", ErrInvalidConfig))
	}

	steps := make([]Step, 0, len(b.steps))
	indexByName := make(map[string]int, len(b.steps))
	descriptors := make([]StepDescriptor, 0, len(b.steps))
	for index, step := range b.steps {
		if step == nil {
			return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: step %d is nil", ErrInvalidConfig, index))
		}
		if invalid, ok := step.(interface{ validationError() error }); ok {
			if err := invalid.validationError(); err != nil {
				return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", err)
			}
		}

		stepName := strings.TrimSpace(step.Name())
		if stepName == "" {
			return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: step %d name is required", ErrInvalidConfig, index))
		}
		if _, exists := indexByName[stepName]; exists {
			return nil, newError(ErrorKindInvalidConfig, "build", "", "", "", fmt.Errorf("%w: duplicate step name %q", ErrInvalidConfig, stepName))
		}

		indexByName[stepName] = index
		steps = append(steps, step)
		descriptors = append(descriptors, StepDescriptor{Name: stepName, Kind: step.Kind()})
	}

	workflowID := normalizeID(name)
	if trimmed := strings.TrimSpace(b.id); trimmed != "" {
		workflowID = trimmed
	}

	return &Chain{
		id:          workflowID,
		name:        name,
		steps:       steps,
		stepIndex:   indexByName,
		hooks:       append([]Hook(nil), b.hooks...),
		retry:       b.retry,
		history:     b.history,
		metadata:    types.CloneMetadata(b.metadata),
		descriptors: descriptors,
	}, nil
}

// WithID overrides the normalized workflow identifier.
func WithID(id string) Option {
	return func(b *Builder) error {
		if b == nil {
			return fmt.Errorf("%w: builder is nil", ErrInvalidConfig)
		}
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return fmt.Errorf("%w: id is required", ErrInvalidConfig)
		}
		b.id = trimmed
		return nil
	}
}

// WithSteps appends all provided steps to the builder.
func WithSteps(steps ...Step) Option {
	return func(b *Builder) error {
		for _, step := range steps {
			b.Step(step)
		}
		return nil
	}
}

// WithMetadata sets workflow metadata.
func WithMetadata(md types.Metadata) Option {
	return func(b *Builder) error {
		b.WithMetadata(md)
		return nil
	}
}

// WithHooks appends workflow hooks.
func WithHooks(hooks ...Hook) Option {
	return func(b *Builder) error {
		b.WithHooks(hooks...)
		return nil
	}
}

// WithRetry sets the workflow retry policy.
func WithRetry(policy RetryPolicy) Option {
	return func(b *Builder) error {
		if policy == nil {
			return fmt.Errorf("%w: retry policy cannot be nil", ErrInvalidConfig)
		}
		b.WithRetry(policy)
		return nil
	}
}

// WithHistory sets the workflow history sink.
func WithHistory(sink HistorySink) Option {
	return func(b *Builder) error {
		if sink == nil {
			return fmt.Errorf("%w: history sink cannot be nil", ErrInvalidConfig)
		}
		b.WithHistory(sink)
		return nil
	}
}

func normalizeID(name string) string {
	id := strings.ToLower(strings.TrimSpace(name))
	id = workflowIDSanitizer.ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	if id == "" {
		return "workflow"
	}
	return id
}

func newRunID(workflowID string) string {
	return fmt.Sprintf("%s-run-%d", workflowID, runSequence.Add(1))
}

var _ Workflow = (*Chain)(nil)
var _ Runnable = (*Chain)(nil)
var _ Hook = HookFunc(nil)
var _ Hook = LifecycleHooks{}
var _ RetryPolicy = RetryFunc(nil)
var _ HistorySink = (*InMemoryHistory)(nil)
