# Spec 020: Repository Architecture

## 1. Objetivo

Este documento define a arquitetura de repositorio da `gaal-lib` para:

- separar contratos publicos estaveis do runtime interno
- reduzir acoplamento entre capacidades centrais
- preparar a base para testes de conformidade, examples executaveis e extensibilidade futura
- tornar explicitas as regras de evolucao e compatibilidade do modulo

Esta spec complementa a `Spec 000` e a `Spec 010`. Ela nao define comportamento de uma feature isolada; ela define a forma como o repositorio deve ser organizado para que as features sejam implementadas sem erosao arquitetural.

## 2. Principios arquiteturais

Os principios abaixo sao normativos para a organizacao do repositorio:

1. API publica pequena e intencional. Apenas contratos, tipos e pontos de extensao que precisem ser importados por usuarios devem viver em `pkg/`.
2. Runtime escondido. Logica de orquestracao, execucao, pipeline interno e detalhes de estado devem viver em `internal/`.
3. Dependencias aciclicas. A arvore de pacotes deve formar camadas claras e sem ciclos.
4. Contratos antes de implementacoes. Interfaces, structs de configuracao, requests, responses e erros observaveis devem ser definidos nas camadas publicas antes da concretizacao do runtime.
5. `context.Context` como fronteira padrao. Cancelamento, timeout e lifecycle devem atravessar a API publica de forma idiomatica.
6. Observabilidade sem vazamento. Eventos internos podem existir para runtime, historico e instrumentacao, mas nao devem aparecer em assinaturas exportadas sem promocao explicita para uma API publica estavel.
7. Extensibilidade por composicao. Tools, memory, guardrails e workflows devem expor contratos estaveis para extensao sem exigir importacao de pacotes internos.
8. Testabilidade por camada. Cada camada deve poder ser validada com testes coerentes com sua responsabilidade.
9. Root package minimo. O pacote raiz `gaal` deve continuar pequeno, reservado para documentacao do modulo e, no maximo, metadados globais estaveis.
10. Compatibilidade observavel acima de detalhe estrutural. O que importa para estabilidade e o contrato importavel e o comportamento observavel, nao o desenho interno do runtime.

## 3. Organizacao de pacotes

### 3.1 Arvore sugerida

```text
.
├── doc.go
├── go.mod
├── pkg
│   ├── agent
│   ├── app
│   ├── guardrail
│   ├── logger
│   ├── memory
│   ├── server
│   ├── tool
│   ├── types
│   └── workflow
├── internal
│   ├── events
│   ├── runtime
│   │   ├── agent
│   │   ├── lifecycle
│   │   ├── memory
│   │   ├── registry
│   │   ├── tool
│   │   └── workflow
│   └── validation
├── test
│   ├── conformance
│   │   ├── agent
│   │   ├── app
│   │   ├── guardrail
│   │   ├── memory
│   │   ├── tool
│   │   └── workflow
│   └── fixtures
│       ├── cases
│       └── golden
├── examples
│   ├── basic-agent
│   ├── custom-memory
│   ├── http-server
│   └── workflow-chain
└── specs
```

### 3.2 Papel de cada pacote

