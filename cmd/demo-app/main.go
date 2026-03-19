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
	"github.com/luanlima/gaal-lib/pkg/guardrail"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/memory"
	"github.com/luanlima/gaal-lib/pkg/types"
	"github.com/luanlima/gaal-lib/pkg/workflow"
)

func main() {
	cfg := normalizeConfig(configFromEnv())

	log := logger.NewSimple(logger.SimpleOptions{Level: cfg.logLevel})

	srv := newHTTPServer(httpServerConfig{
		addr:    cfg.addr,
		appName: cfg.appName,
	})

	instance, err := app.New(
		app.Config{
			Name: cfg.appName,
			Defaults: app.Defaults{
				Logger: log,
			Agent: app.AgentDefaults{
				Memory:           &memory.InMemoryStore{},
				InputGuardrails:  []guardrail.Input{inputBlockGuardrail{}},
				StreamGuardrails: []guardrail.Stream{streamDigitGuardrail{}},
				OutputGuardrails: []guardrail.Output{outputTagGuardrail{}},
			},
				Workflow: app.WorkflowDefaults{
					Metadata: types.Metadata{"demo": "order-processing"},
					Hooks:    []workflow.Hook{workflow.NewLoggingHook(log)},
					History:  &workflow.InMemoryHistory{},
					Retry:    workflow.FixedRetryPolicy{MaxRetries: 1},
				},
			},
		},
		app.WithAgentFactories(agentFactory{agentName: cfg.agentName}),
		app.WithWorkflowFactories(orderWorkflowFactory{}),
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
	fmt.Printf("health:    %s/healthz\n", srv.BaseURL())
	fmt.Printf("ready:     %s/readyz\n", srv.BaseURL())
	fmt.Printf("agents:    %s/agents\n", srv.BaseURL())
	fmt.Printf("run:       %s/agents/%s/runs\n", srv.BaseURL(), cfg.agentName)
	fmt.Printf("stream:    %s/agents/%s/stream\n", srv.BaseURL(), cfg.agentName)
	fmt.Printf("workflows: %s/workflows\n", srv.BaseURL())
	fmt.Printf("wf run:    %s/workflows/%s/runs\n", srv.BaseURL(), orderWorkflowName)

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
