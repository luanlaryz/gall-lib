# Spec 113: Demo Memory Restart Proof

## 1. Objetivo

Provar de forma automatizada, ou justificar formalmente de forma documental, que a memória da demo é apenas in-process e não persiste após reinício.

Esta spec existe para fechar o gap `DPG-003`, reduzindo a confiança documental e aumentando a confiança verificável sobre a semântica da memória da demo.

---

## 2. Contexto

O diagnóstico atual concluiu que:

- a demo usa `memory.InMemoryStore` por padrão
- a memória reaparece com o mesmo `session_id` dentro do mesmo processo
- porém, a perda do estado após reinício está apenas documentada, não automatizada

---

## 3. Problema a resolver

Hoje sabemos, pela documentação, que a memória não persiste entre reinícios.

Mas a evidência dessa semântica ainda não está automatizada.

Isso deixa a confiança nessa parte do comportamento menor do que o restante da demo.

---

## 4. Resultado esperado

Ao final desta spec, deve existir uma destas saídas aceitáveis:

### Saída preferencial
Um teste automatizado que:
- sobe a demo
- cria estado de memória
- encerra o processo
- sobe novamente
- prova que o estado anterior não reaparece

### Saída alternativa aceitável
Caso a automação seja excessivamente complexa para v1, deve existir:
- uma justificativa explícita documentada
- classificação formal dessa verificação como manual/documental
- atualização coerente do relatório/checklist

---

## 5. Requisitos funcionais

### RF-01
A solução adotada deve provar a semântica real da memória da demo.

### RF-02
A prova deve ser coerente com o README e com o comportamento atual.

### RF-03
Se a prova for automatizada, ela deve ser estável o suficiente para rodar em CI.

---

## 6. Critérios de aceitação

Esta spec será considerada concluída quando:

1. existir teste automatizado de reinício, ou justificativa formal para não automatizar
2. a semântica de memória in-process ficar inequivocamente demonstrada
3. README, relatório e comportamento real permanecerem consistentes
4. a decisão tomada ficar registrada

---

## 7. Evidência obrigatória

Uma destas evidências deve existir:

### Caminho A
- teste de integração de reinício
- logs/resultados do teste

### Caminho B
- documento/ADR curto justificando a validação manual
- atualização do relatório/checklist
- explicitação do trade-off

---

## 8. Fora do escopo

- persistência real
- banco de dados
- adapters de memória persistente
- benchmark de memória

---

## 9. Pergunta principal que esta spec responde

"A demo prova de forma confiável que sua memória é apenas in-process e não persiste entre reinícios?"