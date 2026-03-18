# Template de spec gaal-lib

Use este formato como esqueleto minimo, adaptando a numeracao e o titulo ao modulo.

```md
# Spec NNN: <titulo do modulo>

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

## Regras de estilo
- usar linguagem normativa e observavel
- explicitar o que muda e o que nao muda
- evitar palavras vagas sem criterio verificavel
- manter rastreabilidade com specs relacionadas
