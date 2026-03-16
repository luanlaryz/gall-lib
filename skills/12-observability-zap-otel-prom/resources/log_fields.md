# Required Log Fields (zap JSON)

| Field            | Type   | Notes |
|------------------|--------|-------|
| `timestamp`      | ISO8601| auto
| `level`          | string | info/warn/error
| `message`        | string | short, no punctuation at end
| `request_id`     | string | always present
| `trace_id`       | string | OTel trace span context
| `seller_id`      | string | optional for public endpoints, required elsewhere
| `conversation_id`| string | when available
| `command_id`     | string | for async flows
| `component`      | string | e.g., http.api, worker.processor
| `error`          | string | err message when level>=error
| `duration_ms`    | number | when logging operation durations

Add structured payloads via nested objects (zap fields) instead of string concatenation.
