---
name: testing-strategy-regression-load-containers
description: Apply when changing async flows, workers, streams, database, migrations, model routing, or core business logic to keep feature behavior authoritative while enforcing regression, load, and testcontainers-based reliability checks.
---

# Testing Strategy: Regression, Load & Containers
Goal: preserve business behavior first, then prove it with fast unit tests, reliable container-based integration tests, critical-path E2E checks, and repeatable load runs.

## When to Use
- Implementing or changing business rules in `domain` or `application`.
- Modifying async flow pieces: HTTP `202`, workers, SQS FIFO, Redis Streams, SSE/WS replay.
- Changing Postgres schemas, migrations, `seller_model_config`, inbox/idempotency, Redis TTLs, or queue contracts.
- Adding CI gates, `make` targets, or load/performance coverage.
- Reviewing a failing test where the correct fix is unclear and behavior must be checked against contract/ADR.

## Non-negotiables
1. Features and approved contracts win; tests do not invent behavior on their own.
2. If a failing test implies a behavior change, call that out explicitly and require contract/ADR approval before changing production logic.
3. When a test fails, the order is fixed: validate contract/ADR -> adjust the test to match the approved contract -> only then change behavior if the contract/ADR is updated in the same PR.
4. Test pyramid is mandatory:
   - Unit: `domain` + `application/usecases` with fakes only.
   - Integration/component: repos, migrations, Redis Streams, model routing, queues using `testcontainers-go`.
   - E2E: few critical flows covering `POST /messages` -> queue -> worker -> Redis Streams -> SSE/WS.
5. Local regression must have one entrypoint: `make test-regression` = unit + integration + E2E.
6. CI guardrail: if `internal/domain/**` or `internal/application/**` changes, the PR must also change either contract artifacts (`docs/**` or `api/**`) or tests (`internal/**/*_test.go` or `test/**`).
7. CI guardrail: if core behavior files and tests change together, the PR must also change contract artifacts (`docs/**` or `api/**`); tests cannot redefine product behavior alone.
8. Load tests live under `test/load/` and cover ACK latency, stream latency, error rate, and end-to-end completion time.
9. Container-based tests use `testcontainers-go` for Postgres, Redis, and LocalStack; prefer suite-level reuse and always cleanup with context timeouts.
10. Run flaky-prone integration/E2E suites with `-count=1`; avoid hidden shared state.
11. Tests must preserve project invariants: `seller_id`, `request_id`, `conversation_id`, `seq`, async `202`, SQS FIFO grouping/dedup/DLQ, Redis Streams replay, no secret/PII logging.
12. Folder conventions are fixed:
   - Unit: near source as `*_test.go` in `internal/**`
   - Integration: `test/integration/**`
   - E2E: `test/e2e/**`
   - Load: `test/load/**`

## Do / Don't
- **Do** add or update the smallest test layer that proves the intended contract.
- **Do** state plainly when a test change is only reflecting existing business rules versus proposing new behavior.
- **Do** validate seller isolation in tests that touch persistence, queues, caches, or streams.
- **Do** keep container fixtures deterministic: fixed schemas, migrations, seed data, explicit cleanup.
- **Do** prefer fake agent engines in E2E unless the scenario is explicitly about ADK/provider integration.
- **Don't** rewrite business logic just to satisfy an outdated or overly strict test.
- **Don't** turn every change into E2E; keep most assertions in unit or integration layers.
- **Don't** use real cloud dependencies in CI for routine integration coverage; use `testcontainers-go` + LocalStack.
- **Don't** assert on sensitive logs, prompts, or secrets.
- **Don't** make POST handlers wait for worker completion; test `202` ACK, not synchronous completion.

## Interfaces / Contracts
- Read [test_pyramid.md](resources/test_pyramid.md) when deciding which layer to add.
- Read [guardrails_feature_over_tests.md](resources/guardrails_feature_over_tests.md) when a red test suggests changing behavior.
- Read [testcontainers_patterns.md](resources/testcontainers_patterns.md) for container fixture rules and infra scopes.
- Read [load_testing_patterns.md](resources/load_testing_patterns.md) for `k6`/`ghz` scenarios and SLO-style thresholds.
- Read [make_targets.md](resources/make_targets.md) and [ci_jobs.md](resources/ci_jobs.md) when touching automation.
- CI enforcement lives in `scripts/ci/guardrails.sh`; keep the script, workflow, docs, and this skill aligned.
- Canonical contracts under test:
  - HTTP: `POST /v1/conversations/{conversation_id}/messages` always returns `202 Accepted` + `request_id`.
  - Queue: `MessageGroupId = seller_id:conversation_id`, content-based dedup, DLQ `maxReceiveCount=3`.
  - Stream: Redis Streams carry monotonic `seq`; SSE/WS replay with `after` cursor.
  - Tenant safety: data, cache, stream, and queue assertions must scope by `seller_id`.

