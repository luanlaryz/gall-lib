# Demo Parity Report

Date: 2026-03-17

## 1. Executive summary

- Status geral: a demo atual prova o fluxo base local da `gaal-lib`, com evidência executável para boot, probes, listagem de agents, run textual, streaming SSE e memória in-process.
- Blockers to validation: nenhum.
- Conclusão curta: a demo está `APT for base demo parity` como evidência do fluxo base. Todos os gaps identificados anteriormente foram resolvidos.

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
- `Spec 110: Demo Parity Diagnosis`

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

- `go test ./...` -> `PASS`
- `test/smoke` incluído dentro de `go test ./...` -> `PASS`

### Evidência executável observada

- Boot local com `go run ./cmd/demo-app` em `127.0.0.1:18080` -> `PASS`
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

### DPG-002 (RESOLVED)

- Severidade: `S2`
- Descricao: o smoke test automatizado validava apenas o caminho feliz de boot, probes, listagem e run textual; o diagnóstico de streaming e de erros HTTP dependia de evidência manual.
- Resolução: `test/smoke/demo_app_test.go` foi expandido com subtests para streaming SSE (`streaming_sse`), agent inexistente (`agent_not_found_404`), request inválido (`invalid_request_400`) e método inválido (`method_not_allowed_405`). O teste agora usa abordagem de subprocesso, construindo e executando o binário real da demo.

### DPG-003 (RESOLVED)

- Severidade: `S3`
- Descricao: a semântica de perda de memória após reinício estava documentada, mas não estava coberta por automação.
- Resolução: `TestDemoAppMemoryResetAfterRestart` foi adicionado em `test/smoke/demo_app_test.go`. O teste inicia a demo, estabelece memória conversacional com duas chamadas, para o processo, reinicia em porta nova e verifica que a mesma `session_id` volta ao comportamento inicial (`"hello, Ada"` em vez de `"welcome back, Ada"`).

## 5. Aptness decision

`APT for base demo parity`

### Justificativa

- A demo sobe localmente com configuração simples e sem dependência externa obrigatória.
- A demo usa `pkg/app` como composition root efetiva e exercita `app + agent + server + logger + memory`.
- Os endpoints críticos e o streaming estão implementados e verificáveis.
- O binário não importa `internal/*`, conforme a `Spec 100`.
- Existe smoke test automatizado cobrindo o caminho principal, erros HTTP, streaming SSE e limpeza de memória após reinício.
- Todos os gaps identificados no diagnóstico anterior foram resolvidos.

## 6. Next prioritized actions

Nenhuma ação prioritária remanescente para a demo base. Ações futuras possíveis:

1. Avaliar se a demo deve evoluir para demonstrar tools, guardrails ou sub-agents quando a biblioteca suportar.
2. Considerar a adição de exemplos de uso com providers reais quando disponíveis.
3. Monitorar o tamanho de `cmd/demo-app/server.go` e avaliar split se ultrapassar 500 linhas.
