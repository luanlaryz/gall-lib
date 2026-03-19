# Demo Web UI Report

Date: 2026-03-19 (Spec 151: Demo Web UI Diagnosis)

## 1. Executive summary

- Status geral: **PASS**
- Blockers: nenhum.
- A demo web UI da `gaal-lib` atende todos os 6 itens do checklist da Spec 151. A UI sobe localmente sem dependencias de frontend, integra-se corretamente com o backend, suporta fluxo textual e streaming SSE, e a documentacao e clara e suficiente. A experiencia e simples o bastante para uma demo minima.

Resposta a pergunta principal da spec: **"Sim, a demo UI ja aproxima a `gaal-lib` de uma experiencia de starter app utilizavel."**

## 2. Evidence reviewed

### Specs lidas

- `specs/151-demo-web-ui-diagnosis.md` — checklist e criterios do diagnostico
- `specs/150-demo-web-ui.md` — requisitos funcionais da UI (RF-01 a RF-04)
- `specs/100-demo-app.md` — spec da demo app base

### Arquivos inspecionados

- `cmd/demo-app/main.go` — boot, composicao e lifecycle
- `cmd/demo-app/server.go` — rotas HTTP, SSE handler, probes
- `cmd/demo-app/agent.go` — modelo fake, heuristicas de tool call, streaming
- `cmd/demo-app/config.go` — config via env
- `cmd/demo-app/embed.go` — `go:embed static`
- `cmd/demo-app/guardrails.go` — input block, output tag, stream digit
- `cmd/demo-app/tools.go` — `get_time`, `calculate_sum`
- `cmd/demo-app/workflow.go` — `order-processing` workflow
- `cmd/demo-app/static/index.html` — UI completa (387 linhas, HTML+CSS+JS inline)
- `examples/demo-app/README.md` — documentacao da demo (307 linhas)
- `test/smoke/demo_app_test.go` — smoke tests automatizados (714 linhas)

### Ambiente usado

- OS: `darwin 24.6.0`
- Toolchain: `go 1.26.1`
- Porta da demo: `127.0.0.1:18090`

### Testes executados

- `go test ./test/smoke/... -v -count=1` em 2026-03-19: **PASS** (23 subtests + `TestDemoAppMemoryResetAfterRestart`)
- Boot manual com `go run ./cmd/demo-app` em `127.0.0.1:18090`: **PASS**

### Evidencia executavel observada

| Teste | Resultado | Observacao |
| --- | --- | --- |
| `GET /healthz` | PASS | `{"state":"running","health":true,"ready":true,"draining":false}` |
| `GET /readyz` | PASS | `{"state":"running","health":true,"ready":true,"draining":false}` |
| `GET /agents` | PASS | `{"agents":[{"Name":"demo-agent","ID":"demo-agent"}]}` |
| `GET /` (index.html) | PASS | HTTP 200, `text/html; charset=utf-8`, 12937 bytes |
| `POST /agents/demo-agent/runs` (Ada, 1a vez) | PASS | `"output":"hello, Ada [guardrail:ok]"` |
| `POST /agents/demo-agent/runs` (Ada, 2a vez, mesmo session) | PASS | `"output":"welcome back, Ada [guardrail:ok]"` |
| `POST /agents/demo-agent/runs` (get_time) | PASS | `tool_calls[0].name="get_time"`, output com hora UTC |
| `POST /agents/demo-agent/runs` (calculate_sum) | PASS | `tool_calls[0].name="calculate_sum"`, output "10" |
| `POST /agents/demo-agent/runs` (BLOCK_ME) | PASS | HTTP 422, erro de guardrail |
| `POST /agents/demo-agent/runs` (unknown_tool) | PASS | HTTP 500, erro de tool |
| `POST /agents/demo-agent/stream` (Grace) | PASS | Eventos SSE: `agent.started`, `agent.delta` (x2), `agent.completed` |
| `POST /agents/demo-agent/stream` (get_time) | PASS | Eventos: `agent.tool_call`, `agent.tool_result`, `agent.delta` (x5), `agent.completed`. Digits redacted (`****-**-**T**:**:**Z`) |
| `POST /agents/demo-agent/stream` (test 123) | PASS | Deltas com `***` em vez de `123`, output com `[guardrail:ok]` |

