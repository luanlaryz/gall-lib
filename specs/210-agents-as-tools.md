# Spec 210: Agents as Tools

## 1. Objetivo

Adicionar a `gaal-lib` a capacidade de usar um agent como ferramenta de outro agent, fechando o gap S1 de paridade de orquestracao identificado na `Spec 200`.

O mecanismo deve reaproveitar o sistema de tools existente (`tool.Tool`, `Registry`, `Call`, `Result`) e integrar com o registry real do `App`/runtime, sem criar arquitetura paralela.

Esta spec complementa a `Spec 030`, a `Spec 040` e a `Spec 031`.

---

## 2. Motivacao

A `gaal-lib` ja possui:

- `Agent` funcional com `Run` e `Stream`
- `App`/runtime com registry de agents e workflows
- `Tools` com contrato publico, registry, validacao e contexto operacional
- `Memory` com store persistente e working memory efemera
- `Guardrails` de input, output e stream
- `Workflows` com chain, branching e retries
- demos executaveis e testes

Mas ainda nao possui um modo claro e verificavel de:

- fazer um agent coordenador delegar uma tarefa a outro agent
- expor um agent como tool para outro agent
- preservar contexto, rastreabilidade e resultado nessa coordenacao

Como a meta do projeto e paridade funcional com o Voltagent para orquestracao de agentes, essa capability deve existir como feature de primeira classe.

O diagnostico da biblioteca (`Spec 200`, dimensao 7.5) classificou a ausencia de coordenacao entre agents como `FAIL` com severidade `S1`.

---

## 3. Escopo

### Dentro do escopo

- adapter `AgentTool` em `pkg/tool` que implementa `tool.Tool`
- construtor `NewAgentTool` com config validada
- resolucao estatica (agent direto) e lazy (resolver via registry)
- integracao com `app.Runtime().ResolveAgent` como resolver padrao
- propagacao de `context.Context`, `session_id` e metadata
- tratamento de erro do sub-agent como erro de tool
- observabilidade via eventos normais do coordenador e do sub-agent
- testes obrigatorios de caminho feliz, erro, cancelamento e session
- example executavel demonstrando a capability

### Fora do escopo

- orquestracao distribuida
- hierarquias profundas de supervisao (mais de 2 niveis nao sao alvo desta spec)
- scheduler de multi-agent complexo
- streaming multiplexado do sub-agent para o coordenador
- execucao concorrente de multiplos sub-agents em um unico step
- recursao irrestrita entre agents
- tracing distribuido
- policy engine de delegacao
- balanceamento de carga entre agents
- UI especifica para coordenacao multi-agent

---

## 4. Conceitos principais

- `AgentTool`: adapter que implementa `tool.Tool` delegando `Call` para `agent.Run` de forma sincrona.
- `AgentToolConfig`: struct de configuracao com validacao obrigatoria.
- `Resolver`: funcao `func(name string) (*agent.Agent, error)` para resolucao lazy de agent via registry do runtime.
- `Coordenador`: agent que possui um `AgentTool` em sua lista de tools e delega tarefas ao sub-agent.
- `Sub-agent`: agent registrado no runtime que e invocado pelo coordenador via `AgentTool`.

Compatibilidade conceitual com Voltagent:

- O Voltagent permite configurar `subAgents` em um agent, que sao expostos como tools para o modelo.
- A `gaal-lib` atinge o mesmo efeito funcional via `AgentTool`: o sub-agent e adaptado como `tool.Tool` e adicionado a lista de tools do coordenador.
- A divergencia idiomatica e deliberada: em Go, a composicao ocorre por adapter explicito e registry, nao por propriedade magica no agent.

---

## 5. API publica proposta

O adapter deve viver em `pkg/tool`.

### 5.1 Tipos

```go
package tool

import "github.com/luanlima/gaal-lib/pkg/agent"

type AgentToolConfig struct {
    Agent       *agent.Agent
    Name        string
    Description string
    Resolver    func(name string) (*agent.Agent, error)
}
```

### 5.2 Construtor

```go
func NewAgentTool(cfg AgentToolConfig) (Tool, error)
```

### 5.3 Regras de construcao

1. `Name` e obrigatorio e deve obedecer ao regex `^[a-z][a-z0-9_-]{0,63}$`.
2. `Description` e obrigatoria e deve descrever o comportamento do sub-agent.
3. Exatamente uma das duas fontes de agent deve ser fornecida:
   - `Agent` nao nil (modo estatico): o sub-agent e fixo na construcao.
   - `Resolver` nao nil com `Name` nao vazio (modo lazy): o sub-agent e resolvido no primeiro `Call`.
