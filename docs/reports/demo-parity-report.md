# Demo Parity Report — V1 Closure

Date: 2026-03-18 (Spec 114: Demo V1 Closure Diagnosis)

Previous report: 2026-03-17

## 1. Executive summary

- Status geral: `APT for base demo parity` — confirmado.
- Blockers to validation: nenhum.
- Conclusão curta: a checklist da `Spec 110` foi reaplicada integralmente com evidência executável fresca. Todos os 39 itens permanecem marcados. Os 3 gaps identificados anteriormente (DPG-001, DPG-002, DPG-003) continuam resolvidos. Nenhum gap novo foi encontrado. A demo é uma prova fechada do fluxo base da `gaal-lib`.

## 2. Evidence reviewed

### Specs lidas

- [specs/000-compatibility-target.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/000-compatibility-target.md)
- [specs/010-feature-matrix.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/010-feature-matrix.md)
- [specs/020-repository-architecture.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/020-repository-architecture.md)
- [specs/030-agent.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/030-agent.md)
- [specs/031-app-instance.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/031-app-instance.md)
- [specs/050-memory.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/050-memory.md)
- [specs/081-server.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/081-server.md)
- [specs/100-demo-app.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/100-demo-app.md)
- [specs/110-demo-parity-diagnosis.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/110-demo-parity-diagnosis.md)
- [specs/114-demo-v1-closure-diagnosis.md](/Users/luanlima/Documents/study/go/gaal-lib/specs/114-demo-v1-closure-diagnosis.md)

### Arquivos inspecionados

- [go.mod](/Users/luanlima/Documents/study/go/gaal-lib/go.mod)
- [cmd/demo-app/main.go](/Users/luanlima/Documents/study/go/gaal-lib/cmd/demo-app/main.go)
- [cmd/demo-app/config.go](/Users/luanlima/Documents/study/go/gaal-lib/cmd/demo-app/config.go)
- [cmd/demo-app/agent.go](/Users/luanlima/Documents/study/go/gaal-lib/cmd/demo-app/agent.go)
- [cmd/demo-app/server.go](/Users/luanlima/Documents/study/go/gaal-lib/cmd/demo-app/server.go)
- [examples/demo-app/README.md](/Users/luanlima/Documents/study/go/gaal-lib/examples/demo-app/README.md)
- [examples/demo-app/.env.example](/Users/luanlima/Documents/study/go/gaal-lib/examples/demo-app/.env.example)
- [examples/demo-app/http/demo.http](/Users/luanlima/Documents/study/go/gaal-lib/examples/demo-app/http/demo.http)
- [test/smoke/demo_app_test.go](/Users/luanlima/Documents/study/go/gaal-lib/test/smoke/demo_app_test.go)
- [pkg/app/app.go](/Users/luanlima/Documents/study/go/gaal-lib/pkg/app/app.go)
- [pkg/app/observability.go](/Users/luanlima/Documents/study/go/gaal-lib/pkg/app/observability.go)
- [pkg/app/errors.go](/Users/luanlima/Documents/study/go/gaal-lib/pkg/app/errors.go)
- [pkg/server/server.go](/Users/luanlima/Documents/study/go/gaal-lib/pkg/server/server.go)

### Ambiente usado

- OS: `darwin 24.6.0`
- Toolchain exigida: `go 1.26.1`
- Toolchain disponível: `go version go1.26.1 darwin/arm64`

### Testes executados

- `go test ./... -v -count=1` (sem cache) em 2026-03-18: **PASS**
- `test/smoke` incluído: `TestDemoApp` (11 subtests) + `TestDemoAppMemoryResetAfterRestart`: todos PASS

### Evidência executável observada

- Boot local com `go run ./cmd/demo-app` em `127.0.0.1:18080`: PASS
- `GET /healthz` -> `200` com `{"state":"running","health":true,"ready":true,"draining":false}`
- `GET /readyz` -> `200` com `{"state":"running","health":true,"ready":true,"draining":false}`
- `GET /agents` -> `200` com `demo-agent`
- `POST /agents/demo-agent/runs` com `session_id=session-1` -> `hello, Ada`
- Segundo `POST /agents/demo-agent/runs` com mesmo `session_id` -> `welcome back, Ada`
- `POST /agents/demo-agent/stream` -> eventos SSE ordenados `agent.started`, `agent.delta`, `agent.delta`, `agent.completed`
- `POST /agents/missing-agent/runs` -> `404` com erro claro
- `POST /agents/demo-agent/runs` com `session_id` vazio -> `400` com erro claro
- `GET /agents/demo-agent/runs` e `GET /agents/demo-agent/stream` -> `405` com `Allow: POST`

## 3. Scorecard

