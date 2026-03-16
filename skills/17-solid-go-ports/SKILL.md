---
name: solid-go-hexagonal-ports
description: Apply SOLID in Go with hexagonal architecture using small ports, explicit adapters, and strict dependency direction.
---

# SOLID for Go + Ports/Adapters
Goal: enforce SOLID with Go idioms so `domain` and `application` stay stable, testable, and independent from infrastructure.

## When to Use
- Creating or changing ports in `internal/application/ports/*`.
- Implementing new adapters (CRM, queue, provider, secrets, cache).
- Reviewing handlers/use cases/adapters for responsibility leaks.

## Non-negotiables
1. DIP: `application` depends on ports only; adapter wiring belongs to `cmd/*`.
2. ISP: ports are small (1-3 methods) and defined by consumer needs.
3. SRP: handler handles I/O; use case orchestrates rules; adapter integrates external systems.
4. OCP: new CRM/provider arrives as new adapter; core logic stays unchanged.
5. LSP: fakes/mocks preserve invariants, errors, and expected behavior contracts.
6. `domain`/`application` never import `adapters`.
7. Contracts keep tenant context explicit (`seller_id`, `request_id`, `conversation_id`).

## Do / Don't
- **Do** split broad interfaces into focused ports.
- **Do** return domain/app errors from use case, not provider-specific details.
- **Do** keep adapter mapping isolated at boundaries.
- **Don't** put orchestration logic in HTTP handlers.
- **Don't** make a shared mega-port for unrelated capabilities.
- **Don't** branch core logic by provider name; use adapter polymorphism.

## Interfaces / Contracts
- Consumer-owned port:
  ```go
  type MessageRepository interface {
      Save(ctx context.Context, in SaveMessageInput) error
      FindByConversation(ctx context.Context, sellerID SellerID, conversationID ConversationID) ([]Message, error)
  }
  ```
- Use case boundary:
  ```go
  type SendMessageUseCase interface {
      Execute(ctx context.Context, in SendMessageInput) (RequestID, error)
  }
  ```
- Error contract: fakes must return `ErrNotFound`, `ErrConflict`, or wrapped infra errors equivalent to real adapters.

## Checklists
**Before**
- [ ] Decide which layer owns the new behavior (handler/use case/adapter).
- [ ] Define minimal ports from use case needs.
- [ ] Identify invariant/error contract to preserve in fakes.

**During**
- [ ] Keep interfaces in consumer package, not provider package.
- [ ] Ensure adapter only translates protocol/SDK concerns.
- [ ] Keep seller scope explicit in method signatures and logs.

**After**
- [ ] Add unit tests with fakes for use case behavior.
- [ ] Verify new adapter works by wiring changes in `cmd/*` only.
- [ ] Confirm no core package imports adapter package.

## Definition of Done
- Ports are minimal, cohesive, and consumer-driven.
- New providers/CRMs can be added without core edits.
- Fakes behave like production adapters for success and failures.
- Dependency direction remains hexagonal and compile-time safe.

## Minimal Examples
- Add CRM: implement `TicketAdapter` in `adapters/crm/<provider>` and bind in `cmd/api`; keep `application` untouched.
- Split interface:
  ```go
  type ModelResolver interface { Resolve(ctx context.Context, sellerID SellerID) (ModelConfig, error) }
  type SecretReader interface { GetSecret(ctx context.Context, sellerID SellerID, key string) (string, error) }
  ```
- Review checklist: [ports_checklist.md](resources/ports_checklist.md).
