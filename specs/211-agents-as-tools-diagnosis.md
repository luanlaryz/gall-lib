# Spec 211: Agents as Tools Diagnosis

## 1. Objetivo

Definir uma auditoria objetiva para validar se a capability de agents-as-tools da `gaal-lib` realmente prova coordenacao entre agents de forma funcional, observavel e coerente com a arquitetura da biblioteca.

A auditoria avalia feature real, nao intencao. Toda evidencia deve ser verificavel por codigo, testes, examples ou runtime executavel. Nenhum item pode ser considerado coberto apenas por existir na spec de construcao.

---

## 2. Pergunta principal

"A `gaal-lib` prova coordenacao entre agents como capability real, reutilizavel e observavel, e nao apenas como wiring ad hoc?"

---

## 3. Escopo

### Dentro do escopo

- existencia e modelagem do adapter `AgentTool` como `tool.Tool`
- API publica: `NewAgentTool`, `AgentToolConfig`, `AgentRunFunc`, `AgentResolverFunc`
- resolucao estatica (agent direto) e lazy (via `Resolver`)
- integracao com `app.Runtime().ResolveAgent` e `app.AgentResolver`
- propagacao de `context.Context`, `session_id` e metadata
- semantica de memory isolada por agent e working memory efemera
- tratamento de erro do sub-agent como erro de tool
- cancelamento via contexto
- observabilidade via `Result.Metadata` (`sub_agent_id`, `sub_agent_run_id`)
- cobertura de testes mapeada contra os 15 cenarios da `Spec 210` secao 12
- example executavel em `examples/agent-as-tool/`
- bridge `agent.AsRunFunc` em `pkg/agent/as_tool.go`
- documentacao e rastreabilidade na feature matrix

### Fora do escopo

- streaming multiplexado do sub-agent para o coordenador
- recursao profunda entre agents (mais de 2 niveis)
- execucao concorrente de multiplos sub-agents em um unico step
- tracing distribuido ou correlation ID cross-process
- policy engine de delegacao ou autorizacao
- retry automatico pelo adapter
- VoltOps

---

## 4. Artefatos de entrada

O diagnostico deve ler e usar como fonte de verdade:

### Specs

- `specs/210-agents-as-tools.md` — spec de construcao (fonte normativa)
- `specs/030-agent.md` — contrato do Agent
- `specs/031-app-instance.md` — contrato do App e runtime
- `specs/040-tools.md` — contrato de Tools e Toolkits
- `specs/010-feature-matrix.md` — feature matrix
- `specs/200-lib-parity-diagnosis.md` — diagnostico geral de paridade

### Codigo

- `pkg/tool/agent_tool.go` — adapter AgentTool
- `pkg/tool/agent_tool_test.go` — testes do AgentTool
- `pkg/agent/as_tool.go` — bridge `AsRunFunc`
- `pkg/app/agent_tool.go` — `AgentResolver` para resolucao lazy via App
- `pkg/app/app.go` — `Runtime` interface e `ResolveAgent`
- `pkg/tool/tool.go` — interface `Tool`
- `pkg/tool/registry.go` — registry de tools

### Examples

- `examples/agent-as-tool/main.go` — example de coordenacao entre agents

### Reports anteriores

- `docs/reports/lib-parity-report.md`
- `docs/reports/lib-parity-gaps.md`
- `docs/reports/lib-parity-checklist.md`

---

## 5. Dimensoes obrigatorias de avaliacao

### D1 — Existencia e modelagem do adapter

Objetivo:
validar se o adapter `AgentTool` existe, implementa `tool.Tool` e expoe API publica coerente.

Checklist:

- [ ] `AgentTool` existe em `pkg/tool/agent_tool.go`
- [ ] implementa a interface `tool.Tool`
- [ ] `AgentToolConfig` e exportado com campos documentados
- [ ] `NewAgentTool` e exportado e retorna `(Tool, error)`
- [ ] `AgentRunFunc` e `AgentResolverFunc` sao tipos exportados
- [ ] `AgentToolResult` e exportado com `Content`, `AgentID`, `RunID`
- [ ] validacao de `Name` obedece ao regex `^[a-z][a-z0-9_-]{0,63}$`
- [ ] validacao de `Description` nao aceita vazio
- [ ] regra de precedencia: `RunFunc` nao nil tem precedencia sobre `Resolver`
- [ ] construcao com ambos nil retorna erro
- [ ] `InputSchema` expoe `prompt` (string, required)
- [ ] `OutputSchema` expoe `response` (string, required)

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D2 — Resolucao estatica e lazy