## Checklists
**Before**
- [ ] Confirm whether the change is behavior-preserving or behavior-changing.
- [ ] If core behavior will change, identify which contract artifact (`docs/**` or `api/**`) must change in the same PR.
- [ ] Map the change to the minimum required test layer using the pyramid.
- [ ] If infra is involved, decide which `testcontainers-go` fixtures are needed.
- [ ] If load-sensitive, define which latency/error metrics matter before coding.

**During**
- [ ] Keep unit tests near the business code and avoid infra there.
- [ ] Do not let tests become the only artifact describing a behavior change in `domain` or `application`.
- [ ] Use `context.WithTimeout` in container tests and explicit teardown paths.
- [ ] Run migrations inside Postgres integration tests; do not fake schema behavior.
- [ ] Validate `seller_id`, `request_id`, `conversation_id`, and `seq` where the flow crosses boundaries.
- [ ] Mark any behavior-changing test update in the PR/commit notes.

**After**
- [ ] Run `make test-regression`.
- [ ] Run `make ci-guardrails` or confirm the CI guardrail step will evaluate the same diff.
- [ ] Run the relevant load target when latency, throughput, or stream fan-out changed.
- [ ] Ensure CI jobs mirror the local regression entrypoint.
- [ ] Confirm no tests require logging secrets, prompts, or tenant data to pass.
- [ ] Update resources/automation if new test targets or directories were introduced.

## Definition of Done
- Business behavior matches the approved contract/ADR; no silent semantic drift introduced by tests.
- Any core behavior change is accompanied by updated contract artifacts or explicit confirmation that tests reflect the existing contract.
- `make test-regression` covers unit + integration + E2E and is green locally or in CI.
- Container tests validate the touched infra boundaries with `testcontainers-go`.
- Critical async path still proves `202` ACK, FIFO queueing, worker processing, Redis Streams replay, and SSE/WS delivery where applicable.
- Load scripts exist or are updated for changed hot paths, with recorded thresholds for ACK/error/end-to-end behavior.
- Tests remain tenant-safe, deterministic, and free of sensitive-data assertions.

## Minimal Examples
Table-driven unit test:
```go
func TestClassifier_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Ticket
		want Priority
	}{
		{name: "vip escalates", in: Ticket{SellerID: "s1", VIP: true}, want: PriorityHigh},
		{name: "default normal", in: Ticket{SellerID: "s1"}, want: PriorityNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classifier{}.Classify(tt.in)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
```

Integration with `testcontainers-go` + Postgres + migrations:
```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

pg := postgres.RunContainer(ctx, testcontainers.WithImage("postgres:16-alpine"))
t.Cleanup(func() { _ = testcontainers.TerminateContainer(pg) })

dsn := pg.ConnectionString(ctx, "sslmode=disable")
require.NoError(t, migrateUp(ctx, dsn))
repo := postgresrepo.New(dsn)
```

Redis Streams append + read-after:
```go
cursor, err := stream.Publish(ctx, "seller-1", "conv-9", Event{Seq: 7, Type: "agent.delta"})
require.NoError(t, err)

events, next, err := stream.Read(ctx, "seller-1", "conv-9", cursor, 10)
require.NoError(t, err)
require.Len(t, events, 1)
require.Equal(t, int64(7), events[0].Seq)
_ = next
```

SQS FIFO in LocalStack with receive + DLQ path:
```go
stack := localstack.Run(ctx)
client := sqsClientFor(stack)

sendFIFO(t, client, queueURL, Message{
	SellerID: "seller-1", ConversationID: "conv-9", RequestID: "req-1",
})

msg := receiveOne(t, client, queueURL)
require.Equal(t, "seller-1:conv-9", aws.ToString(msg.Attributes["MessageGroupId"]))

forceRetriesToDLQ(t, client, queueURL, dlqURL, msg, 3)
assertEventuallyInDLQ(t, client, dlqURL)
```

E2E happy path with fake engine:
```go
fakeEngine.Emit(
	Event{Type: "agent.delta", Seq: 1},
	Event{Type: "agent.completed", Seq: 2},
)

resp := postMessage(t, apiURL, "seller-1", "conv-9")
require.Equal(t, http.StatusAccepted, resp.StatusCode)

events := readSSE(t, streamURL, "seller-1", "conv-9", "")
require.Equal(t, "agent.delta", events[0].Type)
require.Equal(t, "agent.completed", events[1].Type)
```
