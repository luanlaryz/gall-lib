# Tenant Context Contract

## Headers / Metadata
- `X-Seller-ID` (required) — canonical tenant identifier.
- `X-Request-ID` (required) — upstream-provided, fallback to generated UUID v4.
- `X-Conversation-ID` (optional) — for conversational flows; generate when absent.
- `Authorization` — `Bearer <JWT>` for server-to-server auth or `ApiKey <token>` for key auth.

## Context Struct
```go
type TenantContext struct {
    SellerID       string
    AuthMethod     AuthMethod // JWT or APIKey
    Subject        string      // user_id or integration name
    Scopes         []string
    ConversationID string
    RateLimitTier  string      // standard, burst, premium
    RequestID      string
}
```

## JWT Expectations
- Validate with shared JWKS cache per seller.
- Required claims: `iss` (seller auth domain), `sub`, `exp`, `seller_id`.
- Optional: `scopes`, `roles`, `tier`.

## API Key Expectations
- Stored hashed in Postgres `seller_api_keys` table.
- Keys scoped to features; rate limit tier derived from row.
- Rotate every 90 days; log usage with request metadata.

## Rate Limit Inputs
- Redis bucket key: `rate:seller:{seller_id}:window:{minute}`.
- Limits expressed as requests per rolling minute, override via seller flags.
- Burst handling: allow +20% headroom with TTL-limited counter.

## CRM Adapter Boundary
- Tenant context flows into CRM adapter through interface `CRMContextProvider` returning sanitized fields so CRM-specific code never mutates shared state.
