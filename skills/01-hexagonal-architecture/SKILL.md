---
name: hexagonal-architecture
description: Maintain the hexagonal (ports-and-adapters) structure for sac-agents: domain/application/usecases/ports/adapters/cmd layering with CRM-agnostic core.
---

# Hexagonal Architecture Playbook
Goal: keep the codebase layered (domain, application, adapters) so features remain CRM-agnostic, testable, and multi-tenant ready.

## When to Use
- Adding or modifying domain logic, use cases, adapters, or command binaries.
- Touching wiring/DI for HTTP, worker, or CLI entry points.
- Reviewing PRs that blend responsibilities across layers.

## Non-negotiables
1. Domain layer imports stdlib only; never infra libs.
2. Application layer depends on domain + small ports (interfaces) that describe side effects.
3. Adapters live behind ports; each adapter file implements one interface and is vendor-specific.
4. CMD packages only assemble dependencies, parse config, and start servers/workers.
5. All cross-layer contracts use canonical IDs (`seller_id`, `conversation_id`, `request_id`, `seq`).

## Do / Don't
- **Do** keep structs immutable via constructors or value copies; return domain errors to application.
- **Do** define ports near use cases (e.g., `internal/application/port`).
- **Do** create DTOs in adapters when external schemas differ from domain models.
- **Don't** import adapters into domain/application packages.
- **Don't** call SQL/HTTP SDKs from use cases.
- **Don't** leak CRM names into domain logic; keep them inside CRM adapters.

## Interfaces / Contracts
- Ports should be consumer-driven and tiny (see Effective Go skill). Example:
  ```go
  type ConversationStore interface {
      Create(ctx context.Context, sellerID string, c Conversation) error
      Get(ctx context.Context, sellerID, conversationID string) (Conversation, error)
  }
  ```
- Use resource [repo_layout.md](resources/repo_layout.md) for canonical directory layout.
- Wiring contract: `cmd/api/main.go` builds app via `internal/app.NewServer(cfg)` so tests can supply fake adapters.

## Checklists
**Before coding**
- [ ] Identify which layer changes; confirm dependencies only flow inward.
- [ ] Define/adjust port interfaces before touching adapters.
- [ ] Plan DTOs for any new external schemas.

**During**
- [ ] Keep constructors in adapters for pgx, Redis, SQS, etc., injecting via interfaces.
- [ ] Ensure use cases handle retries/idempotency at application layer, not adapters.
- [ ] Add unit tests per layer (domain pure tests, adapters integration fakes, wiring smoke tests).

**After**
- [ ] Verify `go test ./...` in touched packages.
- [ ] Update diagrams/docs if new adapter exists.
- [ ] Mention port/interface changes in PR description.

## Definition of Done
- Layer boundaries intact; no forbidden imports.
- Ports documented and implemented with fakes in tests.
- CMD wiring compiles and respects config skill.
- Domain/application tests cover new logic; adapters have coverage or manual verification noted.

## Minimal Examples
- Adding CRM-specific HTTP client: create `internal/adapter/crm/talkdesk_client.go` implementing `CRMClient` port; domain unaware of CRM specifics.
- New use case: add `SendAgentReply` in `internal/application/usecase/` using `ConversationStore`, `CommandQueue`, `RateLimiter` ports.
