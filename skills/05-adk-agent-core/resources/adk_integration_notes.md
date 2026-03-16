# ADK Go Integration Notes

## Libraries
- `github.com/google/adk-go` for orchestrating agent plans + steps.
- Enable streaming deltas by using `adk.StreamingClient` with callback signature `func(ctx context.Context, delta adk.Delta) error`.

## Conversation Loop
1. Accept normalized user input (already multi-tenant scoped).
2. Create `adk.Session` keyed by `seller_id` + `conversation_id`.
3. Register tools:
   - `SearchTool`
   - `CRMTool` (adapter-specific)
   - `LLMTool` (pluggable per seller)
4. Provide memory/state via Postgres or Redis depending on size.

## Streaming Deltas
- Hook `delta` callback to publish events:
  ```go
  client.Run(ctx, req, func(delta adk.Delta) error {
      event := DeltaToEvent(delta)
      return stream.Publish(ctx, sellerID, conversationID, event)
  })
  ```
- Deltas include `type` (content/tool/status) + `text`/`payload`.

## Error Handling
- Wrap ADK errors with `%w`; classify as retriable vs terminal.
- Emit metrics: `adk_step_total{status}` and `adk_latency_seconds`.

## Configuration
- Pass seller-specific provider/model from `seller_model_config` (see skill 06).
- Respect feature flags for enabling/disabling toolsets per seller.
