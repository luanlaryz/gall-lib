# seller_model_config Schema

Table: `control.seller_model_config`

| Column              | Type        | Notes |
|---------------------|-------------|-------|
| seller_id           | text PK     | matches tenant context |
| provider            | text        | e.g., `openai`, `anthropic`, `vertex` |
| model               | text        | provider-specific model id |
| params              | jsonb       | arbitrary provider config (temperature, top_p, tool hints) |
| max_output_tokens   | int         | guardrail |
| tools               | jsonb       | list of enabled tools |
| updated_at          | timestamptz | audit |
| updated_by          | text        | human or system |

## Redis Cache
- Key: `seller:model-config:{seller_id}`
- TTL: 5 minutes (refresh early on misses or critical updates).
- Value: JSON copy of table row.

## API Validation Rules
- Validate provider/model against allowlist.
- Validate `params` schema per provider at application layer (not DB constraint).
- Default fallback row stored under seller `default` per environment.

## Multi-provider Routing
1. Fetch config from Redis; on miss, query Postgres and warm cache.
2. Merge feature flags (skill 14) before returning to caller.
3. Provide typed struct:
```go
type SellerModelConfig struct {
    SellerID string
    Provider string
    Model    string
    Params   map[string]any
    MaxOutputTokens int
    Tools   []string
}
```
