# Spec 110: Demo Parity Diagnosis

## 1. Objetivo

Esta spec define como diagnosticar a paridade da demo oficial da `gaal-lib` em relação ao fluxo-alvo que a biblioteca pretende reproduzir.

O foco desta spec nao e validar apenas se a demo "funciona", mas se ela:

- prova o fluxo base da `gaal-lib`
- exercita os modulos principais da biblioteca de forma integrada
- entrega uma experiencia de uso coerente com a proposta do projeto
- se aproxima da experiencia esperada de uma biblioteca inspirada no Voltagent
- deixa explicitos os gaps restantes de runtime, DX e cobertura funcional

Esta spec nao implementa a demo. Ela define como auditar a demo ja existente.

---

## 2. Escopo

### Dentro do escopo
- diagnostico da demo existente
- verificacao de build e ambiente
- verificacao de boot e shutdown
- verificacao de endpoints HTTP expostos
- verificacao de fluxo textual e streaming
- verificacao de memoria in-memory demonstrada pela demo
- verificacao de documentacao e smoke tests
- diagnostico de paridade funcional
- diagnostico de paridade de comportamento
- diagnostico de paridade de DX
- geracao de backlog objetivo de gaps

### Fora do escopo
- auditoria completa de todos os modulos internos da gaal-lib
- paridade com recursos avancados ainda nao demonstrados pela demo
- avaliacao de performance, carga ou benchmarks
- seguranca de producao
- autenticacao, autorizacao e rate limit
- providers reais de LLM
- integracoes hospedadas ou VoltOps

---

## 3. Artefatos de entrada

O diagnostico deve usar como insumos:

- `specs/000-compatibility-target.md`
- `specs/010-feature-matrix.md`
- `specs/020-repository-architecture.md`
- `specs/030-agent.md`
- `specs/031-app-instance.md`
- `specs/050-memory.md`
- `specs/081-server.md`
- `specs/100-demo-app.md`

E os artefatos concretos da demo:

- `cmd/demo-app/main.go`
- `internal/demoapp/*`
- `examples/demo-app/README.md`
- `examples/demo-app/.env.example`
- `examples/demo-app/http/demo.http`
- `test/smoke/demo_app_test.go`

---

## 4. Resultado esperado do diagnostico

Ao final do diagnostico, deve existir um relatorio com:

1. status geral da demo
2. sumario executivo
3. evidencias observadas
4. gaps por severidade
5. classificacao da demo em relacao ao fluxo-alvo
6. proximos passos priorizados

O relatorio recomendado e:

- `docs/reports/demo-parity-report.md`

Opcionalmente, o diagnostico tambem pode produzir:

- `docs/reports/demo-parity-checklist.md`

---

## 5. Perguntas que o diagnostico deve responder

O diagnostico deve responder, de forma objetiva:

1. A demo sobe e encerra de forma previsivel?
2. A demo usa os modulos publicos da `gaal-lib` de forma coerente?
3. A demo prova o caminho base `app + agent + server + logger + memory`?
4. A demo oferece uma experiencia local simples e didatica?
5. A superficie HTTP da demo e suficiente para provar o runtime?
6. A demo prova memoria conversacional in-memory?
7. O fluxo de streaming esta demonstrado de forma utilizavel?
8. A documentacao e suficiente para um usuario rodar a demo?
9. Os smoke tests validam o caminho principal?
10. O que ainda falta para a demo ser considerada uma evidencia forte de paridade com o objetivo da `gaal-lib`?

---

## 6. Dimensoes obrigatorias do diagnostico

### 6.1 Verificabilidade e ambiente

Objetivo:
garantir que a demo possa ser validada de forma reprodutivel.

Checklist obrigatorio:

- verificar a versao de Go declarada no `go.mod`
- verificar se o ambiente de teste possui toolchain compativel
- verificar se o projeto depende de download automatico de toolchain
- verificar se os testes podem ser executados localmente sem dependencia externa inesperada
- registrar qualquer bloqueio de verificabilidade como gap de severidade alta

Regras:

1. Se a demo nao puder ser executada por incompatibilidade de ambiente, isso nao invalida automaticamente a implementacao, mas invalida a verificacao completa.
2. Bloqueios de ambiente devem aparecer no relatorio como "Blockers to validation".
3. O diagnostico deve diferenciar "nao executado" de "falhou".

### 6.2 Paridade funcional da demo

Objetivo:
validar se a demo prova as capacidades minimas esperadas.

Itens obrigatorios:

