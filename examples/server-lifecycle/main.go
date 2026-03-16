package main

import (
	"context"
	"fmt"

	"github.com/luanlima/gaal-lib/pkg/app"
	"github.com/luanlima/gaal-lib/pkg/logger"
	"github.com/luanlima/gaal-lib/pkg/server"
)

func main() {
	ctx := context.Background()

	instance, err := app.New(
		app.Config{
			Name: "server-lifecycle",
			Defaults: app.Defaults{
				Logger: logger.Default(),
			},
		},
		app.WithServerlessHooks(invokeHook{}),
	)
	if err != nil {
		panic(err)
	}

	before := server.Snapshot(instance.State())
	fmt.Println(before.Health, before.Ready)

	if err := server.Invoke(ctx, instance, server.Target{Kind: "agent", Name: "greeter"}, func(ctx context.Context, rt server.Runtime) error {
		fmt.Println(rt.State())
		return nil
	}); err != nil {
		panic(err)
	}

	after := server.Snapshot(instance.State())
	fmt.Println(after.Health, after.Ready)

	if err := instance.Shutdown(ctx); err != nil {
		panic(err)
	}
}

type invokeHook struct{}

func (invokeHook) OnColdStart(context.Context, app.Runtime) error {
	return nil
}

func (invokeHook) OnInvokeStart(ctx context.Context, target app.Target) (context.Context, error) {
	return ctx, nil
}

func (invokeHook) OnInvokeDone(context.Context, app.Target, error) {}
