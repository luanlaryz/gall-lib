# HTTP Contracts

## Middleware Chain (Gin)
1. `RequestID` — ensures `X-Request-ID` header; generates when absent.
2. `TenantAuth` — maps headers to `TenantContext` (skill 02).
3. `RateLimit` — rejects with 429 when quota exceeded.
4. `Recovery` — zap logging + sanitized 500 payload.
5. `Tracing` — OTel HTTP instrumentation.
6. `Metrics` — Prometheus histogram for latency/status.

## Standard Responses
- Success envelope:
  ```json
  {
    "request_id": "...",
    "data": {...}
  }
  ```
- Error envelope:
  ```json
  {
    "request_id": "...",
    "error": {
      "code": "unauthorized",
      "message": "token expired"
    }
  }
  ```
- Async command POST returns `202` with `{ "request_id", "command_id" }`.

## OpenAPI Workflow
1. Update `api/openapi.yaml` after handler changes.
2. Run `make openapi-lint`.
3. Regenerate client/server stubs if needed.
4. Commit spec + generated files in same PR.
