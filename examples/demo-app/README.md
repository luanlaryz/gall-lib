# Demo App

Esta demo executavel prova o fluxo base local da `gaal-lib` com:

- `pkg/app`
- `pkg/agent`
- `pkg/server`
- `pkg/logger`
- `pkg/memory`
- `pkg/tool`

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

## Arquivos uteis

- `examples/demo-app/.env.example`
- `examples/demo-app/http/demo.http`
- `test/smoke/demo_app_test.go`

## Lacunas conhecidas

- o modelo da demo e fake e deterministico; ele existe para smoke test local, nao para provar provider real
- a heuristica de tool call e baseada em keywords na mensagem do usuario, nao em raciocinio real do modelo
- a memoria e apenas in-process via `memory.InMemoryStore`; reiniciar o processo limpa todo o estado conversacional
- o streaming HTTP usa um adapter SSE minimo da demo, sem promover um helper HTTP generico novo em `pkg/server`
- nao ha tool calls paralelas nem workflows multi-step complexos
- o erro de tool e provocado por tool desconhecida, nao por falha real de execucao
- nao ha toolkit na demo; tools sao registradas individualmente
- nao ha autenticacao, rate limit nem OpenAPI formal nesta entrega
