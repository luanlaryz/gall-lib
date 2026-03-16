package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Step is the public unit of work executed by a workflow.
type Step interface {
	Name() string
	Kind() StepKind
	Run(ctx context.Context, stepCtx StepContext) (StepResult, error)
}

// StepKind identifies the observable kind of a step.
type StepKind string

const (
	// StepKindAction executes local workflow logic.
	StepKindAction StepKind = "action"
	// StepKindBranch computes the next workflow transition.
	StepKindBranch StepKind = "branch"
	// StepKindInvoke is reserved for adapters that invoke external executors.
	StepKindInvoke StepKind = "invoke"
)

// StepContext is the immutable public context passed to a step.
type StepContext struct {
	WorkflowID   string
	WorkflowName string
	RunID        string
	SessionID    string
	StepName     string
	Attempt      int
	Input        map[string]any
	State        *State
	Metadata     types.Metadata
}

// StepResult is the observable result produced by a step.
type StepResult struct {
	Output   map[string]any
	Next     Next
	Metadata types.Metadata
}

// Next defines the next workflow transition chosen by a step.
type Next struct {
	Step    string
	End     bool
	Suspend bool
}

// Decision is the public branching result used by conditional steps.
type Decision struct {
	Step     string
	End      bool
	Suspend  bool
	Reason   string
	Metadata types.Metadata
}

// StepFunc executes a regular workflow action step.
type StepFunc func(ctx context.Context, stepCtx StepContext) (StepResult, error)

// DecisionFunc executes a conditional step and returns the next decision.
type DecisionFunc func(ctx context.Context, stepCtx StepContext) (Decision, error)

// StepOption mutates step construction settings.
type StepOption func(*stepOptions) error

type stepOptions struct {
	retry    RetryPolicy
	metadata types.Metadata
}

type configuredStep interface {
	Step
	stepRetry() RetryPolicy
	stepMetadata() types.Metadata
}

// WithStepRetry overrides the workflow retry policy for a single step.
func WithStepRetry(policy RetryPolicy) StepOption {
	return func(opts *stepOptions) error {
		if policy == nil {
			return fmt.Errorf("%w: step retry policy cannot be nil", ErrInvalidConfig)
		}
		opts.retry = policy
		return nil
	}
}

// WithStepMetadata sets additional metadata merged into the step context.
func WithStepMetadata(md types.Metadata) StepOption {
	return func(opts *stepOptions) error {
		opts.metadata = types.CloneMetadata(md)
		return nil
	}
}

// Action creates a named action step.
func Action(name string, fn StepFunc, opts ...StepOption) Step {
	return newActionStep(name, fn, opts...)
}

// Branch creates a named conditional branch step.
func Branch(name string, fn DecisionFunc, opts ...StepOption) Step {
	return newBranchStep(name, fn, opts...)
}

type actionStep struct {
	name     string
	retry    RetryPolicy
	metadata types.Metadata
	run      StepFunc
	err      error
}

func newActionStep(name string, fn StepFunc, opts ...StepOption) *actionStep {
	resolved, err := resolveStepOptions(opts...)
	if err != nil {
		return &actionStep{err: err}
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &actionStep{err: fmt.Errorf("%w: step name is required", ErrInvalidConfig)}
	}
	if fn == nil {
		return &actionStep{err: fmt.Errorf("%w: action step function is required", ErrInvalidConfig)}
	}
	return &actionStep{
		name:     trimmed,
		retry:    resolved.retry,
		metadata: resolved.metadata,
		run:      fn,
	}
}

func (s *actionStep) Name() string                 { return s.name }
func (s *actionStep) Kind() StepKind               { return StepKindAction }
func (s *actionStep) stepRetry() RetryPolicy       { return s.retry }
func (s *actionStep) stepMetadata() types.Metadata { return types.CloneMetadata(s.metadata) }
func (s *actionStep) validationError() error       { return s.err }

func (s *actionStep) Run(ctx context.Context, stepCtx StepContext) (StepResult, error) {
	if s.err != nil {
		return StepResult{}, s.err
	}
	return s.run(ctx, stepCtx)
}

type branchStep struct {
	name     string
	retry    RetryPolicy
	metadata types.Metadata
	decide   DecisionFunc
	err      error
}

func newBranchStep(name string, fn DecisionFunc, opts ...StepOption) *branchStep {
	resolved, err := resolveStepOptions(opts...)
	if err != nil {
		return &branchStep{err: err}
	}
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &branchStep{err: fmt.Errorf("%w: step name is required", ErrInvalidConfig)}
	}
	if fn == nil {
		return &branchStep{err: fmt.Errorf("%w: branch step function is required", ErrInvalidConfig)}
	}
	return &branchStep{
		name:     trimmed,
		retry:    resolved.retry,
		metadata: resolved.metadata,
		decide:   fn,
	}
}

func (s *branchStep) Name() string                 { return s.name }
func (s *branchStep) Kind() StepKind               { return StepKindBranch }
func (s *branchStep) stepRetry() RetryPolicy       { return s.retry }
func (s *branchStep) stepMetadata() types.Metadata { return types.CloneMetadata(s.metadata) }
func (s *branchStep) validationError() error       { return s.err }

func (s *branchStep) Run(ctx context.Context, stepCtx StepContext) (StepResult, error) {
	if s.err != nil {
		return StepResult{}, s.err
	}
	decision, err := s.decide(ctx, stepCtx)
	if err != nil {
		return StepResult{}, err
	}
	return StepResult{
		Next: Next{
			Step:    strings.TrimSpace(decision.Step),
			End:     decision.End,
			Suspend: decision.Suspend,
		},
		Metadata: types.CloneMetadata(decision.Metadata),
	}, nil
}

func resolveStepOptions(opts ...StepOption) (stepOptions, error) {
	var resolved stepOptions
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&resolved); err != nil {
			return stepOptions{}, err
		}
	}
	resolved.metadata = types.CloneMetadata(resolved.metadata)
	return resolved, nil
}