| Area | Status | Observacao curta |
| --- | --- | --- |
| ambiente | PASS | `go.mod` e toolchain local batem; `go test ./...` executou sem bloqueio. |
| boot/lifecycle | PASS | `cmd/demo-app` usa `App.Start()`, trata sinais e chama `App.Shutdown()` com timeout. |
| http surface | PASS | `/healthz`, `/readyz`, `/agents`, `/agents/{name}/runs` e `/agents/{name}/stream` existem e responderam como esperado. |
| runtime integration | PASS | A demo usa runtime real via `Runtime.ListAgents()`, `Runtime.ResolveAgent()`, `Agent.Run()` e `Agent.Stream()`. O binário não importa `internal/*`, conforme a regra arquitetural da `Spec 100`. |
| memory proof | PASS | Há `memory.InMemoryStore` por default, o mesmo `session_id` reaparece no segundo run dentro do mesmo processo, e há teste automatizado provando que a memória é limpa após reinício do processo. |
| streaming proof | PASS | O endpoint SSE usa `Agent.Stream()`, preservou ordem observável até `agent.completed`, e está coberto por smoke test automatizado. |
| docs/dx | PASS | README, `.env.example` e `demo.http` existem, são coerentes com os endpoints reais e documentam lacunas conhecidas. |
| smoke tests | PASS | O caminho principal está coberto, incluindo boot, probes, listagem, run textual, streaming SSE, erros HTTP (`404`, `400`, `405`) e limpeza de memória após reinício. |
| parity confidence | PASS | A demo é uma evidência forte do fluxo base local, com cobertura automatizada completa e aderência à regra arquitetural da `Spec 100`. |

## 4. Gaps found

### DPG-001 (RESOLVED)

- Severidade: `S2`
- Descricao: o binário da demo importava `internal/demoapp`, contrariando a regra explícita da `Spec 100` de não importar `internal/*` a partir de `cmd/demo-app`.
- Resolução: o código de composição da demo foi movido para arquivos locais no pacote `main` em `cmd/demo-app/` (`config.go`, `agent.go`, `server.go`). O `internal/demoapp/` foi removido. Nenhuma API pública nova foi criada. O binário não importa `internal/*`.
- Status na closure: confirmado resolvido. `internal/demoapp/` não existe. Nenhum import de `internal/*` em `cmd/demo-app/`.

### DPG-002 (RESOLVED)

- Severidade: `S2`
- Descricao: o smoke test automatizado validava apenas o caminho feliz de boot, probes, listagem e run textual; o diagnóstico de streaming e de erros HTTP dependia de evidência manual.
- Resolução: `test/smoke/demo_app_test.go` foi expandido com subtests para streaming SSE (`streaming_sse`), agent inexistente (`agent_not_found_404`), request inválido (`invalid_request_400`) e método inválido (`method_not_allowed_405`). O teste agora usa abordagem de subprocesso, construindo e executando o binário real da demo.
- Status na closure: confirmado resolvido. Os subtests executaram e passaram em 2026-03-18.

### DPG-003 (RESOLVED)

- Severidade: `S3`
- Descricao: a semântica de perda de memória após reinício estava documentada, mas não estava coberta por automação.
- Resolução: `TestDemoAppMemoryResetAfterRestart` foi adicionado em `test/smoke/demo_app_test.go`. O teste inicia a demo, estabelece memória conversacional com duas chamadas, para o processo, reinicia em porta nova e verifica que a mesma `session_id` volta ao comportamento inicial (`"hello, Ada"` em vez de `"welcome back, Ada"`).
- Status na closure: confirmado resolvido. O teste executou e passou em 2026-03-18.

### Novos gaps

Nenhum gap novo identificado na closure.

## 5. Aptness decision

`APT for base demo parity`

### Justificativa

- A demo sobe localmente com configuração simples e sem dependência externa obrigatória.
- A demo usa `pkg/app` como composition root efetiva e exercita `app + agent + server + logger + memory`.
- Os endpoints críticos e o streaming estão implementados e verificáveis.
- O binário não importa `internal/*`, conforme a `Spec 100`.
- Existe smoke test automatizado cobrindo o caminho principal, erros HTTP, streaming SSE e limpeza de memória após reinício.
- Todos os gaps identificados no diagnóstico anterior foram resolvidos e confirmados na closure.
- Nenhum gap novo foi encontrado.
- A evidência executável fresca (testes rodados sem cache em 2026-03-18) confirma estabilidade.

## 6. Next prioritized actions

Nenhuma ação remanescente para a demo base v1. Trilha v2 recomendada:

1. **Tools (Spec 040)**: adicionar suporte a tools no agent e demonstrar na demo. Este é o próximo bloco funcional que mais amplia a utilidade prática da biblioteca e já tem spec aprovada.
2. **Guardrails (Spec 060)**: input/output guardrails integrados ao fluxo da demo.
3. **Workflows / Sub-agents (Spec 070)**: composição multi-agent demonstrada na demo.
4. **Providers reais**: integrar pelo menos um provider LLM real como alternativa ao modelo fake.
5. **OpenAPI / docs avançados**: documentação formal da superfície HTTP da demo.

Recomendação: começar pela trilha **Tools (Spec 040)**.

## 7. Comparison with previous diagnosis

| Aspecto | Diagnóstico anterior (2026-03-17) | Closure (2026-03-18) |
| --- | --- | --- |
| Status | `APT for base demo parity` | `APT for base demo parity` (confirmado) |
| Gaps abertos | 0 | 0 |
| Gaps resolvidos | 3 (DPG-001, DPG-002, DPG-003) | 3 (mesmos, nenhum novo) |
| Scorecard PASS | 9/9 | 9/9 |
| Bloqueios de ambiente | 0 | 0 |
| `go test ./...` | PASS | PASS (re-verificado sem cache) |
| Checklist 9.1–9.6 | 39/39 [x] | 39/39 [x] |

### Delta

Nenhuma regressão. Nenhuma mudança de status. Evidência executável fresca confirma estabilidade integral.

### Conclusão da comparação

O relatório de closure confirma integralmente o diagnóstico anterior. A demo v1 está fechada como prova do fluxo base da `gaal-lib`.