| Caminho | Papel | Natureza |
| --- | --- | --- |
| `pkg/types` | Tipos compartilhados de baixo nivel: ids, metadata, envelopes, erros observaveis e contratos pequenos usados por mais de um pacote publico. Nao deve virar deposito generico. | Publico estavel |
| `pkg/logger` | Fachada de logging e adaptadores sobre `log/slog`, incluindo noop/test logger quando necessario. | Publico estavel |
| `pkg/tool` | Contratos de tool, schema de input/output, helpers de registro e composicao de toolkits. | Publico estavel |
| `pkg/memory` | Contratos de memoria, working memory e persistencia conversacional plugavel. | Publico estavel |
| `pkg/guardrail` | Contratos de validacao e decisao antes/depois da execucao, com resultados observaveis de permitir, bloquear, transformar ou falhar. | Publico estavel |
| `pkg/workflow` | Definicao publica de workflows, steps, branching, retry, hooks e historico observavel. | Publico estavel |
| `pkg/agent` | Definicao publica de agentes, requests/responses, configuracao e registries relacionados ao dominio de agente. | Publico estavel |
| `pkg/app` | Composition root publica: builder da aplicacao, wiring de registries, lifecycle, startup e integracao controlada com o runtime interno. | Publico estavel |
| `pkg/server` | Adaptadores publicos para expor uma `app` por transporte sem contaminar os pacotes centrais com detalhes HTTP ou streaming. | Publico estavel, opcional |
| `internal/runtime` | Implementacao concreta do motor de execucao, roteamento, pipelines, registries internos, state transitions e lifecycle operacional. | Interno |
| `internal/events` | Modelo interno de eventos e envelopes usados pelo runtime para historico, hooks e observabilidade local. | Interno |
| `internal/validation` | Validacoes, normalizacao de configuracao e garantias de consistencia antes da execucao. | Interno |
| `test/conformance` | Suites que validam paridade comportamental via API publica usando fixtures e cenarios de referencia. | Teste |
| `test/fixtures` | Dados de entrada, saida esperada, cenarios e golden files usados por suites de conformidade. | Teste |
| `examples` | Programas minimos e executaveis que demonstram o uso da API publica e servem como smoke tests/documentacao viva. | Suporte |
| `specs` | Documentacao normativa de arquitetura, escopo e comportamento esperado. | Documentacao |

### 3.3 Papel das subcamadas internas

- `internal/runtime/agent`: execucao concreta de agentes e handoffs internos.
- `internal/runtime/tool`: pipeline de invocacao de tools, coercao interna e tratamento de falhas.
- `internal/runtime/workflow`: motor de steps, branching, retries e historico.
- `internal/runtime/memory`: adaptacao entre contratos publicos de memoria e armazenamentos concretos.
- `internal/runtime/registry`: registries internos, lookup otimizado e wiring de componentes.
- `internal/runtime/lifecycle`: startup, shutdown, cleanup e coordenacao com `context.Context`.

## 4. Regras de dependencia entre pacotes

### 4.1 Camadas

As camadas do repositorio devem seguir a direcao abaixo:

1. Base publica: `pkg/types`, `pkg/logger`
2. Contratos publicos de extensao: `pkg/tool`, `pkg/memory`, `pkg/guardrail`
3. Contratos publicos de orquestracao: `pkg/workflow`, `pkg/agent`
4. Composicao publica: `pkg/app`
5. Adaptadores publicos: `pkg/server`
6. Implementacao interna: `internal/validation`, `internal/events`, `internal/runtime`
7. Verificacao e suporte: `test/*`, `examples`, `specs`

### 4.2 Regras normativas

As regras abaixo devem ser seguidas ao criar ou revisar imports:

1. `pkg/types` e `pkg/logger` so podem depender de stdlib ou dependencias externas estritamente fundacionais e estaveis.
2. `pkg/tool`, `pkg/memory` e `pkg/guardrail` podem depender de `pkg/types` e `pkg/logger`, mas nao de `pkg/app`, `pkg/server` ou `internal/*`.
3. `pkg/workflow` pode depender de `pkg/types`, `pkg/logger`, `pkg/tool`, `pkg/memory` e `pkg/guardrail`.
4. `pkg/agent` pode depender de `pkg/types`, `pkg/logger`, `pkg/tool`, `pkg/memory` e `pkg/guardrail`; deve evitar dependencia direta de `pkg/workflow` para nao misturar dominios centrais.
5. `pkg/app` e o unico pacote publico autorizado a importar `internal/runtime` e `internal/validation`.
6. `pkg/server` deve depender de `pkg/app` e outros contratos publicos, nunca de `internal/*`.
7. `internal/runtime` pode depender de qualquer `pkg/*`, alem de `internal/events` e `internal/validation`.
8. `internal/events` nao deve depender de `pkg/app` ou `pkg/server`; se um evento precisar ser observavel publicamente, ele deve ser promovido para um contrato estavel em `pkg/*`.
9. `internal/validation` nao deve depender de `internal/runtime`; validacao precisa permanecer reutilizavel e sem acoplamento ao motor.
10. `examples` so podem depender de `pkg/*`.
11. `test/conformance` deve exercitar a biblioteca via `pkg/*`, nunca via `internal/*`.
12. `test/fixtures` nao pode ser importado por `pkg/*` nem por `internal/*`.
13. `specs` e `examples` nunca sao dependencias de `pkg/*`.
14. Nenhum pacote em `pkg/*` pode depender de `pkg/server`.
15. Nenhum pacote em `pkg/*` pode depender de `examples`, `test/*` ou `specs`.
16. Qualquer necessidade de importar `internal/*` fora de `pkg/app` indica erro de fronteira arquitetural e deve ser corrigida.

