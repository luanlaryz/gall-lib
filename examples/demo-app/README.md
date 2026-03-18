# Demo App

Esta demo executavel prova o fluxo base local da `gaal-lib` com:

- `pkg/app`
- `pkg/agent`
- `pkg/server`
- `pkg/logger`
- `pkg/memory`
- `pkg/tool`
- `pkg/workflow`

Ela foi desenhada para ser simples, didatica e local. Nao depende de VoltOps, nao usa provider remoto e sobe com um unico comando.

## Subir a demo

Opcionalmente, copie os valores de `examples/demo-app/.env.example` para o seu ambiente.

Suba a demo com:

```bash
go run ./cmd/demo-app
```

Por default, ela escuta em `127.0.0.1:8080`.

## Variaveis de ambiente

- `DEMO_APP_ADDR`: endereco HTTP da demo. Exemplo: `127.0.0.1:8080`
- `DEMO_APP_NAME`: nome logico do `App`
- `DEMO_AGENT_NAME`: nome logico do agent principal
- `DEMO_LOG_LEVEL`: nivel do logger local. Valores praticos: `debug`, `info`, `warn`, `error`

## Endpoints

- `GET /healthz`: probe de health local
- `GET /readyz`: probe de readiness local
- `GET /agents`: lista os agents registrados
- `POST /agents/{name}/runs`: execucao textual sincrona
- `POST /agents/{name}/stream`: streaming SSE
- `GET /workflows`: lista os workflows registrados
- `POST /workflows/{name}/runs`: executa um workflow

## Tools registradas

O agent da demo registra 2 tools deterministicas:

- `get_time`: retorna a data/hora UTC atual em formato RFC3339. Acionada por mensagens contendo "time" ou "hora".
- `calculate_sum`: soma dois numeros. Acionada por mensagens contendo "sum" ou "soma" seguido de dois numeros.

O modelo fake da demo detecta keywords na mensagem do usuario e simula tool calls. O runtime do engine executa a tool real e re-alimenta o resultado na conversa antes de gerar a resposta final.

Para provocar um erro de tool, envie uma mensagem contendo "use unknown_tool". O engine tentara resolver uma tool inexistente e retornara erro observavel.

## Exemplos com curl

Health:

```bash
curl http://127.0.0.1:8080/healthz
```

Readiness:

```bash
curl http://127.0.0.1:8080/readyz
```

Listagem de agents:

```bash
curl http://127.0.0.1:8080/agents
```

Execucao textual:

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-1",
    "message": "Ada",
    "metadata": {
      "user_id": "user-1",
      "conversation_id": "conv-1"
    }
  }'
```

Executando novamente com o mesmo `session_id`, a resposta muda para refletir a memoria in-memory persistida no processo:

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-1",
    "message": "Ada",
    "metadata": {
      "user_id": "user-1",
      "conversation_id": "conv-1"
    }
  }'
```

Streaming SSE:

```bash
curl -N -X POST http://127.0.0.1:8080/agents/demo-agent/stream \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-stream-1",
    "message": "Grace",
    "metadata": {
      "user_id": "user-1",
      "conversation_id": "conv-stream-1"
    }
  }'
```

Tool call (get_time):

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-tool-1",
    "message": "what time is it?"
  }'
```

Tool call (calculate_sum):

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-tool-2",
    "message": "sum 3 and 7"
  }'
```

Tool error (unknown tool):

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-tool-err",
    "message": "use unknown_tool please"
  }'
```

Tool call via streaming:

```bash
curl -N -X POST http://127.0.0.1:8080/agents/demo-agent/stream \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-tool-stream",
    "message": "what time is it?"
  }'
```

## Workflow registrado

A demo registra o workflow `order-processing`, que simula um fluxo de processamento de pedidos com decisao condicional. O workflow possui 5 steps:

1. `validate_order` (Action) — valida campos obrigatorios (`item`, `amount`) e grava no state
2. `route_order` (Branch) — se `amount > 100`, desvia para `manual_review`; caso contrario, para `auto_approve`
3. `auto_approve` (Action) — marca status como `approved`
4. `manual_review` (Action) — marca status como `pending_review`
5. `confirm` (Action) — monta mensagem final com status e detalhes

O workflow usa `InMemoryHistory` para historico local, `NewLoggingHook` para observabilidade via logs e `FixedRetryPolicy{MaxRetries: 1}` para retry basico.

Listagem de workflows:

```bash
curl http://127.0.0.1:8080/workflows
```

Workflow run (auto-approve, amount <= 100):

```bash
curl -X POST http://127.0.0.1:8080/workflows/order-processing/runs \
  -H "Content-Type: application/json" \
  -d '{
    "item": "notebook",
    "amount": 50
  }'
