# Command Envelope Contract

```json
{
  "version": 1,
  "seller_id": "seller-123",
  "conversation_id": "conv-789",
  "command_id": "uuid",
  "command_type": "send_agent_reply",
  "payload": {
    "message_id": "uuid",
    "body": "...",
    "metadata": {
      "language": "en",
      "priority": "default"
    }
  },
  "dedup_key": "seller-123:conv-789:send_agent_reply:uuid",
  "created_at": "2026-03-05T12:00:00Z"
}
```

- **SQS FIFO** queue name: `cmd-seller-conversation.fifo`.
- **MessageGroupId**: `seller:{seller_id}:conversation:{conversation_id}`.
- **MessageDeduplicationId**: SHA256 of `dedup_key`.
- **DLQ**: `cmd-seller-conversation-dlq` with `maxReceiveCount=3`.
- **Attributes**: `request_id`, `trace_id`, `command_type`.

## Worker Expectations
- Poll via long polling (20s) with visibility timeout >= worker SLA.
- Use idempotent inbox table (Postgres) storing `command_id`, `status`, `processed_at`.
- Confirm processing before deleting message; ack errors by leaving message to retry.
- Publish completion events to Redis Streams when needed for streaming skill.
