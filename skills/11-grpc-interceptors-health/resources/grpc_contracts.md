# gRPC Contracts

## Interceptor Stack (Unary & Stream)
1. `RequestID` — propagate metadata `x-request-id`.
2. `TenantAuth` — builds TenantContext from metadata (mirrors HTTP headers).
3. `Logging` — zap structured logs.
4. `Metrics` — Prometheus histogram/counters per method.
5. `Tracing` — OTel span injection/extraction.
6. `Recovery` — convert panics to gRPC errors.
7. `Authz` — optional per-method scope checks.

## Health Service
- Use `grpc/health/grpc_health_v1`. Register as `health.Server`.
- Endpoint `/grpc.health.v1.Health/Check` must require no auth.

## Validation
- Use `protovalidate` (bufbuild) or `envoyproxy/protoc-gen-validate`.
- Validation runs in interceptor before hitting handlers.
- Errors mapped to `codes.InvalidArgument` with field paths.

## Metadata Expectations
- `x-seller-id`
- `authorization`
- `x-request-id`
- `x-conversation-id` (optional)

## OpenAPI parity
- For shared functionality, keep proto comments + buf registry docs in sync with HTTP spec.
