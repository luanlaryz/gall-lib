---
name: redis-cache-streams
description: Use Redis for caches, rate limits, locks, result stores, and streams with consistent key naming and TTLs.
---

# Redis Cache & Streams
Goal: operate Redis safely for caching, throttling, locking, and streaming without violating seller isolation.

## When to Use
- Touching Redis-based caches, rate limits, locks, or stream publishers/consumers.
- Designing result stores for async operations.
- Tuning TTLs or eviction policies.

## Non-negotiables
1. Keys always namespaced with `seller_id`; include `conversation_id` when relevant.
2. TTLs defined upfront; no immortal cache entries.
3. Use `go-redis/v9` with context deadlines/timeouts.
4. Lua scripts or atomic commands for rate limits/locks to avoid race conditions.
5. Observability counters for hits/misses, rate limit events, lock contention.

## Do / Don't
- **Do** centralize key patterns referencing [redis_keyspace.md](resources/redis_keyspace.md).
- **Do** instrument `redis_client_duration_seconds` histogram.
- **Do** prefer `XADD`/`XREAD` for streaming requirements.
- **Don't** store large payloads (>512KB) without compression.
- **Don't** rely on `KEYS` in production; use SCAN.
- **Don't** swallow Redis errors; propagate up for retries/fallback.

## Interfaces / Contracts
- Cache port:
  ```go
  type Cache interface {
      Get(ctx context.Context, key string, dest any) (bool, error)
      Set(ctx context.Context, key string, value any, ttl time.Duration) error
      Delete(ctx context.Context, key string) error
  }
  ```
- Rate limiter uses script returning remaining tokens; see tenant skill.
- Stream publisher interface defined in streaming skill.

## Checklists
**Before coding**
- [ ] Decide TTL, eviction strategy, serialization format (JSON, MsgPack).
- [ ] Confirm key pattern and ownership.
- [ ] Evaluate failure modes (cache miss path, fallback).

**During**
- [ ] Use pipelining/batching for multiple operations.
- [ ] Wrap Lua scripts in go helpers for reuse.
- [ ] Add structured logs for lock acquisition/release.

**After**
- [ ] Add tests with miniredis or redis-server via docker compose.
- [ ] Update Grafana dashboards for new keys/metrics.
- [ ] Document runbook for clearing caches or replaying streams.

## Definition of Done
- Keys follow canonical naming and TTL.
- Error handling + fallbacks tested.
- Metrics/logs show hit ratio, rate limit counts, lock contention.
- Stream consumers/producers pass load tests.

## Minimal Examples
- Cache: `cache.Set(ctx, fmt.Sprintf("seller:model-config:%s", sellerID), cfg, 5*time.Minute)`.
- Lock: `SET lock:tool:seller-123:conv-1 token NX PX 15000` followed by Lua compare/delete on release.
