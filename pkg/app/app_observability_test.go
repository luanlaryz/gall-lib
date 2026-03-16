package app_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/logger"
)

func TestAppHealthAndReadinessFollowLifecycle(t *testing.T) {
	t.Parallel()

	instance, err := app.New(app.Config{Name: "probe-app"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !instance.Health() || instance.Ready() {
		t.Fatalf("created probe = health:%v ready:%v", instance.Health(), instance.Ready())
	}
	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !instance.Health() || !instance.Ready() {
		t.Fatalf("running probe = health:%v ready:%v", instance.Health(), instance.Ready())
	}
	if err := instance.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if instance.Health() || instance.Ready() {
		t.Fatalf("stopped probe = health:%v ready:%v", instance.Health(), instance.Ready())
	}
}

func TestAppHookPanicIsLogged(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	instance, err := app.New(
		app.Config{
			Name: "hook-panic-log",
			Defaults: app.Defaults{
				Logger: logger.NewSimple(logger.SimpleOptions{Writer: &buf, Level: logger.LevelDebug}),
			},
		},
		app.WithAppHooks(appHookFunc(func(ctx context.Context, event app.Event) {
			if event.Type == app.EventAppStarting {
				panic("boom")
			}
		})),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "app.hook_panic") {
		t.Fatalf("output = %q want app.hook_panic", output)
	}
	if !strings.Contains(output, "event_type=app.starting") {
		t.Fatalf("output = %q want app.starting", output)
	}
}

func TestStartFailureEmitsBootstrapFailureLog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	instance, err := app.New(
		app.Config{
			Name: "bootstrap-log",
			Defaults: app.Defaults{
				Logger: logger.NewSimple(logger.SimpleOptions{Writer: &buf, Level: logger.LevelDebug}),
			},
		},
		app.WithServers(failingServer{name: "broken", startErr: errors.New("listen failed")}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := instance.Start(context.Background()); err == nil {
		t.Fatal("expected startup error")
	}

	output := buf.String()
	if !strings.Contains(output, "app.bootstrap_failed") {
		t.Fatalf("output = %q want app.bootstrap_failed", output)
	}
	if !strings.Contains(output, "listen failed") {
		t.Fatalf("output = %q want startup error", output)
	}
}

type failingServer struct {
	name     string
	startErr error
}

func (s failingServer) Name() string {
	return s.name
}

func (s failingServer) Start(context.Context, app.Runtime) error {
	return s.startErr
}

func (s failingServer) Shutdown(context.Context) error {
	return nil
}
