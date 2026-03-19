# Spec 150: Demo Web UI

## 1. Objetivo

Criar uma demo web/UI simples para a `gaal-lib`, em cima da demo backend já validada, para aproximar a experiência de uso de um starter app.

---

## 2. Escopo

### Dentro do escopo
- uma UI mínima
- interação com endpoint textual
- interação com streaming
- setup local simples
- documentação de execução
- sem dependência de plataforma hospedada

### Fora do escopo
- produto frontend completo
- autenticação
- design system complexo
- observabilidade avançada
- multi-tenant

---

## 3. Requisitos funcionais

### RF-01
Deve existir interface mínima para enviar input ao agent.

### RF-02
Deve existir forma de visualizar resposta textual.

### RF-03
Deve existir forma de visualizar streaming.

### RF-04
A UI deve se conectar ao backend demo existente.

---

## 4. Critérios de aceitação

1. a UI sobe localmente
2. a UI fala com a demo backend
3. há fluxo textual
4. há fluxo streaming
5. existe README claro

---

## 5. Pergunta principal

"A `gaal-lib` já oferece uma experiência de demo mais próxima de um starter app?"