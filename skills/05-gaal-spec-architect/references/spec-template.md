# Templates de spec gaal-lib

Use estes formatos como esqueletos minimos, adaptando a numeracao e o titulo ao modulo.

## Template de spec de construcao

```md
# Spec NNN: <titulo do modulo> - construcao

## 1. Objetivo

Este documento define <contrato/modulo/comportamento> em `gaal-lib`, com foco em:

- <objetivo 1>
- <objetivo 2>
- <objetivo 3>

Esta spec complementa a `Spec 000`, a `Spec 010` e a `Spec 020` quando aplicavel.

Ficam fora do escopo deste documento:

- <fora de escopo 1>
- <fora de escopo 2>

## 2. Contexto e encaixe arquitetural

- Modulos afetados: `pkg/...`, `internal/...`, `test/...`, `examples/...`
- Contratos relacionados: `Spec 0xx`, `Spec 0yy`
- Impacto em paridade: <nenhum | baixo | medio | alto>

## 3. Regras normativas

1. <regra observavel 1>
2. <regra observavel 2>
3. <regra observavel 3>

## 4. Contratos e comportamento observavel

- <entrada>
- <saida>
- <erros>
- <cancelamento>
- <efeitos colaterais controlados>

## 5. Riscos e trade-offs

- Risco: <...>
- Trade-off: <...>

## 6. Estrategia de testes

- <teste de sucesso>
- <teste de falha>
- <teste de cancelamento/limite quando aplicavel>

## 7. Criterios de aceitacao

1. <criterio verificavel 1>
2. <criterio verificavel 2>
3. <criterio verificavel 3>
```

## Template de spec de diagnostico

```md
# Spec NNN: <titulo do modulo> - diagnostico

## 1. Objetivo

Este documento define como observar, depurar e confirmar o comportamento de <contrato/modulo/comportamento> em `gaal-lib`, com foco em:

- <sinais de saude>
- <sinais de regressao>
- <confirmacao diagnostica>

Ficam fora do escopo deste documento:

- <investigacoes nao cobertas>
- <ferramentas externas fora do runtime, se aplicavel>

## 2. Contexto e encaixe arquitetural

- Modulos afetados: `pkg/...`, `internal/...`, `test/...`, `examples/...`
- Contratos relacionados: `Spec 0xx`, `Spec 0yy`
- Dependencias operacionais: <logs | metrics | traces | health checks | consultas>

## 3. Sinais observaveis e modos de falha

1. <sinal saudavel 1>
2. <modo de falha 1>
3. <modo de falha 2>

## 4. Fontes diagnosticas

- Logs: <campos, eventos, correlacao>
- Metricas: <nome, dimensao, interpretacao>
- Traces: <spans, atributos, correlacao>
- Health checks ou consultas: <comandos, endpoints, invariantes>

## 5. Procedimento de troubleshooting

1. <passo 1>
2. <passo 2>
3. <passo 3>

## 6. Riscos e trade-offs

- Risco: <...>
- Trade-off: <...>

## 7. Criterios de confirmacao diagnostica

1. <criterio verificavel 1>
2. <criterio verificavel 2>
3. <criterio verificavel 3>
```

## Regras de estilo
- usar linguagem normativa e observavel
- explicitar o que muda e o que nao muda
- evitar palavras vagas sem criterio verificavel
- manter rastreabilidade com specs relacionadas
- tratar construcao e diagnostico como specs irmas da mesma trilha