### 4.3 Matriz resumida de importacao

| Origem | Pode importar |
| --- | --- |
| `pkg/types` | stdlib |
| `pkg/logger` | stdlib, dependencias de logging aprovadas |
| `pkg/tool` | `pkg/types`, `pkg/logger` |
| `pkg/memory` | `pkg/types`, `pkg/logger` |
| `pkg/guardrail` | `pkg/types`, `pkg/logger` |
| `pkg/workflow` | `pkg/types`, `pkg/logger`, `pkg/tool`, `pkg/memory`, `pkg/guardrail` |
| `pkg/agent` | `pkg/types`, `pkg/logger`, `pkg/tool`, `pkg/memory`, `pkg/guardrail` |
| `pkg/app` | qualquer `pkg/*`, `internal/runtime`, `internal/validation` |
| `pkg/server` | `pkg/app`, `pkg/types`, `pkg/logger` |
| `internal/events` | `pkg/types`, `pkg/logger`, stdlib |
| `internal/validation` | `pkg/*`, stdlib |
| `internal/runtime` | qualquer `pkg/*`, `internal/events`, `internal/validation` |
| `examples` | qualquer `pkg/*` |
| `test/conformance` | qualquer `pkg/*`, `test/fixtures` |

## 5. O que pode ser publico vs interno

### 5.1 Deve ser publico

Devem viver em `pkg/`:

- interfaces e contratos que o usuario implementa
- tipos de request/response que atravessam a API
- erros e resultados observaveis
- builders, registries e configuracoes necessarias para compor a biblioteca
- hooks e pontos de extensao deliberadamente suportados
- adaptadores que o usuario precisa importar para expor a biblioteca em outro contexto

### 5.2 Deve ser interno

Devem viver em `internal/`:

- detalhes do motor de execucao
- state machines, steps intermediarios e pipeline de runtime
- envelopes de eventos usados apenas para coordenacao interna
- caches, validacoes, defaults e normalizacao sem contrato publico
- helpers de wiring e registries que nao precisem ser estendidos pelo usuario
- estruturas criadas apenas para performance, concorrencia ou simplificacao interna

### 5.3 Regras de fronteira

1. `internal/*` nao pode aparecer em exemplos de uso da biblioteca.
2. Tipos de `internal/*` nao podem ser expostos em assinaturas exportadas, campos exportados ou erros retornados por `pkg/*`.
3. Se um conceito interno precisar ser customizado por usuarios, ele deve ser promovido para um contrato publico em `pkg/*` antes de qualquer extensao adicional.
4. Tipos compartilhados entre mais de um pacote publico devem ir para `pkg/types` apenas quando forem realmente transversais e estaveis.
5. Tipos usados por apenas um pacote publico devem permanecer no proprio pacote, nao em `pkg/types`.
6. O pacote raiz `gaal` nao deve virar umbrella API; imports publicos devem continuar explicitos em `pkg/...`.

## 6. Estrategia de versionamento e compatibilidade

### 6.1 Unidade de versionamento

O repositorio deve seguir versionamento semantico no nivel do modulo Go.

- `MAJOR`: quebra de compatibilidade em `pkg/*` ou mudanca observavel nao retrocompativel em comportamento documentado.
- `MINOR`: adicao retrocompativel de capacidades, tipos, funcoes, pacotes publicos ou pontos de extensao.
- `PATCH`: correcao de bugs e ajustes internos sem quebra de contrato publico.

### 6.2 Escopo da garantia de compatibilidade

A garantia de compatibilidade cobre:

- identificadores exportados em `pkg/*`
- comportamento observavel documentado em specs aprovadas
- formatos e semantica de erros publicos
- ordem de execucao relevante quando ela fizer parte do contrato observavel
- examples oficiais e suites de conformidade que representem cenarios suportados

A garantia de compatibilidade nao cobre:

