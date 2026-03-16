# Testcontainers Patterns

## Containers Required by Concern
- Postgres
  - Validate migrations, `jsonb` columns, constraints, inbox idempotency, `seller_model_config`.
- Redis
  - Validate Streams replay, result-store TTL, rate-limit counters, seller-scoped keys.
- LocalStack
  - Validate SQS FIFO ordering/grouping, content-based dedup, retry/DLQ with `maxReceiveCount=3`.

## Fixture Rules
- Create `context.WithTimeout` for every suite and every container startup.
- Prefer one container set per suite/package when tests can isolate by schema/key prefixes/ids.
- Use `t.Cleanup` or suite teardown to terminate containers.
- Run migrations against the actual test Postgres container; never bypass them with hand-created tables.
- Use unique `seller_id`, `conversation_id`, `request_id` values per test to prevent state bleed.
- Set `go test -count=1` for integration/E2E targets.

## Suggested Patterns
- Postgres
  - Start container -> build DSN -> run migrations -> construct repo -> seed fixtures.
- Redis
  - Start container -> create isolated client/db -> publish stream events -> read with `after` cursor.
- LocalStack
  - Start container -> create FIFO queue + DLQ + redrive policy -> publish -> receive -> simulate retry path.

## Failure Diagnostics
- On timeout, dump container logs and connection settings, not secrets.
- Assert on canonical fields only: `seller_id`, `request_id`, `conversation_id`, `seq`, queue attributes, Redis ids.
- Avoid sleeps when possible; use `require.Eventually` with bounded waits.
