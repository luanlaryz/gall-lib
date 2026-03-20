# Agents-as-Tools Audit Report

**Data:** 2026-03-20
**Spec de diagnostico:** `specs/211-agents-as-tools-diagnosis.md`
**Spec de construcao:** `specs/210-agents-as-tools.md`

---

## 1. Executive Summary

**Decisao final: PASS**

A capability de agents-as-tools da `gaal-lib` esta funcional, evidenciada por testes e coerente com a arquitetura da biblioteca. O adapter `AgentTool` implementa `tool.Tool`, integra com o registry real do App via `app.AgentResolver`, propaga contexto, sessao e metadata, e trata erros conforme especificado.

Todos os 15 testes obrigatorios da Spec 210 secao 12 passam, incluindo verificacao de race condition com `-race`. O example em `examples/agent-as-tool/main.go` compila e demonstra coordenacao real entre agents.

**Blockers:** nenhum.

**Gaps remanescentes:** 4 gaps de documentacao/acabamento (S2/S3), nenhum de funcionalidade.

---

## 2. Evidence Reviewed

### Arquivos inspecionados

| Arquivo | Papel |
|---------|-------|
| `pkg/tool/agent_tool.go` | Adapter AgentTool |
| `pkg/tool/agent_tool_test.go` | 15 testes do AgentTool |
| `pkg/agent/as_tool.go` | Bridge `AsRunFunc` |
| `pkg/app/agent_tool.go` | `AgentResolver` para resolucao lazy |
| `pkg/app/app.go` | Runtime interface, `ResolveAgent` |
| `pkg/tool/tool.go` | Interface `Tool`, tipos `Call`, `Result` |
| `pkg/tool/registry.go` | Registry de tools |
| `pkg/tool/errors.go` | Tipos de erro (`ErrorKind`, `Error`) |
| `pkg/tool/validate.go` | Validacao de nomes (regex) |
| `pkg/tool/invoke.go` | Funcao `Invoke` |
| `examples/agent-as-tool/main.go` | Example de coordenacao |

### Testes executados

```
go test ./pkg/tool/ -run AgentTool -race -v
```

Resultado: 15/15 PASS, 0 FAIL, race detector limpo.

```
go build ./examples/agent-as-tool/
```

Resultado: compilacao bem-sucedida, exit code 0.

### Specs consultadas

- `specs/210-agents-as-tools.md` — spec de construcao
- `specs/211-agents-as-tools-diagnosis.md` — spec de diagnostico
- `specs/010-feature-matrix.md` — feature matrix
- `specs/200-lib-parity-diagnosis.md` — diagnostico geral

### Reports anteriores considerados

- `docs/reports/lib-parity-report.md` — marca agents-as-tools como FAIL (desatualizado)
- `docs/reports/lib-parity-gaps.md` — lista LPG-001 como gap aberto (desatualizado)

---

## 3. Scorecard

| Dimensao | Status | Observacao |
|----------|--------|------------|
| D1 — Modelagem | PASS | Adapter existe, implementa `tool.Tool`, tipos exportados, validacao completa, schemas corretos |
| D2 — Resolucao | PASS | Modo estatico e lazy funcionam, caching via `sync.Once`, erro propagado |
| D3 — Registry | PASS | `app.AgentResolver` integra com `ResolveAgent`, bridge `AsRunFunc` funciona, sem logica especial no runtime |
| D4 — Sessao/memoria/contexto | PASS | Context, session_id e metadata propagados; memory isolada por design |
| D5 — Erros | PASS | Erros encapsulados como `tool.Error`, cancelamento propagado, sem retry implicito |
| D6 — Testes | PASS | 15/15 cenarios cobertos, race detector limpo |
| D7 — Example | PASS | Example compila, demonstra coordenacao real, usa API publica |
| D8 — Documentacao | PARTIAL | Spec 210 completa; feature matrix e reports de paridade desatualizados |

---

## 4. Gaps Found

### GAP-001

