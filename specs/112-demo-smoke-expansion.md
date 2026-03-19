# Spec 112: Demo Smoke Expansion

## 1. Objetivo

Expandir a suíte de smoke tests da demo para cobrir os comportamentos HTTP observáveis que hoje ainda dependem de validação manual.

Esta spec existe para corrigir o gap `DPG-002`, aumentando a confiança do diagnóstico da demo sem mudar o escopo funcional do produto.

---

## 2. Contexto

O relatório atual concluiu que:

- a demo já cobre o caminho principal feliz
- a demo já prova boot, health, readiness, listagem e run textual
- porém, streaming SSE e erros HTTP relevantes ainda não estão automatizados

A meta desta etapa é transformar essas evidências manuais em evidências automatizadas.

---

## 3. Comportamentos que precisam ser automatizados

A suíte de smoke deve passar a cobrir, no mínimo:

1. streaming SSE em `/agents/{name}/stream`
2. erro `404` para agent inexistente
3. erro `400` para request inválido
4. erro `405` para método inválido

---

## 4. Resultado esperado

Ao final desta spec:

- a suíte automatizada deve cobrir o caminho feliz e os principais erros observáveis
- a demo deve continuar simples
- os testes devem continuar rápidos e legíveis
- o relatório de paridade poderá reduzir ou eliminar a reserva sobre cobertura insuficiente

---

## 5. Requisitos funcionais

### RF-01
O smoke test deve verificar que o endpoint SSE:
- responde com status esperado
- emite eventos em ordem observável
- termina corretamente

### RF-02
O smoke test deve verificar `404` para um agent que não existe.

### RF-03
O smoke test deve verificar `400` para payload inválido ou `session_id` inválido, conforme o contrato atual da demo.

### RF-04
O smoke test deve verificar `405` para uso de método não permitido nas rotas principais.

---

## 6. Requisitos não funcionais

### RNF-01
Os testes devem permanecer previsíveis e rápidos.

### RNF-02
A validação de SSE não deve depender de timing frágil desnecessário.

### RNF-03
Os testes devem ser legíveis o suficiente para servir de documentação executável.

---

## 7. Critérios de aceitação

Esta spec será considerada concluída quando:

1. existir cobertura automatizada de streaming SSE
2. existir cobertura automatizada de `404`
3. existir cobertura automatizada de `400`
4. existir cobertura automatizada de `405`
5. `go test ./...` continuar passando
6. os testes forem estáveis localmente
7. a suíte continuar legível e curta

---

## 8. Evidência obrigatória

A implementação deve deixar evidência em:

- `test/smoke/demo_app_test.go` ou suíte complementar
- logs de execução bem-sucedida dos testes
- observação breve sobre como o SSE foi validado

---

## 9. Fora do escopo

- testes de carga
- testes de performance
- cobertura exaustiva de todas as combinações de erro
- mocking pesado do runtime

---

## 10. Pergunta principal que esta spec responde

"Os comportamentos principais e os principais erros observáveis da demo estão protegidos por automação?"