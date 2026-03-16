# Metrics Catalog

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | counter | `route`, `method`, `status`, `seller_id` | Count of HTTP requests |
| `http_request_duration_seconds` | histogram | `route`, `seller_id` | Latency |
| `commands_enqueued_total` | counter | `command_type`, `seller_id` | Number of commands queued |
| `command_processing_duration_seconds` | histogram | `command_type`, `seller_id` | Worker processing time |
| `redis_operations_total` | counter | `operation`, `result` | Redis ops |
| `redis_operation_duration_seconds` | histogram | `operation` | Redis latency |
| `rate_limit_blocks_total` | counter | `seller_id`, `tier` | Requests throttled |
| `adk_step_total` | counter | `seller_id`, `step`, `status` | ADK tool usage |
| `adk_run_duration_seconds` | histogram | `seller_id` | Agent run durations |
| `grpc_requests_total` | counter | `service`, `method`, `code` | RPC calls |
| `otel_traces_exported_total` | counter | `backend` | Exporter health |