- boot local com um comando
- shutdown cooperativo
- `GET /healthz`
- `GET /readyz`
- `GET /agents`
- execucao textual sincrona
- streaming
- memoria in-memory por `session_id`
- logger local
- exemplo manual utilizavel

Regras:

1. A demo nao precisa reproduzir cada detalhe de transporte do Voltagent.
2. A demo precisa provar claramente o fluxo base de aplicacao local.
3. Divergencias de naming ou shape HTTP sao aceitaveis se o fluxo funcional equivalente estiver presente.

### 6.3 Paridade comportamental

Objetivo:
validar se a demo se comporta de maneira coerente e previsivel.

Itens obrigatorios:

- probes refletem estado do runtime
- listagem de agents vem do runtime real
- resolucao de agent vem do runtime real
- request invalido gera erro claro
- agent inexistente gera erro claro
- stream preserva ordem observavel
- memoria reaparece com mesmo `session_id`
- reinicio do processo limpa memoria in-process, se documentado

Regras:

1. O diagnostico deve privilegiar comportamento observavel.
2. Nao basta o endpoint existir; ele deve refletir a semantica do runtime.
3. Mudancas aceitaveis por idiomatismo Go devem ser separadas de lacunas reais.

### 6.4 Paridade de DX

Objetivo:
validar se a experiencia da demo e boa o suficiente para servir como porta de entrada da biblioteca.

Itens obrigatorios:

- README explica como subir
- `.env.example` reflete somente configuracoes reais
- `demo.http` cobre o fluxo principal
- os exemplos sao consistentes com os endpoints reais
- a demo usa defaults locais seguros
- a superficie de uso e simples
- as lacunas conhecidas estao documentadas

Regras:

1. A demo deve ser didatica, nao apenas funcional.
2. Se o usuario precisar adivinhar fluxo, configuracao ou protocolo, isso e gap de DX.
3. A qualidade da documentacao conta como criterio de paridade da demo.

### 6.5 Rastreabilidade com as specs

Objetivo:
validar que a demo nao existe isoladamente, mas como prova das specs anteriores.

O diagnostico deve verificar se a demo exerce na pratica:

- `spec 030` Agent
- `spec 031` App
- `spec 050` Memory
- `spec 081` Server
- `spec 100` Demo App

Regras:

1. Cada parte principal da demo deve ser rastreavel para ao menos uma spec.
2. Se houver comportamento implementado sem spec correspondente, isso deve ser marcado.
3. Se houver spec sem evidencia observavel na demo, isso deve ser marcado como cobertura ausente.

---

## 7. Metodo de avaliacao

O diagnostico deve classificar cada area em um destes estados:

- `PASS`
- `PARTIAL`
- `FAIL`
- `BLOCKED`

Definicoes:

### PASS
O item esta implementado, testavel e coerente com a proposta.

### PARTIAL
O item existe, mas com lacunas observaveis ou cobertura incompleta.

### FAIL
O item deveria existir ou funcionar e nao atende ao comportamento esperado.

### BLOCKED
Nao foi possivel verificar por bloqueio de ambiente, toolchain, dependencia ou ausencia de evidencia executavel.

---

## 8. Severidade dos gaps

Cada gap encontrado deve receber severidade:

- `S0` bloqueador de verificacao
- `S1` bloqueador de paridade do fluxo base
- `S2` lacuna relevante, mas nao bloqueante
- `S3` melhoria de DX, naming ou acabamento

Exemplos:

### S0
- impossibilidade de rodar testes por incompatibilidade de ambiente
- demo nao inicializa por falta de dependencia nao documentada

### S1
- sem endpoint de execucao textual
- sem listagem de agents
- sem streaming
- sem evidencias de memoria no fluxo principal

### S2
- smoke test cobre menos do que deveria
- protocolo de stream pouco documentado
- erros poderiam ser mais claros

### S3
- naming divergente, mas funcional
- README pode ser mais didatico
- exemplos de curl incompletos

---

## 9. Checklist obrigatorio do diagnostico

### 9.1 Build e ambiente
- [ ] `go.mod` foi inspecionado
- [ ] versao de Go requerida foi registrada
- [ ] ambiente atual foi comparado com a versao exigida
- [ ] `go test ./...` foi tentado ou justificado como bloqueado
- [ ] bloqueios foram documentados

### 9.2 Boot e lifecycle
- [ ] `cmd/demo-app/main.go` foi inspecionado
- [ ] composicao da demo foi identificada
- [ ] `App.Start()` e usado
- [ ] `App.Shutdown()` e usado
- [ ] sinais do sistema sao tratados
- [ ] base URL e endpoints sao anunciados no boot

