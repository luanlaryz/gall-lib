package app

import (
	"context"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/types"
)

// Health reports whether the app is locally healthy for liveness purposes.
func (a *App) Health() bool {
	return a.State() != StateStopped
}

// Ready reports whether the app is ready to accept new traffic.
func (a *App) Ready() bool {
	return a.State() == StateRunning
}

// Context attaches the app logger to ctx so downstream runs can reuse it.
func (a *App) Context(ctx context.Context) context.Context {
	return logger.WithContext(ctx, a.Logger())
}

// Invoke wraps a short-lived invocation with EnsureStarted and serverless hooks.
func (a *App) Invoke(
	ctx context.Context,
	target Target,
	run func(context.Context, Runtime) error,
) (err error) {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidConfig)
	}
	if run == nil {
		return fmt.Errorf("%w: invoke function is required", ErrInvalidConfig)
	}
	if err := a.EnsureStarted(ctx); err != nil {
		return err
	}

	a.mu.RLock()
	hooks := append([]ServerlessHook(nil), a.serverlessHooks...)
	a.mu.RUnlock()

	invokeCtx := a.Context(ctx)
	defer func() {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			hook.OnInvokeDone(invokeCtx, target, err)
		}
	}()

	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		nextCtx, invokeErr := hook.OnInvokeStart(invokeCtx, target)
		if invokeErr != nil {
			return invokeErr
		}
		invokeCtx = a.Context(nextCtx)
	}

	return run(invokeCtx, a.Runtime())
}

func (a *App) eventMetadata(eventType EventType, extra types.Metadata) types.Metadata {
	a.mu.RLock()
	base := types.CloneMetadata(a.defaults.Metadata)
	appName := a.name
	state := a.state
	a.mu.RUnlock()

	return types.MergeMetadata(base, types.MergeMetadata(types.Metadata{
		"app_name":   appName,
		"component":  "app",
		"event_type": string(eventType),
		"state":      string(state),
	}, extra))
}

func (a *App) logAppEvent(ctx context.Context, event Event) {
	args := append([]any{
		"component", "app",
		"event_type", string(event.Type),
		"app_name", event.AppName,
		"state", event.Metadata["state"],
	}, metadataArgs(event.Metadata)...)
	if event.Err != nil {
		args = append(args, "error", event.Err)
	}

	log := a.Logger()
	switch event.Type {
	case EventBootstrapFailed:
		log.ErrorContext(ctx, string(event.Type), args...)
	case EventAgentRegistered, EventWorkflowRegistered:
		log.DebugContext(ctx, string(event.Type), args...)
	default:
		log.InfoContext(ctx, string(event.Type), args...)
	}
}

func (a *App) logHookPanic(ctx context.Context, eventType EventType, panicValue any) {
	a.Logger().ErrorContext(a.Context(ctx), "app.hook_panic",
		"component", "app",
		"event_type", string(eventType),
		"panic", fmt.Sprint(panicValue),
	)
}

func cloneAppEvent(event Event) Event {
	return Event{
		Type:     event.Type,
		AppName:  event.AppName,
		Time:     event.Time,
		Err:      event.Err,
		Metadata: types.CloneMetadata(event.Metadata),
	}
}

func metadataArgs(md map[string]string) []any {
	if len(md) == 0 {
		return nil
	}
	args := make([]any, 0, len(md)*2)
	for key, value := range md {
		args = append(args, key, value)
	}
	return args
}
