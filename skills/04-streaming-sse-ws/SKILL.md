---
name: streaming-sse-ws
description: Deliver agent updates through Redis Streams with SSE/WebSocket fan-out, cursored replay, and heartbeat guarantees.
---

# Streaming SSE & WebSocket
Goal: provide consistent, replayable streaming of agent events for UI/webhooks using Redis Streams and SSE/WS transports.

## When to Use
- Adding/modifying streaming endpoints, consumers, or event schemas.
- Working on Redis Streams retention/replay logic.
- Handling UI subscription bugs or performance tuning.

## Non-negotiables
1. Redis Stream per conversation: `stream:conversation:{seller_id}:{conversation_id}`.
2. Every event increments `seq` monotonically; use as cursor within payload.
3. SSE default, WebSocket optional but uses same event envelope.
4. Reconnection/resume supported through cursor tokens (Redis IDs).
5. API responses never block on downstream consumers; they publish events asynchronously.

## Do / Don't
- **Do** push events from workers/use cases after durable state write (Postgres).
- **Do** store structured payload JSON; avoid string concatenation.
- **Do** emit `heartbeat` events for idle streams.
- **Don't** keep per-client state in Redis; rely on cursors instead.
- **Don't** drop events silently; log + metric `stream_dropped_events_total`.
- **Don't** re-use SSE endpoint for non-stream payloads.

## Interfaces / Contracts
- Event envelope spec lives in [event_envelope.md](resources/event_envelope.md).
- Publisher port:
  ```go
  type EventStream interface {
      Publish(ctx context.Context, sellerID, conversationID string, event Event) (cursor string, err error)
      Read(ctx context.Context, sellerID, conversationID, cursor string, count int64) ([]Event, string, error)
  }
  ```
- SSE handler contract: use Gin middleware to upgrade connection and flush per event.

## Checklists
**Before coding**
- [ ] Define event types + payload schemas (delta, completed, error, heartbeat).
- [ ] Decide retention + backpressure thresholds.
- [ ] Plan auth via tenant skill (seller_id on context).

**During**
- [ ] Wrap Redis calls with context + timeout; handle `XREAD` cancellation.
- [ ] Write SSE/WS tests for reconnect/resume cases.
- [ ] Ensure gzip disabled (per SSE spec) but TLS enforced.

**After**
- [ ] Update OpenAPI/WebSocket docs with cursor semantics.
- [ ] Add Grafana panels: backlog length, fan-out latency.
- [ ] Run load test for at least 100 concurrent clients.

## Definition of Done
- SSE + WS endpoints resume from cursor and heartbeat on idle.
- Redis retention + metrics configured.
- Event schema documented and versioned.
- Observability includes stream publish/read counters.

## Minimal Examples
- Worker publishes ADK delta: `Event{Type: "agent.delta", Payload: DeltaJSON, Seq: nextSeq}` -> Redis stream -> UI SSE.
- Client reconnect: sends last `cursor`, server `XREAD` from `cursor+1` and replays.