Objetivo:
validar se ambos os modos de resolucao funcionam conforme a `Spec 210` secoes 5.3 e D5.

Checklist:

- [ ] modo estatico: `RunFunc` fornecida diretamente, sub-agent fixo na construcao
- [ ] modo lazy: `Resolver` fornecido, sub-agent resolvido apenas no primeiro `Call`
- [ ] resolver nao e chamado na construcao (apenas no primeiro `Call`)
- [ ] resultado do resolver e cacheado apos primeira resolucao
- [ ] calls subsequentes reutilizam o agent resolvido sem invocar resolver novamente
- [ ] erro de resolucao (agent nao encontrado) propagado como `tool.ErrorKindExecution`
- [ ] resolucao lazy e thread-safe (`sync.Once` ou equivalente)

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D3 — Integracao com App/runtime registry

Objetivo:
validar se o adapter integra com o registry real do App sem criar arquitetura paralela.

Checklist:

- [ ] `app.AgentResolver(rt)` existe e retorna `tool.AgentResolverFunc`
- [ ] `AgentResolver` usa `rt.ResolveAgent(name)` internamente
- [ ] `AgentResolver` converte o agent resolvido via `agent.AsRunFunc`
- [ ] `agent.AsRunFunc` existe em `pkg/agent/as_tool.go` e adapta `Agent.Run` para `AgentRunFunc`
- [ ] o fluxo de bootstrap nao exige dependencia circular entre coordenador e sub-agent
- [ ] `AgentTool` e registravel em qualquer `Registry` como tool normal
- [ ] o runtime nao precisa de logica especial para reconhecer agent-tools

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D4 — Semantica de sessao, memoria e contexto

Objetivo:
validar se propagacao de contexto, sessao e memoria segue o contrato da `Spec 210` secao 7.

Checklist:

- [ ] `context.Context` do coordenador e propagado para `agent.Run` do sub-agent
- [ ] cancelamento do coordenador cancela o sub-agent
- [ ] deadline do coordenador se aplica ao sub-agent
- [ ] `tool.Call.SessionID` e mapeado para `agent.Request.SessionID`
- [ ] se sub-agent tiver memory e `session_id` estiver vazio, erro e propagado
- [ ] se sub-agent nao tiver memory, `session_id` e aceito mesmo vazio
- [ ] memory e isolada por agent (cada agent tem sua propria instancia de `Store`)
- [ ] working memory e efemera por run e nao cruza a fronteira coordenador/sub-agent
- [ ] `AgentTool` nao injeta nem altera a memory do sub-agent
- [ ] `Result.Metadata` contem `coordinator_agent_id` e `coordinator_run_id`
- [ ] `Request.Metadata` do sub-agent contem `coordinator_agent_id` e `coordinator_run_id`
- [ ] `Result.Metadata` contem `sub_agent_id` e `sub_agent_run_id`

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D5 — Tratamento de erro

Objetivo:
validar se erros sao propagados conforme a `Spec 210` secao 10.

Checklist:

- [ ] erro do sub-agent encapsulado como `tool.Error{Kind: ErrorKindExecution}`
- [ ] causa original acessivel via `errors.Is` e `errors.As`
- [ ] erros de resolucao encapsulados da mesma forma
- [ ] `context.Canceled` e `context.DeadlineExceeded` propagados corretamente
- [ ] input invalido (`prompt` ausente ou nao string) rejeitado pela validacao de input
- [ ] `AgentTool` nunca retorna nil error com `Result` vazio apos run bem-sucedido
- [ ] `AgentTool` nunca silencia erros do sub-agent
- [ ] `AgentTool` nunca faz retry implicito

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D6 — Cobertura de testes

Objetivo:
mapear cada cenario da `Spec 210` secao 12 para teste real existente.

A cobertura deve ser validada por correspondencia direta entre cenario normativo e teste executavel. Cenarios sem teste correspondente devem ser classificados como gap.

Checklist (referencia: `Spec 210` secao 12):

Construcao:

- [ ] 12.1: construcao com agent estatico valido e description
- [ ] 12.2: erro ao construir com `RunFunc` nil e `Resolver` nil
- [ ] 12.3: erro ao construir com `Name` invalido
- [ ] 12.4: erro ao construir com `Description` vazia
- [ ] 12.5: construcao com agent estatico deriva nome do agent quando `Name` omitido

Execucao:

