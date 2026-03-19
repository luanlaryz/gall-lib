# Library Parity Gaps

Date: 2026-03-19 (Spec 200: Library Parity Diagnosis)

## Severidade

- `S0` bloqueador de verificabilidade
- `S1` bloqueador de paridade para orquestracao
- `S2` lacuna relevante mas nao bloqueante
- `S3` melhoria de DX, naming ou acabamento

---

## LPG-001

- Severidade: `S1`
- Dimensao: 7.5 Agent Orchestration via Agents-as-Tools
- Descricao: nenhum mecanismo permite que um agent use outro agent como ferramenta ou delegue execucao a um sub-agent. A Spec 200 (secao 7.5) define explicitamente que a ausencia desse mecanismo deve ser classificada como gap relevante de paridade de orquestracao, nao como melhoria opcional.
- Evidencia: busca por `agents.as.tools`, `sub.agent`, `delegate`, `handoff` em `pkg/` retornou zero resultados. `StepKindInvoke` em `pkg/workflow/step.go` esta reservado mas nao implementado. Nenhum exemplo, teste ou demo demonstra coordenacao entre agents.
- Impacto: sem agents-as-tools, a `gaal-lib` nao reproduz a capacidade central de orquestracao multi-agent do Voltagent. Workflows podem encadear steps, mas nao podem delegar a agents como executores.
- Recomendacao: implementar adapter que envolva um `*agent.Agent` como `tool.Tool` ou como step de workflow via `StepKindInvoke`. Criar spec dedicada com contratos, testes e demo de coordenacao.

## LPG-002

- Severidade: `S1`
- Dimensao: 7.6 Memory and Working Memory
- Descricao: conversation persistence esta marcada como `Nao iniciado` na feature matrix (P1, obrigatoria na v1). Nao existe nenhum adapter de persistencia duravel alem do `InMemoryStore` in-process. O historico conversacional e perdido ao reiniciar o processo.
- Evidencia: feature matrix (`specs/010-feature-matrix.md`) lista `Conversation persistence` como P1, status `Nao iniciado`. `TestDemoAppMemoryResetAfterRestart` confirma que memoria e limpa no restart. Nenhum adapter persistente existe em `pkg/memory/`.
- Impacto: a biblioteca nao pode preservar contexto conversacional entre execucoes do processo. Isso limita cenarios de producao e reduz a paridade com o Voltagent, que suporta persistence.
- Recomendacao: definir spec de conversation persistence com interface de adapter plugavel. Implementar ao menos um adapter local (arquivo ou SQLite) para provar a trilha sem dependencia hospedada.

## LPG-003

- Severidade: `S2`
- Dimensao: 7.9 Server / REST Exposure
- Descricao: nao existe abstracoes HTTP reutilizavel em `pkg/server`. A feature matrix marca `HTTP server abstraction` como `Adiado` (P2). O adapter HTTP da demo vive em `cmd/demo-app/server.go` e nao e importavel por terceiros.
- Evidencia: `pkg/server/server.go` contem apenas helpers de probe (`Health`, `Ready`, `Snapshot`) e aliases de tipos de `pkg/app`. O adapter HTTP concreto esta em `cmd/demo-app/server.go` (pacote `main`). Nenhum middleware, router ou handler reutilizavel existe em `pkg/`.
- Impacto: qualquer consumidor da biblioteca precisa escrever seu proprio adapter HTTP. A demo prova que o fluxo funciona, mas nao ha reutilizabilidade.
- Recomendacao: manter como adiado conforme feature matrix. Quando priorizado, extrair adapter HTTP minimo para `pkg/server` ou `pkg/adapter/http` com handlers para probes, listagem, run e stream.

## LPG-004

- Severidade: `S2`
- Dimensao: 7.1 Verificabilidade (testes)
- Descricao: nao existe suite de conformidade dedicada em `test/conformance/`. A spec 020 define que suites de conformidade devem usar apenas a superficie publica em `pkg/*`. Os testes atuais vivem em `pkg/*/` (unitarios) e `test/smoke/` (integracao).
- Evidencia: `test/conformance/` nao existe no repositorio. Relatorios anteriores (DWG-004, DGG-004) ja registraram essa lacuna.
- Impacto: a cobertura de testes existente e adequada para validar comportamento, mas nao ha suite formal de conformidade que prove contratos publicos de forma isolada e rastreavel.
- Recomendacao: criar `test/conformance/` com suites por modulo (agent, tool, workflow, memory, guardrail) usando apenas `pkg/*`.

