# Concurrency Patterns

## Context + ErrGroup
```go
ctx, cancel := context.WithTimeout(parent, 30*time.Second)
defer cancel()
eg, ctx := errgroup.WithContext(ctx)
for _, task := range tasks {
    task := task
    eg.Go(func() error {
        return doWork(ctx, task)
    })
}
if err := eg.Wait(); err != nil {
    return fmt.Errorf("work failed: %w", err)
}
```

- Always respect ctx.Done() inside goroutines.
- No goroutine leaks: ensure go routines exit on cancel.

## Channels
- Prefer typed channels with clear ownership; close only by sender.
- Buffer small amounts when bridging to IO (e.g., streaming deltas) but flush on shutdown.

## Worker Pools
- Use struct encapsulating pool; avoid global channels.
- Provide `Stop()` that closes input and waits for workers.
