# Spec 010: Feature Matrix

## 1. Objetivo da matriz

Esta matriz existe para mapear as funcionalidades do core do Voltagent para capacidades equivalentes em `gaal-lib`, separando o que entra na v1 do que deve ser explicitamente adiado.

O documento complementa a `Spec 000` e a `Spec 001` com tres funcoes praticas:

- servir como checklist de implementacao
- servir como checklist de auditoria de paridade funcional
- reduzir deriva de escopo entre core de biblioteca e recursos de plataforma

## 2. Criterios de classificacao

### Obrigatoria na v1?

- `Sim`: sem esta feature nao existe paridade minima aceitavel com o core alvo do Voltagent para a v1.
- `Nao`: a feature e conhecida e relevante, mas pode ser adiada sem invalidar a v1.

### Prioridade

- `P0`: fundacao do runtime e das abstracoes centrais.
- `P1`: paridade funcional principal apos a fundacao.
- `P2`: expansao ou adaptador que nao deve bloquear a entrega da v1.

### Status

Os status usados nesta matriz sao:

- `Nao iniciado`: feature reconhecida e ainda sem implementacao.
- `Parcial`: existe primeira entrega funcional, mas sem cobrir todos os criterios de pronto da feature.
- `Adiado`: feature conhecida, mas fora da v1 neste momento.

### Risco

- `Baixo`: comportamento relativamente direto e com pouca ambiguidade de paridade.
- `Medio`: exige decisoes de API ou compatibilidade que podem gerar retrabalho.
- `Alto`: depende de semantica mais ambigua, estado persistente, streaming ou risco de deriva para VoltOps.

## 3. Matriz de features

