---
name: go-skill-index
description: Entry point for Go agents working on sac-agents. Use whenever you need to know which specialized skill governs a change or to verify global guardrails before starting or finishing any task.
---

# Codex Go Skill Index
Goal: ensure every change applies the right specialized skill and respects global guardrails for seller-safe, CRM-agnostic systems.

## When to Use
- Always load this index before starting work to route yourself to the correct specialized skill.
- Re-check before code review or PR merge to confirm no relevant skill was missed.

## Non-negotiables
1. **Seller Isolation**: every flow scopes data, caches, queues, rate limits, and logs by `seller_id` (and `conversation_id` when chatty).
2. **CRM-Agnostic Core**: domain/application layers never reference Talkdesk or other CRM names; keep adapters as the only CRM touchpoint.
3. **Async by Default**: inbound HTTP returns ACK 202 quickly and pushes work to queues unless a skill explicitly states otherwise.
4. **Observability Everywhere**: propagate `context.Context`, `request_id`, `correlation_id`, and emit zap JSON + OTel traces + Prom metrics for each hop.
5. **Effective Go**: follow the 13-go skill for context, errors, interfaces, concurrency, and tests.

## Do / Don't
- **Do**: Identify the skill ID(s) below that match the asset you modify and load them fully.
- **Do**: Record in PR description which skills were followed.
- **Don't**: Mix patterns from multiple skills without reconciling terminology (ports, adapters, queues, schemas).
- **Don't**: Introduce new libs or infra outside these skills without design review.

## Interfaces / Contracts
- Use this table to map work areas to skills:
  - `01-hexagonal-architecture`: domain/application layout, ports, adapters, cmd wiring.
  - `02-tenant-auth-quotas`: multi-tenant context, JWT/API keys, per-seller rate limits, CRM adapters.
  - `03-async-command-processing`: ACK 202, SQS FIFO, workers.
  - `04-streaming-sse-ws`: Redis Streams, SSE, WebSocket replay.
  - `05-adk-agent-core`: ADK Go agent orchestration and streaming deltas.
  - `06-multi-provider-model-routing`: seller model routing, Postgres config, Redis cache.
  - `07-postgres-pgx-migrate`: pgx usage, migrations, multi-schema.
  - `08-redis-cache-streams`: caching, rate limit, locks, streams.
  - `09-sqs-fifo-idempotency`: group IDs, dedup, DLQ, inbox pattern.
  - `10-http-gin-openapi`: REST handlers, middleware, OpenAPI updates.
  - `11-grpc-interceptors-health`: gRPC services, interceptors, validation.
  - `12-observability-zap-otel-prom`: logging, tracing, metrics, Grafana.
  - `13-go-idiomatic-effective-go`: Go idioms, context, errors, concurrency, tests.
  - `14-config-viper-flags`: config loading, feature flags, per-seller toggles.
  - `15-devx-ci-precommit-changelog`: tooling, Makefile, CI, pre-commit, changelog.
  - `16-object-calisthenics`: **quando aplicar** em mudanças/refactors de `domain` e `application` para reduzir nesting, usar tipos fortes e funções menores.
  - `17-solid-go-ports`: **quando aplicar** ao definir/revisar ports, use cases e adapters com SOLID + direção de dependência hexagonal.
  - `18-security-owasp-api`: **quando aplicar** em auth/authz, guardas de tenant, endpoints HTTP, validação de entrada, headers, secrets e gates de CI.
  - `19-prompt-injection-llm-safety`: **quando aplicar** em agentes, tools, conteúdo vindo de CRM e ações de orquestração ADK/LLM.
  - `20-testing-strategy-regression-load-containers`: **quando aplicar** em mudanças de SQS, worker, Redis Streams, DB, migrations, model routing, CI de regressão, carga/performance e regras de negócio que precisem de cobertura sem deixar testes redefinirem comportamento.

## Checklists
**Before starting**
- [ ] Identify change scope and map to skill IDs above.
- [ ] Load each skill and required resource files.
- [ ] Confirm terminology alignment (seller_id, conversation_id, request_id, seq).

**During work**
- [ ] Keep architecture boundaries intact (domain vs adapters).
- [ ] Apply Effective Go and observability patterns consistently.
- [ ] Ensure multi-tenant safety (auth, rate limit, data isolation).

**Before PR**
- [ ] Run relevant tests/linters per skill instructions.
- [ ] Update docs/diagrams referenced by skills if behavior changed.
- [ ] Capture skill checklist status in PR template.
- [ ] If touching async flow/infra/core behavior, apply skill `20-testing-strategy-regression-load-containers`.

## Definition of Done
- Referenced skills applied with evidence (code/comments/tests) for every touched subsystem.
- No contradictions with non-negotiables.
- PR checklist attached and green.
- Observability + config updated (or explicitly not needed).

## Minimal Examples
- Example PR note: `Skills: 01-hexagonal-architecture, 03-async-command-processing, 12-observability-zap-otel-prom`. Include checklist results inline.
- Example routing decision: modifying HTTP handler + queue worker => load skills 10 + 03 + 08 + 12 + 13.
- Example testing routing: modifying SQS worker + Redis Streams + migrations => load skills 20 + 03 + 04 + 07 + 09.
