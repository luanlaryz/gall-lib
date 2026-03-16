# CI Jobs

## Required Pre-merge Jobs
1. `guardrails`
   - Runs `bash scripts/ci/guardrails.sh`
   - Fails when core behavior changes are not accompanied by tests and/or contract artifacts according to the repo rules.
1. `unit`
   - Runs `make test-unit`
   - Fails fast on business-rule regressions.
2. `integration`
   - Runs `make test-integration`
   - Requires Docker and validates Postgres, Redis, LocalStack paths via `testcontainers-go`.
3. `e2e`
   - Runs `make test-e2e`
   - Small suite proving `202`, queue handoff, worker output, stream replay.
4. `regression`
   - Runs `make test-regression`
   - Optional aggregator job, required before merge if separate jobs are not mandatory checks.

## Recommended Performance Jobs
- `load-smoke`
  - Trigger on PR label or nightly.
  - Runs short `make load-api` and `make load-e2e`.
- `load-nightly`
  - Runs broader `make load-stream-sse`, `make load-stream-ws`, `make load-e2e`.
  - Publishes trend artifacts for ACK p95/p99, error rate, end-to-end duration.

## CI Rules
- Do not require external shared cloud infra for routine regression jobs.
- Run the guardrail script before tests so invalid PR shapes fail early.
- Keep E2E job small; prefer deterministic fake agent engine.
- Persist artifacts when jobs fail: logs, `k6` summaries, container startup output, flaky retry evidence.
- If business behavior changes, CI must fail until tests and contract docs are updated together.
