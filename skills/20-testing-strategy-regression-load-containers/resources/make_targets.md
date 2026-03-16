# Make Targets

| Target | Purpose | Expected command shape |
|---|---|---|
| `make test` | fast default developer loop | `go test ./...` or repo equivalent |
| `make test-unit` | unit-only feedback | `go test ./internal/...` |
| `make test-integration` | Postgres/Redis/LocalStack via `testcontainers-go` | `go test -count=1 ./test/integration/...` |
| `make test-e2e` | critical async paths only | `go test -count=1 ./test/e2e/...` |
| `make test-regression` | one-liner pre-merge gate | `make test-unit && make test-integration && make test-e2e` |
| `make load-api` | ACK `202` latency and error rate | run `k6` script under `test/load/api` |
| `make load-stream-sse` | SSE fan-out/replay load | run `k6` script under `test/load/stream_sse` |
| `make load-stream-ws` | WebSocket stream load | run `k6` script under `test/load/stream_ws` |
| `make load-e2e` | full pipeline throughput | run `k6` scenario covering POST -> worker -> stream completion |

## Notes
- Keep `make test-regression` stable; extend internals, do not rename casually.
- Integration and E2E targets should set `-count=1` by default.
- Load targets should accept overridable env vars like `BASE_URL`, `SELLER_ID`, `VUS`, `DURATION`.
- If a folder does not exist yet, create it before adding the target:
  - `test/integration/`
  - `test/e2e/`
  - `test/load/`