- [ ] 12.6: caminho feliz — coordenador invoca sub-agent, recebe `Result` correto
- [ ] 12.7: erro do sub-agent propagado como `tool.Error` com causa original
- [ ] 12.8: cancelamento via `context.Context` propagado
- [ ] 12.9: `session_id` propagado corretamente

Resolver:

- [ ] 12.10: resolver lazy invocado apenas no primeiro `Call`
- [ ] 12.11: resolver que retorna erro propagado como `tool.ErrorKindExecution`
- [ ] 12.12: segundo `Call` reutiliza agent resolvido

Memory e concorrencia:

- [ ] 12.13: sub-agent com memory recebe e usa `session_id` correto
- [ ] 12.14: uso concorrente sem race condition (verificavel com `-race`)

Observabilidade:

- [ ] 12.15: `Result.Metadata` contem `sub_agent_id` e `sub_agent_run_id`

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D7 — Example e demo

Objetivo:
validar se existe evidencia executavel que demonstra coordenacao real entre agents.

Checklist:

- [ ] `examples/agent-as-tool/main.go` existe
- [ ] o example compila sem erros
- [ ] o example demonstra criacao de sub-agent e coordenador
- [ ] o example usa `AgentTool` para delegar tarefa
- [ ] o example demonstra fluxo completo: request -> coordenador -> sub-agent -> resposta
- [ ] o example funciona localmente sem dependencias externas
- [ ] o example usa API publica (nao importa `internal/*`)

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

### D8 — Documentacao e rastreabilidade

Objetivo:
validar se a feature esta documentada e rastreavel na spec, feature matrix e codigo.

Checklist:

- [ ] `specs/210-agents-as-tools.md` existe e esta atualizada
- [ ] `specs/010-feature-matrix.md` reflete o status correto de agents-as-tools
- [ ] decisoes de design (D1-D6 da spec 210) estao documentadas
- [ ] semantica de sessao/memoria esta documentada na spec 210 secao 7
- [ ] tratamento de erro esta documentado na spec 210 secao 10
- [ ] tipos exportados possuem comentarios de API no codigo
- [ ] example documenta o uso basico da capability

Classificacao: `PASS` | `PARTIAL` | `FAIL` | `BLOCKED`

---

## 6. Criterios de classificacao

Cada dimensao deve ser classificada como:

### PASS

Funciona, esta evidenciado por codigo e testes, e cobre a expectativa definida na `Spec 210`. Nao existem gaps relevantes na dimensao.

### PARTIAL

Existe e funciona parcialmente, ou falta evidencia, teste ou documentacao suficiente para confirmar cobertura completa. Gaps identificados nao sao bloqueadores.

### FAIL

Deveria existir conforme a `Spec 210` e nao atende. A ausencia compromete a capability de coordenacao entre agents.

### BLOCKED

Nao foi possivel verificar por bloqueio tecnico, ambiental ou ausencia de artefato necessario para a auditoria.

---

## 7. Severidade dos gaps

Cada gap encontrado deve receber severidade:

### S0 — Bloqueador de verificabilidade

- impossibilidade de build/test do adapter
- example nao compilavel
- testes nao executaveis

### S1 — Bloqueador de paridade

- adapter nao implementa `tool.Tool`
- resolucao nao funciona (estatica ou lazy)
- propagacao de `session_id` ou contexto quebrada
- erros do sub-agent silenciados
- nenhum teste cobrindo caminho feliz

### S2 — Lacuna relevante mas nao bloqueante

- cenario de teste da spec 210 sem correspondencia
- cobertura de concorrencia insuficiente
- feature matrix desatualizada
- metadata de rastreabilidade incompleta

### S3 — Melhoria de DX ou acabamento

- documentacao insuficiente nos tipos exportados
- example pouco didatico
- comentarios de API ausentes
- naming inconsistente

---

## 8. Checklist obrigatorio resumido

### Modelagem (D1)

- [ ] `AgentTool` existe e implementa `tool.Tool`
- [ ] tipos publicos exportados: `AgentToolConfig`, `AgentRunFunc`, `AgentResolverFunc`, `AgentToolResult`
- [ ] `NewAgentTool` exportado com validacao
- [ ] `InputSchema` e `OutputSchema` corretos

### Resolucao (D2)

- [ ] modo estatico funciona
- [ ] modo lazy funciona com caching
- [ ] erro de resolucao propagado

### Registry (D3)

- [ ] `app.AgentResolver` existe e usa `ResolveAgent`
- [ ] `agent.AsRunFunc` existe como bridge
- [ ] sem logica especial no runtime para agent-tools