4. Se `Agent` e `Resolver` forem ambos nil, `NewAgentTool` retorna erro.
5. Se `Agent` e `Resolver` forem ambos nao nil, `Agent` tem precedencia e `Resolver` e ignorado.
6. Quando `Agent` e fornecido, `Name` pode ser omitido; nesse caso, o nome e derivado do `agent.Name()` normalizado para o regex de tool.
7. O adapter retornado e seguro para uso concorrente apos construcao.

### 5.4 Modos de uso

Modo estatico (wiring manual, testes):

```go
specialist, _ := agent.New(agent.Config{Name: "specialist", ...})
agentTool, _ := tool.NewAgentTool(tool.AgentToolConfig{
    Agent:       specialist,
    Description: "Delegates research tasks to the specialist agent",
})
coordinator, _ := agent.New(agent.Config{Name: "coordinator", ...},
    agent.WithTools(agentTool),
)
```

Modo lazy via registry (integracao com App):

```go
application, _ := app.New(app.Config{Name: "my-app"},
    app.WithAgents(specialist, coordinator),
)
application.Start(ctx)

rt := application.Runtime()
agentTool, _ := tool.NewAgentTool(tool.AgentToolConfig{
    Name:        "specialist",
    Description: "Delegates research tasks to the specialist agent",
    Resolver:    rt.ResolveAgent,
})
```

---

## 6. Decisoes obrigatorias de modelagem

### D1 — AgentTool e tool.Tool generica

O `AgentTool` implementa `tool.Tool` diretamente. Nao ha interface ou tipo especializado separado. O coordenador trata o sub-agent como qualquer outra tool. O runtime nao precisa de logica especial para agent-tools.

Justificativa: reuso maximo do sistema de tools existente; o coordenador e o runtime nao precisam saber que uma tool e um agent.

### D2 — session_id propagado do coordenador

O `session_id` presente em `tool.Call.SessionID` e repassado como `agent.Request.SessionID` para o sub-agent.

Consequencias:

- Se o sub-agent tiver memory configurada, ele carrega e salva historico sob o mesmo `session_id` do coordenador.
- Cada agent mantem sua propria instancia de `memory.Store`, entao nao ha mistura de historico entre agents, mesmo com o mesmo `session_id`.
- Se o sub-agent nao tiver memory, ele funciona stateless e o `session_id` e apenas informativo.

Justificativa: o comportamento padrao deve preservar a sessao do usuario de ponta a ponta. Isolamento logico por agent e garantido pela separacao de stores.

### D3 — Sub-agent herda defaults do App

Quando o sub-agent e registrado no App e materializado por factory ou como ready agent, ele recebe os defaults normais do App (engine, memory, hooks, guardrails) conforme a `Spec 031`.

O `AgentTool` nao interfere nos defaults do sub-agent. O adapter apenas invoca `agent.Run` no agent ja configurado.

### D4 — Memory isolada por agent, working memory efemera

Cada agent possui sua propria instancia de `memory.Store`. O `AgentTool` nao compartilha nem injeta memory no sub-agent.

Working memory do sub-agent e criada por `WorkingMemoryFactory.NewRunState` a cada invocacao de `agent.Run`, como em qualquer run normal. Ela e efemera e isolada por run.

O coordenador nao tem acesso a working memory do sub-agent. O unico dado visivel ao coordenador e o `tool.Result` retornado.

### D5 — Resolucao por nome via Resolver

No modo lazy, o `Resolver` e invocado no primeiro `Call` e o resultado e cacheado. Calls subsequentes reutilizam o agent resolvido.

O resolver padrao para integracao com App e `app.Runtime().ResolveAgent`, que retorna o agent materializado pelo bootstrap.

Se o resolver falhar, o erro e retornado como `tool.Error{Kind: ErrorKindExecution}` com a causa original.

### D6 — Streaming do sub-agent fora do escopo v1

Nesta versao, o `AgentTool` invoca apenas `agent.Run` (sincrono). O resultado textual do sub-agent e retornado como `tool.Result`.

Streaming do sub-agent para o coordenador (`agent.Stream` + multiplexacao de eventos) fica explicitamente fora do escopo desta spec e pode ser tratado em spec futura.

---

## 7. Semantica de contexto, sessao e memoria

### 7.1 context.Context

O `context.Context` recebido em `tool.Call` via o runtime do coordenador e propagado diretamente para `agent.Run` do sub-agent.

