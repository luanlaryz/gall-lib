# Redis Keyspace Map

| Purpose         | Key Pattern                                           | TTL          |
|-----------------|-------------------------------------------------------|--------------|
| Seller config   | `seller:model-config:{seller_id}`                     | 5m           |
| Rate limit      | `rate:seller:{seller_id}:window:{minute}`             | 60s          |
| Locks           | `lock:{namespace}:{seller_id}:{resource}`             | 15s default  |
| Conversation KV | `conv:state:{seller_id}:{conversation_id}`            | 24h          |
| Result store    | `result:{command_id}`                                 | 15m          |
| Redis Stream    | `stream:conversation:{seller_id}:{conversation_id}`   | retention 24h|
| Idempotency     | `inbox:{seller_id}:{command_id}`                      | 24h          |

## Scripts
- Lua script for rate limit: atomic increment + expiry when first set.
- Lock acquisition: `SET lock ... NX PX 15000` + unique token, release via Lua compare/delete.

## Observability Keys
- `metrics:rate_limit:{seller_id}` for aggregated counters.
- Use Redis MONITOR only locally (heavy!).
