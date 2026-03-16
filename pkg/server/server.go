// Package server provides additive public helpers around app lifecycle and
// probe semantics without coupling gaal-lib to a concrete transport.
package server

import (
	"context"

	"github.com/luanlima/gaal-lib/pkg/app"
)

type (
	// Server aliases the long-lived managed server contract exposed by pkg/app.
	Server = app.Server
	// ServerlessHook aliases the short-lived invocation hook contract.
	ServerlessHook = app.ServerlessHook
	// Target aliases the observable invocation target descriptor.
	Target = app.Target
	// Runtime aliases the read-only runtime view consumed by adapters.
	Runtime = app.Runtime
)

// Probe reports local health and readiness derived from app lifecycle state.
type Probe struct {
	State    app.State
	Health   bool
	Ready    bool
	Draining bool
}

// Health reports whether the given lifecycle state is locally healthy.
func Health(state app.State) bool {
	return state != app.StateStopped
}

// Ready reports whether the given lifecycle state is ready for new traffic.
func Ready(state app.State) bool {
	return state == app.StateRunning
}

// Snapshot derives probe semantics from the current lifecycle state.
func Snapshot(state app.State) Probe {
	return Probe{
		State:    state,
		Health:   Health(state),
		Ready:    Ready(state),
		Draining: state == app.StateStopping,
	}
}

// Invoke wraps a short-lived invocation using the app lifecycle helper.
func Invoke(
	ctx context.Context,
	instance *app.App,
	target Target,
	run func(context.Context, Runtime) error,
) error {
	return instance.Invoke(ctx, target, run)
}