| Feature | Descricao | Obrigatoria na v1? | Prioridade | Modulo provavel na gaal-lib | Status inicial | Risco | Observacoes de compatibilidade |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Application entrypoint | Bootstrap da aplicacao, composicao de registries, runtime e lifecycle. | Sim | P0 | `app` | Parcial | Medio | Primeira versao funcional entregue com registries, defaults, lifecycle e graceful shutdown basico; rollback e serverless continuam minimos. |
| Agent | Unidade principal de execucao com instrucoes, modelo, tools e estado associado. | Sim | P0 | `agent` | Parcial | Alto | Primeira versao funcional entregue com `Run`, `Stream` basico, tool loop sequencial, cancelamento e erros classificados; streaming incremental de modelo continua dependente de evolucao posterior. |
| Agent registry | Registro e resolucao de agentes por nome ou id logico. | Sim | P0 | `registry/agent` | Parcial | Medio | Cadastro, duplicidade, factories e lookup deterministico ja existem no `App`; ainda faltam suites de conformidade mais amplas. |
| Workflow registry | Registro e descoberta de workflows reutilizaveis. | Sim | P1 | `registry/workflow` | Parcial | Baixo | Registro e resolucao minimos existem no `App`; execucao de workflow continua fora desta etapa. |
| Tool | Contrato de ferramenta invocavel pelo runtime do agente. | Sim | P0 | `tool` | Parcial | Medio | Contrato publico agora cobre registry, resolucao deterministica, contexto operacional e validacao basica de input/output; ainda faltam suites de conformidade mais amplas e integracao futura com fluxos mais ricos do agent. |
| Toolkit | Agrupamento e distribuicao de um conjunto coerente de tools. | Sim | P1 | `toolkit` | Parcial | Baixo | Registro atomico, namespace, conflito de nomes e flattening por registry ja existem; consumo direto por APIs mais altas pode evoluir depois sem perder a semantica de composicao. |
| Reasoning tools | Conjunto de tools auxiliares para raciocinio estruturado e etapas intermediarias. | Nao | P2 | `tool/reasoning` | Parcial | Medio | `think`, `analyze` e o toolkit interno `reasoning_tools` agora existem no runtime privado com budgets, failure policy, exemplos e testes focados em `continue`, `validate` e resposta final; a promocao para API publica continua fora da v1. |
| Memory | Abstracao de memoria de mais longa duracao ou compartilhada entre execucoes. | Sim | P1 | `memory` | Parcial | Alto | Contratos publicos e integracao plugavel no runtime do agent existem; backends concretos e suites de paridade ainda faltam. |
| Working memory | Estado temporario de execucao para uma conversa, turno ou workflow. | Sim | P1 | `memory/working` | Parcial | Medio | Working memory efemera por run ja existe no core runtime; customizacao avancada e cobertura de workflow continuam pendentes. |
| Conversation persistence | Persistencia de historico e contexto conversacional entre execucoes. | Sim | P1 | `persistence/conversation` | Nao iniciado | Alto | V1 deve limitar-se a persistencia local ou plugavel, sem dependencia de plataforma hospedada. |
| Guardrails de input | Validacao, bloqueio ou saneamento antes da execucao principal. | Sim | P1 | `guardrail` | Parcial | Medio | Ordem, allow, block e transform basicos ja existem no agent runtime; guardrails de streaming e maior riqueza semantica seguem pendentes. |
| Guardrails de output | Validacao, bloqueio ou transformacao antes da resposta final. | Sim | P1 | `guardrail` | Parcial | Medio | Transformacao e bloqueio basicos ja existem antes da persistencia; suites de paridade completas ainda faltam. |
| Guardrails de streaming | Interceptacao e controle de eventos parciais durante streaming. | Nao | P2 | `guardrail/stream` | Adiado | Alto | Streaming aumenta a complexidade de paridade e nao deve bloquear a v1. |
| Workflow chain | Encadeamento sequencial de etapas com passagem de contexto e resultados. | Sim | P0 | `workflow` | Nao iniciado | Medio | Equivale ao fluxo sequencial do core; ordem de etapas e propagacao de erro devem ser identicas. |
| Workflow branching | Desvio condicional entre caminhos alternativos de workflow. | Sim | P1 | `workflow` | Nao iniciado | Medio | A API pode usar funcoes ou structs, mas a decisao e as transicoes precisam ser testaveis. |
| Workflow retry | Politica de repeticao para etapas falhas com limites e estrategia. | Sim | P1 | `workflow/retry` | Nao iniciado | Medio | Deve definir semantica de erro, backoff e idempotencia esperada. |
| Workflow hooks | Hooks locais antes, durante e depois da execucao de workflows. | Sim | P1 | `workflow/hooks` | Nao iniciado | Medio | Pode absorver parte da extensibilidade do Voltagent sem introduzir sistema de plugins completo. |
| Workflow execution history | Registro observavel das transicoes, etapas e resultados do workflow. | Sim | P1 | `workflow/history` | Nao iniciado | Medio | Deve ser local e consultavel para auditoria, sem virar observabilidade hospedada. |
| Logger | Logging do runtime e das operacoes principais. | Sim | P0 | `logging` | Parcial | Baixo | Fachada minima sobre `slog` e `logger.Nop()` ja existem; instrumentacao mais rica segue para etapas futuras. |
| Observability hooks locais | Hooks para metricas, tracing local e eventos de instrumentacao. | Sim | P1 | `observability` | Parcial | Medio | Hooks locais de `agent` e `app` ja existem com recuperacao de panic; integracoes adicionais continuam fora desta etapa. |
| HTTP server abstraction | Adaptador para expor runtime por HTTP sem acoplar a um framework especifico. | Nao | P2 | `adapter/http` | Adiado | Medio | Nao faz parte do nucleo minimo; so deve entrar apos estabilizacao do runtime central. |
| Serverless abstraction | Adaptador para empacotar runtime em ambientes serverless. | Nao | P2 | `adapter/serverless` | Adiado | Alto | Risco alto de acoplamento com plataforma e de escopo de deploy, portanto fora da v1. |
| Triggers/extensibility | Pontos de extensao para acionar agentes ou workflows a partir de eventos externos. | Nao | P2 | `trigger` | Adiado | Alto | Na v1, hooks e APIs explicitas devem bastar; um subsistema de triggers fica para depois. |
| Graceful shutdown | Encerramento ordenado com flush de estado, hooks finais e cancelamento cooperativo. | Sim | P1 | `lifecycle` | Parcial | Medio | `Shutdown` cooperativo e idempotente ja existe no `App`; cobertura de cenarios de rollback e servidores long-lived ainda pode crescer. |