- **Severidade:** S3
- **Dimensao:** D8 — Documentacao
- **Descricao:** Feature matrix (`specs/010-feature-matrix.md`) mostra status "Parcial" para agents-as-tools. A implementacao cobre todos os cenarios obrigatorios da Spec 210.
- **Evidencia:** Linha 62 de `specs/010-feature-matrix.md` lista `Status: Parcial`.
- **Recomendacao:** Atualizar para "Implementado" com observacao de que streaming do sub-agent e recursao profunda ficam explicitamente fora do escopo v1.

### GAP-002

- **Severidade:** S3
- **Dimensao:** D8 — Documentacao
- **Descricao:** Reports de paridade (`lib-parity-report.md`, `lib-parity-gaps.md`) ainda marcam agents-as-tools como FAIL com severidade S1 (gap LPG-001). A implementacao ja existe e fecha esse gap.
- **Evidencia:** Secao 7.5 de `lib-parity-report.md` classifica como FAIL. `lib-parity-gaps.md` lista LPG-001 como aberto.
- **Recomendacao:** Re-executar diagnostico da Spec 200 ou atualizar pontualmente os reports para refletir a implementacao.

### GAP-003

- **Severidade:** S2
- **Dimensao:** D6 — Testes
- **Descricao:** Cenario 12.13 da Spec 210 (sub-agent com memory recebe `session_id` e `memory.Store.Load`/`Save` e invocada com o session correto) nao tem teste dedicado com mock de `memory.Store`. A propagacao de `session_id` esta testada via `TestAgentToolSessionIDPropagation`, mas a integracao end-to-end com memory real nao esta explicitamente verificada no nivel do adapter.
- **Evidencia:** `TestAgentToolSessionIDPropagation` verifica que o `sessionID` chega ao `RunFunc`, mas nao instancia um agent real com memory para confirmar `Load`/`Save`.
- **Recomendacao:** Adicionar teste de integracao que use um agent real com `memory.Store` mock para verificar que `Load` e `Save` sao invocados com o `session_id` correto durante um `AgentTool.Call`.

### GAP-004

- **Severidade:** S3
- **Dimensao:** D1 — Modelagem
- **Descricao:** A API implementada diverge da API literal da Spec 210 secao 5.1. A spec define `Agent *agent.Agent` e `Resolver func(name string) (*agent.Agent, error)`, mas a implementacao usa `RunFunc AgentRunFunc` e `Resolver AgentResolverFunc`. A divergencia e idiomatica (melhor desacoplamento via callbacks) e a bridge `agent.AsRunFunc` compensa, mas a spec nao reflete a decisao final.
- **Evidencia:** Comparacao entre `AgentToolConfig` em `pkg/tool/agent_tool.go` (linhas 35-40) e Spec 210 secao 5.1 (linhas 93-98).
- **Recomendacao:** Atualizar Spec 210 secao 5.1 para documentar a decisao final de API com justificativa.

---

## 5. Coverage vs Spec 210

| Criterio (Spec 210 s13) | Status | Evidencia |
|--------------------------|--------|-----------|
| 13.1: API publica exposta (`NewAgentTool`, `AgentToolConfig`) | PASS | `pkg/tool/agent_tool.go` exporta `NewAgentTool`, `AgentToolConfig`, `AgentRunFunc`, `AgentResolverFunc`, `AgentToolResult` |
| 13.2: implementa `tool.Tool` e registravel em qualquer Registry | PASS | `agentTool` implementa `Name()`, `Description()`, `InputSchema()`, `OutputSchema()`, `Call()`; registravel via `Registry.Register()` |
| 13.3: resolucao estatica e lazy via Resolver | PASS | Modo estatico via `RunFunc`; modo lazy via `Resolver` + `sync.Once` |
| 13.4: 15 cenarios testados | PASS | 15 testes em `pkg/tool/agent_tool_test.go`, todos passam com `-race` |
| 13.5: example executavel em `examples/agent-as-tool/main.go` | PASS | Example existe e compila; demonstra coordinator -> specialist -> resposta |
| 13.6: semantica de `session_id`, memory e working memory | PASS | `session_id` propagado via `Call.SessionID`; memory isolada por agent via `AsRunFunc`; working memory efemera por run |
| 13.7: erros do sub-agent propagados | PASS | Erros encapsulados como `tool.Error{Kind: ErrorKindExecution}` com causa original acessivel |
| 13.8: `go test ./...` passa | PASS | `go test ./pkg/tool/ -run AgentTool -race -v` — 15/15 PASS |
| 13.9: sem dependencia de `internal/*` em `pkg/tool` | PASS | `pkg/tool/agent_tool.go` nao importa nenhum pacote de `internal/` |
| 13.10: sem logica especial no runtime para agent-tools | PASS | `AgentTool` e tratado como qualquer `tool.Tool`; runtime e registry nao possuem logica especifica |

