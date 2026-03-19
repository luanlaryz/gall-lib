# Demo Guardrails Report — V2 Guardrails Diagnosis

Date: 2026-03-18 (Spec 141: Demo V2 Guardrails Diagnosis)

Previous report: [demo-workflow-report.md](demo-workflow-report.md) (V2 Workflow Diagnosis, 2026-03-18)

## 1. Executive summary

- Status geral: **PASS**
- Blockers to validation: nenhum.
- Conclusao curta: a demo v2 prova a trilha de guardrails da `gaal-lib` de forma executavel, observavel e bem documentada. Todos os 6 itens do checklist da Spec 141 foram atendidos com evidencia concreta. Os 3 tipos de guardrail (input, output, stream) existem na demo como implementacoes reais dos contratos publicos de `pkg/guardrail`, registrados via `App.Defaults` e herdados automaticamente pelo agent. Ha caso de intervencao observavel em cada fase: bloqueio de input com HTTP 422, transformacao de output com sufixo visivel e redacao de digitos em chunks de stream. A trilha esta coberta por 3 subtests automatizados no smoke e documentada com curls de exemplo no README.

## 2. Evidence reviewed

### Specs lidas

- [specs/060-guardrails.md](../../specs/060-guardrails.md)
- [specs/140-demo-guardrails.md](../../specs/140-demo-guardrails.md)
- [specs/141-demo-guardrails-diagnosis.md](../../specs/141-demo-guardrails-diagnosis.md)
- [specs/010-feature-matrix.md](../../specs/010-feature-matrix.md)
- [specs/030-agent.md](../../specs/030-agent.md)

### Arquivos inspecionados

- [cmd/demo-app/guardrails.go](../../cmd/demo-app/guardrails.go) — 3 guardrails: `inputBlockGuardrail`, `outputTagGuardrail`, `streamDigitGuardrail`
- [cmd/demo-app/main.go](../../cmd/demo-app/main.go) — registro via `App.Defaults.Agent` com `InputGuardrails`, `OutputGuardrails`, `StreamGuardrails`
- [pkg/guardrail/guardrail.go](../../pkg/guardrail/guardrail.go) — contratos publicos: `Input`, `Output`, `Stream`, `Decision`, `Action`, `Phase`, `Context`
- [pkg/agent/agent.go](../../pkg/agent/agent.go) — options `WithInputGuardrails`, `WithOutputGuardrails`, `WithStreamGuardrails`; `GuardrailDecision`, `GuardrailEvent`
- [pkg/agent/errors.go](../../pkg/agent/errors.go) — `ErrGuardrailBlocked`
- [pkg/app/app.go](../../pkg/app/app.go) — `AgentDefaults` com heranca de guardrails
- [internal/runtime/engine.go](../../internal/runtime/engine.go) — `applyInputGuardrails`, `applyOutputGuardrails`, `applyStreamGuardrails`, `validateGuardrailDecision`
- [internal/runtime/guardrail_stream_test.go](../../internal/runtime/guardrail_stream_test.go) — testes de chaining, drop, abort e buffer
- [pkg/agent/agent_test.go](../../pkg/agent/agent_test.go) — `TestDefinitionIncludesStreamGuardrails`, `TestRunGuardrailBlocked`, `TestOutputGuardrailTransform`
- [pkg/app/app_test.go](../../pkg/app/app_test.go) — `TestAgentFactoryInheritsAppStreamGuardrails`, `TestAgentFactoryAppStreamGuardrailsPrecedeLocalOnes`
- [test/smoke/demo_app_test.go](../../test/smoke/demo_app_test.go) — 3 subtests de guardrails
- [examples/demo-app/README.md](../../examples/demo-app/README.md) — secao "Guardrails" (linhas 204-251)
- [examples/stream-redaction/main.go](../../examples/stream-redaction/main.go) — exemplo isolado de stream/output guardrails

### Testes executados

- `go test ./pkg/agent/... -v -count=1 -run "Guardrail"` (sem cache) em 2026-03-18: **PASS** (3 testes)
- `go test ./internal/runtime/... -v -count=1 -run "Guardrail|guardrail"` (sem cache) em 2026-03-18: **PASS** (5 testes, incluindo 2 subcases)
- `go test ./pkg/app/... -v -count=1 -run "Guardrail|guardrail"` (sem cache) em 2026-03-18: **PASS** (2 testes)
- `go test ./test/smoke/... -v -count=1 -run "TestDemoApp"` (sem cache) em 2026-03-18: **PASS** (23 subtests + memory restart)
- Subtests de guardrails confirmados:
  - `guardrail_input_block`: PASS
  - `guardrail_output_tag_on_run`: PASS
  - `guardrail_stream_digit_redaction`: PASS