### 9.3 Superficie HTTP
- [ ] `GET /healthz` existe
- [ ] `GET /readyz` existe
- [ ] `GET /agents` existe
- [ ] `POST /agents/{name}/runs` existe
- [ ] `POST /agents/{name}/stream` existe
- [ ] erro de metodo invalido existe
- [ ] erro de agent nao encontrado existe
- [ ] erro de request invalido existe

### 9.4 Runtime demonstrado
- [ ] ha um agent da demo
- [ ] ha memoria in-memory por default
- [ ] ha logger local
- [ ] o runtime usa registry real de agents
- [ ] o stream vem de `Agent.Stream`
- [ ] o run sincrono vem de `Agent.Run`

### 9.5 DX e docs
- [ ] README existe
- [ ] `.env.example` existe
- [ ] `demo.http` existe
- [ ] o README bate com os endpoints reais
- [ ] o README documenta lacunas
- [ ] o README descreve smoke manual

### 9.6 Testes
- [ ] existe smoke test automatizado
- [ ] smoke testa boot
- [ ] smoke testa health
- [ ] smoke testa readiness
- [ ] smoke testa listagem
- [ ] smoke testa text run
- [ ] lacunas de teste foram registradas

---

## 10. Criticos obrigatorios para considerar a demo apta

A demo so pode ser considerada "apta como prova do fluxo base" se todos estes itens forem verdadeiros:

1. a demo sobe com configuracao local simples
2. a demo usa `pkg/app` como composition root
3. a demo registra pelo menos um agent real
4. a demo expoe probes e listagem de agents
5. a demo consegue executar request textual
6. a demo consegue executar streaming
7. a demo demonstra memoria conversacional in-memory
8. a demo possui documentacao suficiente para uso local
9. a demo possui ao menos um smoke test automatizado do caminho principal
10. nao existe dependencia de VoltOps ou servico hospedado obrigatorio

---

## 11. Itens que nao impedem aptidao da demo

As seguintes diferencas sao aceitaveis nesta fase e nao devem impedir o status de apta:

- nomes de endpoints diferentes do shape inicialmente sugerido, desde que o fluxo funcional equivalente exista
- uso de modelo fake/deterministico
- uso de helper interno para wiring da demo
- ausencia de UI web
- ausencia de OpenAPI
- ausencia de provider real
- memoria apenas in-process

---

## 12. Itens que impedem aptidao da demo

Os seguintes pontos impedem considerar a demo apta:

- incapacidade de provar boot local
- incapacidade de provar run textual
- incapacidade de provar streaming
- incapacidade de provar listagem de agents
- incapacidade de provar composicao via `pkg/app`
- dependencia obrigatoria de servico externo nao documentado
- documentacao inconsistente com o comportamento real
- smoke tests inexistentes para o caminho principal

---

## 13. Formato obrigatorio do relatorio de saida

O diagnostico deve produzir um relatorio com esta estrutura:

### 1. Executive summary
- status geral
- blockers
- conclusao curta

### 2. Evidence reviewed
- arquivos inspecionados
- testes executados ou bloqueados
- ambiente usado

### 3. Scorecard
Tabela com:
- area
- status
- observacao curta

Areas minimas:
- ambiente
- boot/lifecycle
- http surface
- runtime integration
- memory proof
- streaming proof
- docs/dx
- smoke tests
- parity confidence

### 4. Gaps found
Para cada gap:
- id
- severidade
- descricao
- evidencia
- recomendacao

### 5. Aptness decision
Uma destas decisoes:
- `APT for base demo parity`
- `APT with reservations`
- `NOT YET APT`
- `BLOCKED TO VERIFY`

### 6. Next prioritized actions
Lista curta ordenada por impacto.

---

## 14. Criterios de aceitacao desta spec

Esta spec so pode ser considerada atendida quando:

1. existir um procedimento claro e repetivel de diagnostico
2. o procedimento diferenciar implementacao de verificabilidade
3. o procedimento cobrir funcionalidade, comportamento e DX
4. o procedimento gerar um relatorio objetivo
5. o procedimento conseguir classificar a demo como apta, apta com ressalvas, nao apta ou bloqueada para verificar

---

## 15. Observacao normativa

Esta spec diagnostica a demo como evidencia do fluxo base da `gaal-lib`.

Ela nao substitui:
- a `spec 010` como matriz de cobertura
- os testes de conformidade da biblioteca
- futuras demos mais avancadas
- futuras auditorias de paridade por modulo

Ela existe para responder uma pergunta especifica:

"A demo atual ja prova, com confianca razoavel, que a gaal-lib entrega o fluxo base que se espera de uma biblioteca inspirada no Voltagent?"