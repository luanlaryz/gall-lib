# Demo Workflow Report — V2 Workflow Diagnosis

Date: 2026-03-18 (Spec 131: Demo V2 Workflow Diagnosis)

Previous report: [demo-tools-report.md](demo-tools-report.md) (V2 Tools Diagnosis, 2026-03-18)

## 1. Executive summary

- Status geral: **PASS**
- Blockers to validation: nenhum.
- Conclusao curta: a demo v2 prova a trilha de workflows da `gaal-lib` de forma executavel, observavel e bem documentada. Todos os 7 itens do checklist da Spec 131 foram atendidos com evidencia concreta. O workflow `order-processing` existe, esta registrado no app via factory, possui 5 steps com branching condicional, produz evidencia observavel via HTTP, esta coberto por 5 subtests automatizados no smoke e documentado no README com curls de exemplo e smoke manual.

## 2. Evidence reviewed

### Specs lidas

- [specs/070-workflows.md](../../specs/070-workflows.md)
- [specs/130-demo-workflow.md](../../specs/130-demo-workflow.md)
- [specs/131-demo-workflow-diagnosis.md](../../specs/131-demo-workflow-diagnosis.md)
- [specs/010-feature-matrix.md](../../specs/010-feature-matrix.md)
- [specs/020-repository-architecture.md](../../specs/020-repository-architecture.md)

### Arquivos inspecionados

- [cmd/demo-app/workflow.go](../../cmd/demo-app/workflow.go) — factory `orderWorkflowFactory`, 5 steps (`validate_order`, `route_order`, `auto_approve`, `manual_review`, `confirm`)
- [cmd/demo-app/main.go](../../cmd/demo-app/main.go) — registro via `app.WithWorkflowFactories(orderWorkflowFactory{})`
- [cmd/demo-app/server.go](../../cmd/demo-app/server.go) — endpoints `GET /workflows` e `POST /workflows/{name}/runs`
- [pkg/workflow/workflow.go](../../pkg/workflow/workflow.go) — contratos `Workflow`, `Runnable`, `Hook`, `HistorySink`, `RetryPolicy`
- [pkg/workflow/step.go](../../pkg/workflow/step.go) — `Step`, `Action`, `Branch`, `Decision`, `StepResult`, `Next`
- [pkg/workflow/chain.go](../../pkg/workflow/chain.go) — implementacao `Chain` com execucao sequencial, branching, retry, hooks e history
- [pkg/workflow/builder.go](../../pkg/workflow/builder.go) — `Builder`, `New`, options
- [pkg/workflow/state.go](../../pkg/workflow/state.go) — `State` compartilhado entre steps
- [pkg/workflow/history.go](../../pkg/workflow/history.go) — `InMemoryHistory`
- [pkg/workflow/retry.go](../../pkg/workflow/retry.go) — `FixedRetryPolicy`
- [pkg/workflow/logging_hook.go](../../pkg/workflow/logging_hook.go) — `NewLoggingHook`
- [pkg/workflow/errors.go](../../pkg/workflow/errors.go) — erros tipados (`ErrorKind`, sentinels)
- [pkg/workflow/workflow_test.go](../../pkg/workflow/workflow_test.go) — 11 testes unitarios
- [pkg/workflow/workflow_observability_test.go](../../pkg/workflow/workflow_observability_test.go) — 2 testes de observabilidade
- [pkg/app/workflow_registry.go](../../pkg/app/workflow_registry.go) — `WorkflowRegistry`, `Register`, `Resolve`, `List`
- [test/smoke/demo_app_test.go](../../test/smoke/demo_app_test.go) — 5 subtests de workflow
- [examples/demo-app/README.md](../../examples/demo-app/README.md) — secao "Workflow registrado", curls e smoke manual (passos 12–14)
- [examples/demo-app/http/demo.http](../../examples/demo-app/http/demo.http) — 3 cenarios HTTP de workflow

### Testes executados