### Evidencia executavel observada

- `POST /agents/demo-agent/runs` com `"message": "BLOCK_ME"` retorna HTTP 422 com erro contendo `"guardrail"`
- `POST /agents/demo-agent/runs` com `"message": "Eve"` retorna output `"hello, Eve [guardrail:ok]"` (sufixo do output guardrail)
- `POST /agents/demo-agent/stream` com `"message": "test 123"` retorna deltas SSE com digitos redacted (`***` em vez de `123`) e output final com sufixo `" [guardrail:ok]"`
- Output guardrail visivel em toda chamada bem-sucedida (inclusive `text_run_with_memory` que valida `"hello, Ada [guardrail:ok]"`)

## 3. Scorecard

| # | Criterio | Status | Evidencia |
| --- | --- | --- | --- |
| 1 | ha input guardrail | PASS | `inputBlockGuardrail` em `cmd/demo-app/guardrails.go` implementa `guardrail.Input`; bloqueia runs contendo `"BLOCK_ME"` com `ActionBlock`; registrado via `App.Defaults.Agent.InputGuardrails`. |
| 2 | ha output guardrail | PASS | `outputTagGuardrail` em `cmd/demo-app/guardrails.go` implementa `guardrail.Output`; aplica `ActionTransform` adicionando ` [guardrail:ok]` a toda resposta; registrado via `App.Defaults.Agent.OutputGuardrails`. |
| 3 | ha stream guardrail | PASS | `streamDigitGuardrail` em `cmd/demo-app/guardrails.go` implementa `guardrail.Stream`; aplica `ActionTransform` substituindo digitos por `*` ou `ActionAllow` quando nao ha digitos; registrado via `App.Defaults.Agent.StreamGuardrails`. |
| 4 | ha caso de intervencao observavel | PASS | 3 intervencoes distintas: (a) input block retorna HTTP 422 com erro classificado de guardrail, (b) output transform adiciona sufixo visivel em toda resposta, (c) stream transform redacta digitos em chunks SSE observaveis. |
| 5 | ha teste | PASS | 3 subtests dedicados no smoke: `guardrail_input_block` (valida 422 e mensagem), `guardrail_output_tag_on_run` (valida sufixo), `guardrail_stream_digit_redaction` (valida ausencia de digitos em deltas e sufixo no output final). Testes adicionais no runtime cobrem chaining, drop, abort e buffer. |
| 6 | ha documentacao reproduzivel | PASS | `examples/demo-app/README.md` secao "Guardrails" (linhas 204-251) documenta os 3 guardrails com descricao, curls de exemplo e resultado esperado; `examples/demo-app/http/demo.http` inclui cenarios HTTP; smoke manual lista passos de verificacao. |

Scorecard: **6/6 PASS**

## 4. Gaps found

### DGG-001

- Severidade: `S4`
- Descricao: a demo mostra apenas `block` para input e `transform` para output/stream. Acoes como `drop` e `abort` de stream nao sao demonstradas na demo HTTP.
- Impacto: as acoes `drop` e `abort` existem na implementacao e estao cobertas por testes unitarios (`TestStreamGuardrailsModifyDropAndOutputUsesApprovedBuffer`, `TestStreamGuardrailAbortFailsRunWithoutOutputOrPersistence`), mas nao sao provadas end-to-end pela demo.
- Recomendacao: considerar adicionar guardrail de `drop` ou `abort` em trilha futura para demonstrar a capacidade completa via HTTP.

### DGG-002

- Severidade: `S4`
- Descricao: nao ha testes unitarios dedicados para `cmd/demo-app/guardrails.go`. A cobertura vem exclusivamente dos smoke tests em `test/smoke/demo_app_test.go`.
- Impacto: aceitavel para o escopo de demo. Os smoke tests validam o comportamento end-to-end de cada guardrail, que e o nivel de teste mais relevante para uma demo.
- Recomendacao: manter como esta; testes unitarios de implementacoes de demo nao sao prioritarios.

### DGG-003

- Severidade: `S4`
- Descricao: `examples/stream-redaction` existe como exemplo isolado de stream/output guardrails, separado da trilha principal da demo em `cmd/demo-app`.
- Impacto: nao compromete a validacao. O exemplo complementa a demo, oferecendo uma referencia minima focada em stream guardrails sem o contexto completo do app.
- Recomendacao: manter como esta; o exemplo serve como referencia complementar.

