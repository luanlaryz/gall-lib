---
name: postgres-pgx-migrate
description: Work with Postgres via pgx (pool, transactions, jsonb) and manage multi-schema migrations using golang-migrate.
---

# Postgres + pgx + Migrations
Goal: keep Postgres the reliable source with safe migrations, pgx best practices, and multi-schema isolation.

## When to Use
- Writing repositories, transactions, or raw SQL via pgx.
- Adding/modifying migrations, schemas, or indexes.
- Handling jsonb fields or multi-schema setups.

## Non-negotiables
1. Use `pgxpool.Pool` injected via ports; no global connections.
2. Context passed to every query with deadlines.
3. Migrations organized per module/schema, executed via Make target.
4. Multi-tenant tables always include `seller_id` and relevant indexes.
5. jsonb used only when schema flexibility needed; accompany with constraints if critical.

## Do / Don't
- **Do** use `pgx.NamedArgs` or struct scanning for clarity.
- **Do** wrap multi-step writes in transactions (`BeginTx` + `defer tx.Rollback`).
- **Do** log slow queries (>200ms) with SQL + plan hints.
- **Don't** use `database/sql` directly; stay on pgx.
- **Don't** rely on implicit transactions in migrations.
- **Don't** cast jsonb to text for filtering when indexes can be used.

## Interfaces / Contracts
- Repository port example:
  ```go
  type ConversationRepository interface {
      WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error
      Save(ctx context.Context, tx pgx.Tx, conv Conversation) error
  }
  ```
- Migration standards documented in [migration_rules.md](resources/migration_rules.md).

## Checklists
**Before coding**
- [ ] Determine schema (control/conversation/telemetry) and access pattern.
- [ ] Decide if transaction boundaries need saga/support.
- [ ] Plan indexes for new queries.

**During**
- [ ] Use prepared statements/batch when iterating.
- [ ] Handle pgx errors with `errors.Is(err, pgx.ErrNoRows)` etc.
- [ ] Structure migrations with reversible down scripts.

**After**
- [ ] Run `make migrate MODULE=<module>` locally.
- [ ] Add integration tests hitting new queries.
- [ ] Update ERD/reference docs when schema changes.

## Definition of Done
- Queries context-aware, tested, and instrumented.
- Migrations applied locally without conflicts.
- Rollback path documented.
- Schema changes communicated to dependent skills (config, routing, async).

## Minimal Examples
- Transaction: `repo.WithTx(ctx, func(ctx context.Context, tx pgx.Tx) error { ... })` ensuring commit/rollback.
- Migration snippet adding jsonb column with index:
  ```sql
  ALTER TABLE conversation.messages ADD COLUMN metadata jsonb DEFAULT '{}'::jsonb NOT NULL;
  CREATE INDEX CONCURRENTLY idx_messages_metadata ON conversation.messages USING GIN (metadata);
  ```