Consequencias:

- Cancelamento do coordenador cancela o sub-agent.
- Deadline do coordenador se aplica ao sub-agent.
- Values do contexto sao visiveis ao sub-agent.

### 7.2 session_id

Regras:

1. `tool.Call.SessionID` e mapeado para `agent.Request.SessionID` do sub-agent.
2. Se o sub-agent tiver memory e o `session_id` estiver vazio, `agent.Run` retorna `ErrInvalidRequest` conforme a `Spec 030`. O `AgentTool` propaga esse erro.
3. Se o sub-agent nao tiver memory, o `session_id` e aceito mesmo se vazio.

### 7.3 Memory

Regras:

1. O `AgentTool` nao configura, injeta ou altera a memory do sub-agent.
2. O sub-agent usa a memory que lhe foi configurada (pelo App defaults ou por `agent.WithMemory`).
3. Se ambos coordenador e sub-agent tiverem memory com o mesmo backend, ambos podem persistir sob o mesmo `session_id`, mas cada um em sua propria instancia de `Store`. Nao ha conflito porque `Store.Load`/`Store.Save` sao isolados por instancia.
4. Working memory e efemera por run e nao cruza a fronteira entre coordenador e sub-agent.

### 7.4 Metadata

O `AgentTool` constroi o `agent.Request.Metadata` a partir de `tool.Call.Metadata`, acrescentando:

- `coordinator_agent_id`: o `AgentID` do coordenador (vindo de `tool.Call.AgentID`)
- `coordinator_run_id`: o `RunID` do coordenador (vindo de `tool.Call.RunID`)

O sub-agent pode usar essa metadata em hooks ou logs para rastreabilidade.

---

## 8. Integracao com App/runtime registry

### 8.1 Fluxo de bootstrap tipico

```
1. Criar specialist agent
2. Criar coordinator agent com AgentTool(specialist) na lista de tools
3. Registrar ambos no App
4. App.Start() materializa agents com defaults
5. Coordenador recebe request
6. Modelo do coordenador decide chamar a tool do specialist
7. Runtime invoca AgentTool.Call()
8. AgentTool invoca specialist.Run()
9. Resultado volta como tool.Result para o coordenador
```

### 8.2 Registro no App

O `AgentTool` e uma tool normal. Ele entra na lista de tools do coordenador via `agent.WithTools`. O App nao precisa de logica especial para reconhecer agent-tools.

O sub-agent deve estar registrado no App independentemente para receber defaults (engine, memory, hooks). Se o sub-agent for passado diretamente via modo estatico, ele nao precisa estar registrado no App, mas nao recebera defaults do App.

### 8.3 Resolver e bootstrap

No modo lazy, o `Resolver` permite desacoplar a construcao do `AgentTool` do bootstrap do App:

1. O `AgentTool` e criado com `Resolver = rt.ResolveAgent`.
2. Na construcao, o resolver nao e chamado.
3. No primeiro `Call`, o resolver e invocado e o agent e cacheado.
4. Se o resolver falhar, o `Call` falha com erro de execucao.

Isso permite que o coordenador e o sub-agent sejam registrados no App simultaneamente sem dependencia circular de construcao.

### 8.4 Validacao de integridade

O `AgentTool` nao valida se o sub-agent esta registrado no App. Essa responsabilidade e do `Resolver`. Se o `Resolver` for `Runtime.ResolveAgent` e o agent nao estiver registrado, o erro `ErrAgentNotFound` e retornado no momento do `Call`.

---

## 9. InputSchema e OutputSchema

### 9.1 InputSchema

```go
Schema{
    Type:        "object",
    Description: "Input for the sub-agent tool",
    Properties: map[string]Schema{
        "prompt": {
            Type:        "string",
            Description: "The task or question to delegate to the sub-agent",
        },
    },
    Required: []string{"prompt"},
}
```

### 9.2 OutputSchema

```go
Schema{
    Type:        "object",
    Description: "Output from the sub-agent tool",
    Properties: map[string]Schema{
        "response": {
            Type:        "string",
            Description: "The text response produced by the sub-agent",
        },
    },
    Required: []string{"response"},
}
```

### 9.3 Mapeamento de dados

Entrada:

- `Call.Input["prompt"]` e convertido em uma mensagem de role `user` com `Content = prompt`.
- A request enviada ao sub-agent contem exatamente uma mensagem: `types.Message{Role: "user", Content: prompt}`.

Saida:

