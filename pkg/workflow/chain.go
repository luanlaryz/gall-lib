package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Chain is the first concrete runnable workflow implementation.
//
// It intentionally implements only the minimal sequential semantics required by
// the first workflow phase. Full suspend/resume orchestration remains for a
// later stage; this version only exposes StatusSuspended and Checkpoint.
type Chain struct {
	id          string
	name        string
	steps       []Step
	stepIndex   map[string]int
	hooks       []Hook
	retry       RetryPolicy
	history     HistorySink
	metadata    types.Metadata
	descriptors []StepDescriptor
}

// ID returns the stable logical workflow identifier.
func (w *Chain) ID() string {
	if w == nil {
		return ""
	}
	return w.id
}

// Name returns the logical workflow name.
func (w *Chain) Name() string {
	if w == nil {
		return ""
	}
	return w.name
}

// Descriptor returns the registry descriptor for the workflow.
func (w *Chain) Descriptor() Descriptor {
	return Descriptor{
		Name: w.Name(),
		ID:   w.ID(),
	}
}

// Definition returns the immutable workflow definition snapshot.
func (w *Chain) Definition() Definition {
	if w == nil {
		return Definition{}
	}

	return Definition{
		Descriptor: w.Descriptor(),
		Steps:      append([]StepDescriptor(nil), w.descriptors...),
		Hooks:      append([]Hook(nil), w.hooks...),
		Retry:      w.retry,
		History:    w.history,
		Metadata:   types.CloneMetadata(w.metadata),
	}
}

