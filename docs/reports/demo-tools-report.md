# Demo Tools Report — V2 Tools Diagnosis

Date: 2026-03-18 (Spec 121: Demo V2 Tools Diagnosis)

Previous report: [demo-parity-report.md](demo-parity-report.md) (V1 Closure, 2026-03-18)

## 1. Executive summary

- Status geral: **PASS**
- Blockers to validation: nenhum.
- Conclusão curta: a demo v2 prova a trilha de tools da `gaal-lib` de forma executável, didática e coerente com a arquitetura. Todos os 7 itens do checklist da `Spec 121` foram atendidos com evidência concreta. As tools existem, são acionadas pelo agent, produzem evidência observável, cobrem cenário de erro, possuem testes automatizados, documentação e entradas no `demo.http`.

## 2. Evidence reviewed

### Specs lidas

- [specs/040-tools.md](../../specs/040-tools.md)
- [specs/120-demo-tools.md](../../specs/120-demo-tools.md)
- [specs/121-demo-tools-diagnosis.md](../../specs/121-demo-tools-diagnosis.md)
- [specs/010-feature-matrix.md](../../specs/010-feature-matrix.md)
- [specs/020-repository-architecture.md](../../specs/020-repository-architecture.md)
- [specs/114-demo-v1-closure-diagnosis.md](../../specs/114-demo-v1-closure-diagnosis.md)

### Arquivos inspecionados

- [cmd/demo-app/tools.go](../../cmd/demo-app/tools.go) — definição das 2 tools (`get_time`, `calculate_sum`)
- [cmd/demo-app/agent.go](../../cmd/demo-app/agent.go) — wiring via `agent.WithTools(demoTools()...)`, `detectToolCalls()`, `responseFromToolResult()`
- [cmd/demo-app/main.go](../../cmd/demo-app/main.go) — entry point da demo
- [pkg/tool/tool.go](../../pkg/tool/tool.go) — interface `Tool`, tipos `Call`, `Result`, `Schema`
- [pkg/tool/registry.go](../../pkg/tool/registry.go) — `Registry` com `Register`, `Resolve`, `List`
- [pkg/tool/invoke.go](../../pkg/tool/invoke.go) — `Invoke()` com validação de input/output
- [pkg/tool/errors.go](../../pkg/tool/errors.go) — erros tipados (`ErrorKind`, sentinels)
- [pkg/tool/validate.go](../../pkg/tool/validate.go) — validação de nomes e schemas
- [pkg/tool/tool_test.go](../../pkg/tool/tool_test.go) — 11 testes unitários do pacote
- [pkg/agent/agent_test.go](../../pkg/agent/agent_test.go) — `TestRunWithToolCallFeedsNextModelStep`
- [test/smoke/demo_app_test.go](../../test/smoke/demo_app_test.go) — 4 subtests de tools
- [examples/demo-app/README.md](../../examples/demo-app/README.md) — documentação da demo
- [examples/demo-app/http/demo.http](../../examples/demo-app/http/demo.http) — cenários HTTP de tools

### Testes executados

- `go test ./... -v -count=1 -run "TestDemoApp"` (sem cache) em 2026-03-18: **PASS**
- `test/smoke`: `TestDemoApp` (15 subtests) + `TestDemoAppMemoryResetAfterRestart`: todos PASS
- Subtests de tools confirmados:
  - `tool_call_get_time`: PASS
  - `tool_call_calculate_sum`: PASS
  - `tool_call_error_unknown_tool`: PASS
  - `tool_call_stream_get_time`: PASS

## 3. Scorecard

| # | Critério | Status | Evidência |
| --- | --- | --- | --- |
| 1 | Ao menos 2 tools existem | PASS | `get_time` e `calculate_sum` em `cmd/demo-app/tools.go`, ambas implementando `tool.Tool` de `pkg/tool`. |
| 2 | Agent consegue acioná-las | PASS | Wiring via `agent.WithTools(demoTools()...)` em `cmd/demo-app/agent.go`; engine resolve e invoca via `tool.Invoke()` em `internal/runtime/engine.go`. |
| 3 | Evidência observável de uso | PASS | Resposta JSON inclui campo `tool_calls` com nome da tool; streaming SSE emite eventos `agent.tool_call` e `agent.tool_result`. |
| 4 | Pelo menos 1 caso de erro | PASS | Cenário `unknown_tool` retorna HTTP 500 com mensagem de erro; `calculate_sum` valida tipos em `toFloat64()`. |
| 5 | Teste cobrindo a trilha | PASS | 4 subtests em `test/smoke/demo_app_test.go`; 11 testes unitários em `pkg/tool/tool_test.go`; `TestRunWithToolCallFeedsNextModelStep` em `pkg/agent/agent_test.go`. |
| 6 | README/documentação cobre o uso | PASS | `examples/demo-app/README.md` documenta tools registradas, keywords de ativação, exemplos curl e smoke manual (passos 8–11). |
| 7 | `demo.http` cobre o uso | PASS | 4 cenários: `get_time`, `calculate_sum`, `unknown_tool` (erro), streaming com tool. |

