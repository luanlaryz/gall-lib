# Event Envelope (Redis Streams + SSE/WS)

```
Stream: stream:conversation:{seller_id}:{conversation_id}
Entry ID: millisecond seq from Redis (e.g., 1719580000000-0)
Fields:
  seq            -> incrementing int maintained per conversation
  cursor         -> `${timestamp}-${seq}` used for replay tokens
  seller_id      -> canonical seller
  conversation_id-> canonical conversation
  event_type     -> e.g., agent.delta, agent.completed, system.error
  payload        -> JSON string describing body
  request_id     -> originating HTTP request id
```

## SSE Contract
- Endpoint: `/conversations/:id/events`.
- Query params: `cursor` (optional). Default = latest delivered.
- Response headers: `Cache-Control: no-store`, `Content-Type: text/event-stream`.
- Event format: `event: <event_type>`, `id: <cursor>`, `data: <payload JSON>`.

## WebSocket Contract
- Path: `/ws/conversations/:id/stream`.
- Protocol message:
  ```json
  {
    "type": "subscribe",
    "conversation_id": "conv-1",
    "cursor": "1719580000000-0"
  }
  ```
- Server pushes JSON with fields `cursor`, `event_type`, `payload`.
- Heartbeat every 15s with `event_type=heartbeat`.

## Replay Rules
- Clients send `cursor`; server reads stream via `XREAD` from `cursor+1`.
- Retention: keep last 24h or 500 events, whichever longer.
- When cursor missing/expired, send `410 Gone` SSE or WS error advising restart without cursor.
