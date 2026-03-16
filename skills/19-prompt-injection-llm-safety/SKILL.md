---
name: prompt-injection-llm-safety
description: Protect agent orchestration and tools against prompt injection, tool injection, data exfiltration, and cross-tenant leakage.
---

# Prompt Injection and LLM Safety
Goal: enforce policy gates and least-privilege boundaries so agent/tool flows remain safe, tenant-scoped, and observable.

## When to Use
- Changing ADK orchestration, tool registration, tool execution, or handoff actions.
- Ingesting untrusted CRM/email/HTML content into prompts.
- Implementing agent features that read/write tenant data or call external systems.

## Non-negotiables
1. Separate trusted `system/policy` instructions from untrusted `user/tool` content.
2. Enforce tool allowlist by agent profile and `seller_id` feature flag.
3. Apply least privilege: every tool call scoped to `seller_id` + `conversation_id` + `request_id`.
4. Execute mandatory `PolicyGate` before applying `OrchestratorActions` (handoff/transfer/escalate/tool-call).
5. Sanitize/normalize inbound content (strip active HTML/script, truncate, normalize encoding).
6. Emit logs + metrics for all blocked actions and policy denials.
7. Never allow cross-tenant context retrieval or tool execution.

## Do / Don't
- **Do** classify content source (`trusted`, `untrusted`, `external`) before prompting.
- **Do** include deny reasons and policy IDs in audit logs.
- **Do** fail closed on validator/policy timeouts.
- **Don't** let model output directly execute tools without validation.
- **Don't** pass raw HTML/email bodies as trusted instructions.
- **Don't** return hidden prompts, secrets, or other-tenant data in responses.

## Interfaces / Contracts
- Suggested ports:
  ```go
  type PolicyGate interface {
      Evaluate(ctx context.Context, action OrchestratorAction, context PolicyContext) (Decision, error)
  }

  type ToolInputValidator interface {
      Validate(ctx context.Context, tool string, input map[string]any) (Decision, error)
  }

  type ContentSanitizer interface {
      Sanitize(ctx context.Context, content ContentInput) (SanitizedContent, error)
  }
  ```
- Decision contract:
  ```go
  type Decision struct {
      Allow  bool
      Reason string
      RuleID string
  }
  ```
- References:
  - [threat_model.md](resources/threat_model.md)
  - [policy_gate_examples.md](resources/policy_gate_examples.md)

## Checklists
**Before**
- [ ] Define trust boundaries and untrusted sources.
- [ ] Define per-seller tool allowlist and feature flags.
- [ ] Define deny behavior and observability fields.

**During**
- [ ] Sanitize inbound CRM/tool text before prompt assembly.
- [ ] Evaluate policy gate before any privileged action.
- [ ] Validate tool inputs and enforce seller/conversation scope.

**After**
- [ ] Add tests for allow/deny paths, including timeout/fail-closed cases.
- [ ] Verify metrics/logs for blocked prompt/tool injection attempts.
- [ ] Confirm no cross-tenant leakage in traces, logs, or stream events.

## Definition of Done
- Prompt/tool execution path has explicit policy checkpoints.
- Unsafe actions are denied with auditable reasons.
- Tool access is least-privilege and seller-scoped.
- Injection and exfiltration scenarios are covered by tests.

## Minimal Examples
- Fail-closed gate:
  ```go
  decision, err := gate.Evaluate(ctx, action, policyCtx)
  if err != nil || !decision.Allow {
      return ErrPolicyDenied
  }
  ```
- Scope check before tool run:
  ```go
  if input.SellerID != ctxSellerID || input.ConversationID != ctxConversationID {
      return ErrScopeViolation
  }
  ```