## 4. Lacunas e riscos

As principais lacunas atuais para atingir paridade funcional sao:

- falta um catalogo mais detalhado de cenarios de referencia do Voltagent para cada feature listada
- a fronteira entre `memory`, `working memory` e `conversation persistence` ainda pode gerar sobreposicao de responsabilidades
- guardrails e workflow hooks precisam de uma ordem de execucao precisa para evitar divergencias comportamentais
- `workflow execution history` e `observability hooks locais` precisam compartilhar um modelo de eventos coerente sem virar uma camada de plataforma
- adapters como HTTP, serverless e triggers tendem a puxar o projeto para escopo operacional cedo demais

Riscos tecnicos que merecem vigilancia especial:

- traducao de semantica async do ecossistema original para `context.Context`, cancelamento e concorrencia em Go
- definicao de contratos de erro observaveis sem copiar mecanicamente a API textual do Voltagent
- persistencia conversacional local que seja suficiente para paridade sem introduzir dependencia de infraestrutura externa

## 5. Ordem recomendada de implementacao

1. Fundacao do runtime: `Logger`, `Application entrypoint`, `Agent`, `Tool` e `Workflow chain`.
2. Composicao do nucleo: `Agent registry`, `Workflow registry`, `Toolkit` e `Graceful shutdown`.
3. Workflows completos: `Workflow branching`, `Workflow retry`, `Workflow hooks` e `Workflow execution history`.
4. Estado e continuidade: `Working memory`, `Memory` e `Conversation persistence`.
5. Confiabilidade e instrumentacao: `Guardrails de input`, `Guardrails de output` e `Observability hooks locais`.
6. Expansoes apos estabilizacao da v1: promocao publica de `Reasoning tools`, `Guardrails de streaming`, `HTTP server abstraction`, `Serverless abstraction` e `Triggers/extensibility`.

## 6. Criterios de pronto por feature

Uma feature desta matriz so pode ser considerada pronta quando cumprir todos os itens abaixo:

1. Possui contrato publico ou interno claramente especificado em documento ou teste de aceitacao.
2. Possui pelo menos um cenario de sucesso e um de falha com comportamento observavel definido.
3. Possui testes de paridade conceitual ou comportamental cobrindo a semantica central da feature.
4. Nao introduz dependencia de VoltOps nem exige servico hospedado para funcionar localmente.
5. Registra qualquer divergencia intencional em relacao ao Voltagent em spec ou decisao arquitetural.

Adicionais por classe de feature:

- Registries: precisam cobrir cadastro, lookup, duplicidade e erro de referencia ausente.
- Tools e toolkits: precisam cobrir validacao de input, execucao, cancelamento e propagacao de erro.
- Workflows: precisam cobrir ordem de execucao, transicoes, retries, hooks e historico observavel.
- Memory e persistencia: precisam cobrir escopo, serializacao, restauracao e isolamento entre execucoes.
- Guardrails: precisam cobrir permitir, bloquear, transformar e erro, incluindo a ordem em relacao ao fluxo principal.
- Runtime e lifecycle: precisam cobrir startup, shutdown, cleanup e cooperacao com `context.Context`.

## Fora do escopo v1

Tudo que for VoltOps ou depender de plataforma hospedada permanece fora do escopo da v1, mesmo quando existir conceito superficialmente parecido no Voltagent. Isso inclui:

- control plane, dashboard, paineis operacionais e administracao web
- observabilidade hospedada, tracing centralizado, log aggregation e telemetria SaaS
- runners gerenciados, execucao remota hospedada e orquestracao operacional
- multi tenancy, organizacoes, RBAC, billing, quotas e governanca operacional
- deploy tooling, autoscaling, filas distribuidas, schedulers operacionais e release orchestration
- secret management hospedado, configuracao remota e qualquer dependencia arquitetural de servico externo para o runtime funcionar

Se uma futura proposta de `HTTP server abstraction`, `Serverless abstraction` ou `Triggers/extensibility` exigir qualquer item acima, ela deve permanecer fora do escopo v1 mesmo que a abstracao local correspondente venha a ser considerada mais tarde.