## 3. Checklist Spec 151

| # | Item | Status | Evidencia |
| --- | --- | --- | --- |
| 1 | UI sobe localmente | PASS | `go run ./cmd/demo-app` sobe na porta configurada. `GET /` retorna `index.html` via `embed.FS`. Zero dependencias de frontend. |
| 2 | Backend e UI integram corretamente | PASS | `GET /agents` popula dropdown. Send -> `POST /agents/{name}/runs`. Stream -> `POST /agents/{name}/stream`. Session ID auto-gerado e editavel. Erros tratados inline. |
| 3 | Ha fluxo textual | PASS | Botao Send dispara run sincrono. Resposta exibida no chat bubble. Tool calls exibidas. Erros de guardrail e rede tratados. |
| 4 | Ha fluxo streaming | PASS | Botao Stream dispara SSE. Parser de `event:`/`data:` no frontend. Eventos tratados: `agent.delta`, `agent.completed`, `agent.tool_call`, `agent.tool_result`, `agent.error`, `agent.failed`. Cursor piscante durante stream. |
| 5 | A documentacao e clara | PASS | README com secao "Web UI" (linhas 253-282), 6 exemplos de uso, secao "Lacunas conhecidas". |
| 6 | A experiencia e simples o suficiente para demo | PASS | Arquivo unico embutido, sem build step. Tema escuro limpo. Seletor de agent, session ID, Send, Stream, Clear. Textarea com auto-resize. Enter para enviar. |

**Resultado: 6/6 PASS**

## 4. Scorecard

| Area | Status | Observacao |
| --- | --- | --- |
| Boot/embed | PASS | `index.html` embutido via `go:embed static`, servido em `/` com `text/html`. Sem toolchain frontend. |
| Integracao UI-backend | PASS | JS faz fetch correto para `/agents`, `/agents/{name}/runs` e `/agents/{name}/stream`. Session ID propagado. |
| Fluxo textual | PASS | `POST /agents/{name}/runs` funciona. Resposta, tool calls e erros exibidos corretamente. |
| Fluxo streaming SSE | PASS | `POST /agents/{name}/stream` funciona. SSE parseado no frontend. Deltas acumulados com cursor piscante. Eventos `agent.tool_call` e `agent.tool_result` tratados. |
| Guardrails na UI | PASS | Input block exibe erro. Output tag visivel no output. Stream digit redaction observavel nos deltas. |
| Documentacao | PASS | README completo com secao Web UI, exemplos de uso e lacunas conhecidas. |
| Smoke tests | PASS | 23 subtests + `TestDemoAppMemoryResetAfterRestart`, todos PASS em 2026-03-19. |
| UX/simplicidade | PASS | Layout chat minimalista, tema escuro, controles claros, auto-resize, atalhos de teclado. |

## 5. Gaps

### 5.1. Gaps de UI (cosmeticos/UX, nao blockers)

| ID | Severidade | Descricao | Impacto |
| --- | --- | --- | --- |
| DWUG-001 | S4 | Sem UI para workflows — o workflow `order-processing` esta registrado mas a UI nao expoe interacao visual. So acessivel via curl/API. | Baixo. Workflows sao demonstraveis via API. UI foca corretamente no agent. |
| DWUG-002 | S4 | Sem indicador visual de loading — durante send/stream, apenas botoes ficam disabled. Nao ha spinner ou animacao de progresso. | Baixo. O cursor piscante do stream e feedback visual suficiente para demo. |
| DWUG-003 | S5 | Sem renderizacao de markdown — respostas do agent sao exibidas como texto plano em `pre-wrap`. | Negligenciavel. O modelo fake nao gera markdown. |
| DWUG-004 | S5 | Sem persistencia de chat — recarregar a pagina limpa a conversa visual (memoria backend persiste no processo). | Negligenciavel. Esperado para demo minima. |
| DWUG-005 | S5 | Clear nao reseta session ID — limpar o chat mantem o mesmo session, o que pode confundir se a memoria backend preservou o contexto. | Negligenciavel. Comportamento documentavel. |

