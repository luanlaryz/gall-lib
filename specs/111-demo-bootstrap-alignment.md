# Spec 111: Demo Bootstrap Alignment

## 1. Objetivo

Esta spec corrige o desalinhamento arquitetural identificado pelo diagnostico de paridade da demo (DPG-001): o binario em `cmd/demo-app/main.go` importava `internal/demoapp`, violando a regra explicita da `Spec 100` (secao 3, item 3) e a regra de fronteira da `Spec 020` (secao 4.2, item 16).

O objetivo e estritamente corretivo. Nenhuma feature nova e introduzida, nenhuma API publica e criada e nenhum endpoint HTTP e alterado.

Esta spec complementa a `Spec 020` e a `Spec 100`.

---

## 2. Escopo

### Dentro do escopo

- eliminar a importacao de `internal/demoapp` a partir de `cmd/demo-app/main.go`
- reorganizar o wiring da demo para que todo codigo de composicao viva no pacote `main` de `cmd/demo-app/`
- preservar o comportamento atual da demo e seus endpoints
- manter a suite de testes verde

### Fora do escopo

- criacao de API publica nova em `pkg/*`
- alteracao do contrato HTTP existente
- adicao de features, endpoints ou capacidades
- refatoracao de outros modulos

---

## 3. Problema identificado

### DPG-001

- Severidade: `S2`
- Origem: diagnostico de paridade da demo (`Spec 110`)
- Descricao: `cmd/demo-app/main.go` importava o pacote `internal/demoapp` para obter o wiring da demo (config, agent factory, HTTP server). Isso viola duas regras normativas:
  - `Spec 100`, secao 3, item 3: "A demo nao pode importar `internal/*` a partir do binario em `cmd/demo-app`."
  - `Spec 020`, secao 4.2, item 16: "Qualquer necessidade de importar `internal/*` fora de `pkg/app` indica erro de fronteira arquitetural e deve ser corrigida."

---

## 4. Decisao arquitetural

### 4.1 Abordagem adotada: inlining no pacote `main`

O codigo de composicao da demo foi movido de `internal/demoapp/` para arquivos locais no pacote `main` em `cmd/demo-app/`, organizados por responsabilidade:

| Arquivo | Responsabilidade |
| --- | --- |
| `cmd/demo-app/config.go` | leitura de variaveis de ambiente e defaults |
| `cmd/demo-app/agent.go` | factory do agent demo e modelo deterministico local |
| `cmd/demo-app/server.go` | adapter HTTP, handlers, probes, routing e SSE |
| `cmd/demo-app/main.go` | composicao final e lifecycle |

O diretorio `internal/demoapp/` foi removido.

### 4.2 Alternativas descartadas

1. **Criar API publica nova em `pkg/*`**: descartada porque o wiring da demo e especifico do binario demo e nao justifica superficie publica adicional. A `Spec 100` (secao 3, item 4) orienta que compartilhamento de wiring deve ocorrer "por helper interno pequeno e nao por nova API publica", mas como o binario e o unico consumidor, o inlining e mais simples.

2. **Manter em `internal/demoapp/` e ajustar a regra**: descartada porque a regra da `Spec 020` e clara e intencional. A demo deve demonstrar uso exclusivo de `pkg/*`.

### 4.3 Trade-off aceito

Se futuramente outro binario ou teste precisar do mesmo wiring de composicao, havera duplicacao. Isso e preferivel a criar API publica desnecessaria ou reintroduzir import de `internal/*` a partir de `cmd/`.

---

## 5. Rastreabilidade

| Spec | Secao | Regra satisfeita |
| --- | --- | --- |
| `Spec 020` | 4.2, item 16 | nenhum pacote em `cmd/` importa `internal/*` |
| `Spec 020` | 4.3, linha `examples` | `cmd/demo-app` usa apenas imports de `pkg/*` |
| `Spec 100` | 3, item 3 | a demo nao importa `internal/*` a partir do binario |
| `Spec 100` | 3, item 1 | a demo usa `pkg/app` como composition root |
| `Spec 100` | 3, item 4 | nenhuma API publica nova foi criada |
| `Spec 110` | gap DPG-001 | gap resolvido |

---

## 6. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `cmd/demo-app/main.go` nao importar nenhum pacote de `internal/*`.
2. O diretorio `internal/demoapp/` nao existir.
3. O wiring da demo residir inteiramente no pacote `main` de `cmd/demo-app/`.
4. O comportamento da demo permanecer inalterado: mesmos endpoints, mesmos payloads, mesma semantica.
5. `go test ./...` continuar passando, incluindo os smoke tests em `test/smoke/`.
6. Nenhuma API publica nova ter sido criada em `pkg/*`.
7. O relatorio de paridade (`docs/reports/demo-parity-report.md`) registrar DPG-001 como resolvido.