- qualquer pacote em `internal/*`
- organizacao interna do runtime
- utilitarios de `test/*`
- fixtures auxiliares que nao representem contrato publico
- documentos em `specs` que ainda nao tenham sido convertidos em comportamento implementado

### 6.3 Politica para fase pre-v1

Antes de `v1.0.0`, mudancas breaking em `pkg/*` ainda podem ocorrer, mas somente quando:

1. a spec correspondente for atualizada
2. a mudanca vier acompanhada de ajuste nas suites de conformidade e examples afetados
3. a divergencia estiver claramente registrada no changelog ou release note

### 6.4 Promocao de contratos

Qualquer artefato que hoje exista em `internal/*` e precise de garantia de estabilidade deve seguir este fluxo:

1. promover o conceito para `pkg/*`
2. documentar o novo contrato em spec
3. adicionar teste de conformidade ou contrato publico correspondente
4. so depois simplificar ou trocar a implementacao interna

## 7. Estrategia de testes por camada

### 7.1 `pkg/*`

Objetivo: validar contratos publicos, ergonomia e comportamento local de cada pacote.

- testes unitarios por pacote em `pkg/<nome>/*_test.go`
- foco em construtores, validacao de configuracao, erros publicos e comportamento documentado
- exemplos de package doc e exemplos executaveis devem compilar sem importar `internal/*`

### 7.2 `internal/*`

Objetivo: validar corretude do motor e detalhes de execucao.

- testes unitarios deterministas em `internal/.../*_test.go`
- foco em concorrencia, ordering, retries, lifecycle, pipelines e normalizacao
- mocks ou fakes devem consumir contratos de `pkg/*`, nao recriar contratos paralelos

### 7.3 `test/conformance`

Objetivo: validar paridade conceitual e comportamental a partir da superficie publica.

- testes devem usar `pkg/app` e demais `pkg/*`
- fixtures em `test/fixtures` devem descrever entradas, eventos observaveis, erros e saidas esperadas
- cenarios devem cobrir sucesso, falha, cancelamento e ordem relevante de operacoes
- suites de conformidade sao o principal gate de compatibilidade antes de release

### 7.4 `examples`

Objetivo: servir como documentacao viva e smoke test de integracao.

- cada exemplo deve demonstrar um caminho de uso real da API publica
- exemplos devem ser pequenos, executaveis e sem infraestrutura externa obrigatoria
- examples oficiais devem ser cobertos por `go test` ou por smoke test equivalente

### 7.5 Estrategia por camada funcional

| Camada | Tipo principal de teste | Resultado esperado |
| --- | --- | --- |
| `pkg/types`, `pkg/logger` | unitario | contratos basicos e estabilidade de tipos |
| `pkg/tool`, `pkg/memory`, `pkg/guardrail` | unitario + contrato | extensibilidade e erros observaveis |
| `pkg/workflow`, `pkg/agent` | unitario + contrato | definicao publica coerente e previsivel |
| `pkg/app` | integracao local | wiring correto com runtime interno |
| `pkg/server` | integracao local | adaptacao de transporte sem contaminar o core |
| `internal/runtime` | unitario determinista | execucao correta e acoplamento interno sob controle |
| `test/conformance` | conformidade | paridade comportamental do modulo |
| `examples` | smoke test | usabilidade minima e compilacao real da API |

## 8. Regras curtas para enforcement

As regras abaixo devem ser usadas como checklist rapido em PRs:

- `pkg` nunca depende de `examples`, `test` ou `specs`
- `pkg/server` nunca sobe dependencia para o core
- `pkg/app` e a unica ponte publica para `internal/runtime`
- `internal` nao vaza para API publica
- `test/conformance` valida comportamento via API publica, nao via runtime interno
- `examples` mostram apenas imports publicos
- `pkg/types` nao vira pacote deposito
- contratos publicos sobem antes de implementacoes internas
- mudanca breaking em `pkg/*` exige revisao de spec e teste de conformidade

## 9. Decisao

Fica adotada a organizacao com `pkg/` como superficie publica estavel e `internal/` como implementacao privada do repositorio, com `pkg/app` como unica ponte oficial entre contratos publicos e runtime interno. Esta arquitetura deve ser usada como base para as proximas specs e para a implementacao incremental da `gaal-lib`.