### DGG-004

- Severidade: `S4`
- Descricao: nao existe suite de conformidade dedicada em `test/conformance/` para guardrails. A feature matrix reconhece que suites mais amplas ainda faltam.
- Impacto: a cobertura existente (testes unitarios em `pkg/agent`, `pkg/app`, `internal/runtime` e smoke tests) e adequada para validar a demo. A ausencia de suite dedicada nao compromete o diagnostico.
- Recomendacao: criar suite de conformidade quando guardrails atingirem maturidade para criterios de pronto completos da feature matrix.

## 5. Decision

**PASS**

### Justificativa

- Os 3 tipos de guardrail (input, output, stream) existem como implementacoes reais dos contratos publicos de `pkg/guardrail`.
- Cada guardrail demonstra uma acao de intervencao distinta: `block` em input, `transform` em output e `transform` em stream.
- Os guardrails sao registrados via `App.Defaults.Agent` e herdados automaticamente pelo agent, provando o mecanismo de heranca da `Spec 031`.
- Ha evidencia observavel em cada fase: HTTP 422 para bloqueio, sufixo visivel para transformacao de output, redacao de digitos para transformacao de stream.
- A trilha esta coberta por 3 subtests automatizados no smoke, que validam o comportamento end-to-end de cada guardrail.
- A documentacao (`README.md`) explica cada guardrail com descricao, curls de exemplo e resultado esperado.
- Testes adicionais no runtime cobrem cenarios mais profundos (chaining, drop, abort, buffer, fail-closed) que complementam a validacao.
- Os 4 gaps identificados sao menores (S4), nao bloqueantes, e nao comprometem a validade da prova.
- Todos os testes passaram em execucao fresca sem cache em 2026-03-18.

## 6. Answer to the main question

> "A demo ja prova guardrails como capacidade observavel e confiavel da `gaal-lib`?"

**Sim.** A demo v2 prova guardrails de forma funcional, executavel e observavel. Os 3 tipos de guardrail (input, output, stream) existem como implementacoes reais dos contratos publicos, registrados via heranca do `App` e exercitados por testes automatizados. Cada fase demonstra intervencao observavel: bloqueio de input com erro classificado, transformacao de output com sufixo visivel e redacao de digitos em chunks de stream. A trilha esta documentada, reproduzivel e validada. A `gaal-lib` prova guardrails como capacidade real.

## 7. Next prioritized actions

1. **Conversation persistence (Spec 010)**: proxima feature de alta prioridade na feature matrix que ainda esta `Nao iniciado`.
2. **DGG-001**: considerar adicionar cenario de `drop` ou `abort` em stream para completar a demonstracao de todas as acoes de guardrail.
3. **DGG-004**: criar suite de conformidade em `test/conformance/` para guardrails quando o modulo atingir maturidade completa.
4. **Feature matrix**: atualizar status de guardrails de input, output e stream para refletir a cobertura atual da demo e dos testes.

## 8. Comparison with previous V2 diagnoses

| Aspecto | V2 Tools (2026-03-18) | V2 Workflow (2026-03-18) | V2 Guardrails (2026-03-18) |
| --- | --- | --- | --- |
| Status | PASS | PASS | PASS |
| Trilha provada | tools (2 tools, wiring, erro) | workflow (5 steps, branching) | guardrails (input, output, stream) |
| Subtests no smoke | 4 de tools | 5 de workflow | 3 de guardrails |
| Documentacao | README + demo.http | README + demo.http | README + demo.http |
| Gaps abertos | 4 (S3/S4) | 4 (S3/S4) | 4 (S4) |
| Scorecard | 7/7 PASS | 7/7 PASS | 6/6 PASS |

### Delta

A demo v2 agora cobre 4 trilhas validadas: base (v1), tools, workflows e guardrails. A trilha de guardrails adiciona prova de interceptacao e controle em 3 fases do ciclo de execucao do agent (input, stream, output), que e a capacidade central de seguranca e confiabilidade da `gaal-lib`.

### Conclusao da comparacao

A trilha de guardrails complementa tools e workflows de forma coerente e incremental. As 3 trilhas atingem PASS com gaps menores e nao bloqueantes. A demo demonstra as capacidades centrais da `gaal-lib` — execucao de agentes com tools, orquestracao via workflows e interceptacao via guardrails — de forma progressiva e rastreavel.
