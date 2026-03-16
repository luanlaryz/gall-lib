# Ports Review Checklist (Go + Hexagonal)

## Architecture
- [ ] Port is defined on consumer side (`application` use case package).
- [ ] `domain`/`application` import no adapter package.
- [ ] Wiring for concrete adapter exists only in `cmd/*`.

## Interface Shape (ISP + DIP)
- [ ] Interface has 1-3 methods.
- [ ] Method names express business intent, not transport/SDK terms.
- [ ] Method arguments carry required scope (`seller_id`, `conversation_id`, `request_id`).
- [ ] No interface combines unrelated responsibilities.

## Responsibilities (SRP)
- [ ] Handler only parses/validates I/O and returns HTTP/gRPC response.
- [ ] Use case coordinates policy, idempotency, and orchestration.
- [ ] Adapter maps between core DTOs and external APIs/DB/queue.

## Extension Safety (OCP)
- [ ] Adding provider/CRM requires only new adapter + wiring.
- [ ] No core switch/case by provider in `domain`/`application`.
- [ ] Shared contracts remain stable after extension.

## Substitutability (LSP)
- [ ] Fakes return same semantic errors as real adapter.
- [ ] Fakes preserve constraints (timeouts, not-found, conflicts).
- [ ] Tests validate behavior, not implementation details.

## Observability and Tenant Safety
- [ ] Logs include `seller_id`, `request_id`, `conversation_id` when relevant.
- [ ] Metrics/traces preserve same correlation fields.
- [ ] No cross-tenant read/write path in adapter logic.