// Run executes the workflow sequentially until completion, suspension,
// cancellation or terminal failure.
func (w *Chain) Run(ctx context.Context, req Request) (resp Response, retErr error) {
	if w == nil {
		return Response{}, newError(ErrorKindInvalidConfig, "run", "", "", "", fmt.Errorf("%w: workflow is nil", ErrInvalidConfig))
	}
	if ctx == nil {
		return Response{}, newError(ErrorKindInvalidRequest, "run", w.id, req.RunID, "", fmt.Errorf("%w: context is required", ErrInvalidRequest))
	}

	normalized, err := w.normalizeRequest(req)
	if err != nil {
		return Response{}, err
	}

	runState := NewState(normalized.State)
	currentInput := cloneMap(normalized.Input)
	currentStepIndex := 0
	lastStepName := ""
	lastOutput := map[string]any(nil)
	status := StatusCompleted
	var checkpoint *Checkpoint

	finishCalled := false
	endStatus := func() Status {
		if retErr != nil {
			if errorsIsCanceled(retErr) {
				return StatusCanceled
			}
			return StatusFailed
		}
		if checkpoint != nil {
			return StatusSuspended
		}
		return status
	}

	runEventMeta := types.MergeMetadata(w.metadata, normalized.Metadata)

	defer func() {
		finalStatus := endStatus()
		endEvent := Event{
			Type:         EventWorkflowEnded,
			WorkflowID:   w.id,
			WorkflowName: w.name,
			RunID:        normalized.RunID,
			SessionID:    normalized.SessionID,
			StepName:     lastStepName,
			Status:       finalStatus,
			Output:       cloneMap(lastOutput),
			State:        runState.Snapshot(),
			Err:          retErr,
			Time:         time.Now(),
			Metadata:     types.CloneMetadata(runEventMeta),
		}
		if err := w.emitLifecycle(ctx, endEvent); err != nil && retErr == nil {
			retErr = err
			resp = Response{}
		}
	}()

	startEvent := Event{
		Type:         EventWorkflowStarted,
		WorkflowID:   w.id,
		WorkflowName: w.name,
		RunID:        normalized.RunID,
		SessionID:    normalized.SessionID,
		Status:       StatusCompleted,
		Output:       cloneMap(currentInput),
		State:        runState.Snapshot(),
		Time:         time.Now(),
		Metadata:     types.CloneMetadata(runEventMeta),
	}
	if err := w.emitLifecycle(ctx, startEvent); err != nil {
		return Response{}, err
	}

	for currentStepIndex < len(w.steps) {
		if err := ctx.Err(); err != nil {
			retErr = newError(ErrorKindCanceled, "run", w.id, normalized.RunID, lastStepName, err)
			return Response{}, retErr
		}

		step := w.steps[currentStepIndex]
		lastStepName = step.Name()
		retryPolicy := w.effectiveRetry(step)

		for attempt := 1; ; attempt++ {
			stepMeta := types.MergeMetadata(runEventMeta, stepMetadata(step))
			stepStart := Event{
				Type:         EventStepStarted,
				WorkflowID:   w.id,
				WorkflowName: w.name,
				RunID:        normalized.RunID,
				SessionID:    normalized.SessionID,
				StepName:     lastStepName,
				Attempt:      attempt,
				Status:       StatusCompleted,
				Output:       cloneMap(currentInput),
				State:        runState.Snapshot(),
				Time:         time.Now(),
				Metadata:     types.CloneMetadata(stepMeta),
			}
			if err := w.emitLifecycle(ctx, stepStart); err != nil {
				return Response{}, err
			}

			stepCtx := StepContext{
				WorkflowID:   w.id,
				WorkflowName: w.name,
				RunID:        normalized.RunID,
				SessionID:    normalized.SessionID,
				StepName:     lastStepName,
				Attempt:      attempt,
				Input:        cloneMap(currentInput),
				State:        runState,
				Metadata:     types.CloneMetadata(stepMeta),
			}

			result, err := step.Run(ctx, stepCtx)
			if err != nil {
				errorEvent := Event{
					Type:         EventWorkflowError,
					WorkflowID:   w.id,
					WorkflowName: w.name,
					RunID:        normalized.RunID,
					SessionID:    normalized.SessionID,
					StepName:     lastStepName,
					Attempt:      attempt,
					Status:       StatusFailed,
					Output:       cloneMap(currentInput),
					State:        runState.Snapshot(),
					Err:          err,
					Time:         time.Now(),
					Metadata:     types.CloneMetadata(stepMeta),
				}
				if emitErr := w.emitLifecycle(ctx, errorEvent); emitErr != nil {
					return Response{}, emitErr
				}

				if retryPolicy == nil {
					retErr = newError(ErrorKindStep, "run", w.id, normalized.RunID, lastStepName, err)
					return Response{}, retErr
				}

				delay, ok := retryPolicy.Next(attempt, err)
				if !ok {
					retErr = newError(ErrorKindStep, "run", w.id, normalized.RunID, lastStepName, err)
					return Response{}, retErr
				}

				if scheduleErr := w.appendHistory(ctx, HistoryEntry{
					Kind:         "workflow.retry_scheduled",
					WorkflowID:   w.id,
					WorkflowName: w.name,
					RunID:        normalized.RunID,
					SessionID:    normalized.SessionID,
					StepName:     lastStepName,
					Attempt:      attempt,
					Status:       StatusFailed,
					Time:         time.Now(),
					Metadata:     types.CloneMetadata(stepMeta),
				}); scheduleErr != nil {
					return Response{}, scheduleErr
				}

				if waitErr := waitForRetry(ctx, delay); waitErr != nil {
					retErr = newError(ErrorKindCanceled, "run", w.id, normalized.RunID, lastStepName, waitErr)
					return Response{}, retErr
				}
				continue
			}

			lastOutput = cloneMap(result.Output)
			stepEnd := Event{
				Type:         EventStepEnded,
				WorkflowID:   w.id,
				WorkflowName: w.name,
				RunID:        normalized.RunID,
				SessionID:    normalized.SessionID,
				StepName:     lastStepName,
				Attempt:      attempt,
				Status:       StatusCompleted,
				Output:       cloneMap(result.Output),
				State:        runState.Snapshot(),
				Time:         time.Now(),
				Metadata:     types.MergeMetadata(stepMeta, result.Metadata),
			}
			if err := w.emitLifecycle(ctx, stepEnd); err != nil {
				return Response{}, err
			}

			currentInput = cloneMap(result.Output)
			nextIndex, nextCheckpoint, done, err := w.resolveNext(currentStepIndex, lastStepName, runState, result.Next, types.MergeMetadata(stepMeta, result.Metadata))
			if err != nil {
				retErr = err
				return Response{}, retErr
			}
			if nextCheckpoint != nil {
				checkpoint = nextCheckpoint
				status = StatusSuspended
				if err := w.appendHistory(ctx, HistoryEntry{
					Kind:         "workflow.suspended",
					WorkflowID:   w.id,
					WorkflowName: w.name,
					RunID:        normalized.RunID,
					SessionID:    normalized.SessionID,
					StepName:     lastStepName,
					Attempt:      attempt,
					Status:       StatusSuspended,
					Time:         time.Now(),
					Output:       cloneMap(lastOutput),
					Checkpoint:   cloneCheckpoint(checkpoint),
					Metadata:     types.CloneMetadata(runEventMeta),
				}); err != nil {
					return Response{}, err
				}
				finishEvent := Event{
					Type:         EventWorkflowFinished,
					WorkflowID:   w.id,
					WorkflowName: w.name,
					RunID:        normalized.RunID,
					SessionID:    normalized.SessionID,
					StepName:     lastStepName,
					Status:       StatusSuspended,
					Output:       cloneMap(lastOutput),
					State:        runState.Snapshot(),
					Time:         time.Now(),
					Metadata:     types.CloneMetadata(runEventMeta),
				}
				if err := w.emitLifecycle(ctx, finishEvent); err != nil {
					return Response{}, err
				}
				finishCalled = true
				resp = Response{
					RunID:        normalized.RunID,
					WorkflowID:   w.id,
					WorkflowName: w.name,
					SessionID:    normalized.SessionID,
					Status:       StatusSuspended,
					CurrentStep:  lastStepName,
					Output:       cloneMap(lastOutput),
					State:        runState.Snapshot(),
					Checkpoint:   cloneCheckpoint(checkpoint),
					Metadata:     types.CloneMetadata(runEventMeta),
				}
				return resp, nil
			}
			if done {
				status = StatusCompleted
				finishEvent := Event{
					Type:         EventWorkflowFinished,
					WorkflowID:   w.id,
					WorkflowName: w.name,
					RunID:        normalized.RunID,
					SessionID:    normalized.SessionID,
					StepName:     lastStepName,
					Status:       StatusCompleted,
					Output:       cloneMap(lastOutput),
					State:        runState.Snapshot(),
					Time:         time.Now(),
					Metadata:     types.CloneMetadata(runEventMeta),
				}
				if err := w.emitLifecycle(ctx, finishEvent); err != nil {
					return Response{}, err
				}
				finishCalled = true
				resp = Response{
					RunID:        normalized.RunID,
					WorkflowID:   w.id,
					WorkflowName: w.name,
					SessionID:    normalized.SessionID,
					Status:       StatusCompleted,
					CurrentStep:  lastStepName,
					Output:       cloneMap(lastOutput),
					State:        runState.Snapshot(),
					Metadata:     types.CloneMetadata(runEventMeta),
				}
				return resp, nil
			}

			currentStepIndex = nextIndex
			break
		}
	}

	if !finishCalled {
		finishEvent := Event{
			Type:         EventWorkflowFinished,
			WorkflowID:   w.id,
			WorkflowName: w.name,
			RunID:        normalized.RunID,
			SessionID:    normalized.SessionID,
			StepName:     lastStepName,
			Status:       status,
			Output:       cloneMap(lastOutput),
			State:        runState.Snapshot(),
			Time:         time.Now(),
			Metadata:     types.CloneMetadata(runEventMeta),
		}
		if err := w.emitLifecycle(ctx, finishEvent); err != nil {
			return Response{}, err
		}
	}

	resp = Response{
		RunID:        normalized.RunID,
		WorkflowID:   w.id,
		WorkflowName: w.name,
		SessionID:    normalized.SessionID,
		Status:       status,
		CurrentStep:  lastStepName,
		Output:       cloneMap(lastOutput),
		State:        runState.Snapshot(),
		Checkpoint:   cloneCheckpoint(checkpoint),
		Metadata:     types.CloneMetadata(runEventMeta),
	}
	return resp, nil
}

