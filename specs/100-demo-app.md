# Spec 100: Demo App

## 1. Objetivo

Este documento define a demo executavel oficial da `gaal-lib`.

Os objetivos desta spec sao:

- fornecer uma demo local, simples e didatica, inspirada nos starter apps do ecossistema VoltAgent
- provar o fluxo base `app + agent + server + logger + memory` usando apenas contratos publicos da biblioteca
- disponibilizar um smoke test manual e automatizado para o caminho principal da biblioteca
- expor endpoints HTTP locais para probes, descoberta de agents, execucao textual e streaming
- manter a demo livre de VoltOps, servicos hospedados obrigatorios e infraestrutura externa

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020`, a `Spec 030`, a `Spec 031`, a `Spec 050` e a `Spec 081`.

Compatibilidade com a `Spec 010`:

- `Application entrypoint` e exercitado pela composicao de `pkg/app`
- `Agent` e exercitado por execucao sincrona e streaming
- `Memory` e exercitada com `memory.InMemoryStore`
- `Logger` e exercitado como logger local do `App`
- `HTTP server abstraction` continua opcional, mas esta demo prova um adapter HTTP local sobre `pkg/app`

Ficam fora do escopo desta spec:

- autenticacao, autorizacao, rate limiting e middlewares de producao
- providers reais de LLM, secrets remotos ou servicos hospedados
- painel web, dashboard, telemetria hospedada ou qualquer recurso de VoltOps
- persistencia duravel fora do processo
- OpenAPI formal, SDK cliente ou contratos de estabilidade de transporte alem do necessario para a demo

## 2. Resultado esperado

A entrega desta spec deve produzir:

1. uma spec normativa para a demo
2. um binario executavel com um unico comando
3. documentacao de uso e smoke manual
4. um arquivo `.env.example`
5. um arquivo HTTP de exemplos
6. smoke tests automatizados minimos

## 3. Regras arquiteturais

1. A demo deve usar `pkg/app` como composition root publica.
2. O transporte HTTP da demo deve ser um adapter local sobre `pkg/app.Server`.
3. A demo nao pode importar `internal/*` a partir do binario em `cmd/demo-app`.
4. Se for necessario compartilhar wiring entre binario e testes, isso deve ocorrer por helper interno pequeno e nao por nova API publica.
5. A demo deve usar `memory.InMemoryStore` por default.
6. O logger global deve vir de `pkg/logger`.
7. O agent da demo deve ser deterministico o suficiente para smoke tests locais.
8. A demo nao deve criar uma API paralela ao core da biblioteca; ela deve apenas adaptar `pkg/app`, `pkg/agent`, `pkg/memory`, `pkg/logger` e `pkg/server`.

## 4. Composicao normativa

### 4.1 App

A demo deve construir uma instancia de `pkg/app.App` com:

- `Config.Name` estavel para a demo
- logger global local
- defaults de agent herdando `memory.InMemoryStore`
- pelo menos um `AgentFactory`
- pelo menos um `Server` long-lived baseado em HTTP

### 4.2 Agent

O agent minimo da demo deve:

- possuir nome logico estavel
- aceitar `Run` textual simples
- aceitar `Stream` com eventos ordenados
- funcionar com `SessionID` para demonstrar `memory.Store`
- usar modelo fake ou deterministico local

O agent nao precisa provar tools, reasoning, providers remotos ou guardrails avancados nesta fase.

### 4.3 Memory

A demo deve usar `memory.InMemoryStore` por default.

Regras normativas:

1. `SessionID` deve ser aceito nos endpoints de execucao.
2. Quando `SessionID` for repetido, a demo deve conseguir reaproveitar memoria conversacional in-process.
3. Reiniciar o processo pode limpar a memoria; isso deve ser documentado explicitamente.

### 4.4 Server

O servidor da demo deve satisfazer o contrato de `pkg/app.Server` e ser gerenciado pelo lifecycle do `App`.

Regras normativas:

1. `Server.Start` deve receber `Runtime` e usalo para resolver agents.
2. `Server.Shutdown` deve desligar o listener HTTP cooperativamente.
3. Probes de health e readiness devem refletir a semantica de `pkg/server.Snapshot`.
4. O adapter HTTP pode usar `net/http`; nao ha obrigacao de framework adicional.

## 5. Configuracao

O binario da demo deve subir com um unico comando.

Configuracoes minimas recomendadas:

- `DEMO_APP_ADDR`: endereco de bind HTTP local
- `DEMO_APP_NAME`: nome logico da aplicacao
- `DEMO_AGENT_NAME`: nome logico do agent principal
- `DEMO_LOG_LEVEL`: opcional, apenas se o logger local suportar ajuste simples

Regras normativas:

1. A ausencia de configuracao explicita deve levar a defaults locais seguros.
2. O default deve permitir subir a demo sem dependencias externas.
3. `.env.example` deve documentar apenas configuracoes realmente suportadas pelo binario.

## 6. Endpoints HTTP

### 6.1 Probes

A demo deve expor:

- `GET /healthz`
- `GET /readyz`

Semantica normativa:

1. `GET /healthz` deve retornar sucesso enquanto a instancia estiver localmente saudavel.
2. `GET /readyz` deve retornar sucesso apenas quando o `App` estiver apto a receber novo trafego.
3. Os payloads podem ser JSON simples, desde que incluam ao menos o estado observavel do `App`.

### 6.2 Descoberta de agents

A demo deve expor:

- `GET /agents`

Semantica normativa:

1. O endpoint deve listar os agents registrados via `Runtime.ListAgents()`.
2. A listagem deve refletir nomes e ids observaveis.
3. O endpoint nao deve manter registry paralelo fora do `Runtime`.

### 6.3 Execucao textual

A demo deve expor um endpoint HTTP sincrono para execucao textual.

Contrato minimo recomendado:

- `POST /agents/{name}/runs`

Payload minimo:

```json
{
  "session_id": "session-1",
  "message": "Ada",
  "metadata": {
    "user_id": "user-1",
    "conversation_id": "conv-1"
  }
}
```

Resposta minima:

```json
{
  "run_id": "run-123",
  "agent_id": "demo-agent",
  "session_id": "session-1",
  "output": "hello, Ada"
}
```

Regras normativas:

1. O endpoint deve resolver o agent por nome usando `Runtime.ResolveAgent`.
2. O endpoint deve adaptar a mensagem textual para `agent.Request`.
3. Erro de agent ausente deve ser observavel como erro de transporte explicito.
4. Erro de request invalido deve ser observavel como erro de cliente.
5. O endpoint deve permanecer simples; nao ha obrigacao de envelope generico complexo.

### 6.4 Streaming

A demo deve expor um endpoint HTTP local para streaming textual.

Contrato minimo recomendado:

- `POST /agents/{name}/stream`

Transporte normativo:

- SSE com `Content-Type: text/event-stream`

Regras normativas:

1. O endpoint deve iniciar `Agent.Stream`.
2. Os eventos devem preservar a ordem observavel recebida de `Recv()`.
3. O stream deve encerrar em EOF apos evento terminal.
4. A demo nao precisa estabilizar um schema universal de eventos alem do necessario para uso local e didatico.
5. O README e o arquivo `demo.http` devem mostrar como consumir o stream manualmente.

## 7. Documentacao obrigatoria

### 7.1 README

`examples/demo-app/README.md` deve incluir:

- objetivo da demo
- como subir com um unico comando
- variaveis de ambiente suportadas
- exemplos de `curl`
- fluxo de smoke manual
- lacunas conhecidas

### 7.2 .env.example

`examples/demo-app/.env.example` deve incluir apenas variaveis de ambiente realmente suportadas.

### 7.3 Arquivo HTTP

`examples/demo-app/http/demo.http` deve cobrir ao menos:

- health
- readiness
- listagem de agents
- execucao textual
- streaming

## 8. Smoke tests automatizados

`test/smoke/demo_app_test.go` deve cobrir no minimo:

1. boot do app da demo
2. `GET /healthz`
3. `GET /readyz`
4. `GET /agents`
5. `POST` de execucao textual com sucesso

Regras normativas:

1. Os testes devem validar comportamento observavel do adapter HTTP local.
2. Os testes devem usar a superficie publica sempre que possivel.
3. O smoke test nao precisa cobrir streaming nesta primeira entrega, desde que README e `demo.http` cubram o fluxo manual.

## 9. Lacunas aceitaveis

As seguintes lacunas sao aceitaveis nesta fase, desde que documentadas:

- uso de modelo fake/deterministico em vez de provider real
- ausencia de autenticacao
- ausencia de OpenAPI formal
- streaming HTTP local sem helper reutilizavel novo em `pkg/server`
- memoria apenas in-process

As seguintes lacunas nao sao aceitaveis:

- dependencia de VoltOps
- dependencia obrigatoria de servico hospedado
- bypass de `pkg/app` para subir runtime
- demo que nao prove o caminho basico de `health`, `agents` e execucao textual

## 10. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `specs/100-demo-app.md` estiver presente e rastreavel para as specs base.
2. `cmd/demo-app/main.go` subir a demo com um unico comando local.
3. a demo usar `pkg/app`, `pkg/agent`, `pkg/memory`, `pkg/logger` e `pkg/server` de forma coerente com suas specs.
4. `memory.InMemoryStore` for o default de memoria conversacional.
5. `GET /healthz`, `GET /readyz` e `GET /agents` estiverem funcionando.
6. existir um endpoint HTTP de execucao textual sincrona.
7. existir um endpoint HTTP local de streaming.
8. `examples/demo-app/README.md`, `examples/demo-app/.env.example` e `examples/demo-app/http/demo.http` estiverem presentes.
9. `test/smoke/demo_app_test.go` cobrir boot, health, readiness, listagem e execucao textual.
10. qualquer lacuna restante estiver documentada explicitamente.