### Sessao/memoria/contexto (D4)

- [ ] `context.Context` propagado com cancelamento e deadline
- [ ] `session_id` propagado do coordenador para o sub-agent
- [ ] memory isolada por agent
- [ ] metadata de rastreabilidade presente

### Erros (D5)

- [ ] erros encapsulados como `tool.Error`
- [ ] cancelamento propagado
- [ ] sem retry implicito

### Testes (D6)

- [ ] 15 cenarios da spec 210 mapeados para testes reais
- [ ] testes executam com `go test` e `-race`

### Example (D7)

- [ ] `examples/agent-as-tool/main.go` existe e compila
- [ ] demonstra coordenacao real entre agents

### Documentacao (D8)

- [ ] spec de construcao existe e esta atualizada
- [ ] feature matrix reflete status correto

---

## 9. Saida obrigatoria

O diagnostico deve gerar:

- `docs/reports/agents-as-tools-report.md`

### Conteudo obrigatorio do relatorio

#### 1. Executive Summary

- decisao final da trilha
- blockers encontrados
- conclusao curta

#### 2. Evidence Reviewed

- lista de arquivos inspecionados
- lista de testes executados
- lista de specs consultadas
- reports anteriores considerados

#### 3. Scorecard

Tabela com:

| Dimensao | Status | Observacao |
|----------|--------|------------|
| D1 — Modelagem | | |
| D2 — Resolucao | | |
| D3 — Registry | | |
| D4 — Sessao/memoria/contexto | | |
| D5 — Erros | | |
| D6 — Testes | | |
| D7 — Example | | |
| D8 — Documentacao | | |

#### 4. Gaps Found

Para cada gap:

- id
- severidade (`S0` | `S1` | `S2` | `S3`)
- dimensao afetada
- descricao
- evidencia
- recomendacao

#### 5. Coverage vs Spec 210

Tabela mapeando cada criterio de aceitacao da `Spec 210` secao 13 para status verificado:

| Criterio (Spec 210 s13) | Status | Evidencia |
|--------------------------|--------|-----------|
| 13.1: API publica exposta | | |
| 13.2: implementa tool.Tool | | |
| 13.3: resolucao estatica e lazy | | |
| 13.4: 15 cenarios testados | | |
| 13.5: example executavel | | |
| 13.6: semantica de sessao/memoria | | |
| 13.7: erros propagados | | |
| 13.8: go test passa | | |
| 13.9: sem dependencia de internal em pkg/tool | | |
| 13.10: sem logica especial no runtime | | |

#### 6. Test Coverage Map

Tabela mapeando cada cenario da `Spec 210` secao 12 para teste real:

| Cenario | Teste correspondente | Status |
|---------|---------------------|--------|
| 12.1 | | |
| 12.2 | | |
| ... | | |
| 12.15 | | |

#### 7. Final Decision

Uma destas:

- `PASS` — a capability de agents-as-tools esta funcional, evidenciada e coerente
- `PARTIAL` — a capability existe mas tem gaps relevantes
- `FAIL` — a capability nao atende os requisitos da spec de construcao
- `BLOCKED` — nao foi possivel verificar

#### 8. Next Prioritized Actions

Lista curta ordenada por impacto e severidade.

---

## 10. Criterios de aceitacao

Esta spec sera considerada concluida quando:

1. a feature for auditada com criterios objetivos contra a `Spec 210`
2. cada dimensao (D1-D8) for classificada individualmente
3. cada cenario de teste da `Spec 210` secao 12 for mapeado para teste real ou declarado como gap
4. cada criterio de aceitacao da `Spec 210` secao 13 for verificado
5. o relatorio `docs/reports/agents-as-tools-report.md` for gerado com todas as secoes obrigatorias
6. a decisao final da trilha for explicita
7. gaps remanescentes forem nomeados com severidade
8. a feature matrix for consultada e divergencias apontadas

---

## 11. Observacao normativa

Esta e a spec de diagnostico para a trilha de agents-as-tools.

Ela complementa:

- a `Spec 210`, que e a spec de construcao desta trilha
- a `Spec 200`, que define o diagnostico geral de paridade da biblioteca
- a `Spec 030`, que define o contrato do Agent
- a `Spec 040`, que define o contrato de Tools e Toolkits

Ela fecha a regra de dual-spec do `AGENTS.md`: toda trilha nova deve possuir spec de construcao e spec de diagnostico.

Esta spec nao substitui a `Spec 210`. Ela define como auditar o que a `Spec 210` mandou construir.
