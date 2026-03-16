---
name: http-gin-openapi
description: Build REST endpoints with Gin + standard middleware, 202 ACK flows, and synchronized OpenAPI specs.
---

# HTTP (Gin + OpenAPI)
Goal: ship REST handlers that honor tenant middleware chain, async ACK pattern, and stay documented via OpenAPI.

## When to Use
- Creating/updating HTTP handlers, middleware, routing, or OpenAPI specs.
- Working on REST-specific validation or response envelopes.
- Adding endpoints exposing streaming cursors or command IDs.

## Non-negotiables
1. Middleware order matches [http_contracts.md](resources/http_contracts.md).
2. All handlers accept/propagate `context.Context` from `*gin.Context`.
3. Responses follow standard envelopes (data/error) and include `request_id`.
4. Async endpoints return `202 Accepted` with command info.
5. OpenAPI kept in sync and linted.

## Do / Don't
- **Do** validate payloads with binding + struct tags; add explicit enums.
- **Do** convert domain errors to HTTP codes using centralized mapper.
- **Do** attach OTel span attributes (seller_id, route, status).
- **Don't** perform long-running work inside handler; enqueue commands.
- **Don't** leak internal error messages; map to sanitized codes.
- **Don't** forget CORS/CSRF requirements when exposing to browsers.

## Interfaces / Contracts
- Handler skeleton:
  ```go
  func (h *Handler) CreateCommand(c *gin.Context) {
      ctx := c.Request.Context()
      tenant := tenantctx.From(ctx)
      var req CreateCommandRequest
      if err := c.ShouldBindJSON(&req); err != nil {
          h.respondError(c, ErrInvalidPayload)
          return
      }
      cmdID, err := h.usecase.Enqueue(ctx, tenant, req)
      if err != nil {
          h.respondError(c, err)
          return
      }
      c.JSON(http.StatusAccepted, gin.H{"request_id": tenant.RequestID, "command_id": cmdID})
  }
  ```
- Contracts + envelopes detailed in [http_contracts.md](resources/http_contracts.md).

## Checklists
**Before coding**
- [ ] Confirm OpenAPI paths/methods needed.
- [ ] Determine auth scopes + rate limit tiers.
- [ ] Define request/response schemas + examples.

**During**
- [ ] Implement handler + tests (table-driven) covering success/error.
- [ ] Update OpenAPI + run lint.
- [ ] Wire route in `cmd/api/main.go` respecting middleware order.

**After**
- [ ] Verify `go test ./internal/adapter/http/...`.
- [ ] Hit endpoint via `curl` or Postman to confirm 202 + headers.
- [ ] Update docs/changelog referencing endpoint.

## Definition of Done
- Handler integrated with middleware, tests, and OpenAPI spec.
- Responses consistent and observability emitting route metrics.
- Async flows enqueued correctly.
- Docs include sample request/response.

## Minimal Examples
- `GET /healthz` returns 200 soon with build info (no tenant context required) but still logs/traces.
- `POST /conversations/:id/messages` binds JSON, enqueues command, returns 202 with command_id.
