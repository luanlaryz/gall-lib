# Spec 200: Library Parity Diagnosis for Agent Orchestration

## 1. Objetivo

Definir um diagnóstico sistemático para validar se a `gaal-lib`:

1. está funcional como biblioteca executável e integrável
2. cobre os recursos essenciais do Voltagent para orquestração de agentes
3. oferece evidências verificáveis por código, testes, exemplos e demos
4. exclui corretamente recursos pertencentes a VoltOps

Esta spec não implementa novos recursos. Ela define como auditar a biblioteca existente.

---

## 2. Pergunta principal

"A `gaal-lib` já está funcional e cobre, com confiança razoável, os recursos do Voltagent necessários para construir e orquestrar agentes, excluindo VoltOps?"

---

## 3. Escopo

### Dentro do escopo
- funcionalidade da biblioteca
- APIs públicas relevantes
- integração entre módulos
- exemplos e demos
- testes automatizados
- paridade funcional de orquestração
- paridade comportamental observável
- paridade de DX básica para uso da biblioteca
- uso direto por código
- uso via server/http quando aplicável
- coordenação entre agents quando suportada
- workflows e runtime de execução
- memory, tools, guardrails e streaming

### Fora do escopo
- VoltOps observability
- VoltOps hosted console
- VoltOps RAG
- deploy hospedado
- dashboards hospedados
- integrações cloud proprietárias
- comparação de performance com produção
- benchmarking
- hardening de segurança de produção

---

## 4. Base de comparação com o Voltagent

O diagnóstico deve considerar como alvo funcional do Voltagent, no mínimo:

1. Agent com `name`, `instructions` e `model`
2. uso direto por método e uso via REST API
3. memory com fallback in-memory e adapters opcionais
4. tools e tool calling
5. agents como tools para coordenação entre agents
6. guardrails de input/output e transformação em streaming
7. workflows com chain de steps, branching e retries
8. app/runtime equivalente ao `VoltAgent` para registrar agents e workflows
9. server/http para expor agents
10. working memory e estado quando houver suporte
11. logs/observabilidade local não hospedada
12. graceful shutdown e lifecycle do runtime

---

## 5. Artefatos de entrada

O diagnóstico deve ler e usar como fonte de verdade:

### Specs do projeto
- `specs/000-compatibility-target.md`
- `specs/010-feature-matrix.md`
- `specs/020-repository-architecture.md`
- `specs/030-agent.md`
- `specs/031-app-instance.md`
- `specs/040-tools.md`
- `specs/041-reasoning-tools.md`
- `specs/050-memory.md`
- `specs/060-guardrails.md`
- `specs/070-workflows.md`
- `specs/080-runtime-observability.md`
- `specs/081-server.md`
- `specs/100-demo-app.md`
- `specs/110-demo-parity-diagnosis.md`
- relatórios de diagnóstico já gerados

### Código
- `go.mod`
- `README.md`
- `AGENTS.md`, se existir
- `/pkg/**`
- `/internal/**`
- `/cmd/**`
- `/examples/**`
- `/test/**`

### Evidências executáveis
- unit tests
- integration tests
- smoke tests
- demo(s)
- reports existentes

---

## 6. Saída obrigatória

O diagnóstico deve gerar:

### 6.1 Relatório principal
- `docs/reports/lib-parity-report.md`

### 6.2 Checklist detalhado
- `docs/reports/lib-parity-checklist.md`

### 6.3 Backlog de gaps
- `docs/reports/lib-parity-gaps.md`

---

## 7. Dimensões obrigatórias de avaliação

### 7.1 Verificabilidade e ambiente

Objetivo:
validar se a biblioteca pode ser verificada de forma reprodutível.

Checklist:
- versão de Go no `go.mod`
- compatibilidade da toolchain
- dependências necessárias para build/test
- se `go test ./...` roda ou está bloqueado
- se demos e examples podem ser executados localmente
- se há dependências externas inesperadas

Classificação:
- `PASS`
- `PARTIAL`
- `FAIL`
- `BLOCKED`

---

### 7.2 Core Agent Capability

Objetivo:
validar se a biblioteca implementa o núcleo de agent.

Checklist:
- existe tipo principal de agent
- agent aceita nome/id equivalente
- agent aceita instructions
- agent aceita model/provider
- agent suporta execução síncrona
- agent suporta streaming
- agent suporta contexto/cancelamento
- agent suporta integração com memory
- agent suporta integração com tools
- agent suporta integração com guardrails
- erros e cancelamento são observáveis

Resultado esperado:
a `gaal-lib` deve provar o equivalente funcional do `Agent` do Voltagent.

---

### 7.3 App / Runtime Capability

Objetivo:
validar se existe um runtime equivalente ao `VoltAgent`.

Checklist:
- existe entrypoint de aplicação/runtime
- agents podem ser registrados no app
- workflows podem ser registrados no app
- defaults globais podem ser aplicados
- logger global pode ser aplicado
- memory default pode ser aplicada
- server/http pode ser acoplado ao runtime
- lifecycle de start/shutdown existe
- graceful shutdown existe ou está claramente parcial