- `agent.Response.Message.Content` e mapeado para `Result.Content`.
- `Result.Value` e `map[string]any{"response": agent.Response.Message.Content}`.
- `Result.Metadata` contem `sub_agent_id` e `sub_agent_run_id` extraidos da response.

---

## 10. Tratamento de erro

### 10.1 Erros do sub-agent

Todo erro retornado por `agent.Run` e encapsulado como:

```go
&tool.Error{
    Kind:     tool.ErrorKindExecution,
    Op:       "call",
    ToolName: effectiveName,
    CallID:   call.ID,
    Cause:    originalErr,
}
```

`errors.Is` e `errors.As` continuam funcionando para desempacotar o erro original do agent (ex.: `agent.ErrGuardrailBlocked`, `agent.ErrMaxStepsExceeded`).

### 10.2 Erros de resolucao

Se o `Resolver` retornar erro (ex.: `app.ErrAgentNotFound`), o erro e encapsulado da mesma forma como `tool.ErrorKindExecution` com a causa original.

### 10.3 Cancelamento

`context.Canceled` e `context.DeadlineExceeded` sao propagados sem encapsulamento adicional alem do que `agent.Run` ja faz. O runtime do coordenador deve conseguir detectar cancelamento via `errors.Is(err, context.Canceled)`.

### 10.4 Input invalido

Se `Call.Input["prompt"]` estiver ausente ou nao for string, a validacao de input da `Spec 040` rejeita antes de `Call` ser invocado. O `AgentTool` nao precisa de validacao customizada de input.

### 10.5 Invariantes

- O `AgentTool` nunca retorna `nil` error com `Result` vazio apos `agent.Run` bem-sucedido; sempre ha `Content` ou `Value`.
- O `AgentTool` nunca silencia erros do sub-agent.
- O `AgentTool` nunca faz retry implicito.

---

## 11. Observabilidade

### 11.1 Eventos do coordenador

O runtime do coordenador emite os eventos normais de tool call:

- `EventToolCall` com `ToolCallStatus = ToolCallStarted` antes de `Call`.
- `EventToolResult` com `ToolCallStatus = ToolCallSucceeded` ou `ToolCallFailed` apos `Call`.
- `ToolCallRecord.Name` contem o effective name do `AgentTool`.

Esses eventos sao indistinguiveis de qualquer outra tool call.

### 11.2 Eventos do sub-agent

O sub-agent emite seus proprios eventos via seus hooks e engine:

- `EventAgentStarted`, `EventAgentCompleted`, `EventAgentFailed`, etc.
- Esses eventos sao observaveis por hooks registrados no sub-agent.

### 11.3 Rastreabilidade

O `Result.Metadata` retornado pelo `AgentTool` contem:

- `sub_agent_id`: o ID do sub-agent.
- `sub_agent_run_id`: o RunID gerado para o run do sub-agent.

O coordenador pode logar ou inspecionar esses campos para rastrear a delegacao.

---

## 12. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao sao:

### Construcao

1. Construcao bem-sucedida com agent estatico valido e description.
2. Erro ao construir com `Agent` nil e `Resolver` nil.
3. Erro ao construir com `Name` invalido (fora do regex).
4. Erro ao construir com `Description` vazia.
5. Construcao com agent estatico deve derivar nome do `agent.Name()` quando `Name` nao for fornecido.

### Execucao

6. Caminho feliz: coordenador invoca sub-agent via `AgentTool`, recebe `Result` com `Content` e `Value` corretos.
7. Erro do sub-agent propagado como `tool.Error{Kind: ErrorKindExecution}` com causa original acessivel.
8. Cancelamento via `context.Context` propagado ao sub-agent; erro resultante e verificavel com `errors.Is(err, context.Canceled)`.
9. `session_id` de `tool.Call.SessionID` propagado corretamente para `agent.Request.SessionID`.

### Resolver

10. Resolver lazy invocado apenas no primeiro `Call`, nao na construcao.
11. Resolver que retorna erro (agent nao encontrado) propagado como `tool.ErrorKindExecution`.
12. Segundo `Call` reutiliza agent resolvido sem invocar resolver novamente.

### Memory e concorrencia

13. Sub-agent com memory recebe e usa o `session_id`; memory `Load`/`Save` e invocada com o session correto.
14. Uso concorrente do mesmo `AgentTool` em runs distintos sem race condition (verificavel com `-race`).

### Observabilidade

15. `Result.Metadata` contem `sub_agent_id` e `sub_agent_run_id` apos execucao bem-sucedida.

---

## 13. Criterios de aceitacao

