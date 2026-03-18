# Spec 140: Demo V2 Guardrails

## 1. Objetivo

Criar uma trilha de demo que prove guardrails de input, output e streaming no runtime da `gaal-lib`.

---

## 2. Escopo

### Dentro do escopo
- um input guardrail simples
- um output guardrail simples
- um stream guardrail simples
- pelo menos um caso de bloqueio ou modificação
- teste cobrindo comportamento observável
- documentação

### Fora do escopo
- políticas complexas
- moderação externa
- regras dinâmicas avançadas
- integrações hosted

---

## 3. Requisitos funcionais

### RF-01
Deve existir input guardrail demonstrável.

### RF-02
Deve existir output guardrail demonstrável.

### RF-03
Deve existir stream guardrail demonstrável.

### RF-04
Ao menos um guardrail deve bloquear ou modificar a execução.

### RF-05
Deve haver teste cobrindo pelo menos um caso feliz e um caso de intervenção.

---

## 4. Critérios de aceitação

1. os 3 tipos de guardrail existem na demo
2. há evidência observável de intervenção
3. há teste
4. a documentação explica como reproduzir

---

## 5. Pergunta principal

"A demo já prova guardrails como comportamento real do runtime da `gaal-lib`?"