Resultado esperado:
a `gaal-lib` deve provar o equivalente funcional do runtime do Voltagent sem depender de VoltOps.

---

### 7.4 Tools and Tool Calling

Objetivo:
validar a cobertura da trilha de tools.

Checklist:
- existe interface/contrato de tool
- existe registro/resolução de tool
- existe execução de tool
- erros de tool são observáveis
- exemplos demonstram uso de tool
- testes cobrem tool path
- tool calling em fluxo de agent é verificável

Classificação:
- `PASS`
- `PARTIAL`
- `FAIL`
- `BLOCKED`

---

### 7.5 Agent Orchestration via Agents-as-Tools

Objetivo:
validar a cobertura de coordenação entre agents.

Checklist:
- um agent pode ser usado por outro agent como ferramenta, ou há mecanismo equivalente
- existe evidência de coordenação/supervisão simples
- o runtime preserva contexto e resultado entre chamada do coordenador e sub-agent
- há teste ou demo desse comportamento

Observação:
se a biblioteca ainda não suportar agents-as-tools, isso deve ser classificado como gap relevante de paridade de orquestração, não apenas como melhoria opcional.

---

### 7.6 Memory and Working Memory

Objetivo:
validar a cobertura de memória.

Checklist:
- fallback in-memory existe
- modo stateless é suportado ou claramente modelado
- adapters são possíveis
- histórico conversacional é preservado no mesmo processo
- semântica de persistência está clara
- working memory existe, ou sua ausência está explicitamente marcada como gap
- flush/checkpoint/step persistence, se houver, estão especificados e testados

Observação:
working memory e conversation persistence em nível de step/finish são parte importante do alvo de comparação, então ausência ou parcialidade precisa aparecer explicitamente no relatório. 

---

### 7.7 Guardrails

Objetivo:
validar a cobertura de guardrails de runtime.

Checklist:
- input guardrails existem
- output guardrails existem
- stream guardrails existem
- modify/drop/abort são possíveis ou sua ausência está explicitada
- defaults em nível de app e override em nível de agent, quando previstos, são verificáveis
- demos/testes provam comportamento observável

---

### 7.8 Workflows

Objetivo:
validar a cobertura de workflows para orquestração.

Checklist:
- existe engine/builder de workflow
- há chain de steps
- há branching simples
- há retry policy
- há hooks ou equivalente
- há histórico ou estado observável
- a integração com app/runtime é verificável
- há demo/teste executável

Observação:
workflows são parte central do alvo de orquestração. Ausência de demo executável aqui reduz fortemente a confiança de paridade.

---

### 7.9 Server / REST Exposure

Objetivo:
validar a cobertura HTTP/REST.

Checklist:
- agents podem ser expostos por server/http
- existem endpoints ou shape equivalente para interação
- há health/readiness quando aplicável
- existe fluxo textual
- existe fluxo streaming
- tratamento de erros HTTP está testado
- documentação do contrato existe

Observação:
não é obrigatório reproduzir o shape exato das rotas do Voltagent, mas é obrigatório provar o fluxo funcional equivalente.

---

### 7.10 Observabilidade local, logs e tracing não hospedado

Objetivo:
validar recursos locais de inspeção do runtime sem VoltOps.

Checklist:
- logger local existe
- eventos relevantes podem ser observados
- workflow/agent execution pode ser rastreada localmente
- não há dependência de serviço hospedado para diagnóstico básico
- limitações são explícitas

---

### 7.11 DX e integrabilidade

Objetivo:
validar se a biblioteca pode ser usada por terceiros de forma razoável.

Checklist:
- README descreve uso básico
- examples existem e funcionam
- demo(s) servem como porta de entrada
- APIs públicas são coerentes
- a separação entre `pkg` e `internal` faz sentido
- mensagens de erro são úteis
- configuração local é simples

---

## 8. Matriz de classificação

Cada dimensão deve ser classificada como:

- `PASS`
- `PARTIAL`
- `FAIL`
- `BLOCKED`

Definições:

### PASS
Funciona, está evidenciado e cobre a expectativa principal.

### PARTIAL
Existe e funciona parcialmente, ou falta evidência/dx/teste suficiente.

### FAIL
Deveria existir para paridade de orquestração e não atende.

### BLOCKED
Não foi possível verificar por ambiente, ausência de demo/teste, ou bloqueio técnico.

---

## 9. Severidade dos gaps

Cada gap deve receber severidade:

- `S0` bloqueador de verificabilidade
- `S1` bloqueador de paridade para orquestração
- `S2` lacuna relevante mas não bloqueante
- `S3` melhoria de DX, naming ou acabamento

Exemplos:

### S0
- impossibilidade de build/test
- demos não executáveis
- toolchain incompatível sem instrução clara

### S1
- ausência de tools
- ausência de workflows
- ausência de server/http funcional
- ausência de coordenação entre agents quando a proposta é cobrir orquestração
- ausência de guardrails ou memory em nível de runtime, se definidos como alvo

