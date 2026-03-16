# Spec 041: Reasoning Tools

## 1. Objetivo

Este documento define as `reasoning tools` da `gaal-lib`, equivalentes conceituais a `think` e `analyze`, para uso interno do runtime do `Agent`.

Os objetivos desta spec sao:

- definir o contrato estrutural de `think` e `analyze`
- especificar o toolkit interno `reasoning_tools`
- estabelecer limites de uso, budget e semantica de decisao do runtime
- permitir planejamento e analise intermediaria sem ampliar a API publica nesta etapa
- preservar paridade conceitual com o core do Voltagent sem exigir exposicao de cadeia textual completa de raciocinio

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020`, a `Spec 030` e a `Spec 040`.

Escopo desta spec:

- contrato interno de runtime
- uso intermediario em planejamento, validacao e decisao de proximo passo
- integracao com o sistema de `tools` e `toolkits` ja definido

Ficam fora do escopo desta spec:

- exposicao direta de `think` e `analyze` na API publica de `pkg/agent`
- qualquer obrigacao de expor chain-of-thought, traces textuais completos ou raciocinio bruto ao usuario final
- promocao imediata para `pkg/tool/reasoning`
- qualquer dependencia de VoltOps ou servico hospedado

## 2. Conceitos

- `Reasoning tool`: tool interna do runtime que segue o modelo conceitual da `Spec 040`, mas nao e tratada como tool publica do usuario.
- `Reasoning step`: etapa intermediaria do runtime usada para planejar, analisar ou validar o proximo movimento do agente.
- `Reasoning artifact`: saida estruturada de `think` ou `analyze`, mantida de forma efemera no run.
- `Reasoning decision`: decisao interna usada pelo runtime para escolher entre continuar, validar, chamar tool publica, responder ou falhar.
- `Reasoning budget`: limite proprio de invocacoes de reasoning por run. Ele nao altera a semantica de `MaxSteps` da `Spec 030`, que continua contando apenas iteracoes do modelo.
- `Reserved reasoning namespace`: namespace interno reservado para o toolkit `reasoning_tools` e para os effective names `reasoning.think` e `reasoning.analyze`.

Compatibilidade conceitual com Voltagent:

- `think` representa um passo curto de planejamento interno.
- `analyze` representa um passo de avaliacao intermediaria ou validacao antes de seguir.
- A divergencia idiomatica em Go e deliberada: `gaal-lib` especifica contratos estruturados, sem exigir exposicao publica de raciocinio livre.

Regras normativas:

1. Reasoning tools pertencem ao processo interno do runtime, nao ao contrato publico primario do `Agent`.
2. Reasoning tools nao sao `ToolCallRecord` publicos e nao contam como tool calls da `Spec 030`.
3. Reasoning tools nao satisfazem `ToolChoiceRequired`.
4. Reasoning tools nao podem ser disparadas por input do usuario, por `AllowedTools` ou por tool call emitida diretamente pelo modelo.
5. O runtime pode armazenar artifacts de reasoning apenas em working memory efemera, salvo nova spec.

## 3. Tool think

### 3.1 Proposito

`think` e a tool interna de planejamento rapido. Ela recebe um snapshot estruturado do run e retorna uma recomendacao sobre qual deve ser o proximo passo do runtime.

`think` existe para:

- resumir o estado relevante do run em um plano curto
- decidir se o agente deve continuar o loop, validar, chamar uma tool publica ou responder
- evitar que o runtime dependa de interpretacao informal do estado intermediario

### 3.2 Entrada

`think` deve receber um `tool.Call` interno cujo `ToolName` efetivo e `reasoning.think`.

`Call.Input` deve conter pelo menos os campos abaixo:

| Campo | Tipo | Obrigatorio | Descricao |
| --- | --- | --- | --- |
| `objective` | `string` | Sim | Objetivo atual do run ou da iteracao corrente. |
| `step_index` | `number` | Sim | Indice da iteracao do modelo em andamento, iniciando em `1`. |
| `max_steps` | `number` | Sim | Teto efetivo de iteracoes do modelo para o run. |
| `messages` | `[]any` | Sim | Snapshot normalizado das mensagens efetivas do run apos input guardrails e memory load. |
| `available_tools` | `[]any` | Sim | Descritores minimos das tools publicas realmente disponiveis no run. |
| `constraints` | `map[string]any` | Sim | Restricoes efetivas do run, incluindo `tool_choice`, `allowed_tools`, `agent_id`, `run_id` e `session_id` quando existirem. |
| `last_model_output` | `map[string]any` | Nao | Ultima saida relevante do modelo, quando houver. |
| `last_tool_result` | `map[string]any` | Nao | Ultimo resultado de tool publica executada, quando houver. |
| `candidate_response` | `string` | Nao | Resposta candidata ja existente, quando o runtime estiver decidindo se pode responder. |
| `metadata` | `map[string]any` | Nao | Metadata efetiva do run. |

### 3.3 Saida

`think` deve retornar `tool.Result` com `Result.Value` contendo um objeto com os campos abaixo:

| Campo | Tipo | Obrigatorio | Descricao |
| --- | --- | --- | --- |
| `decision` | `string` | Sim | Um entre `continue`, `call_tool`, `validate`, `respond` ou `fail`. |
| `plan` | `[]any` | Sim | Lista curta e estruturada de proximos passos internos. |
| `reason` | `string` | Sim | Justificativa curta e estruturada para a decisao. |
| `candidate_tool_name` | `string` | Nao | Nome da tool publica sugerida quando `decision = call_tool`. |
| `candidate_tool_input` | `map[string]any` | Nao | Input sugerido para a tool publica quando `decision = call_tool`. |
| `candidate_response` | `string` | Nao | Resposta sugerida quando `decision = respond`. |
| `needs_validation` | `bool` | Nao | Indica se o runtime deve passar por `analyze` antes de responder. |

### 3.4 Regras normativas

1. `think` nao executa side effects externos, nao chama tools publicas e nao grava em memory persistente.
2. `think` retorna recomendacao, nunca efeito final. O runtime continua sendo o unico responsavel por executar ou rejeitar a acao sugerida.
3. Se `decision = call_tool`, `candidate_tool_name` e `candidate_tool_input` tornam-se obrigatorios.
4. Se `decision = respond`, `candidate_response` deve existir ou o runtime deve possuir uma resposta final candidata valida fora da propria saida de `think`.
5. Se `decision = validate`, o runtime deve invocar `analyze` ou falhar/degradar conforme a politica configurada.
6. Se `decision = fail`, `reason` deve explicar a impossibilidade de prosseguir de forma curta e classificavel.
7. `think` nao pode sugerir `reasoning.think`, `reasoning.analyze` nem qualquer outro nome reservado como `candidate_tool_name`.
8. `plan` e `reason` sao artifacts internos resumidos. Esta spec nao exige armazenar texto livre completo de raciocinio.
9. Em `mode = think_only`, `decision = validate` e invalida.

## 4. Tool analyze

### 4.1 Proposito

`analyze` e a tool interna de avaliacao intermediaria. Ela recebe uma resposta candidata, um resultado de tool publica, um plano ou outro estado relevante do run e devolve um veredito estruturado sobre como seguir.

`analyze` existe para:

- validar se o estado atual permite resposta final
- identificar lacunas, inconsistencias ou necessidade de nova tool publica
- reduzir respostas finais prematuras quando a politica do runtime exigir validacao intermediaria

### 4.2 Entrada

`analyze` deve receber um `tool.Call` interno cujo `ToolName` efetivo e `reasoning.analyze`.

`Call.Input` deve conter pelo menos os campos abaixo:

| Campo | Tipo | Obrigatorio | Descricao |
| --- | --- | --- | --- |
| `objective` | `string` | Sim | Objetivo atual do run. |
| `step_index` | `number` | Sim | Iteracao do modelo associada ao estado analisado. |
| `max_steps` | `number` | Sim | Teto efetivo de iteracoes do modelo para o run. |
| `candidate` | `map[string]any` | Sim | Objeto que representa o estado a validar. Pode conter resposta candidata, plano, tool result ou combinacao destes. |
| `checks` | `[]any` | Sim | Lista de verificacoes desejadas, como `consistency`, `completeness`, `grounding`, `tool_relevance` ou `budget`. |
| `messages` | `[]any` | Nao | Snapshot de mensagens relevantes para a analise. |
| `supporting_tool_results` | `[]any` | Nao | Resultados de tools publicas que sustentam a resposta ou o proximo passo. |
| `metadata` | `map[string]any` | Nao | Metadata efetiva do run. |

### 4.3 Saida

`analyze` deve retornar `tool.Result` com `Result.Value` contendo um objeto com os campos abaixo:

| Campo | Tipo | Obrigatorio | Descricao |
| --- | --- | --- | --- |
| `verdict` | `string` | Sim | Um entre `approved`, `needs_more_work` ou `blocked`. |
| `recommended_action` | `string` | Sim | Um entre `continue`, `call_tool`, `respond` ou `fail`. |
| `findings` | `[]any` | Sim | Achados estruturados usados para diagnostico interno e proximo passo do runtime. |
| `reason` | `string` | Sim | Resumo curto do veredito. |
| `candidate_tool_name` | `string` | Nao | Tool publica sugerida quando `recommended_action = call_tool`. |
| `candidate_tool_input` | `map[string]any` | Nao | Input sugerido quando `recommended_action = call_tool`. |
| `candidate_response` | `string` | Nao | Resposta sugerida quando `recommended_action = respond`. |

### 4.4 Regras normativas

1. `analyze` nao responde ao usuario final diretamente; ele apenas produz um veredito interno.
2. `approved` permite `recommended_action = respond` ou `continue`.
3. `needs_more_work` permite `recommended_action = continue` ou `call_tool`.
4. `blocked` exige `recommended_action = fail`.
5. O runtime deve rejeitar combinacoes inconsistentes de `verdict` e `recommended_action`.
6. Se `recommended_action = call_tool`, `candidate_tool_name` e `candidate_tool_input` tornam-se obrigatorios.
7. Se `recommended_action = respond`, `candidate_response` deve existir ou o runtime deve possuir uma resposta final candidata valida fora da propria saida de `analyze`.
8. `analyze` nao pode recomendar tools reservadas do namespace `reasoning`.
9. `findings` devem ser estruturados e adequados para working memory interna; esta spec nao exige exposicao textual completa ao usuario final.

## 5. Toolkit reasoning_tools

`reasoning_tools` e o toolkit interno que agrupa `think` e `analyze`.

Contrato conceitual:

- `Toolkit.Name() = "reasoning_tools"`
- `Toolkit.Namespace() = "reasoning"`
- `Toolkit.Description()` deve descrever o conjunto como toolkit interno de planejamento e analise intermediaria
- `Toolkit.Tools()` deve retornar exatamente `think` e `analyze` quando o modo configurado os habilitar

Effective names reservados:

- `reasoning.think`
- `reasoning.analyze`

Regras normativas:

1. O toolkit deve ser registrado de forma atomica em um registry privado do runtime, separado do registry publico de tools do usuario.
2. O toolkit pode ser omitido integralmente quando a configuracao de reasoning estiver desabilitada.
3. `reasoning_tools` nao deve aparecer em `Registry.ListToolkits()` exposto ao usuario final enquanto permanecer uma capacidade interna.
4. `reasoning.think` e `reasoning.analyze` sao nomes reservados e nao podem ser resolvidos como tools publicas.
5. Se o modelo ou o usuario tentar invocar `reasoning.*` como tool publica, o runtime deve rejeitar a tentativa antes de qualquer execucao.
6. A integracao com a `Spec 040` e conceitual: `reasoning_tools` usa o mesmo modelo de `Tool`, `Toolkit`, validacao de input/output e `tool.Result`, mas sua registracao e privada ao runtime.
7. Qualquer futura promocao de `reasoning_tools` para `pkg/tool/reasoning` exige atualizacao concomitante desta spec, da `Spec 030` e da `Spec 040`.

## 6. Opcoes de configuracao

Esta spec define opcoes de configuracao normativas para a implementacao futura, mas elas pertencem ao runtime interno enquanto nao houver promocao explicita para API publica.

Opcoes minimas:

| Opcao | Tipo | Default | Semantica |
| --- | --- | --- | --- |
| `enabled` | `bool` | `false` | Habilita ou desabilita o toolkit `reasoning_tools` para o run. |
| `mode` | `string` | `disabled` | Um entre `disabled`, `think_only` ou `think_and_analyze`. |
| `max_reasoning_steps` | `number` | `2` | Teto de invocacoes internas de `think` e `analyze` por run. |
| `max_analyze_passes` | `number` | `1` | Teto especifico de invocacoes de `analyze` por run. |
| `require_analyze_before_response` | `bool` | `false` | Quando `true`, nenhuma resposta final pode ser emitida sem `analyze` aprovado. |
| `analyze_after_tool_result` | `bool` | `false` | Quando `true`, o runtime executa `analyze` apos cada resultado de tool publica relevante. |
| `store_artifacts_in_working_memory` | `bool` | `true` | Permite manter artifacts estruturados de reasoning apenas na working memory do run. |
| `emit_internal_diagnostics` | `bool` | `false` | Permite eventos ou logs internos resumidos de reasoning para observabilidade local. |
| `failure_policy` | `string` | `fail_open` | Um entre `fail_open` ou `fail_closed`. Define se falha de reasoning degrada para o loop base ou aborta o run. |

Regras normativas:

1. `enabled = false` implica `mode = disabled`.
2. `mode = think_only` nao pode registrar `analyze`.
3. `mode = think_and_analyze` pode registrar `think` e `analyze`.
4. `max_reasoning_steps` deve ser maior que zero quando `enabled = true`.
5. `max_analyze_passes` nao pode exceder `max_reasoning_steps`.
6. `failure_policy = fail_closed` e obrigatoria quando `require_analyze_before_response = true`.
7. `emit_internal_diagnostics` nao autoriza exposicao ao usuario final; autoriza apenas diagnostico interno resumido.
8. Qualquer futura exposicao dessas opcoes via `pkg/agent.Option` exige atualizacao da `Spec 030`.

## 7. Regras de uso pelo runtime

### 7.1 Ordem de execucao

O uso de reasoning tools deve obedecer as seguintes regras de ordenacao:

1. Input guardrails da `Spec 030` executam antes de qualquer reasoning step.
2. `memory.Store.Load`, quando existir, executa antes de qualquer reasoning step.
3. Working memory do run deve existir antes da primeira invocacao de `think` ou `analyze`.
4. O runtime pode invocar `think`:
   - antes da primeira chamada ao modelo, para planejamento inicial
   - depois de uma saida nao terminal do modelo
   - depois de um resultado de tool publica
5. O runtime pode invocar `analyze`:
   - depois de `think` retornar `validate`
   - depois de uma resposta candidata do modelo
   - depois de resultado de tool publica, quando a configuracao exigir
6. Output guardrails continuam executando apenas depois de existir resposta final candidata aprovada para o usuario.

### 7.2 Interpretacao das saidas

O runtime deve interpretar as saidas assim:

1. `think.decision = continue`: registrar artifact interno e seguir para nova iteracao do modelo ou para o proximo passo do loop base.
2. `think.decision = call_tool`: validar `candidate_tool_name` e `candidate_tool_input` contra as regras da `Spec 040`, contra `AllowedTools` e contra `ToolChoice`; somente depois disso a tool publica pode ser executada.
3. `think.decision = validate`: invocar `analyze` se ele estiver habilitado; caso contrario, aplicar `failure_policy`.
4. `think.decision = respond`: produzir resposta final somente apos qualquer `analyze` obrigatorio e apos output guardrails.
5. `think.decision = fail`: abortar ou degradar o run conforme `failure_policy`.
6. `analyze.verdict = approved` com `recommended_action = respond`: aplicar output guardrails e, se aprovados, responder ao usuario.
7. `analyze.verdict = approved` com `recommended_action = continue`: reinjetar findings resumidos na working memory e seguir o loop.
8. `analyze.verdict = needs_more_work` com `recommended_action = continue`: seguir o loop do modelo com os findings anexados como artifact interno.
9. `analyze.verdict = needs_more_work` com `recommended_action = call_tool`: validar e executar a tool publica sugerida.
10. `analyze.verdict = blocked` com `recommended_action = fail`: encerrar o run com erro interno classificado ou outra classificacao mais especifica quando a causa ja estiver definida.

### 7.3 Limites, validacao e cancelamento

1. Invocacoes de reasoning nao alteram a semantica observavel de `Request.MaxSteps`; elas consomem apenas `max_reasoning_steps`.
2. `think` e `analyze` devem ter input e output validados sob as mesmas regras estruturais da `Spec 040`.
3. O runtime nunca deve executar uma tool publica sugerida por reasoning sem passar pela validacao normal de registry, schema, allowlist, `ToolChoice` e cancelamento.
4. Reasoning tools nao podem chamar reasoning tools de forma recursiva.
5. O runtime nao deve reinjetar artifacts de reasoning como mensagens de role `tool` nem como mensagens do usuario.
6. Se `store_artifacts_in_working_memory = true`, artifacts de reasoning podem ser armazenados como records internos do run, mas nao devem ir para memory persistente por default.
7. `context.Context` continua sendo a fonte de verdade para cancelamento e deadline.
8. Se `max_reasoning_steps` ou `max_analyze_passes` forem excedidos, o runtime deve parar novas invocacoes de reasoning e aplicar `failure_policy`.
9. Em `fail_open`, a falha de reasoning desabilita reasoning para o run e retorna ao loop base sem expor artifacts ao usuario.
10. Em `fail_closed`, a falha de reasoning aborta o run antes de qualquer resposta final nao validada.

### 7.4 Relacao com o contrato publico do Agent

1. Reasoning steps nao sao `ToolCallRecord` publicos.
2. Reasoning steps nao aparecem como `EventToolCall` ou `EventToolResult` publicos.
3. Reasoning steps nao contam para satisfazer `ToolChoiceRequired`.
4. O runtime continua proibido de inventar tool calls publicas fora das regras da `Spec 030`; sugestoes de reasoning so produzem tool call publica depois de validacao interna e execucao deliberada do runtime.
5. Nenhum novo `EventType` publico e introduzido por esta spec.

## 8. Restricoes de exposicao ao usuario final

1. `reasoning_tools` nao faz parte da lista de tools que o usuario configura diretamente no `Agent`.
2. `reasoning.think` e `reasoning.analyze` nao podem aparecer em `AllowedTools`, examples de usuario, docs de uso publico ou respostas finais.
3. Artifacts de reasoning nao devem ser serializados em `Response.Message`, `ToolCallRecord`, `GuardrailDecision` ou eventos publicos de stream.
4. Artifacts de reasoning nao devem ser persistidos em `memory.Store.Save` por default.
5. O usuario final nao deve conseguir invocar reasoning tools por prompt, por nome de tool ou por qualquer payload de request.
6. O modelo nao deve receber `reasoning_tools` como parte do catalogo de tools publicas disponiveis.
7. Se `emit_internal_diagnostics = true`, o payload observavel deve continuar resumido, estruturado e restrito a diagnostico interno.
8. Esta spec nao cria obrigacao de expor cadeia textual completa de raciocinio ao usuario final, a hooks publicos ou a transporte de streaming.

## 9. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao futura sao:

1. Registro atomico bem-sucedido de `reasoning_tools` no runtime privado quando `enabled = true` e `mode = think_and_analyze`.
2. Ausencia completa de registry interno de reasoning quando `enabled = false`.
3. `think` recebendo snapshot estruturado valido e retornando `decision = continue`.
4. `think` com `decision = call_tool` disparando validacao normal da tool publica antes de qualquer execucao.
5. `think` tentando sugerir tool ausente, proibida, fora de `AllowedTools` ou do namespace reservado sendo rejeitado sem side effect externo.
6. `think` com `decision = respond` nao produzindo resposta final quando `require_analyze_before_response = true` sem passar por `analyze`.
7. `think` retornando `decision = validate` em `mode = think_only` sendo tratado como saida invalida.
8. `analyze` com `approved/respond` produzindo resposta final somente apos output guardrails.
9. `analyze` com `approved/continue` retornando ao loop do modelo com findings internos.
10. `analyze` com `needs_more_work/call_tool` executando a tool publica sugerida somente apos validacao normal.
11. `analyze` com combinacao inconsistente de `verdict` e `recommended_action` sendo rejeitado.
12. Tentativa do modelo ou do usuario de invocar `reasoning.think` ou `reasoning.analyze` como tool publica sendo bloqueada.
13. Reasoning steps nao aparecendo em `ToolCallRecord`, `EventToolCall`, `EventToolResult`, `Response` nem listas publicas de registry.
14. `ToolChoiceRequired` nao sendo satisfeito por `think` ou `analyze`.
15. Cancelamento por `context.Context` interrompendo invocacao ativa de reasoning.
16. Exaustao de `max_reasoning_steps` desligando reasoning no run em `fail_open`.
17. Falha de reasoning abortando o run em `fail_closed`.
18. Artifacts de reasoning ficando apenas na working memory efemera quando `store_artifacts_in_working_memory = true`.
19. `emit_internal_diagnostics = false` impedindo qualquer exposicao observavel de payload de reasoning fora do runtime interno.
20. `emit_internal_diagnostics = true` emitindo apenas diagnostico resumido, sem expor resposta final oculta nem chain-of-thought bruto.

## 10. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `think` e `analyze` tiverem contrato de entrada e saida estruturado, validavel e rastreavel a partir desta spec.
2. O toolkit `reasoning_tools` integrar `think` e `analyze` sob namespace reservado `reasoning`, com registro privado ao runtime.
3. O runtime souber decidir de forma deterministica quando continuar, validar, chamar tool publica, responder ou falhar a partir dos artifacts de reasoning.
4. Nenhuma tool publica sugerida por reasoning puder contornar validacao, allowlist, `ToolChoice`, cancelamento ou schemas da `Spec 040`.
5. Reasoning tools permanecerem parte do processo interno do runtime, sem ampliar a superficie publica do `Agent` nesta etapa.
6. Reasoning steps nao vazarem para `Response`, streaming publico, examples publicos ou persistencia conversacional por default.
7. `failure_policy`, budgets e ordem relativa com guardrails, memory e modelo estiverem definidos e testados.
8. Os casos de teste da secao 9 estiverem implementados nas camadas apropriadas.
9. Qualquer promocao futura para API publica atualizar explicitamente a `Spec 010`, a `Spec 030` e a `Spec 040`.
10. A existencia desta spec, por si so, nao altera o fato de `Reasoning tools` permanecer fora da v1 ate implementacao e cobertura de testes correspondentes.
