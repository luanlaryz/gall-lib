# DLQ Policy

- Primary queue: `cmd-seller-conversation.fifo`
- DLQ: `cmd-seller-conversation-dlq`
- Redrive policy: `maxReceiveCount=3`

## Alerting
- CloudWatch alarm when DLQ inflight > 0 for >5m.
- PagerDuty auto-trigger with seller_id tags.

## Replay Procedure
1. Investigate root cause using logs (filter by `command_id`).
2. Fix bug/config if needed.
3. Use script `cmd/replay_dlq`:
   - Pull messages from DLQ
   - Recompute dedup id if necessary
   - Push back to main queue with same attributes
4. Document incident with command/seller details.

## Retention
- DLQ message retention 14 days.
- Encrypt with KMS (same key as main queue).
