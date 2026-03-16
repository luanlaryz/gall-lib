# LLM Safety Threat Model

| Threat | Vector | Impact | Required Mitigations | Detection Signals |
|---|---|---|---|---|
| Direct prompt injection | User message tries to override system policy | Unsafe tool calls, policy bypass | Strict system/user separation, PolicyGate before actions, fail-closed | Spike in policy denials by rule ID |
| Indirect prompt injection | CRM/email/HTML includes hidden instructions | Agent follows attacker text from external content | ContentSanitizer, source tagging (`untrusted`), tool allowlist | Sanitizer strips script/tags; blocked tool attempts |
| Tool injection | Model fabricates tool name/params outside allowlist | Unauthorized actions or data writes | ToolInputValidator + allowlist by seller profile | Denied tool names/params in logs |
| Data exfiltration | Prompt asks for secrets/internal prompts | Secret leakage, privacy incident | Secret redaction, deny sensitive intents, output filters | Redaction counters, denied exfil intents |
| Cross-tenant leakage | Tool query omits or tampers `seller_id` filter | Tenant data breach | Mandatory seller scope in all tool contracts, query guards, tests | Scope violation metrics/logs |
| Context window poisoning | Large hostile content dominates prompt | Policy dilution and wrong actions | Truncation + priority ordering + policy restatement | High truncation count + repeated deny |
| Action hijack via handoff | Malicious content requests transfer/escalation | Unauthorized workflow transitions | PolicyGate on `handoff`/`transfer` actions | Denied orchestrator action metrics |

## Invariant Fields
- Always carry `seller_id`, `request_id`, `conversation_id`, and `seq` in decision logs/metrics.
- Block decisions must include `reason` and `rule_id`.
