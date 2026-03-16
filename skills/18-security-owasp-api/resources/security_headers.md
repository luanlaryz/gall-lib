# Recommended Security Headers

Apply at HTTP edge (Gin middleware/reverse proxy). Keep consistent across API and docs endpoints where applicable.

## Required
- `X-Content-Type-Options: nosniff`
  - Prevent MIME sniffing.
- `X-Frame-Options: DENY`
  - Prevent clickjacking for API surfaces.
- `Referrer-Policy: no-referrer`
  - Minimize referrer leakage.
- `Permissions-Policy: geolocation=(), microphone=(), camera=()`
  - Disable unused browser capabilities.
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`
  - For API responses/docs where feasible; tune per route if serving assets.
- `Cache-Control: no-store` (for auth/session/sensitive endpoints)
  - Avoid caching sensitive data.

## Conditional
- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - Enable only in HTTPS production environments.
- `Access-Control-Allow-Origin`
  - Never `*` for authenticated endpoints; use explicit allowlist.
- `Access-Control-Allow-Headers`
  - Keep minimal (`Authorization`, `Content-Type`, correlation headers as needed).

## Correlation and Safety
- Always propagate `request_id` in response headers (for support/audit).
- Never include secrets, raw tokens, or provider stack traces in headers.