## LPG-005

- Severidade: `S2`
- Dimensao: 7.6 Memory and Working Memory
- Descricao: working memory (`InMemoryWorkingMemoryFactory`) funciona para agent runs, mas nao esta integrada a workflows. Workflows usam `State` proprio para compartilhar dados entre steps.
- Evidencia: `pkg/workflow/state.go` define `State` como mecanismo de compartilhamento entre steps. `pkg/memory/in_memory_working.go` define `WorkingSet` apenas para agent runs. Nenhuma integracao entre os dois.
- Impacto: workflows nao podem compartilhar estado de working memory com agents executados dentro de steps. Isso limita cenarios de orquestracao com contexto persistente entre agent e workflow.
- Recomendacao: avaliar se a integracao e necessaria para paridade. Se sim, definir contrato de propagacao de working memory entre workflow e agent steps.

## LPG-006

- Severidade: `S2`
- Dimensao: 7.7 Guardrails
- Descricao: `pkg/guardrail/` nao possui arquivo de teste. Os contratos sao testados indiretamente via `pkg/agent/agent_test.go`, `pkg/app/app_test.go` e `internal/runtime/guardrail_stream_test.go`.
- Evidencia: `ls pkg/guardrail/` retorna apenas `guardrail.go`. Nenhum `*_test.go`.
- Impacto: os contratos de guardrail sao validados por testes de integracao, o que e aceitavel, mas nao ha testes unitarios isolados para o pacote.
- Recomendacao: adicionar `pkg/guardrail/guardrail_test.go` com testes basicos de construcao de `Decision`, validacao de `Action` e coerencia dos tipos exportados.

## LPG-007

- Severidade: `S3`
- Dimensao: 7.8 Workflows
- Descricao: `StepKindInvoke` esta definido como constante reservada em `pkg/workflow/step.go` mas nao possui nenhum step builder ou adapter que o utilize.
- Evidencia: `StepKindInvoke StepKind = "invoke"` em `pkg/workflow/step.go` linha 27 com comentario "reserved for adapters that invoke external executors". Nenhuma funcao construtora no pacote retorna step desse tipo.
- Impacto: a reserva e correta para extensibilidade futura, mas nao ha prova funcional. Nao e bloqueante.
- Recomendacao: implementar junto com agents-as-tools (LPG-001) como adapter que invoca `Agent.Run` dentro de um workflow step.

## LPG-008

- Severidade: `S3`
- Dimensao: 7.11 DX e integrabilidade
- Descricao: `pkg/types/` nao possui testes unitarios. O pacote define `Metadata`, `Message`, `MessageRole`, `ToolCall`, `Usage` e funcoes auxiliares.
- Evidencia: `go test ./pkg/types/...` retorna `[no test files]`.
- Impacto: as funcoes utilitarias (`CloneMetadata`, `MergeMetadata`, `CloneMessages`) nao tem cobertura direta. Sao exercitadas indiretamente por outros pacotes.
- Recomendacao: adicionar `pkg/types/types_test.go` com testes de clone, merge e imutabilidade.

## LPG-009

- Severidade: `S3`
- Dimensao: 7.3 App/Runtime (arquitetura interna)
- Descricao: a spec 020 preve subpacotes em `internal/runtime/` (`agent`, `tool`, `workflow`, `memory`, `registry`, `lifecycle`), mas toda a logica vive em `internal/runtime/` como pacote unico com `engine.go` (~1478 linhas) e `reasoning.go` (~1255 linhas).
- Evidencia: `ls internal/runtime/` mostra arquivos no mesmo pacote sem subpastas. `engine.go` concentra execucao de agent, tools, memory e guardrails.
- Impacto: nao afeta funcionalidade ou API publica. Afeta manutenibilidade e legibilidade interna.
- Recomendacao: refatorar `internal/runtime/` em subpacotes conforme spec 020 quando a maturidade do codigo permitir sem regressao.

---

## Resumo por severidade

| Severidade | Quantidade | IDs |
| --- | --- | --- |
| S0 | 0 | — |
| S1 | 2 | LPG-001, LPG-002 |
| S2 | 4 | LPG-003, LPG-004, LPG-005, LPG-006 |
| S3 | 3 | LPG-007, LPG-008, LPG-009 |
| **Total** | **9** | |