### S2
- demos insuficientes
- cobertura de teste fraca
- parcialidade de working memory
- retries/hook incompletos
- logs/tracing locais insuficientes

### S3
- documentação fraca
- exemplos pouco didáticos
- inconsistência de naming
- ergonomia que pode melhorar

---

## 10. Itens críticos para considerar a biblioteca apta

A `gaal-lib` só pode ser considerada:

### 10.1 `APT for base orchestration parity`
se todos os itens abaixo forem verdadeiros:

1. build e testes executam sem bloqueio relevante
2. existe agent funcional com run e stream
3. existe runtime/app funcional
4. existe server/http funcional ou equivalentemente demonstrado
5. existe memory in-memory funcional
6. existem tools funcionais
7. existe evidência de workflows executáveis
8. existem guardrails demonstráveis
9. existe ao menos uma demo funcional cobrindo o fluxo base
10. não existe dependência obrigatória de VoltOps

### 10.2 `APT with reservations`
se o núcleo existir e funcionar, mas houver lacunas relevantes em:
- workflows
- agents-as-tools
- working memory
- cobertura de testes
- demos insuficientes
- dx/documentação

### 10.3 `NOT YET APT`
se faltarem capacidades centrais de orquestração.

### 10.4 `BLOCKED TO VERIFY`
se o ambiente impedir verificação confiável.

---

## 11. Itens explicitamente excluídos desta auditoria

Não devem contar contra a `gaal-lib` nesta spec:

- ausência de VoltOps Console
- ausência de VoltOps hosted tracing
- ausência de VoltOps RAG
- ausência de chaves `VOLTAGENT_PUBLIC_KEY` / `VOLTAGENT_SECRET_KEY`
- ausência de dashboard hospedado
- ausência de features gerenciadas de plataforma

Esses recursos pertencem à camada VoltOps e não ao alvo principal desta auditoria.

---

## 12. Checklist obrigatório resumido

### Ambiente
- [ ] `go.mod` inspecionado
- [ ] toolchain compatível
- [ ] `go test ./...` executado ou bloqueio documentado

### Agent
- [ ] run síncrono
- [ ] stream
- [ ] context/cancelamento
- [ ] memory/tools/guardrails integráveis

### App/Runtime
- [ ] registry de agents
- [ ] registry de workflows
- [ ] defaults
- [ ] start/shutdown

### Tools
- [ ] tools existem
- [ ] tool path testado
- [ ] erros de tool testados

### Orquestração entre agents
- [ ] agents-as-tools ou equivalente
- [ ] demo/teste de coordenação

### Memory
- [ ] in-memory default
- [ ] stateless/disable semantics claras
- [ ] working memory avaliada
- [ ] persistence semantics avaliadas

### Guardrails
- [ ] input
- [ ] output
- [ ] streaming
- [ ] intervenção observável

### Workflows
- [ ] chain
- [ ] branching
- [ ] retries
- [ ] hooks/estado
- [ ] demo/teste

### HTTP/Server
- [ ] health/readiness
- [ ] listagem/execução
- [ ] stream http
- [ ] erros cobertos

### DX
- [ ] README
- [ ] examples
- [ ] demo(s)
- [ ] documentação coerente

---

## 13. Relatório obrigatório de saída

O relatório principal deve conter:

### 1. Executive Summary
- decisão final
- blockers
- conclusão curta

### 2. Scope and Exclusions
- o que foi avaliado
- o que ficou fora
- confirmação de exclusão de VoltOps

### 3. Evidence Reviewed
- arquivos
- testes
- demos
- relatórios anteriores

### 4. Scorecard
Tabela com:
- dimensão
- status
- observação curta

### 5. Gaps Found
Para cada gap:
- id
- severidade
- descrição
- evidência
- recomendação

### 6. Coverage vs Voltagent
Tabela resumida:
- capability alvo
- status na gaal-lib
- evidência
- observação

### 7. Final Decision
Uma destas:
- `APT for base orchestration parity`
- `APT with reservations`
- `NOT YET APT`
- `BLOCKED TO VERIFY`

### 8. Next Prioritized Actions
Lista curta ordenada por impacto.

---

## 14. Critérios de aceitação desta spec

Esta spec será considerada concluída quando:

1. existir um procedimento repetível de auditoria da biblioteca
2. a auditoria diferenciar funcionalidade, evidência e DX
3. a auditoria cobrir os principais eixos de orquestração do Voltagent
4. a auditoria excluir corretamente VoltOps
5. a auditoria gerar relatório, checklist e backlog de gaps
6. a auditoria permitir classificar a `gaal-lib` de forma objetiva

---

## 15. Observação normativa

Esta spec é uma auditoria de paridade funcional para a biblioteca como um todo.

Ela complementa:
- a `spec 110`, focada na demo base
- a feature matrix
- os diagnósticos por trilha

Ela não substitui:
- specs de implementação
- testes de conformidade por módulo
- diagnósticos específicos de demo/trilha