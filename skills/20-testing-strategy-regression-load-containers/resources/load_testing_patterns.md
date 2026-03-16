# Load Testing Patterns

## Tools
- Preferred: `k6` for HTTP, SSE, WebSocket, and full pipeline scenarios.
- Optional: `ghz` for gRPC-specific endpoints if the repo adds critical gRPC hot paths.

## Required Scenarios
- `make load-api`
  - Focus: `POST /messages` ACK only.
  - Validate API returns `202` quickly without waiting for worker completion.
- `make load-stream-sse`
  - Focus: SSE subscribe, reconnect, replay via `after`, event fan-out.
- `make load-stream-ws`
  - Focus: WebSocket subscribe, steady-state delivery, reconnect.
- `make load-e2e`
  - Focus: POST -> queue -> worker -> Redis Streams -> client receives `agent.completed`.

## Minimum Metrics
- ACK latency: p95 and p99.
- Stream delivery latency: first delta and completion.
- End-to-end completion time.
- Error rate.
- Throughput: requests/sec or conversations/min.

## Suggested Thresholds
- ACK `POST /messages`
  - p95 <= 300 ms
  - p99 <= 750 ms
  - error rate < 1%
- SSE / WS first event
  - p95 <= 1 s after worker emits
- End-to-end completion
  - define per use case; start with p95 <= 10 s for fake-engine local runs

## Guardrails
- Use seller-scoped ids in load data.
- Do not disable auth/tenant checks just to reach higher throughput.
- Keep fake agent engines deterministic unless the goal is provider benchmarking.
- Store summaries as CI artifacts so regressions are comparable over time.
