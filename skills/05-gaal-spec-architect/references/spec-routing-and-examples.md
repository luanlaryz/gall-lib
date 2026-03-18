# Routing and examples for gaal-lib

## Dominios canonicos
- `030-agent`: API publica do agent, execucao sincrona/streaming, tools, memory, guardrails, hooks.
- `050-memory`: semantica de memoria conversacional, `SessionID`, `Store`, persistencia, isolamento por sessao.
- `070-workflows`: builder, workflow runnable, state, retry, branching, hooks, history.

## Heuristicas de roteamento
- Pedidos sobre execucao do agent, output streaming, hooks ou tool loop tendem a `030-agent`.
- Pedidos sobre continuidade de conversa, persistencia, `SessionID`, adapters de storage ou snapshots tendem a `050-memory`.
- Pedidos sobre cadeia de steps, branching, retry, suspend/resume ou execution history tendem a `070-workflows`.

## Exemplo 1: pedido vago sobre agent
### Pedido inicial
"quero melhorar o streaming do agent"

### Refinamento esperado
- O que muda no comportamento observavel do stream?
- Afeta ordem de eventos, cancelamento, flush final ou tratamento de erro?
- O contrato e do `pkg/agent` ou apenas runtime interno?
- Quais criterios de aceitacao validam a melhoria?

### Direcao recomendada
- Comecar por `030-agent`
- Amendar spec existente se o contrato ja existir parcialmente

## Exemplo 2: pedido vago sobre memory
### Pedido inicial
"quero salvar melhor a memoria"

### Refinamento esperado
- O problema e semantica de `SessionID`, estrategia de persistencia ou isolamento?
- O store atual falha em `Load`, `Save` ou concorrencia?
- A mudanca altera contrato publico de `Store`?
- Quais cenarios de sucesso, ausencia de sessao e falha devem ser cobertos?

### Direcao recomendada
- Comecar por `050-memory`
- Amendar spec existente se continuar sendo o mesmo contrato de memoria

## Exemplo 3: pedido vago sobre workflows
### Pedido inicial
"quero workflows mais flexiveis"

### Refinamento esperado
- Flexiveis em branching, retry, suspend/resume ou history?
- A mudanca afeta `Builder`, `Workflow`, `State`, `StepResult` ou hooks?
- O comportamento e novo dominio ou extensao do contrato atual?
- Como validar por testes a nova flexibilidade?

### Direcao recomendada
- Comecar por `070-workflows`
- Criar nova spec apenas se surgir um dominio separado do contrato atual

## Bundle de saida esperado
Depois da spec fechada, entregue:
1. leitura arquitetural
2. spec final
3. prompt Cursor
4. prompt Codex
5. checklist de PR
