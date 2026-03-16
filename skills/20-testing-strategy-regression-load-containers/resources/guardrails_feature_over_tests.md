# Guardrails: Feature Over Tests

## Decision Rule
- Contract, ADR, and approved business behavior come first.
- Tests verify behavior; they do not create new behavior by themselves.

## When a Test Fails
1. Ask: is the test asserting documented behavior or inventing a new rule?
2. Validate the contract artifact first: `docs/**`, `api/**`, event schema, OpenAPI/proto, ADR.
3. If documented behavior is correct, fix code, fixture, or the test to reflect that contract and keep the contract intact.
4. If the test proposes new behavior, stop and call it out explicitly in the PR/issue before changing production logic.
5. Update code, tests, and contract docs together when the business rule truly changes.

## CI Enforcement
- Guardrail CI #1:
  - If `internal/domain/**` or `internal/application/**` changed, require at least one change in `docs/**`, `api/**`, `internal/**/*_test.go`, or `test/**`.
- Guardrail CI #2:
  - If core files and tests changed together, also require a change in `docs/**` or `api/**`.
- Implementation entrypoint:
  - `bash scripts/ci/guardrails.sh`

## Required Agent Language
- Use wording like:
  - `This test failure indicates a behavior mismatch with the approved contract.`
  - `This test update would change business behavior; contract/ADR approval is required before modifying production code.`
  - `I am updating the test to reflect existing behavior, not introducing a new rule.`

## Anti-patterns
- Changing queue/stream/tenant semantics only because a brittle test assumed the wrong behavior.
- Making handlers synchronous so an E2E test can assert completion in one HTTP response.
- Relaxing constraints or idempotency rules to avoid fixing fixtures or migrations.