func (w *Chain) normalizeRequest(req Request) (Request, error) {
	if strings.TrimSpace(req.RunID) == "" {
		req.RunID = newRunID(w.id)
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Input = cloneMap(req.Input)
	req.State = StateSnapshot(cloneMap(req.State))
	req.Metadata = types.CloneMetadata(req.Metadata)
	return req, nil
}

func (w *Chain) effectiveRetry(step Step) RetryPolicy {
	if configured, ok := step.(configuredStep); ok && configured.stepRetry() != nil {
		return configured.stepRetry()
	}
	return w.retry
}

func stepMetadata(step Step) types.Metadata {
	configured, ok := step.(configuredStep)
	if !ok {
		return nil
	}
	return configured.stepMetadata()
}

func (w *Chain) resolveNext(currentIndex int, stepName string, state *State, next Next, metadata types.Metadata) (int, *Checkpoint, bool, error) {
	next.Step = strings.TrimSpace(next.Step)
	switch {
	case next.Step != "" && next.End:
		return 0, nil, false, newError(ErrorKindTransition, "run", w.id, "", stepName, fmt.Errorf("%w: step and end are mutually exclusive", ErrInvalidTransition))
	case next.Step != "" && next.Suspend:
		return 0, nil, false, newError(ErrorKindTransition, "run", w.id, "", stepName, fmt.Errorf("%w: step and suspend are mutually exclusive", ErrInvalidTransition))
	case next.End && next.Suspend:
		return 0, nil, false, newError(ErrorKindTransition, "run", w.id, "", stepName, fmt.Errorf("%w: end and suspend are mutually exclusive", ErrInvalidTransition))
	case next.Suspend:
		return currentIndex, &Checkpoint{
			StepName: stepName,
			State:    state.Snapshot(),
			Time:     time.Now(),
			Metadata: types.CloneMetadata(metadata),
		}, false, nil
	case next.End:
		return currentIndex, nil, true, nil
	case next.Step != "":
		nextIndex, ok := w.stepIndex[next.Step]
		if !ok {
			return 0, nil, false, newError(ErrorKindTransition, "run", w.id, "", stepName, fmt.Errorf("%w: unknown next step %q", ErrInvalidTransition, next.Step))
		}
		return nextIndex, nil, false, nil
	default:
		if currentIndex+1 >= len(w.steps) {
			return currentIndex, nil, true, nil
		}
		return currentIndex + 1, nil, false, nil
	}
}

func (w *Chain) emitLifecycle(ctx context.Context, event Event) error {
	if err := w.appendHistory(ctx, HistoryEntry{
		Kind:         string(event.Type),
		WorkflowID:   event.WorkflowID,
		WorkflowName: event.WorkflowName,
		RunID:        event.RunID,
		SessionID:    event.SessionID,
		StepName:     event.StepName,
		Attempt:      event.Attempt,
		Status:       event.Status,
		Time:         event.Time,
		Output:       cloneMap(event.Output),
		Metadata:     types.CloneMetadata(event.Metadata),
	}); err != nil {
		return err
	}

	for _, hook := range w.hooks {
		if hook == nil {
			continue
		}
		func(h Hook) {
			defer func() {
				_ = recover()
			}()
			h.OnEvent(ctx, cloneEvent(event))
		}(hook)
	}
	return nil
}

func (w *Chain) appendHistory(ctx context.Context, entry HistoryEntry) error {
	if w.history == nil {
		return nil
	}
	if err := w.history.Append(ctx, cloneHistoryEntry(entry)); err != nil {
		return newError(ErrorKindHistory, "run", w.id, entry.RunID, entry.StepName, fmt.Errorf("%w: %v", ErrHistoryWrite, err))
	}
	return nil
}

func cloneEvent(event Event) Event {
	return Event{
		Type:         event.Type,
		WorkflowID:   event.WorkflowID,
		WorkflowName: event.WorkflowName,
		RunID:        event.RunID,
		SessionID:    event.SessionID,
		StepName:     event.StepName,
		Attempt:      event.Attempt,
		Status:       event.Status,
		Output:       cloneMap(event.Output),
		State:        StateSnapshot(cloneMap(event.State)),
		Err:          event.Err,
		Time:         event.Time,
		Metadata:     types.CloneMetadata(event.Metadata),
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func errorsIsCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
