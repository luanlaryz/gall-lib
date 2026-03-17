package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luanlima/gaal-lib/internal/demoapp"
)

func main() {
	cfg := demoapp.ConfigFromEnv()

	bundle, err := demoapp.New(cfg)
	if err != nil {
		fatalf("build demo app: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bundle.App.Start(ctx); err != nil {
		fatalf("start demo app: %v", err)
	}

	fmt.Printf("demo app running at %s\n", bundle.Server.BaseURL())
	fmt.Printf("health: %s/healthz\n", bundle.Server.BaseURL())
	fmt.Printf("ready:  %s/readyz\n", bundle.Server.BaseURL())
	fmt.Printf("agents: %s/agents\n", bundle.Server.BaseURL())
	fmt.Printf("run:    %s/agents/%s/runs\n", bundle.Server.BaseURL(), bundle.Config.AgentName)
	fmt.Printf("stream: %s/agents/%s/stream\n", bundle.Server.BaseURL(), bundle.Config.AgentName)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := bundle.App.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		fatalf("shutdown demo app: %v", err)
	}
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
