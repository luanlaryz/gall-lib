# Test Pyramid

## Layer Rules

| Layer | Scope | Infra | Typical assertions | Location |
|---|---|---|---|---|
| Unit | `domain`, `application`, validators, pure mappers | none; use fakes/stubs | business rules, errors, branching, seller isolation logic | `internal/**/**/*_test.go` |
| Integration / Component | repos, migrations, Redis Streams, seller model routing, queue publishers/consumers | `testcontainers-go` for Postgres, Redis, LocalStack | schema constraints, `jsonb`, inbox idempotency, TTL, stream replay, FIFO attributes | `test/integration/**` |
| E2E | critical async journeys only | local app stack + fake agent engine + real infra containers | `202` ACK, worker handoff, emitted events, replay/resume, correlation ids | `test/e2e/**` |
| Load | hot-path performance and availability | local stack or ephemeral env | p95/p99 ACK, error %, end-to-end completion, stream fan-out latency | `test/load/**` |

## Selection Heuristics
- Prefer unit tests when only one use case or domain rule changes.
- Prefer integration when correctness depends on SQL, migrations, Redis behavior, queue semantics, or serialization.
- Prefer E2E only for cross-boundary contracts the business depends on: ACK, worker pipeline, stream delivery, replay.
- Prefer load tests when touching request throughput, stream fan-out, worker concurrency, or storage latency.

## Required Coverage by Subsystem
- `domain` / `application`: unit first; add integration only if persistence or queue contracts changed.
- Postgres repositories: integration mandatory for migrations, constraints, `jsonb`, inbox, `seller_model_config`.
- Redis streams / result store / rate limit: integration mandatory for replay, TTL, and isolation by `seller_id`.
- SQS FIFO / worker loops: integration mandatory with LocalStack; E2E for the business-critical path only.
- HTTP `POST /messages`: E2E mandatory to prove `202` and decoupling from worker completion.

## Anti-patterns
- Replacing missing integration coverage with mocks around SQL, Redis streams, or SQS semantics.
- Adding broad E2E suites for simple use-case branching.
- Keeping flaky cross-test shared state instead of resetting fixtures or using unique ids.