- `go test ./pkg/workflow/... -v -count=1` (sem cache) em 2026-03-18: **PASS** (13 testes)
- `go test ./test/smoke/... -v -count=1 -run "TestDemoApp"` (sem cache) em 2026-03-18: **PASS** (20 subtests + memory restart)
- Subtests de workflow confirmados:
  - `workflow_list`: PASS
  - `workflow_auto_approve`: PASS
  - `workflow_manual_review`: PASS
  - `workflow_invalid_input`: PASS
  - `workflow_not_found_404`: PASS

### Evidencia executavel observada

- `GET /workflows` retorna lista com `order-processing` (nome e id)
- `POST /workflows/order-processing/runs` com `{"item": "notebook", "amount": 50}` retorna `status: "completed"`, output com `status: "approved"`
- `POST /workflows/order-processing/runs` com `{"item": "server-rack", "amount": 200}` retorna `status: "completed"`, output com `status: "pending_review"`
- `POST /workflows/order-processing/runs` com input invalido retorna HTTP 400
- `POST /workflows/missing/runs` retorna HTTP 404

## 3. Scorecard

| # | Criterio | Status | Evidencia |
| --- | --- | --- | --- |
| 1 | workflow existe | PASS | `order-processing` definido em `cmd/demo-app/workflow.go` com factory `orderWorkflowFactory`, 5 steps usando `workflow.Action` e `workflow.Branch` de `pkg/workflow`. |
| 2 | workflow registrado no app | PASS | Registrado via `app.WithWorkflowFactories(orderWorkflowFactory{})` em `cmd/demo-app/main.go`; resolvido por `Runtime.ResolveWorkflow("order-processing")` em `server.go`. |
| 3 | ao menos 2 steps | PASS | 5 steps: `validate_order` (Action), `route_order` (Branch), `auto_approve` (Action), `manual_review` (Action), `confirm` (Action). |
| 4 | branch simples | PASS | `route_order` implementa `workflow.Branch` com `DecisionFunc` que avalia `amount > 100` e decide entre `auto_approve` e `manual_review`. |
| 5 | evidencia observavel da execucao | PASS | Resposta JSON via HTTP inclui `run_id`, `workflow_name`, `status` e `output` com campos `message`, `item`, `amount`, `status`; dois caminhos de branching produzem resultados distintos (`approved` vs `pending_review`). |
| 6 | teste cobrindo a trilha | PASS | 5 subtests no smoke (`workflow_list`, `workflow_auto_approve`, `workflow_manual_review`, `workflow_invalid_input`, `workflow_not_found_404`); 13 testes unitarios em `pkg/workflow/` cobrindo builder, steps, branching, suspend, retry, hooks, history, cancelamento e observabilidade. |
| 7 | documentacao explica como usar | PASS | `examples/demo-app/README.md` documenta o workflow `order-processing` com descricao dos 5 steps, logica de branching, curls de exemplo para ambos os caminhos e passos 12–14 do smoke manual; `demo.http` inclui 3 cenarios de workflow. |

Scorecard: **7/7 PASS**

## 4. Gaps found

### DWG-001

- Severidade: `S3`
- Descricao: a demo nao demonstra retry de forma explicita via HTTP. O `FixedRetryPolicy{MaxRetries: 1}` esta configurado nos defaults do workflow, mas nao ha cenario na demo que force uma falha recuperavel para observar o retry em acao.
- Impacto: a capacidade de retry existe e esta coberta por testes unitarios (`TestWorkflowRetryAppliesWhenStepHasNoOverride`, `TestStepRetryOverridesWorkflowRetry`), mas nao e provada end-to-end pela demo HTTP.
- Recomendacao: considerar adicionar step que falha na primeira execucao e sucede no retry para demonstrar a capacidade via HTTP.

### DWG-002

- Severidade: `S4`
- Descricao: hooks de lifecycle (`NewLoggingHook`) estao configurados nos defaults do workflow via factory, mas a evidencia de hooks so e observavel nos logs do processo, nao na resposta HTTP.
- Impacto: aceitavel para demo local. Os hooks existem, estao testados (`TestHooksObserveLifecycleOrder`, `TestWorkflowEventsCarryStandardMetadata`) e funcionam corretamente; a observabilidade via logs e suficiente para o escopo da demo.
- Recomendacao: manter como esta; exposicao de eventos de hook na resposta HTTP e candidata para evolucao futura.

