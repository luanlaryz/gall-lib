# Spec 120: Demo V2 Tools

## 1. Objetivo

Criar a primeira expansão da demo para provar a trilha de tools da `gaal-lib`, mantendo o fluxo base da v1 e adicionando evidência executável de uso de tools em runtime real.

---

## 2. Motivação

A demo v1 já prova:
- app
- agent
- server
- logger
- memory
- streaming

A próxima trilha com melhor custo-benefício para paridade é tools, porque ela aproxima a demo do uso real de frameworks de agents e fornece nova evidência de integração do runtime.

---

## 3. Escopo

### Dentro do escopo
- pelo menos 2 tools reais na demo
- registro das tools no agent ou app, conforme arquitetura existente
- pelo menos um fluxo em que o agent usa tool
- resposta observável indicando uso de tool
- erros básicos de tool observáveis
- smoke ou integração mínima para tool path
- atualização do README da demo

### Fora do escopo
- marketplace de tools
- toolkits complexos
- tools remotas
- autenticação
- workflow multi-step complexo

---

## 4. Requisitos funcionais

### RF-01
A demo deve incluir ao menos 2 tools simples e determinísticas.

Exemplos aceitáveis:
- `get_time`
- `lookup_customer_tier`
- `echo_context`
- `calculate_sum`

### RF-02
Deve existir pelo menos um caso demonstrável em que o agent use uma tool.

### RF-03
A evidência de uso da tool deve ser observável por:
- resposta final
- logs
- evento
- ou combinação desses

### RF-04
Deve existir pelo menos um caso de erro observável relacionado à tool.

---

## 5. Critérios de aceitação

1. a demo continua subindo normalmente
2. pelo menos 2 tools existem
3. há ao menos um fluxo feliz com tool call
4. há ao menos um fluxo de erro observável
5. existe teste cobrindo a trilha de tool
6. a documentação explica como acionar o comportamento

---

## 6. Evidência obrigatória

- código das tools
- wiring das tools
- exemplo manual em `demo.http`
- teste
- README atualizado

---

## 7. Pergunta principal

"A demo da `gaal-lib` já prova integração real de tools no runtime?"