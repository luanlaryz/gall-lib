# Spec 060: Guardrails

## 1. Objetivo

Este documento define o sistema de `guardrails` da `gaal-lib`.

Os objetivos desta spec sao:

- estabelecer os contratos publicos de guardrails em `pkg/guardrail`
- definir guardrails de input, output e streaming como fases distintas do ciclo de execucao do `Agent`
- especificar como cada guardrail valida, modifica, bloqueia, descarta ou aborta artefatos observaveis
- tornar normativa a ordem de execucao entre guardrails, memory, modelo, tools, hooks e resposta final
- preservar paridade conceitual com o core do Voltagent sem exigir copia textual de API

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020`, a `Spec 030`, a `Spec 031`, a `Spec 041` e a `Spec 050`.

Quando houver mais detalhe sobre guardrails neste documento, ele prevalece sobre referencias resumidas em specs mais gerais.

Compatibilidade com a `Spec 010`:

- `guardrails de input` e `guardrails de output` continuam obrigatorios na v1
- `guardrails de streaming` passam a ter contrato especificado neste documento, mas continuam `Nao`, `P2` e `Adiado` na matriz ate que a `Spec 010` seja atualizada
- esta spec nao reclassifica sozinha a prioridade, obrigatoriedade ou status da feature matrix

Ficam fora do escopo desta spec:

- policy engines hospedados, dashboards de moderacao, rule management remoto ou qualquer capacidade de VoltOps
- reescrita retroativa de eventos de stream ja entregues ao consumidor
- aplicacao de guardrails sobre artifacts internos de reasoning expostos publicamente
- guardrails obrigatorios sobre eventos de tool call ou tool result diferentes de chunks textuais de assistant

## 2. Tipos de guardrails

`gaal-lib` reconhece tres tipos normativos de guardrail:

| Tipo | Momento | Payload observado | Acoes validas | Efeito principal |
| --- | --- | --- | --- | --- |
| `input` | depois da validacao estrutural do request e antes de `memory.Load` | `[]types.Message` do request efetivo | `allow`, `transform`, `block` | valida, saneia ou bloqueia o input do run |
| `stream` | antes de cada `EventAgentDelta` visivel ao consumidor | `types.MessageDelta` candidato | `allow`, `transform`, `drop`, `abort` | controla chunks parciais do stream |
| `output` | depois de existir resposta final candidata e antes de `memory.Save` | `types.Message` final candidato | `allow`, `transform`, `block` | valida, reescreve ou bloqueia a resposta final |

Observacoes normativas:

1. Guardrails sao locais ao runtime e plugados por contratos publicos. Nenhum guardrail pode exigir importacao de `internal/*`.
2. O runtime deve tratar guardrails como extensoes fail-closed: erro tecnico do guardrail falha o run.
3. Guardrails nao recebem system prompts internos, reasoning artifacts internos ou estruturas de `internal/runtime`.
4. Guardrails devem ser seguros para uso concorrente depois de registrados no `Agent` ou herdados pelo `App`.

## 3. Contratos e interfaces

O contrato principal deve viver em `pkg/guardrail`.

API publica proposta:

```go
package guardrail

import (
    "context"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Phase string

const (
    PhaseInput  Phase = "input"
    PhaseStream Phase = "stream"
    PhaseOutput Phase = "output"
)

type Action string

const (
    ActionAllow     Action = "allow"
    ActionBlock     Action = "block"
    ActionTransform Action = "transform"
    ActionDrop      Action = "drop"
    ActionAbort     Action = "abort"
)

type Context struct {
    Phase     Phase
    AgentID   string
    AgentName string
    RunID     string
    SessionID string
    Metadata  types.Metadata
}

type InputRequest struct {
    Context
    Messages []types.Message
}

type StreamRequest struct {
    Context
    ChunkIndex       int64
    Delta            types.MessageDelta
    BufferedContent  string
}

type OutputRequest struct {
    Context
    Message types.Message
}

type Decision struct {
    Name     string
    Action   Action
    Reason   string
    Messages []types.Message
    Delta    *types.MessageDelta
    Message  *types.Message
    Metadata types.Metadata
}

type Input interface {
    CheckInput(ctx context.Context, req InputRequest) (Decision, error)
}

type Stream interface {
    CheckStream(ctx context.Context, req StreamRequest) (Decision, error)
}

type Output interface {
    CheckOutput(ctx context.Context, req OutputRequest) (Decision, error)
}
```

### 3.1 Contexto recebido por cada guardrail

Cada guardrail deve receber dois canais de contexto:

1. `context.Context` do run corrente, para cancelamento, deadline e tracing local.
2. Um envelope publico e imutavel de fase com snapshots defensivos.

Garantias normativas do envelope:

- `AgentID`, `AgentName`, `RunID` e `SessionID` devem refletir o run efetivo
- `Metadata` deve ser o merge efetivo de metadata do `Agent` e do request, com sobrescrita da configuracao mais especifica
- `InputRequest.Messages`, `StreamRequest.Delta`, `StreamRequest.BufferedContent` e `OutputRequest.Message` devem ser snapshots; o guardrail nao pode depender de mutacao compartilhada
- `StreamRequest.ChunkIndex` deve ser monotonicamente crescente por resposta parcial candidata, iniciando em `1`
- nenhum guardrail recebe ponteiros para working memory, memory snapshot, instructions internas ou artifacts internos de reasoning

### 3.2 Regras normativas das interfaces

1. `Input.CheckInput` pode apenas decidir sobre `Messages` do request efetivo.
2. `Stream.CheckStream` pode apenas decidir sobre o chunk textual candidato e sobre sua entrega ao consumidor.
3. `Output.CheckOutput` pode apenas decidir sobre a mensagem final candidata.
4. `Decision.Action` deve ser explicitamente preenchido. Action vazia ou combinacao invalida de action e payload e erro tecnico do guardrail.
5. `Decision.Name` e opcional; quando vier vazio, o runtime deve preencher um nome derivado do tipo concreto do guardrail para observabilidade.
6. `Decision.Metadata` e apenas observabilidade local. Ela nao muta `Request.Metadata`, `Response.Metadata` ou metadata do run.
7. `ActionTransform` em `input` exige `Decision.Messages`.
8. `ActionTransform` em `stream` exige `Decision.Delta`.
9. `ActionTransform` em `output` exige `Decision.Message`.
10. `ActionDrop` e valido apenas para `stream`.
11. `ActionAbort` e valido apenas para `stream`.
12. `ActionBlock` e valido apenas para `input` e `output`.

### 3.3 Restricoes de payload

Para manter contratos observaveis coerentes:

- `input` pode substituir a slice completa de mensagens efetivas, inclusive removendo, reordenando ou inserindo mensagens, desde que todas continuem validas para a `Spec 030`
- `stream` pode alterar apenas o chunk parcial efetivo; `RunID` e `Role` do delta transformado devem permanecer equivalentes ao delta recebido
- `output` pode reescrever a mensagem final inteira, mas a `Role` resultante deve continuar semanticamente `assistant`
- nenhum guardrail pode introduzir artifacts internos de reasoning, types de `internal/*` ou eventos de runtime dentro de `Messages`, `Delta` ou `Message`

## 4. Fluxo de execucao

### 4.1 Ordem normativa geral do run

O ciclo de execucao do `Agent` deve obedecer a seguinte ordem:

1. Validar `context.Context`, configuracao efetiva do `Agent` e `Request`.
2. Materializar a definicao efetiva do agente, incluindo metadata, hooks, memory, tools e listas de guardrails ja mescladas.
3. Emitir `EventAgentStarted`.
4. Executar `input guardrails` em ordem.
5. Executar `memory.Store.Load`, quando houver provider configurado.
6. Criar working memory isolada para o run.
7. Entrar no loop modelo -> tools -> modelo.
8. Quando houver streaming de assistant, executar `stream guardrails` antes de cada `EventAgentDelta`.
9. Quando existir resposta final candidata, executar `output guardrails` em ordem.
10. Persistir `memory.Store.Save`, quando houver provider configurado.
11. Construir `Response`.
12. Emitir `EventAgentCompleted`.
13. Retornar sucesso.

### 4.2 Ordem normativa por fase

#### Input

1. O runtime clona `Request.Messages`.
2. Cada guardrail recebe o snapshot efetivo produzido pelo guardrail anterior.
3. Se todos permitirem, o resultado final da fase de input se torna o input canonico do run.
4. O `memory.Store.Load` sempre acontece depois da fase de input.

#### Stream

1. A fase de stream so existe quando o runtime estiver processando deltas textuais de assistant.
2. Cada chunk parcial candidato passa pela cadeia de stream guardrails antes de qualquer `EventAgentDelta`.
3. O runtime deve manter um `effective stream buffer` com o conteudo ja aprovado para aquele output parcial.
4. O buffer deve ser atualizado apenas com chunks `allow` ou `transform`.
5. Chunks `drop` nao entram no buffer.
6. Chunks `abort` nao entram no buffer e encerram o run.

#### Output

1. A fase de output so comeca quando existir uma mensagem final candidata observavel.
2. O payload inicial da fase de output deve ser a mensagem final canonica da execucao.
3. Se houver transformacoes ou drops em stream, o payload inicial de output deve refletir o conteudo efetivamente aprovado na fase de stream.
4. `memory.Store.Save` sempre acontece depois da fase de output e apenas em sucesso.

## 5. Regras de encadeamento

### 5.1 Formacao da cadeia efetiva

A cadeia efetiva de guardrails de cada fase e congelada no inicio do run.

Regras normativas:

1. Guardrails herdados de `App.Defaults.Agent` entram antes dos guardrails locais do `Agent`, seguindo a regra de append ordenado da `Spec 031`.
2. Depois de iniciado o run, a ordem da cadeia nao pode mudar.
3. Cada fase usa sua propria cadeia. Guardrails de `input` nunca executam na fase de `output`, e vice-versa.
4. Se a implementacao ainda nao suportar `stream guardrails`, a cadeia de stream e vazia e isso nao altera `input` nem `output`.

### 5.2 Regras de passagem entre guardrails

Dentro da mesma fase:

1. `allow` passa o payload atual, sem alteracao, ao proximo guardrail.
2. `transform` substitui o payload atual e entrega a nova versao ao proximo guardrail.
3. `block` encerra imediatamente a cadeia da fase e o run.
4. `drop` encerra imediatamente a cadeia do chunk corrente e suprime aquele chunk.
5. `abort` encerra imediatamente a cadeia do chunk corrente e o run.
6. Erro tecnico encerra imediatamente a cadeia e o run.

Observacoes importantes:

- `drop` e `abort` nunca avancam para o proximo stream guardrail do mesmo chunk
- `transform` nunca muta retroativamente payloads ja vistos por guardrails anteriores
- o resultado observado por uma fase posterior sempre parte do payload final aprovado da fase anterior

### 5.3 Ordem cronologica observavel

Quando o run for bem-sucedido, `Response.GuardrailDecisions` deve preservar ordem cronologica real das decisoes observaveis:

1. todas as decisoes de `input`
2. todas as decisoes de `stream` que realmente ocorreram
3. todas as decisoes de `output`

Quando o run falhar antes da resposta final:

- `EventGuardrail` continua sendo a fonte canonica de observabilidade da decisao que levou ao bloqueio ou abort
- `Response` nao existe

## 6. Semantica de retorno

### 6.1 Semantica por acao

| Action | Fases validas | Efeito |
| --- | --- | --- |
| `allow` | `input`, `stream`, `output` | aceita o payload atual sem alteracao |
| `transform` | `input`, `stream`, `output` | substitui o payload atual pela versao retornada |
| `block` | `input`, `output` | rejeita o run com erro classificado de guardrail |
| `drop` | `stream` | descarta apenas o chunk corrente e permite a execucao continuar |
| `abort` | `stream` | interrompe o run imediatamente como rejeicao de guardrail em meio ao stream |

### 6.2 Semantica por fase

#### Input

- `allow`: segue para o proximo guardrail ou para `memory.Load`
- `transform`: `Decision.Messages` substitui integralmente as mensagens efetivas do run
- `block`: o run falha antes de `memory.Load`, modelo, tools e output

#### Stream

- `allow`: o chunk atual pode ser emitido e anexado ao buffer efetivo
- `transform`: o chunk transformado e o unico visivel ao proximo guardrail, ao consumidor e ao buffer efetivo
- `drop`: o chunk nao gera `EventAgentDelta`, nao altera o buffer efetivo e nao gera output final por si so
- `abort`: o chunk nao e emitido, o buffer nao muda e o run termina sem `EventAgentCompleted`

#### Output

- `allow`: a mensagem final candidata segue para o proximo guardrail ou para persistencia
- `transform`: `Decision.Message` substitui integralmente a mensagem final candidata
- `block`: o run falha antes de `memory.Save`, `EventAgentCompleted` e retorno de sucesso

### 6.3 Validade do retorno

As combinacoes abaixo sao invalidas e devem falhar como erro tecnico do guardrail:

1. `transform` sem payload substituto da fase correspondente
2. `drop` ou `abort` fora da fase de stream
3. `block` na fase de stream
4. payload substituto com role invalida ou semanticamente incompativel com a fase
5. retorno com `Action` desconhecida

## 7. Abort, drop e modify

Nesta spec, `modify` corresponde ao efeito observavel de `ActionTransform`.

### 7.1 Modify

`modify` troca o payload efetivo da fase corrente:

- em `input`, reescreve as mensagens que o runtime ainda vai consumir
- em `stream`, reescreve o chunk antes de qualquer entrega ao consumidor
- em `output`, reescreve a mensagem final canonica

`modify` sempre afeta o que o proximo guardrail da mesma fase recebe.

### 7.2 Drop

`drop` existe apenas em `stream` e significa:

- descartar somente o chunk corrente
- nao emitir `EventAgentDelta` para aquele chunk
- nao anexar o chunk ao buffer efetivo
- permitir que o modelo continue produzindo novos chunks ou chegue ao final da resposta

`drop` nao e erro. Ele apenas remove visibilidade daquele chunk.

### 7.3 Abort

`abort` existe apenas em `stream` e significa:

- interromper imediatamente a geracao em andamento
- cancelar cooperativamente o trabalho subsequente do modelo naquele run
- impedir novas tool calls, novos chunks, output guardrails e persistencia
- encerrar o run como rejeicao de guardrail, nao como cancelamento do consumidor

Para o contrato publico do `Agent`, `abort` e uma forma de bloqueio tardio:

- o erro terminal deve continuar classificado como erro de guardrail
- `EventAgentCanceled` continua reservado para cancelamento do `context.Context` ou `Stream.Close()`
- `abort` deve levar a `EventAgentFailed` com erro classificado de guardrail, depois do `EventGuardrail` correspondente

### 7.4 Diferenca entre block e abort

- `block` rejeita o run em um ponto de fronteira entre fases, antes de seguir para a proxima etapa
- `abort` rejeita o run durante a emissao parcial, quando a execucao ja esta em andamento e pode haver chunks anteriores ja entregues

## 8. Interacao com streaming

### 8.1 Escopo

`stream guardrails` governam apenas chunks textuais de assistant que gerariam `EventAgentDelta`.

Eles nao se aplicam por default a:

- `EventAgentStarted`
- `EventGuardrail`
- `EventToolCall`
- `EventToolResult`
- `EventAgentCompleted`
- `EventAgentFailed`
- `EventAgentCanceled`

Evita-se, assim, recursao de guardrails sobre seus proprios eventos.

### 8.2 Ordem de eventos

Quando um chunk parcial e observado:

1. o runtime cria o `types.MessageDelta` candidato
2. executa a cadeia de stream guardrails em ordem
3. emite `EventGuardrail` para cada decisao observavel
4. se o resultado final do chunk for `allow` ou `transform`, emite `EventAgentDelta`
5. se o resultado final do chunk for `drop`, nao emite `EventAgentDelta`
6. se o resultado final do chunk for `abort`, nao emite `EventAgentDelta` e em seguida encerra o run com evento terminal de falha classificada

### 8.3 Buffer efetivo do stream

O runtime deve manter um buffer efetivo da resposta parcial aprovada:

- chunks `allow` entram no buffer como recebidos
- chunks `transform` entram no buffer ja transformados
- chunks `drop` nao entram no buffer
- chunks `abort` nao entram no buffer e encerram o run

Se a resposta final for montada a partir de streaming, este buffer e a fonte de verdade para o conteudo ja aprovado.

### 8.4 Consistencia entre deltas e resultado final

Os deltas vistos durante o stream sao sempre provisorios ate `EventAgentCompleted`.

Regras obrigatorias:

1. se nao houver `output transform`, a resposta final deve refletir o conteudo efetivamente aprovado por stream guardrails
2. se houver `output transform`, `EventAgentCompleted.Response` e o resultado final canonico, mesmo que diverja do texto acumulado pelos deltas
3. a v1 nao exige evento extra de reconciliacao ou replay de correcoes retroativas; o consumidor deve tratar o evento terminal de sucesso como fonte de verdade final

## 9. Interacao com resultado final

### 9.1 Formacao da mensagem final candidata

A mensagem final candidata entregue aos output guardrails deve seguir esta regra:

1. se nao houve stream guardrails ativos ou eles nao alteraram o conteudo, o runtime pode usar a mensagem final do modelo
2. se algum stream guardrail aplicou `transform` ou `drop`, o runtime deve derivar o conteudo final candidato a partir do buffer efetivo aprovado na fase de stream
3. se o run nao teve streaming publico, a mensagem final candidata vem diretamente do loop sincrono do modelo

### 9.2 Reescrita do output final

Output guardrails podem reescrever a mensagem final inteira.

Quando isso ocorrer, a nova mensagem deve ser a unica versao canonica usada em:

- `Response.Message`
- `EventAgentCompleted.Response.Message`
- `memory.Delta.Response`
- qualquer snapshot final entregue a hooks ou observabilidade local depois da fase de output

### 9.3 Efeitos de bloqueio no resultado final

Se um output guardrail retornar `block`:

- nao existe resposta final valida para o usuario
- `memory.Save` nao pode acontecer
- `EventAgentCompleted` nao pode acontecer
- o run falha com erro classificado de guardrail

## 10. Erros e observabilidade local

### 10.1 Erros

Guardrails devem diferenciar claramente:

1. `decisao de politica`: retorno bem-formado com `block`, `drop`, `abort` ou `transform`
2. `erro tecnico`: erro retornado pela interface do guardrail ou decisao invalida

Semantica normativa:

- `block` e `abort` devem produzir erro terminal classificado de guardrail
- `drop` nao produz erro
- erro tecnico do guardrail deve falhar o run em modo fail-closed
- `Op` do erro deve identificar a fase: `guardrail.input`, `guardrail.stream` ou `guardrail.output`

Para manter compatibilidade com o contrato atual de `Agent`:

- `ErrGuardrailBlocked` continua sendo o sentinel minimo para rejeicao por guardrail
- diferencas entre `block` e `abort` devem ficar observaveis por fase e action, nao obrigatoriamente por um novo sentinel

### 10.2 Eventos e hooks locais

Toda decisao observavel de guardrail deve produzir `EventGuardrail`.

O `GuardrailDecision` observado pelo `Agent` deve expor ao menos:

- fase
- nome
- action
- reason
- metadata associada a decisao

Quando `stream guardrails` forem implementados na superficie publica, a enumeracao de fase observavel do `Agent` deve incluir `stream`.

Quando a fase for `stream`, `GuardrailDecision.Metadata` deve poder carregar campos adicionais de observabilidade local, por exemplo:

- `chunk_index`
- `policy_id`
- `rewrite_kind`

Hooks locais devem observar a mesma ordem de eventos entregue ao stream, exceto no caso de `Stream.Close()`, em que o consumidor pode nao receber o terminal mas hooks ainda podem observar o encerramento.

### 10.3 Sem dependencia hospedada

Observabilidade de guardrails na v1 deve ser local ao processo:

- eventos do `Agent`
- hooks locais
- logger local
- integracoes locais opcionais de metricas ou tracing

Nenhum contrato desta spec exige servico hospedado para validar, bloquear ou auditar guardrails.

## 11. Casos de teste obrigatorios

Toda implementacao desta spec deve cobrir, no minimo, os casos abaixo.

### 11.1 Input

1. `input allow` mantendo o request inalterado.
2. `input transform` reescrevendo as mensagens entregues ao proximo guardrail e ao modelo.
3. `input block` impedindo `memory.Load`.
4. erro tecnico em input guardrail falhando o run antes do modelo.
5. ordem efetiva `App defaults -> Agent local` sendo respeitada.
6. propagacao correta de `AgentID`, `RunID`, `SessionID` e metadata para `InputRequest`.

### 11.2 Stream

1. `stream allow` emitindo o chunk original.
2. `stream transform` emitindo o chunk transformado e anexando o valor transformado ao buffer efetivo.
3. `stream drop` suprimindo `EventAgentDelta` e mantendo a execucao.
4. `stream abort` emitindo `EventGuardrail`, encerrando o run e impedindo output guardrails e persistencia.
5. ordem de stream guardrails por chunk, incluindo transformacao encadeada.
6. `BufferedContent` refletindo somente chunks aprovados.
7. hooks observando a mesma ordem de `EventGuardrail` e `EventAgentDelta`.

### 11.3 Output

1. `output allow` preservando a mensagem final.
2. `output transform` reescrevendo `Response.Message`, `EventAgentCompleted.Response.Message` e `memory.Delta.Response`.
3. `output block` impedindo sucesso e persistencia.
4. erro tecnico em output guardrail falhando o run em modo fail-closed.

### 11.4 Integracao entre fases

1. `memory.Load` ocorrendo depois de input guardrails.
2. `memory.Save` ocorrendo depois de output guardrails.
3. output guardrail recebendo como base o conteudo final aprovado por stream guardrails quando houver `transform` ou `drop`.
4. `Response.GuardrailDecisions` preservando ordem cronologica real em runs bem-sucedidos.
5. ausencia de `Response` em bloqueio de input, abort de stream ou bloqueio de output.

## 12. Criterios de aceitacao

Uma implementacao so pode ser considerada conforme quando todos os itens abaixo forem verdadeiros:

1. A ordem de execucao entre input guardrails, memory, stream guardrails, output guardrails e persistencia estiver implementada exatamente como especificada.
2. Cada guardrail receber `context.Context` e um envelope publico imutavel com `AgentID`, `RunID`, `SessionID`, metadata e payload da fase correspondente.
3. `transform` modificar de forma observavel o payload consumido pelo proximo guardrail e pelo runtime da mesma fase.
4. `drop` descartar apenas o chunk corrente, sem emitir delta e sem abortar o run.
5. `abort` interromper o run em meio ao stream, com erro classificado de guardrail e sem `EventAgentCompleted`.
6. `block` impedir a progressao para a proxima fase e produzir erro classificado de guardrail.
7. O output final poder ser reescrito integralmente por output guardrails e essa versao reescrita se tornar a unica resposta canonica.
8. Toda decisao observavel de guardrail gerar `EventGuardrail` e ficar disponivel para hooks locais.
9. Erros tecnicos de guardrail falharem o run em modo fail-closed.
10. Nenhuma parte da feature exigir dependencia de VoltOps ou servico hospedado para funcionar localmente.