### DWG-003

- Severidade: `S4`
- Descricao: nao ha cenario de `Suspend` (interrupcao com checkpoint) na demo HTTP. O mecanismo existe e esta coberto por teste unitario (`TestBranchSuspendReturnsCheckpoint`).
- Impacto: fora do escopo da Spec 130. Suspend e uma capacidade avancada que pode ser demonstrada em trilha futura.
- Recomendacao: manter como candidato para demo v3.

### DWG-004

- Severidade: `S4`
- Descricao: nao ha suite de conformidade dedicada em `test/conformance/` para workflows. Os testes de workflow vivem em `pkg/workflow/` (unitarios) e `test/smoke/` (integracao).
- Impacto: a cobertura existe e e adequada para a demo atual. A ausencia de suite dedicada em `test/conformance/` nao compromete a validacao.
- Recomendacao: considerar criar suite de conformidade quando workflows atingirem maturidade para criterios de pronto completos da feature matrix.

## 5. Decision

**PASS**

### Justificativa

- O workflow `order-processing` existe como implementacao real usando os contratos publicos de `pkg/workflow`.
- Esta registrado no app via `app.WithWorkflowFactories()` e exposto por HTTP via `GET /workflows` e `POST /workflows/{name}/runs`.
- Possui 5 steps, incluindo 1 branch condicional (`route_order`) que escolhe entre 2 caminhos distintos.
- A execucao produz evidencia observavel na resposta JSON: `run_id`, `workflow_name`, `status`, `output` com campos especificos por caminho.
- A trilha esta coberta por 5 subtests automatizados no smoke e 13 testes unitarios no pacote `pkg/workflow`.
- A documentacao (`README.md`) explica o workflow com descricao dos steps, curls de exemplo e smoke manual; `demo.http` inclui 3 cenarios.
- Os 4 gaps identificados sao menores (S3/S4), nao bloqueantes, e nao comprometem a validade da prova.
- Todos os testes passaram em execucao fresca sem cache em 2026-03-18.

## 6. Answer to the main question

> "A demo ja prova workflows como capacidade executavel e observavel da `gaal-lib`?"

**Sim.** A demo v2 prova a trilha de workflows de forma funcional, executavel e observavel. O workflow `order-processing` demonstra steps sequenciais, branching condicional, state compartilhado, validacao de input e evidencia distinta por caminho de execucao. A trilha esta coberta por testes automatizados, documentada com curls de exemplo e integrada ao app via registry e factory. A `gaal-lib` prova workflows como capacidade real.

## 7. Next prioritized actions

1. **DWG-001**: considerar adicionar cenario de retry observavel na demo (step que falha e recupera) para completar a demonstracao da capacidade de retry.
2. **Guardrails (Spec 060)**: proxima trilha recomendada para expansao da demo v2.
3. **DWG-004**: criar suite de conformidade em `test/conformance/` para workflows quando o modulo atingir maturidade completa.
4. **Suspend/resume**: demonstrar suspend com checkpoint em trilha futura (v3).

## 8. Comparison with V2 Tools diagnosis

| Aspecto | V2 Tools (2026-03-18) | V2 Workflow (2026-03-18) |
| --- | --- | --- |
| Status | PASS | PASS |
| Trilha provada | tools (2 tools, wiring, erro) | workflow (5 steps, branching, validacao) |
| Subtests no smoke | 4 de tools | 5 de workflow |
| `demo.http` cenarios | 4 cenarios de tools | 3 cenarios de workflow |
| Gaps abertos | 4 (S3/S4) | 4 (S3/S4) |
| Scorecard | 7/7 PASS | 7/7 PASS |

### Delta

A demo v2 agora cobre 3 trilhas validadas: base (v1), tools e workflows. A trilha de workflow adiciona prova de execucao multi-step com branching condicional, que e a capacidade central de orquestracao da `gaal-lib`.

### Conclusao da comparacao

A trilha de workflow complementa a de tools de forma coerente e incremental. Ambas atingem PASS com gaps menores e nao bloqueantes. A demo demonstra as capacidades centrais da `gaal-lib` de forma progressiva e rastreavel.