Esta spec sera considerada atendida quando:

1. `pkg/tool` expuser `NewAgentTool` e `AgentToolConfig` como API publica.
2. O `AgentTool` implementar `tool.Tool` e for registravel em qualquer `Registry` e consumivel por qualquer `Agent`.
3. O adapter funcionar com resolucao estatica e lazy via `Resolver`.
4. Houver teste cobrindo todos os 15 cenarios da secao 12.
5. Houver example executavel em `examples/agent-as-tool/main.go` demonstrando coordenacao entre dois agents.
6. A semantica de `session_id`, memory e working memory estiver implementada conforme a secao 7.
7. Erros do sub-agent forem propagados conforme a secao 10.
8. `go test ./...` continuar passando, incluindo os novos testes.
9. A implementacao nao introduzir dependencia de `internal/*` em `pkg/tool` (o adapter usa apenas `pkg/agent` e `pkg/types`).
10. A implementacao nao exigir logica especial no runtime do coordenador para reconhecer agent-tools.

---

## 14. Evidencias obrigatorias

A implementacao deve deixar evidencia em:

- Codigo: `pkg/tool/agent_tool.go` com o adapter e construtor.
- Testes: `pkg/tool/agent_tool_test.go` com os cenarios da secao 12.
- Example: `examples/agent-as-tool/main.go` com coordenacao entre dois agents.
- Documentacao: decisoes de design documentadas nesta spec (secao 6) e no codigo via comentarios de API.
- Feature matrix: atualizar `specs/010-feature-matrix.md` para refletir o novo status da capability.

---

## 15. Fora do escopo tecnico explicito

- Streaming multiplexado do sub-agent para o coordenador.
- Execucao concorrente de multiplos sub-agents em um unico step do coordenador.
- Recursao irrestrita entre agents (mais de 2 niveis nao sao alvo).
- Tracing distribuido ou correlation ID cross-process.
- Policy engine de delegacao ou autorizacao.
- Retry automatico de sub-agent por parte do adapter.
- Customizacao de `InputSchema`/`OutputSchema` por instancia (schema e fixo na v1).
- Propagacao de guardrails do coordenador para o sub-agent (cada agent mantem seus proprios).
- Heranca de tools do coordenador para o sub-agent.

---

## 16. Modelo funcional de referencia

O fluxo minimo que a implementacao deve suportar:

```
1. Existe um coordinator-agent com instructions de delegacao
2. Existe um specialist-agent registrado no App
3. specialist-agent e exposto ao coordenador como AgentTool
4. Coordenador recebe request do usuario
5. Modelo do coordenador decide chamar a tool do specialist
6. Runtime do coordenador invoca AgentTool.Call(ctx, call)
7. AgentTool resolve o specialist (estatico ou via resolver)
8. AgentTool constroi agent.Request com prompt e session_id
9. AgentTool invoca specialist.Run(ctx, request)
10. specialist executa normalmente (memory, tools, guardrails)
11. AgentTool converte agent.Response em tool.Result
12. Runtime do coordenador recebe tool.Result
13. Modelo do coordenador usa o resultado para compor resposta final
14. Coordenador retorna response ao usuario
```

---

## 17. Questoes resolvidas

As questoes listadas no draft anterior desta spec estao todas resolvidas:

| Questao | Decisao | Secao |
|---------|---------|-------|
| Sub-agent e tool generica ou tipo especializado? | Tool generica (`tool.Tool`) | D1 |
| Como `session_id` e propagado? | Propagado do coordenador via `Call.SessionID` | D2, 7.2 |
| Sub-agent herda defaults do app? | Sim, normalmente | D3 |
| Sub-agent usa memory propria ou compartilhada? | Propria; working memory efemera por run | D4, 7.3 |
| Sub-agent pode ser chamado por nome, referencia ou ambos? | Ambos: estatico ou via resolver | D5, 5.3 |
| Streaming do sub-agent entra nesta fase? | Nao; fora do escopo v1 | D6 |

---

## 18. Observacao normativa

Esta spec e a spec de construcao para a trilha de agents-as-tools.

Ela complementa:

- a `Spec 030`, que define o contrato do Agent
- a `Spec 031`, que define o contrato do App e runtime
- a `Spec 040`, que define o contrato de Tools e Toolkits
- a `Spec 200`, que identificou o gap de paridade

A spec de diagnostico correspondente para esta trilha devera ser criada como `specs/211-agents-as-tools-diagnosis.md` conforme a regra de dual-spec do `AGENTS.md`.