```

Workflow run (manual review, amount > 100):

```bash
curl -X POST http://127.0.0.1:8080/workflows/order-processing/runs \
  -H "Content-Type: application/json" \
  -d '{
    "item": "server-rack",
    "amount": 200
  }'
```

## Guardrails

A demo registra 3 guardrails como defaults do `App`, herdados automaticamente pelo agent:

### Input guardrail: `inputBlockGuardrail`

Bloqueia qualquer run cuja mensagem do usuario contenha `"BLOCK_ME"`. O run falha com HTTP 422 e erro classificado de guardrail.

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-guardrail-block",
    "message": "BLOCK_ME"
  }'
```

Resultado esperado: HTTP 422 com erro contendo `"guardrail"`.

### Output guardrail: `outputTagGuardrail`

Acrescenta ` [guardrail:ok]` ao final de toda resposta. Toda chamada bem-sucedida tera o sufixo observavel no output.

```bash
curl -X POST http://127.0.0.1:8080/agents/demo-agent/runs \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-guardrail-tag",
    "message": "Eve"
  }'
```

Resultado esperado: `"output": "hello, Eve [guardrail:ok]"`.

### Stream guardrail: `streamDigitGuardrail`

Substitui digitos `[0-9]` por `*` em cada chunk de stream. Observavel via SSE nos eventos `agent.delta`.

```bash
curl -N -X POST http://127.0.0.1:8080/agents/demo-agent/stream \
  -H "Content-Type: application/json" \
  -d '{
    "session_id": "session-guardrail-stream",
    "message": "test 123"
  }'
```

Resultado esperado: deltas contem `***` em vez de `123`, e o output final termina com ` [guardrail:ok]`.

## Smoke manual

1. Suba a demo com `go run ./cmd/demo-app`.
2. Verifique `GET /healthz` e confirme `health=true`.
3. Verifique `GET /readyz` e confirme `ready=true`.
4. Chame `GET /agents` e confirme a presenca do `demo-agent`.
5. Execute `POST /agents/demo-agent/runs` e confirme uma resposta textual.
6. Repita a mesma chamada com o mesmo `session_id` e confirme a resposta de retorno de memoria.
7. Execute `POST /agents/demo-agent/stream` com `curl -N` e observe eventos `agent.started`, `agent.delta` e `agent.completed`.
8. Execute `POST /agents/demo-agent/runs` com `"message": "what time is it?"` e confirme `tool_calls` na resposta e output com hora UTC.
9. Execute `POST /agents/demo-agent/runs` com `"message": "sum 3 and 7"` e confirme output contendo "10".
10. Execute `POST /agents/demo-agent/runs` com `"message": "use unknown_tool please"` e confirme erro de tool na resposta.
11. Execute `POST /agents/demo-agent/stream` com `"message": "what time is it?"` e observe eventos `agent.tool_call` e `agent.tool_result` no SSE.
12. Chame `GET /workflows` e confirme a presenca do `order-processing`.
13. Execute `POST /workflows/order-processing/runs` com `"item": "notebook", "amount": 50` e confirme `status: approved`.
14. Execute `POST /workflows/order-processing/runs` com `"item": "server-rack", "amount": 200` e confirme `status: pending_review`.
15. Execute `POST /agents/demo-agent/runs` com `"message": "BLOCK_ME"` e confirme HTTP 422 com erro de guardrail.
16. Execute `POST /agents/demo-agent/runs` com qualquer mensagem e confirme que o output termina com ` [guardrail:ok]`.
17. Execute `POST /agents/demo-agent/stream` com `"message": "test 123"` e confirme que os deltas tem digitos substituidos por `*`.

## Arquivos uteis

- `examples/demo-app/.env.example`
- `examples/demo-app/http/demo.http`
- `test/smoke/demo_app_test.go`

## Lacunas conhecidas

- o modelo da demo e fake e deterministico; ele existe para smoke test local, nao para provar provider real
- a heuristica de tool call e baseada em keywords na mensagem do usuario, nao em raciocinio real do modelo
- a memoria e apenas in-process via `memory.InMemoryStore`; reiniciar o processo limpa todo o estado conversacional
- o streaming HTTP usa um adapter SSE minimo da demo, sem promover um helper HTTP generico novo em `pkg/server`
- o erro de tool e provocado por tool desconhecida, nao por falha real de execucao
- nao ha toolkit na demo; tools sao registradas individualmente
- nao ha autenticacao, rate limit nem OpenAPI formal nesta entrega
