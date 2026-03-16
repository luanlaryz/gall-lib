# Error Handling Patterns

## Wrapping
- Use `fmt.Errorf("context: %w", err)` for propagation.
- Define sentinel errors in package and compare with `errors.Is`.

## Classification
- Domain errors (`ErrNotFound`, `ErrUnauthorized`) stay in application layer for mapping to transport codes.
- Infrastructure errors bubble up with contextual message; add retries in caller.

## Logging
- Log errors once per request at boundary (transport or worker) with request metadata.

## Retries
- Use `backoff` (e.g., `cenkalti/backoff/v4`) or custom linear/exponential for IO operations.
- Cancel retries when context done.

## Testing
- Provide fake implementations returning errors to test handling.
