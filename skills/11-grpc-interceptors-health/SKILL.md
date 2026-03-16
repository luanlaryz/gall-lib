---
name: grpc-interceptors-health
description: Build gRPC services with standardized interceptor chains, auth, validation, and health checks.
---

# gRPC Interceptors & Health
Goal: keep our gRPC services consistent with HTTP behavior, including auth/tenant context, validation, and observability.

## When to Use
- Creating/modifying gRPC services, protos, or server wiring.
- Adjusting interceptors, metadata handling, or health endpoints.
- Adding validation via protovalidate/PGV.

## Non-negotiables
1. Interceptor order follows [grpc_contracts.md](resources/grpc_contracts.md) for both unary and streaming.
2. Metadata carries seller context identical to HTTP headers.
3. Validation errors map to `codes.InvalidArgument` with details.
4. Health service (`grpc.health.v1.Health`) always registered and ready state tied to dependencies.
5. TLS + mTLS enforced per environment configuration.

## Do / Don't
- **Do** generate protos via buf; keep versioned.
- **Do** instrument metrics/traces per method.
- **Do** convert domain errors to gRPC status via central mapper.
- **Don't** bypass interceptors to run raw handlers.
- **Don't** expose experimental methods without feature flags.
- **Don't** break backwards compatibility without version bump.

## Interfaces / Contracts
- Server builder signature:
  ```go
  func NewServer(cfg Config, deps Deps) *grpc.Server {
      unary := grpc.ChainUnaryInterceptor(requestID, tenantAuth, logging, metrics, tracing, recovery, authz)
      stream := grpc.ChainStreamInterceptor(...)
      return grpc.NewServer(grpc.Creds(cfg.Creds), grpc.UnaryInterceptor(unary), grpc.StreamInterceptor(stream))
  }
  ```
- Contracts for metadata, validation, interceptors in [grpc_contracts.md](resources/grpc_contracts.md).

## Checklists
**Before coding**
- [ ] Update proto definitions + buf.yaml when needed.
- [ ] Determine auth scopes + metadata requirements.
- [ ] Plan validation rules (protovalidate) and mapping to errors.

**During**
- [ ] Regenerate Go code (`buf generate`).
- [ ] Implement service logic inside application/use case layers (not adapter) calling ports.
- [ ] Add unit tests using bufconn or grpc-go testing harness.

**After**
- [ ] Run `buf lint` + `buf breaking` against main.
- [ ] Verify health endpoint responds `SERVING` locally.
- [ ] Update docs referencing new RPCs.

## Definition of Done
- Service builds with interceptors, validation, and health wiring.
- Tests cover success, auth failure, validation failure.
- Observability dashboards updated for RPC metrics.
- Buf artifacts committed.

## Minimal Examples
- New RPC `RunAgent`: validate request via protovalidate, call use case, return streaming updates via server-side streaming with same interceptors.
- Health check toggled to `NOT_SERVING` when Redis unavailable by hooking dependency watcher.
