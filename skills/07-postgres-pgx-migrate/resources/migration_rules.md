# Migration Rules

## Tooling
- Use `golang-migrate/migrate` with `.sql` files stored under `db/migrations/<module>`.
- Naming: `<timestamp>_<module>_<action>.up.sql` / `.down.sql`.
- Run via Make target `make migrate MODULE=<module>`.

## Multi-schema Layout
- `control` — tenant + config tables.
- `conversation` — chat state, inbox, transcripts.
- `telemetry` — durable metrics/log aggregates.

## Conventions
- Always include `seller_id` first column on multi-tenant tables.
- Use `jsonb` for flexible payloads; add GIN indexes when querying nested fields.
- Default `updated_at`/`created_at` via triggers or DEFAULT.

## Transaction Rules
- Wrap migrations altering multiple tables in a transaction.
- Avoid DDL in long-running transactions during peak hours.
- For large data backfills, use `ALTER TABLE ... ADD COLUMN` with defaults `NULL`, then backfill in batches.

## Testing
- Use `pgxpool` against `docker compose up postgres` for integration tests.
- Provide helper SQL under `db/seeds/` for fixture loading.