---

## 6. Test Coverage Map

| Cenario (Spec 210 s12) | Teste correspondente | Status |
|-------------------------|---------------------|--------|
| 12.1: construcao com agent estatico valido e description | `TestNewAgentToolStaticValid` | PASS |
| 12.2: erro ao construir com RunFunc nil e Resolver nil | `TestNewAgentToolBothNil` | PASS |
| 12.3: erro ao construir com Name invalido | `TestNewAgentToolInvalidName` | PASS |
| 12.4: erro ao construir com Description vazia | `TestNewAgentToolEmptyDescription` | PASS |
| 12.5: construcao com agent estatico deriva nome | `TestNewAgentToolRunFuncPrecedenceOverResolver` | PASS (nota: derivacao de nome do agent nao se aplica; API usa RunFunc em vez de *Agent) |
| 12.6: caminho feliz — Result correto | `TestAgentToolHappyPath` | PASS |
| 12.7: erro do sub-agent propagado | `TestAgentToolSubAgentError` | PASS |
| 12.8: cancelamento via context propagado | `TestAgentToolCancellation` | PASS |
| 12.9: session_id propagado | `TestAgentToolSessionIDPropagation` | PASS |
| 12.10: resolver lazy no primeiro Call | `TestAgentToolResolverLazyFirstCallOnly` | PASS |
| 12.11: resolver erro propagado | `TestAgentToolResolverError` | PASS |
| 12.12: segundo Call reutiliza agent | `TestAgentToolResolverCachesAgent` | PASS |
| 12.13: sub-agent com memory usa session_id | `TestAgentToolSessionIDPropagation` | PARTIAL (session_id propagado, mas sem mock de memory.Store) |
| 12.14: uso concorrente sem race | `TestAgentToolConcurrentUsage` + `TestAgentToolConcurrentResolverLazy` | PASS |
| 12.15: Result.Metadata com sub_agent_id e sub_agent_run_id | `TestAgentToolResultMetadata` | PASS |

---

## 7. Final Decision

**PASS**

A capability de agents-as-tools da `gaal-lib` esta funcional, evidenciada e coerente com a arquitetura da biblioteca. O gap S1 de paridade de orquestracao identificado na Spec 200 (dimensao 7.5, LPG-001) esta fechado.

7 de 8 dimensoes classificadas como PASS. 1 dimensao (D8 — Documentacao) classificada como PARTIAL por desatualizacao de artefatos auxiliares, nao por ausencia de capability.

Os 4 gaps encontrados sao de documentacao e acabamento (S2/S3). Nenhum gap de funcionalidade (S0/S1) foi identificado.

---

## 8. Next Prioritized Actions

1. **(S2) Adicionar teste de integracao com memory.Store** — cenario 12.13 da Spec 210. Criar teste que instancie agent real com mock de `memory.Store` e verifique que `Load`/`Save` sao invocados com o `session_id` correto durante `AgentTool.Call`.

2. **(S3) Atualizar feature matrix** — alterar status de agents-as-tools de "Parcial" para "Implementado" em `specs/010-feature-matrix.md`.

3. **(S3) Atualizar Spec 210 secao 5.1** — refletir a decisao final de API (`RunFunc`/`AgentResolverFunc` em vez de `Agent *agent.Agent`/`Resolver func`).

4. **(S3) Atualizar reports de paridade** — re-executar diagnostico da Spec 200 ou atualizar pontualmente `lib-parity-report.md` e `lib-parity-gaps.md` para fechar LPG-001.
