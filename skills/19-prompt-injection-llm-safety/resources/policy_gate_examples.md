# Policy Gate Examples

## 1) Deny cross-tenant tool call

Action:
```json
{
  "type": "tool_call",
  "tool": "crm_get_ticket",
  "input": {
    "seller_id": "seller-b",
    "conversation_id": "conv-10",
    "ticket_id": "123"
  }
}
```

Context:
```json
{
  "seller_id": "seller-a",
  "conversation_id": "conv-10",
  "request_id": "req-1"
}
```

Decision:
```json
{
  "allow": false,
  "reason": "seller scope mismatch",
  "rule_id": "scope.seller.match"
}
```

## 2) Deny disallowed tool by profile

Action:
```json
{
  "type": "tool_call",
  "tool": "secrets_dump",
  "input": {}
}
```

Decision:
```json
{
  "allow": false,
  "reason": "tool not allowlisted for seller profile",
  "rule_id": "tool.allowlist.profile"
}
```

## 3) Allow sanitized CRM summary

Input content:
```json
{
  "source": "crm_email_html",
  "text": "<html><body>Pedido 123<script>alert(1)</script></body></html>"
}
```

Sanitized:
```json
{
  "text": "Pedido 123",
  "truncated": false
}
```

Decision:
```json
{
  "allow": true,
  "reason": "content sanitized and scope valid",
  "rule_id": "content.sanitized.ok"
}
```

## 4) Fail-closed on validator timeout

Decision:
```json
{
  "allow": false,
  "reason": "tool input validator timeout",
  "rule_id": "validator.timeout.fail_closed"
}
```
