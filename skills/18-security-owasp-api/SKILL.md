---
name: security-owasp-api-baseline
description: Enforce OWASP API Security baseline for auth, tenant isolation, validation, headers, secrets handling, and CI security checks.
---

# OWASP API Security Baseline
Goal: keep APIs tenant-safe and production-hardened with consistent controls from request entry to CI gates.

## When to Use
- Any change in auth, authorization, tenant resolution, middleware, or endpoints.
- Touching request parsing, validation, headers, rate limiting, secrets, or logging.
- Updating CI, dependencies, linters, or release automation affecting security posture.

## Non-negotiables
1. AuthN/AuthZ mandatory and scoped by `seller_id`; deny by default on missing/invalid scope.
2. Rate limiting required per `seller_id` and per endpoint.
3. Validate and bound all inputs (body/query/header/path) with explicit size limits.
4. Use secure defaults: CORS policy explicit, TLS in production, security headers enabled.
5. Return standardized errors; never leak stack traces, internals, credentials, or provider payloads.
6. Access secrets only via `SecretsProvider`; never log tokens/keys/secrets.
7. Logs must sanitize PII and sensitive content while preserving `seller_id`, `request_id`, `conversation_id`, `seq`.
8. CI must run `govulncheck` and `gosec` (directly or via `golangci-lint`) and keep dependency versions pinned/reviewed.

## Do / Don't
- **Do** enforce tenant scope in middleware and re-check in use case for critical actions.
- **Do** reject oversized payloads early (413/400).
- **Do** keep audit-safe logs and metrics for auth/rate-limit denials.
- **Don't** trust CRM/webhook payloads without validation/sanitization.
- **Don't** accept wildcard CORS in production.
- **Don't** expose raw error values from drivers/SDKs directly to clients.

## Interfaces / Contracts
- Auth context contract:
  ```go
  type AuthContext struct {
      SellerID       string
      Subject        string
      Scopes         []string
      RequestID      string
      ConversationID string
  }
  ```
- Error envelope contract:
  ```json
  {"error":{"code":"forbidden","message":"access denied","request_id":"req-123"}}
  ```
- References:
  - [security_headers.md](resources/security_headers.md)
  - [ci_security_checks.md](resources/ci_security_checks.md)

## Checklists
**Before**
- [ ] Identify attack surface (endpoint, middleware, adapter, worker input).
- [ ] Define authz scope and rate-limit strategy by `seller_id`.
- [ ] Define max input sizes and accepted content types.

**During**
- [ ] Add/keep validation for path/query/header/body.
- [ ] Enforce secure headers and CORS policy.
- [ ] Sanitize logs and map internal errors to safe public codes.

**After**
- [ ] Run security checks (`govulncheck`, `gosec`/`golangci-lint`).
- [ ] Verify no secrets or PII leakage in logs and errors.
- [ ] Confirm observability includes denied/auth/rate-limit metrics.

## Definition of Done
- Endpoint/auth flow is tenant-scoped and deny-by-default.
- Input size/shape constraints are explicit and tested.
- Security headers and error envelope are consistent.
- CI security gates are active and passing.

## Minimal Examples
- Reject oversized body:
  ```go
  c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20) // 1 MiB
  ```
- Safe error mapping:
  ```go
  if errors.Is(err, ErrForbidden) {
      writeError(c, http.StatusForbidden, "forbidden", "access denied")
      return
  }
  ```
