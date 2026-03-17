package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/memory"
)

func main() {
	cfg := normalizeConfig(configFromEnv())

	srv := newHTTPServer(httpServerConfig{
		addr:    cfg.addr,
		appName: cfg.appName,
	})

	instance, err := app.New(
		app.Config{
			Name: cfg.appName,
			Defaults: app.Defaults{
				Logger: logger.NewSimple(logger.SimpleOptions{
					Level: cfg.logLevel,
				}),
				Agent: app.AgentDefaults{
					Memory: &memory.InMemoryStore{},
				},
			},
		},
		app.WithAgentFactories(agentFactory{agentName: cfg.agentName}),
		app.WithServers(srv),
	)
	if err != nil {
		fatalf("build demo app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := instance.Start(ctx); err != nil {
		fatalf("start demo app: %v", err)
	}

	fmt.Printf("demo app running at %s\n", srv.BaseURL())
	fmt.Printf("health: %s/healthz\n", srv.BaseURL())
	fmt.Printf("ready:  %s/readyz\n", srv.BaseURL())
	fmt.Printf("agents: %s/agents\n", srv.BaseURL())
	fmt.Printf("run:    %s/agents/%s/runs\n", srv.BaseURL(), cfg.agentName)
	fmt.Printf("stream: %s/agents/%s/stream\n", srv.BaseURL(), cfg.agentName)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := instance.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		fatalf("shutdown demo app: %v", err)
	}
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