### 5.2. Gaps de backend (ja conhecidos, confirmados)

| ID | Descricao | Status |
| --- | --- | --- |
| BKG-001 | Modelo fake e deterministico, sem provider real | Conhecido. Fora do escopo da demo minima. |
| BKG-002 | Memoria in-process (`InMemoryStore`), limpa no restart | Conhecido. Documentado no README. |
| BKG-003 | Sem auth, rate limit ou OpenAPI formal | Conhecido. Fora do escopo da demo minima. |
| BKG-004 | Heuristica de tool call baseada em keywords, nao em raciocinio do modelo | Conhecido. Esperado para modelo fake. |

## 6. Validacao contra Spec 150 (requisitos funcionais)

| RF | Requisito | Status | Evidencia |
| --- | --- | --- | --- |
| RF-01 | Interface minima para enviar input ao agent | PASS | Textarea + botoes Send/Stream |
| RF-02 | Forma de visualizar resposta textual | PASS | Chat bubble com output do agent |
| RF-03 | Forma de visualizar streaming | PASS | Deltas SSE com cursor piscante |
| RF-04 | UI se conecta ao backend demo existente | PASS | `GET /agents`, `POST .../runs`, `POST .../stream` |

**Todos os 4 RFs da Spec 150 atendidos.**

## 7. Decisao final

### **PASS**

A demo web UI da `gaal-lib` atende integralmente os 6 itens do checklist da Spec 151 e os 4 requisitos funcionais da Spec 150. A UI e funcional, integrada ao backend, suporta fluxo textual e streaming SSE, e oferece uma experiencia de demo minima e simples. Os gaps identificados sao cosmeticos (S4/S5) e nao comprometem a proposta.

A `gaal-lib` ja oferece uma experiencia de demo proxima de um starter app utilizavel.

## 8. Proximos passos sugeridos

### Prioridade alta (ampliam utilidade pratica)

1. **Provider real**: integrar pelo menos um provider LLM real (ex: OpenAI, Anthropic) como alternativa ao modelo fake. Isso transformaria a demo de prova de conceito em experiencia funcional.
2. **UI para workflows**: adicionar aba ou secao na UI para demonstrar o workflow `order-processing` visualmente, com formulario de input e exibicao do resultado.

### Prioridade media (melhoram DX e UX)

3. **Indicador de loading**: adicionar spinner ou animacao sutil no botao Send durante request sincrono.
4. **Makefile target**: adicionar `make demo` como atalho para `go run ./cmd/demo-app`.
5. **OpenAPI/Swagger**: documentar formalmente a superficie HTTP da demo.

### Prioridade baixa (nice-to-have)

6. **Renderizacao de markdown**: usar biblioteca leve (ex: `marked.js` via CDN) para renderizar respostas do agent com formatacao.
7. **Persistencia local de chat**: usar `localStorage` para manter historico visual entre reloads.
8. **Tema claro/escuro**: toggle de tema para acessibilidade.

## 9. Comparacao com relatorios anteriores

| Aspecto | Demo Parity Report (2026-03-18) | Web UI Diagnosis (2026-03-19) |
| --- | --- | --- |
| Escopo | Backend + API (paridade funcional) | UI web + integracao visual |
| Status | APT for base demo parity | PASS |
| Gaps abertos | 0 | 5 (UI, todos S4/S5) |
| Backend gaps | 0 (todos resolvidos) | 4 (conhecidos, confirmados) |
| Smoke tests | PASS (23 subtests) | PASS (23 subtests, re-verificado) |
| Checklist | 39/39 | 6/6 (Spec 151) + 4/4 (Spec 150 RFs) |

### Conclusao

A Web UI complementa a demo backend ja validada, fechando a trilha de experiencia visual minima da `gaal-lib`. O projeto esta pronto para evoluir na direcao de provider real e UX mais rica.
