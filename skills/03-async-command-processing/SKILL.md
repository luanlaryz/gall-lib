---
name: async-command-processing
description: Enforce HTTP 202 ACK pattern, SQS FIFO command queueing, and idempotent worker loops for asynchronous commands.
---

# Async Command Processing
Goal: keep API surfaces responsive (ACK 202) while guaranteeing exactly-once semantics through SQS FIFO and idempotent workers.

## When to Use
- Implementing or updating command handlers, queues, or worker services.
- Changing HTTP endpoints that enqueue work.
- Adjusting DLQ handling or replay tooling.

## Non-negotiables
1. HTTP handlers enqueue command then return `202 Accepted` + `request_id` + `command_id`.
2. Queue is AWS SQS FIFO with `MessageGroupId = seller:conversation` and content-based dedup.
3. Worker uses inbox table (Postgres) for idempotency — insert-first, skip if exists.
4. Visibility timeout >= 2x max processing time; extend when doing long calls.
5. DLQ reprocessing documented; no silent drops.

## Do / Don't
- **Do** log enqueue success/failure with metrics `commands_enqueued_total`.
- **Do** batch deletes when finishing messages to reduce API calls.
- **Do** publish completion events to Redis Streams for streaming consumers.
- **Don't** block HTTP handlers waiting for worker completion.
- **Don't** disable dedup; tune TTL if collisions happen.
- **Don't** forget to set `command_type` attribute for observability.

## Interfaces / Contracts
- Command envelope lives in [command_contract.md](resources/command_contract.md).
- Application port example:
  ```go
  type CommandQueue interface {
      Enqueue(ctx context.Context, cmd CommandEnvelope) (commandID string, err error)
  }
  ```
- Worker signature: `func (w *Worker) Handle(ctx context.Context, msg sqs.Message) error` returning error to retry.

## Checklists
**Before coding**
- [ ] Confirm command schema changes across producers/consumers.
- [ ] Determine ack payload (ids, estimated SLA).
- [ ] Size visibility timeout and DLQ config.

**During**
- [ ] Wrap enqueue in context-aware retries with backoff.
- [ ] Write inbox UPSERT logic to enforce idempotency.
- [ ] Emit metrics (enqueue/processing/dlq) + structured logs.

**After**
- [ ] Add integration test using LocalStack for enqueue/dequeue.
- [ ] Document runbook for DLQ replay.
- [ ] Update OpenAPI to state 202 behavior.

## Definition of Done
- HTTP + worker flows compile and pass tests.
- Commands survive retries without duplication.
- DLQ alarm configured (CloudWatch or similar).
- Observability dashboards updated (latency, backlog, DLQ size).

## Minimal Examples
- Gin handler: validate payload, build `CommandEnvelope`, call `queue.Enqueue`, respond `202` with JSON `{ "command_id": "...", "request_id": "..." }`.
- Worker: fetch message, insert inbox row, execute use case, delete message when success; on failure, log and return error.
