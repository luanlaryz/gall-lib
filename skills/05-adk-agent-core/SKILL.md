---
name: adk-agent-core
description: Build and maintain the agent core on google/adk-go with streaming deltas, tool wiring, and seller-aware sessions.
---

# ADK Agent Core
Goal: deliver deterministic, seller-aware agent orchestrations using `google/adk-go`, streaming deltas into Redis Streams.

## When to Use
- Touching agent orchestration, tool registration, or ADK session lifecycle.
- Adjusting streaming delta hooks or conversation state storage.
- Integrating new tools/providers into the agent core.

## Non-negotiables
1. ADK session keyed by `(seller_id, conversation_id)`; never global.
2. Tools registered via dependency injection; no inline singletons.
3. Streaming callback publishes deltas immediately to streaming skill.
4. Seller's provider/model selection fetched via skill 06 config before starting ADK run.
5. Errors surfaced to async command skill for retries when transient.

## Do / Don't
- **Do** wrap ADK invocation with context deadlines and cancellation.
- **Do** record metrics per step (start/end, success/failure).
- **Do** persist intermediate state if workflows need resume.
- **Don't** block streaming callback; use buffered channel or go routine with proper cancel.
- **Don't** expose ADK internals to HTTP/gRPC layers; keep behind use case.
- **Don't** mutate shared memory without locks/errgroup.

## Interfaces / Contracts
- Reference [adk_integration_notes.md](resources/adk_integration_notes.md) for wiring details.
- Core use case signature:
  ```go
  type AgentRunner interface {
      Run(ctx context.Context, input RunInput) (RunResult, error)
  }
  ```
- `RunInput` includes tenant context, seller model config, and conversation state pointer.

## Checklists
**Before coding**
- [ ] Confirm seller's provider/model + params via config skill.
- [ ] Decide which tools to register and required permissions.
- [ ] Define delta payload schema for streaming.

**During**
- [ ] Attach streaming callback to ADK client; convert deltas to events.
- [ ] Propagate context cancellations from HTTP/gRPC to ADK run.
- [ ] Capture metrics/logs for each step and tool call.

**After**
- [ ] Unit test delta conversion + event publishing (use fake stream port).
- [ ] Load test long conversations to probe memory leaks.
- [ ] Document toolset changes in resources file if new behavior arises.

## Definition of Done
- Agent run respects seller config, streams deltas, and cleans up session.
- Tool wiring tested with fakes/mocks.
- Metrics/logs/traces confirm step and delta coverage.
- Error categories defined (retriable vs terminal) and surfaced to queue skill.

## Minimal Examples
- Add tool: implement adapter struct satisfying ADK `Tool` interface, register via `runner.Register(tool)` in wiring package.
- Streaming: `client.Run(ctx, req, publishDelta)` where `publishDelta` pushes to Redis stream and returns quickly.