Scorecard: **7/7 PASS**

## 4. Gaps found

### DTG-001

- Severidade: `S3`
- Descrição: o cenário de erro de tool é coberto apenas por tool desconhecida (`unknown_tool` → 500). Não há cenário de erro de execução real via HTTP (ex: input inválido passado ao `calculate_sum` como `"abc"` em vez de número).
- Impacto: a validação de `toFloat64()` existe no código mas não é exercitada end-to-end pela demo nem pelo smoke test.
- Recomendação: adicionar cenário de input inválido em `demo.http` e subtest correspondente.

### DTG-002

- Severidade: `S3`
- Descrição: nenhum toolkit é registrado na demo; as tools são registradas individualmente via `agent.WithTools()`.
- Impacto: a trilha de toolkit (`pkg/tool` suporta `Toolkit`, `RegisterToolkits`, namespace) não é provada pela demo.
- Recomendação: considerar adicionar um toolkit simples na demo para provar a trilha completa.

### DTG-003

- Severidade: `S4`
- Descrição: o modelo fake usa heurística por keyword (`detectToolCalls()`), não simula o ciclo completo de function calling de um LLM real.
- Impacto: aceitável para demo local. A demo prova o wiring e o runtime, não a integração com provider real.
- Recomendação: manter como está; integração com provider real é escopo de trilha futura.

### DTG-004

- Severidade: `S4`
- Descrição: sem tool calls paralelas nem workflows multi-step na demo.
- Impacto: fora do escopo da Spec 120. A demo prova o caminho básico de tool call.
- Recomendação: manter como candidato para demo v3.

## 5. Decision

**PASS**

### Justificativa

- A demo inclui 2 tools reais (`get_time`, `calculate_sum`) que implementam `tool.Tool` corretamente.
- O agent aciona as tools via `agent.WithTools()` e o engine executa via `tool.Invoke()` com validação de input/output.
- A evidência de uso é observável tanto na resposta JSON (`tool_calls`) quanto no streaming SSE (`agent.tool_call`, `agent.tool_result`).
- Existe cenário de erro observável (`unknown_tool` → 500).
- A trilha de tools está coberta por 4 subtests automatizados no smoke, 11 testes unitários no pacote `pkg/tool` e testes de integração no pacote `pkg/agent`.
- A documentação (`README.md`) e o `demo.http` cobrem uso e acionamento das tools.
- Os 4 gaps identificados são menores (S3/S4), não bloqueantes, e não comprometem a validade da prova.
- Todos os testes passaram em execução fresca sem cache em 2026-03-18.

## 6. Answer to the main question

> "A demo v2 já prova tools como uma capacidade real da `gaal-lib`?"

**Sim.** A demo v2 prova a trilha de tools de forma funcional, observável e bem documentada. As tools existem, são registradas no agent, acionadas pelo engine, produzem evidência observável (resposta e streaming), têm cenário de erro, cobertura de testes automatizados e documentação. A trilha de tools está validada como capacidade real da `gaal-lib`.

## 7. Next prioritized actions

1. **DTG-001**: adicionar cenário de erro de execução real (input inválido em `calculate_sum`) no `demo.http` e no smoke test.
2. **DTG-002**: considerar adicionar um toolkit simples na demo (ex: `math` toolkit com `sum` e `multiply`) para provar a trilha completa de toolkit.
3. **Guardrails (Spec 060)**: próxima trilha recomendada para expansão da demo.
4. **Provider real**: integrar pelo menos um provider LLM para demonstrar tool calling com modelo real.

## 8. Comparison with V1 closure

| Aspecto | V1 Closure (2026-03-18) | V2 Tools Diagnosis (2026-03-18) |
| --- | --- | --- |
| Status | `APT for base demo parity` | `PASS` para trilha de tools |
| Trilhas provadas | app, agent, server, logger, memory, streaming | + tools |
| Subtests no smoke | 11 (v1) + memory restart | 15 (v1 + 4 tools) + memory restart |
| `demo.http` cenários | boot, probes, runs, streaming, erros HTTP | + 4 cenários de tools |
| Gaps abertos | 0 | 4 (menores, S3/S4) |

### Conclusão da comparação

A demo v2 estende a v1 de forma incremental e coerente. A trilha de tools é uma prova nova e sólida que amplia a cobertura da demo sem regressão nas trilhas anteriores.
