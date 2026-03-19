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
- Quais sinais, logs ou traces distinguem stream saudavel de stream regressivo?

### Direcao recomendada
- Comecar por `030-agent`
- Amendar specs existentes se o contrato ja existir parcialmente
- Exigir spec de construcao e spec de diagnostico antes de gerar prompt de execucao

## Exemplo 2: pedido vago sobre memory
### Pedido inicial
"quero salvar melhor a memoria"

### Refinamento esperado
- O problema e semantica de `SessionID`, estrategia de persistencia ou isolamento?
- O store atual falha em `Load`, `Save` ou concorrencia?
- A mudanca altera contrato publico de `Store`?
- Quais cenarios de sucesso, ausencia de sessao e falha devem ser cobertos?
- Como diagnosticar corrupcao, ausencia ou isolamento incorreto de memoria?

### Direcao recomendada
- Comecar por `050-memory`
- Amendar specs existentes se continuar sendo o mesmo contrato de memoria
- Exigir sinais diagnosticos para confirmar isolamento, persistencia e falhas de store

## Exemplo 3: pedido vago sobre workflows
### Pedido inicial
"quero workflows mais flexiveis"

### Refinamento esperado
- Flexiveis em branching, retry, suspend/resume ou history?
- A mudanca afeta `Builder`, `Workflow`, `State`, `StepResult` ou hooks?
- O comportamento e novo dominio ou extensao do contrato atual?
- Como validar por testes a nova flexibilidade?
- Como diagnosticar loops incorretos, retries excessivos ou history inconsistente?

### Direcao recomendada
- Comecar por `070-workflows`
- Criar nova trilha dual-spec apenas se surgir um dominio separado do contrato atual
- Exigir spec de diagnostico para sinais de runtime, retries e consistencia de history

## Bundle de saida esperado
Depois das specs fechadas, entregue:
1. leitura arquitetural
2. spec de construcao
3. spec de diagnostico
4. prompt Cursor
5. prompt Codex
6. checklist de PR
