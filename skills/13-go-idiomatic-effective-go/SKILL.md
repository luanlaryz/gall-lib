---
name: go-idiomatic-effective-go
description: Apply Effective Go patterns: context propagation, error handling, small interfaces, safe concurrency, table-driven tests.
---

# Go Idiomatic Guide
Goal: ensure every Go change follows Effective Go practices for maintainability and safety.

## When to Use
- All Go changes (default skill) especially domain/application logic, concurrency, and testing.
- Reviewing PRs for idiomatic pitfalls.

## Non-negotiables
1. Every IO/public function accepts `context.Context` as first parameter.
2. Errors wrapped with `%w`, no panics for control flow.
3. Interfaces live close to consumers and remain minimal.
4. Concurrency uses context-aware `errgroup`/channels; no leaks.
5. Tests are table-driven with deterministic seeds/fakes.

## Do / Don't
- **Do** return `(value, error)` instead of `(bool)` flags.
- **Do** prefer composition over inheritance-like embedding when clarity suffers.
- **Do** implement `String()`/`MarshalJSON` on domain types when helpful.
- **Don't** expose struct fields publicly without need.
- **Don't** swallow errors; wrap and propagate.
- **Don't** spin goroutines without cancellation plan.

## Interfaces / Contracts
- Error handling reference: [error_handling.md](resources/error_handling.md).
- Concurrency patterns: [concurrency_patterns.md](resources/concurrency_patterns.md).
- Interface template:
  ```go
  type CommandQueue interface {
      Enqueue(ctx context.Context, cmd CommandEnvelope) (string, error)
  }
  ```

## Checklists
**Before coding**
- [ ] Decide ownership of context + cancellation.
- [ ] Identify domain boundaries requiring interfaces.
- [ ] Plan tests (unit vs integration) + fakes.

**During**
- [ ] Keep functions short; extract helpers when logic grows.
- [ ] Use `errors.Is/As` when branching on errors.
- [ ] Manage goroutines with errgroup or explicit Stop channels.

**After**
- [ ] Run `go test ./...` and linters (go vet, staticcheck if available).
- [ ] Review diff for wrapped errors and context usage.
- [ ] Update tests to cover regressions + edge cases.

## Definition of Done
- Code builds, tests pass, lints clean.
- No data races (run `go test -race` when concurrency touched).
- Interfaces documented and fakes implemented for tests.
- Error messages short, actionable, no punctuation.

## Minimal Examples
- Table-driven test snippet:
  ```go
  func TestRateLimiter(t *testing.T) {
      cases := []struct {
          name string
          limit int
          wantAllow bool
      }{
          {"under", 10, true},
          {"over", 0, false},
      }
      for _, tc := range cases {
          t.Run(tc.name, func(t *testing.T) {
              got := limiter.Allow(tc.limit)
              if got != tc.wantAllow {
                  t.Fatalf("Allow()=%v want %v", got, tc.wantAllow)
              }
          })
      }
  }
  ```
- Context usage: `func (s *Service) Handle(ctx context.Context, req Request) error { ctx, span := tracer.Start(ctx, "Service.Handle"); defer span.End(); ... }`.
