# Spec 130: Demo V2 Workflow

## 1. Objetivo

Criar uma trilha de demo que prove a capacidade de workflows da `gaal-lib` em cenário executável, com steps sequenciais, decisão simples e evidência de execução.

---

## 2. Escopo

### Dentro do escopo
- um workflow de exemplo
- pelo menos 2 steps sequenciais
- pelo menos 1 decisão condicional simples
- retry ou tratamento de falha simples, se já existir na lib
- observabilidade local mínima
- teste de integração
- documentação

### Fora do escopo
- workflow visual
- branch altamente complexa
- persistência distribuída
- resume cross-process sofisticado

---

## 3. Requisitos funcionais

### RF-01
Deve existir um workflow real registrado no app.

### RF-02
O workflow deve ter ao menos 2 steps.

### RF-03
Deve existir ao menos uma ramificação simples.

### RF-04
A execução deve ser observável por logs, resultado ou estado.

### RF-05
Deve haver teste cobrindo o caminho principal.

---

## 4. Critérios de aceitação

1. workflow existe e é registrável
2. a demo prova execução do workflow
3. há evidência de steps e branching
4. há teste
5. a documentação explica como acionar o workflow

---

## 5. Pergunta principal

"A `gaal-lib` já prova, por demo, uma trilha real de workflows?"