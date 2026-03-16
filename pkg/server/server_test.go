package server_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/server"
)

func TestSnapshotReflectsLifecycleSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state  app.State
		health bool
		ready  bool
	}{
		{state: app.StateCreated, health: true, ready: false},
		{state: app.StateStarting, health: true, ready: false},
		{state: app.StateRunning, health: true, ready: true},
		{state: app.StateStopping, health: true, ready: false},
		{state: app.StateStopped, health: false, ready: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			probe := server.Snapshot(tt.state)
			if probe.Health != tt.health {
				t.Fatalf("Health = %v want %v", probe.Health, tt.health)
			}
			if probe.Ready != tt.ready {
				t.Fatalf("Ready = %v want %v", probe.Ready, tt.ready)
			}
		})
	}
}

func TestInvokeWrapsServerlessHooks(t *testing.T) {
	t.Parallel()

	var calls []string
	instance, err := app.New(
		app.Config{Name: "invoke-app"},
		app.WithServerlessHooks(serverlessHookStub{
			onStart: func(ctx context.Context, target app.Target) (context.Context, error) {
				calls = append(calls, "start:"+target.Name)
				return context.WithValue(ctx, hookContextKey{}, "ok"), nil
			},
			onDone: func(ctx context.Context, target app.Target, err error) {
				calls = append(calls, "done:"+target.Name)
				if got := ctx.Value(hookContextKey{}); got != "ok" {
					t.Fatalf("hook context value = %v want ok", got)
				}
			},
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = server.Invoke(context.Background(), instance, server.Target{Kind: "agent", Name: "greeter"}, func(ctx context.Context, rt server.Runtime) error {
		calls = append(calls, "run:"+string(rt.State()))
		if rt.State() != app.StateRunning {
			t.Fatalf("runtime state = %q want %q", rt.State(), app.StateRunning)
		}
		if got := ctx.Value(hookContextKey{}); got != "ok" {
			t.Fatalf("run context value = %v want ok", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	want := []string{"start:greeter", "run:running", "done:greeter"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v want %v", calls, want)
	}
}

type hookContextKey struct{}

type serverlessHookStub struct {
	onStart func(context.Context, app.Target) (context.Context, error)
	onDone  func(context.Context, app.Target, error)
}

func (h serverlessHookStub) OnColdStart(context.Context, app.Runtime) error {
	return nil
}

func (h serverlessHookStub) OnInvokeStart(ctx context.Context, target app.Target) (context.Context, error) {
	if h.onStart == nil {
		return ctx, nil
	}
	return h.onStart(ctx, target)
}

func (h serverlessHookStub) OnInvokeDone(ctx context.Context, target app.Target, err error) {
	if h.onDone != nil {
		h.onDone(ctx, target, err)
	}
}
