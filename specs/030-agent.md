# Spec 030: Agent Public API

## 1. Objetivo

Este documento define a API publica do `Agent` em `gaal-lib`, com foco em:

- estabelecer uma superficie idiomatica em Go para execucao de agentes
- preservar paridade conceitual com o core do Voltagent sem copiar sua API textual
- especificar o ciclo de vida de execucao sincrona e em streaming
- definir a integracao obrigatoria com tools, memory, guardrails e hooks de observabilidade
- tornar explicitos os comportamentos esperados em sucesso, erro, cancelamento e stream abortado

Esta spec e normativa para o contrato publico. Ela nao implementa o runtime.

Ficam fora do escopo deste documento:

- HTTP, gRPC, SSE, WebSocket ou qualquer adaptador de transporte
- recursos de VoltOps, observabilidade hospedada ou control plane
- handoffs, multi-agent orchestration e workflow composition detalhada
- definicao detalhada de adapters de transporte para expor streaming fora da API publica do `Agent`

## 2. Terminologia

- `Agent`: unidade principal de execucao. Combina instrucoes, modelo, tools, memory, guardrails e hooks.
- `Run`: uma execucao completa do agente, do input inicial ate a resposta final ou erro terminal.
- `Stream`: uma execucao do agente que expoe eventos ordenados durante o processamento.
- `Session`: identificador logico que liga multiplos runs ao mesmo contexto conversacional.
- `Working memory`: estado efemero de um unico run, usado para acumular contexto temporario, tool calls e resultados intermediarios.
- `Memory`: estado persistente ou compartilhado entre runs, associado a uma `Session`.
- `Tool call`: invocacao de uma tool registrada pelo agente durante o loop de execucao.
- `Guardrail`: validacao, bloqueio ou transformacao aplicada a input, streaming parcial ou output final.
- `Hook`: observador local de eventos do run, usado para tracing, metricas e logging.
- `Cancelamento`: encerramento cooperativo disparado por `context.Context`.
- `Stream abortado`: encerramento iniciado pelo consumidor do stream via `Close()`.

## 3. Responsabilidades do Agent

O `Agent` deve ser responsavel por:

1. validar e congelar sua propria configuracao no momento da construcao
2. ser seguro para uso concorrente em multiplos `Run` e `Stream`
3. orquestrar o loop modelo -> tool -> modelo ate resposta final ou condicao terminal
4. aplicar guardrails na ordem correta e com semantica observavel
5. carregar memory persistente e manter working memory isolada por run
6. emitir eventos estaveis para streaming e hooks locais
7. retornar resposta final consistente ou erro classificado

O `Agent` nao deve ser responsavel por:

- detalhes de transporte, serializacao de rede ou protocolo
- lifecycle global da aplicacao ou shutdown de infraestrutura compartilhada
- retries implicitos de tools, modelos ou memory fora do que for explicitamente especificado
- qualquer recurso operacional de plataforma hospedada

## 4. API publica proposta

### 4.1 Pacotes publicos envolvidos

O contrato principal deve viver em `pkg/agent`.

Tipos compartilhados de mensagem, metadata e usage devem convergir para `pkg/types` quando esse pacote for introduzido. Para esta spec, `types.Message`, `types.MessageDelta`, `types.Metadata` e `types.Usage` representam os shared types minimos que o `Agent` consome.

As integracoes obrigatorias do `Agent` dependem dos seguintes pacotes publicos:

- `pkg/agent`
- `pkg/tool`
- `pkg/memory`
- `pkg/guardrail`
- `pkg/types`

### 4.2 Tipos principais exportados por `pkg/agent`

```go
package agent

type Agent struct {
    // opaco e imutavel apos New
}

type Config struct {
    Name         string
    Instructions string
    Model        Model
}

type Option func(*options) error

func New(cfg Config, opts ...Option) (*Agent, error)

func (a *Agent) ID() string
func (a *Agent) Name() string
func (a *Agent) Descriptor() Descriptor
func (a *Agent) Definition() Definition
func (a *Agent) Run(ctx context.Context, req Request) (Response, error)
func (a *Agent) Stream(ctx context.Context, req Request) (Stream, error)

type Descriptor struct {
    Name string
    ID   string
}

type Definition struct {
    Descriptor       Descriptor
    Instructions     string
    Model            Model
    MaxSteps         int
    Tools            []tool.Tool
    Memory           memory.Store
    WorkingMemory    memory.WorkingMemoryFactory
    InputGuardrails  []guardrail.Input
    StreamGuardrails []guardrail.Stream
    OutputGuardrails []guardrail.Output
    Hooks            []Hook
    Metadata         types.Metadata
}

type Request struct {
    RunID        string
    SessionID    string
    Messages     []types.Message
    Metadata     types.Metadata
    MaxSteps     int
    ToolChoice   ToolChoice
    AllowedTools []string
}

type Response struct {
    RunID              string
    AgentID            string
    SessionID          string
    Message            types.Message
    Usage              types.Usage
    ToolCalls          []ToolCallRecord
    GuardrailDecisions []GuardrailDecision
    Metadata           types.Metadata
}

type Stream interface {
    Recv() (Event, error)
    Close() error
}

type Event struct {
    Sequence  int64
    Type      EventType
    RunID     string
    AgentID   string
    SessionID string
    Time      time.Time
    Delta     *types.MessageDelta
    ToolCall  *ToolCallEvent
    Guardrail *GuardrailEvent
    Response  *Response
    Err       error
    Metadata  types.Metadata
}

type Hook interface {
    OnEvent(ctx context.Context, event Event)
}

type Engine interface {
    Run(ctx context.Context, agent *Agent, req Request) (Response, error)
    Stream(ctx context.Context, agent *Agent, req Request) (Stream, error)
}

type ToolChoice string

const (
    ToolChoiceAuto     ToolChoice = "auto"
    ToolChoiceNone     ToolChoice = "none"
    ToolChoiceRequired ToolChoice = "required"
)

type EventType string

const (
    EventAgentStarted   EventType = "agent.started"
    EventAgentDelta     EventType = "agent.delta"
    EventToolCall       EventType = "agent.tool_call"
    EventToolResult     EventType = "agent.tool_result"
    EventGuardrail      EventType = "agent.guardrail"
    EventAgentCompleted EventType = "agent.completed"
    EventAgentFailed    EventType = "agent.failed"
    EventAgentCanceled  EventType = "agent.canceled"
)
```

### 4.3 Tipos auxiliares minimos de `pkg/agent`

```go
type ToolCallRecord struct {
    ID       string
    Name     string
    Input    map[string]any
    Output   tool.Result
    Duration time.Duration
}

type ToolCallEvent struct {
    Call   ToolCallRecord
    Status ToolCallStatus
}

type ToolCallStatus string

const (
    ToolCallStarted   ToolCallStatus = "started"
    ToolCallSucceeded ToolCallStatus = "succeeded"
    ToolCallFailed    ToolCallStatus = "failed"
)

type GuardrailDecision struct {
    Phase    GuardrailPhase
    Name     string
    Action   guardrail.Action
    Reason   string
    Metadata types.Metadata
}

type GuardrailEvent struct {
    Decision GuardrailDecision
}

type GuardrailPhase string

const (
    GuardrailPhaseInput  GuardrailPhase = "input"
    GuardrailPhaseStream GuardrailPhase = "stream"
    GuardrailPhaseOutput GuardrailPhase = "output"
)
```

### 4.4 Contratos minimos consumidos pelo Agent

Os contratos abaixo podem migrar para pacotes dedicados depois, mas o `Agent` precisa destes limites minimos para existir sem ambiguidade.

```go
type Model interface {
    Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)
    Stream(ctx context.Context, req ModelRequest) (ModelStream, error)
}

type ModelStream interface {
    Recv() (ModelEvent, error)
    Close() error
}

type ToolSpec struct {
    Name        string
    Description string
}

type ModelToolCall struct {
    ID    string
    Name  string
    Input map[string]any
}

type ModelRequest struct {
    AgentID      string
    RunID        string
    SessionID    string
    Instructions string
    Messages     []types.Message
    Memory       memory.Snapshot
    Metadata     types.Metadata
    MaxSteps     int
    ToolChoice   ToolChoice
    AllowedTools []string
    Tools        []ToolSpec
}

type ModelResponse struct {
    Message   types.Message
    Usage     types.Usage
    ToolCalls []ModelToolCall
    Metadata  types.Metadata
}

type ModelEvent struct {
    Delta    *types.MessageDelta
    Message  *types.Message
    ToolCall *ModelToolCall
    Usage    types.Usage
    Done     bool
}
```

```go
package tool

type Tool interface {
    Name() string
    Description() string
    Schema() Schema
    Call(ctx context.Context, call Call) (Result, error)
}
```

```go
package memory

type Store interface {
    Load(ctx context.Context, sessionID string) (Snapshot, error)
    Save(ctx context.Context, sessionID string, delta Delta) error
}

type WorkingMemoryFactory interface {
    NewRunState(ctx context.Context, agentID, runID string) (WorkingSet, error)
}

type Record struct {
    Kind string
    Name string
    Data map[string]any
}

type WorkingSet interface {
    AddMessage(msg types.Message)
    AddRecord(record Record)
    Snapshot() Snapshot
}
```

```go
package guardrail

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

### 4.5 Observacoes normativas da superficie publica

- `Agent` deve ser imutavel apos `New`.
- `Run` e `Stream` devem ser seguros para concorrencia no mesmo `Agent`.
- `Definition()` e um snapshot somente-leitura para runtimes e composicao avancada; ele nao deve permitir mutacao da configuracao do `Agent`.
- `Request.Messages` deve aceitar ao menos roles `user`, `assistant` e `tool`.
- `types.MessageDelta` deve suportar pelo menos delta textual e correlacao com o `RunID`.
- `Response` de `Run` e `Response` carregado por `EventAgentCompleted` devem ser equivalentes para a mesma execucao.
- `Engine` e um ponto de extensao avancado. Seu uso principal e `pkg/app` injetar o runtime concreto sem expor `internal/*`.

## 5. Configuracao do Agent

### 5.1 Campos obrigatorios

- `Config.Name` e obrigatorio e nao pode ser vazio.
- `Config.Model` e obrigatorio.

### 5.2 Campos opcionais

- `Config.Instructions` e opcional; vazio significa "sem system instruction fixa".
- `WithID(id string)` define um identificador logico estavel.
- `WithTools(tools ...tool.Tool)` registra tools disponiveis ao agente.
- `WithMemory(store memory.Store)` habilita memory persistente por `SessionID`.
- `WithWorkingMemory(factory memory.WorkingMemoryFactory)` substitui a working memory padrao por run.
- `WithInputGuardrails(gs ...guardrail.Input)` registra guardrails de input em ordem.
- `WithStreamGuardrails(gs ...guardrail.Stream)` registra guardrails de stream em ordem.
- `WithOutputGuardrails(gs ...guardrail.Output)` registra guardrails de output em ordem.
- `WithHooks(hooks ...Hook)` registra hooks observaveis em ordem.
- `WithMaxSteps(n int)` define teto maximo de iteracoes do loop de execucao.
- `WithMetadata(md types.Metadata)` define metadata padrao do agente.
- `WithExecutionEngine(engine Engine)` injeta o runtime concreto usado por `Run` e `Stream`.

### 5.3 Defaults e validacoes

- Se `WithID` nao for usado, o `ID` deve ser derivado de `Name` por normalizacao deterministica.
- `WithMaxSteps` deve rejeitar `n <= 0`.
- O default de `MaxSteps` deve ser `8`.
- `WithTools` deve rejeitar nomes duplicados.
- `WithMemory(nil)`, `WithWorkingMemory(nil)` e `WithExecutionEngine(nil)` devem ser tratados como configuracao invalida quando a option for fornecida explicitamente.
- `WithMetadata` deve copiar o mapa defensivamente.
- `Request.MaxSteps == 0` usa o `MaxSteps` do agente.
- `Request.MaxSteps > 0` aplica um override por run, mas o valor efetivo nao pode ultrapassar o teto configurado no agente.
- `Request.AllowedTools`, quando fornecido, deve ser subconjunto das tools configuradas. Caso contrario, o request e invalido.
- Se o agente tiver `memory.Store` configurado e `Request.SessionID` vier vazio, `Run` e `Stream` devem falhar com erro de request invalido.
- Se `Run` ou `Stream` forem chamados sem `Engine`, o resultado deve ser `ErrNoExecutionEngine`.

### 5.4 Merge de metadata

- Metadata do agente deve ser aplicada como base.
- Metadata do request deve sobrescrever chaves coincidentes.
- Chaves reservadas para tracing local sao `trace_id`, `span_id` e `parent_span_id`.

## 6. Fluxo de execucao sincrona

O fluxo sincrono de `Run` deve obedecer a ordem abaixo:

1. Validar `context.Context`, configuracao efetiva e `Request`.
2. Criar `RunID` se nao vier no request.
3. Emitir `EventAgentStarted`.
4. Executar input guardrails em ordem de registro.
5. Carregar memory persistente, se configurada.
6. Criar working memory isolada para o run.
7. Montar o contexto do modelo com:
   - `Instructions`
   - snapshot de memory
   - mensagens do request
   - resultados de tools acumulados no run
8. Executar o loop modelo -> tool -> modelo ate obter resposta final ou erro terminal.
9. Executar output guardrails em ordem de registro.
10. Persistir memory do run, se configurada.
11. Construir `Response`.
12. Emitir `EventAgentCompleted`.
13. Retornar `Response` e `nil`.

Regras adicionais:

- Tools devem ser executadas de forma sequencial na v1 para manter determinismo.
- `MaxSteps` conta iteracoes do modelo. Tool calls isoladas nao aumentam o contador sozinhas.
- Se o modelo pedir uma tool desabilitada, ausente ou fora de `AllowedTools`, o run falha antes da invocacao.
- Em sucesso, nao pode haver erro junto com `Response`.
- Em erro, o `Response` retornado por `Run` deve ser o zero value.

## 7. Fluxo de execucao em streaming

`Stream` deve compartilhar a mesma semantica central de `Run`, mas expor eventos ordenados durante a execucao.

Fluxo esperado:

1. `Stream(ctx, req)` valida a entrada e inicia o run.
2. O consumidor chama `Recv()` repetidamente.
3. `Recv()` bloqueia ate existir um novo `Event`, `io.EOF` ou erro terminal.
4. O primeiro evento deve ser `EventAgentStarted`.
5. Durante geracao incremental, deltas textuais devem sair como `EventAgentDelta`.
6. Cada tool call deve gerar ao menos:
   - um `EventToolCall` com status `started`
   - um `EventToolResult` com status `succeeded` ou `failed`
7. Cada decisao de guardrail observavel deve gerar `EventGuardrail`.
8. Quando o consumidor nao aborta localmente o stream, o ultimo evento entregue deve ser exatamente um entre:
   - `EventAgentCompleted`
   - `EventAgentFailed`
   - `EventAgentCanceled`
9. Depois do evento terminal entregue ao consumidor, `Recv()` deve retornar `io.EOF`.

Garantias de streaming:

- `Sequence` deve ser monotonicamente crescente por run, iniciando em `1`.
- O `Response` dentro de `EventAgentCompleted` deve ser o resultado final canonico do run.
- Deltas vistos antes de `EventAgentCompleted` sao provisorios. So o evento terminal de sucesso confirma o resultado final.
- Se houver erro depois de deltas ja emitidos, o consumidor deve considerar o run falho; nao ha resposta final valida.
- Hooks locais devem observar a mesma ordem de eventos do stream enquanto o consumidor mantiver o stream aberto. Em abort local, hooks ainda podem observar o cancelamento final mesmo quando o consumidor nao o receber.

## 8. Integracao com tools

O `Agent` deve tratar tools como extensoes declarativas e plugaveis.

Regras obrigatorias:

- A lista de tools efetiva por run e a intersecao entre:
  - tools registradas no agente
  - `AllowedTools` do request, quando fornecido
  - politica representada por `ToolChoice`
- `ToolChoiceAuto` permite uso normal de tools.
- `ToolChoiceNone` proibe qualquer tool call naquele run.
- `ToolChoiceRequired` exige que ao menos uma tool call seja usada antes da resposta final; se isso nao ocorrer, o run falha.
- A ordem de tool calls deve seguir a ordem emitida pelo modelo.
- O runtime nao deve inventar tool calls fora do que o modelo pediu ou do que o request permitiu.
- O input bruto de uma tool call precisa ficar observavel em `ToolCallRecord.Input`.
- O output bruto da tool precisa ficar observavel em `ToolCallRecord.Output`.
- Resultados de tools devem entrar no contexto do modelo como mensagens de role `tool`.
- Falhas de tool devem encerrar o run na v1. Nao ha retry implicito nem fallback automatico no `Agent`.

## 9. Integracao com memory

O `Agent` deve separar claramente memory persistente de working memory.

### 9.1 Working memory

- Toda execucao deve possuir working memory, mesmo sem store persistente.
- A working memory default deve ser efemera, local ao run e descartada ao final.
- Working memory deve registrar ao menos:
  - mensagens efetivas do run
  - records representando tool calls executadas
  - resultados de tools
  - resposta final antes da persistencia

### 9.2 Memory persistente

- `memory.Store.Load` deve ocorrer depois dos input guardrails e antes da primeira chamada ao modelo.
- `memory.Store.Save` deve ocorrer depois dos output guardrails e antes de `EventAgentCompleted`.
- Se `Load` falhar, o run falha antes de qualquer geracao do modelo.
- Se `Save` falhar, o run falha mesmo que a resposta final ja tenha sido gerada.
- Em caso de erro, cancelamento ou stream abortado, nenhuma resposta final deve ser considerada persistida.
- Quando o backend de memory suportar transacao, a persistencia do run deve ser atomica.
- Quando o backend nao suportar transacao, a ordem minima de persistencia deve ser deterministica e documentada, mas o contrato observavel continua sendo: falha de `Save` invalida o run.

## 10. Integracao com guardrails

O `Agent` deve suportar guardrails de input, stream e output como parte do contrato principal.

### 10.1 Input guardrails

- Executam depois da validacao estrutural do request e antes do acesso a memory ou modelo.
- Sao avaliados na ordem de registro.
- Cada guardrail pode:
  - `allow`: seguir sem alteracoes
  - `block`: interromper o run com erro classificado
  - `transform`: substituir o input efetivo consumido pelo proximo guardrail e pelo runtime

### 10.2 Output guardrails

- Executam depois da resposta final do modelo e antes da persistencia e do retorno ao usuario.
- Tambem obedecem a ordem de registro.
- Um `transform` de output altera a resposta final observavel.
- Um `block` de output invalida o run e impede `EventAgentCompleted`.

### 10.3 Stream guardrails

- Executam apenas durante `Stream`, antes de cada `EventAgentDelta` candidato.
- Tambem obedecem a ordem de registro e recebem snapshots publicos do delta atual e do buffer efetivo aprovado.
- Cada stream guardrail pode:
  - `allow`: emitir o chunk inalterado
  - `transform`: substituir o chunk parcial visivel ao proximo guardrail, ao consumidor e ao buffer efetivo
  - `drop`: suprimir apenas o chunk corrente sem abortar o run
  - `abort`: encerrar o run com erro classificado de guardrail, sem `EventAgentCompleted`
- O output final usado pelos output guardrails deve partir do buffer efetivo aprovado quando houver `transform` ou `drop` em stream.

### 10.4 Regras de semantica

- Guardrail bloqueado e diferente de guardrail com falha tecnica.
- `block` deve produzir `ErrGuardrailBlocked` com detalhes da decisao.
- `abort` em stream tambem deve produzir erro classificado de guardrail, observavel por `Phase = stream` e `Action = abort`.
- Erro tecnico do guardrail deve falhar o run com erro encapsulado e `EventAgentFailed`.
- Toda decisao observavel de guardrail deve aparecer em `GuardrailDecision`.
- Decisoes invalidas de guardrail devem falhar em modo fail-closed.

## 11. Erros e cancelamento

`pkg/agent` deve expor erros publicos classificaveis e compativeis com `errors.Is` e `errors.As`.

API minima:

```go
type ErrorKind string

const (
    ErrorKindInvalidConfig    ErrorKind = "invalid_config"
    ErrorKindInvalidRequest   ErrorKind = "invalid_request"
    ErrorKindNoEngine         ErrorKind = "no_engine"
    ErrorKindGuardrailBlocked ErrorKind = "guardrail_blocked"
    ErrorKindTool             ErrorKind = "tool"
    ErrorKindMemory           ErrorKind = "memory"
    ErrorKindModel            ErrorKind = "model"
    ErrorKindMaxSteps         ErrorKind = "max_steps"
    ErrorKindCanceled         ErrorKind = "canceled"
    ErrorKindStreamAborted    ErrorKind = "stream_aborted"
    ErrorKindInternal         ErrorKind = "internal"
)

type Error struct {
    Kind    ErrorKind
    Op      string
    AgentID string
    RunID   string
    Cause   error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error
```

Sentinels minimos:

- `ErrInvalidConfig`
- `ErrInvalidRequest`
- `ErrNoExecutionEngine`
- `ErrGuardrailBlocked`
- `ErrMaxStepsExceeded`
- `ErrStreamAborted`

Semantica obrigatoria:

- `errors.Is(err, context.Canceled)` deve funcionar para cancelamento por contexto.
- `errors.Is(err, context.DeadlineExceeded)` deve funcionar para deadline excedido.
- `errors.Is(err, ErrStreamAborted)` deve funcionar quando o consumidor abortar um stream com `Close()`.
- Cancelamento por contexto deve parar novas chamadas ao modelo e a tools o mais cedo possivel.
- `Stream.Close()` deve ser idempotente.
- Se `Close()` for chamado antes do evento terminal, o runtime deve cancelar o run com causa `ErrStreamAborted`.
- Depois de `Close()`, `Recv()` pode ainda drenar eventos ja bufferizados, mas nao pode entregar `EventAgentCompleted`.
- Ao final de um stream abortado, `Recv()` deve retornar `ErrStreamAborted` ou `io.EOF` se todos os eventos bufferizados ja tiverem sido drenados e o consumidor fechou apos um terminal nao-sucesso.

## 12. Eventos e tracing hooks

Eventos sao o contrato estavel de observabilidade local do `Agent`.

Regras obrigatorias:

- Todo evento deve conter `RunID`, `AgentID`, `Sequence` e `Time`.
- `Metadata` do evento deve carregar, quando disponiveis, `trace_id`, `span_id` e `parent_span_id`.
- Hooks devem receber os mesmos eventos publicos que um stream nao abortado observaria, na mesma ordem logica. Em abort local, o hook pode observar o cancelamento final sem que ele seja entregue ao consumidor.
- Hooks sao observadores e nao podem alterar o fluxo do run.
- Panics em hooks devem ser recuperados pelo runtime e tratados como diagnostico interno; nao podem derrubar o processo nem mudar o resultado do run.
- `EventAgentFailed` deve incluir `Err`.
- `EventAgentCanceled` deve incluir a causa de cancelamento em `Err`.
- `EventAgentCompleted` deve incluir `Response`.

O conjunto minimo de eventos publicos estaveis da v1 e:

- `agent.started`
- `agent.delta`
- `agent.tool_call`
- `agent.tool_result`
- `agent.guardrail`
- `agent.completed`
- `agent.failed`
- `agent.canceled`

Eventos mais granulares, como `memory.loaded`, `memory.saved` ou spans internos de modelo, podem existir no runtime, mas nao devem ser promovidos a API publica nesta fase sem nova spec.

## 13. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `pkg/agent` expuser `Agent`, `Config`, `Request`, `Response`, `Stream`, `Event` e `Error`.
2. `Agent` puder ser construido com configuracao valida e rejeitar configuracao invalida de forma deterministica.
3. `Run` e `Stream` preservarem a mesma semantica de negocio para o mesmo input.
4. O `Agent` for seguro para uso concorrente por multiplos goroutines.
5. Tools, memory e guardrails puderem ser plugados apenas por contratos publicos.
6. O fluxo de sucesso persistir memory antes de sinalizar `agent.completed`.
7. `ErrGuardrailBlocked` diferenciar bloqueio de erro tecnico do guardrail.
8. Cancelamento por contexto interromper o run cooperativamente e nao produzir sucesso falso.
9. Stream abortado por `Close()` cancelar a execucao e nao produzir `agent.completed`.
10. Hooks observarem eventos sem exigir importacao de `internal/*`.
11. Toda divergencia intencional em relacao ao Voltagent ficar registrada nesta spec ou em specs derivadas.

## 14. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao futura sao:

1. Construcao valida de `Agent` com `Name`, `Model` e defaults.
2. Erro ao construir `Agent` sem `Name`.
3. Erro ao construir `Agent` sem `Model`.
4. Erro ao registrar duas tools com o mesmo nome.
5. `Run` bem-sucedido sem tools e sem memory.
6. `Run` com tool call bem-sucedida e tool result entrando no contexto seguinte.
7. Erro quando o modelo pede tool ausente ou proibida.
8. `Run` com `ToolChoiceRequired` e nenhuma tool call emitida, resultando em erro.
9. Input guardrail com `allow`.
10. Input guardrail com `transform`.
11. Input guardrail com `block`, retornando `ErrGuardrailBlocked`.
12. Output guardrail com `transform`, alterando a resposta final.
13. Output guardrail com `block`, impedindo persistencia e sucesso.
14. Stream guardrail com `transform`, alterando `EventAgentDelta` e o buffer efetivo.
15. Stream guardrail com `drop`, suprimindo apenas o chunk corrente.
16. Stream guardrail com `abort`, gerando `EventGuardrail`, `EventAgentFailed` e sem `EventAgentCompleted`.
17. Carregamento de memory bem-sucedido antes da primeira chamada ao modelo.
18. Falha em `memory.Load`, impedindo geracao do modelo.
19. Falha em `memory.Save` apos resposta gerada, invalidando o run.
20. `Run` cancelado por `context.Context`, com `errors.Is(err, context.Canceled)`.
21. `Run` abortado por deadline, com `errors.Is(err, context.DeadlineExceeded)`.
22. `Stream` com ordem correta de eventos: started -> deltas/tool events/guardrails -> terminal.
23. Equivalencia entre `Response` de `Run` e `Response` em `EventAgentCompleted` quando nao houver transformacoes exclusivas do caminho de stream.
24. `Stream.Close()` idempotente.
25. Stream abortado pelo consumidor nao entrega `EventAgentCompleted`.
26. Hook recebendo os mesmos eventos, na mesma ordem, que o stream.
27. Panic em hook sendo recuperado sem derrubar o run.
28. Uso concorrente do mesmo `Agent` por multiplos runs sem race de configuracao compartilhada.

## 15. Questoes em aberto

1. O contrato `Model` deve permanecer em `pkg/agent` inicialmente ou merece um `pkg/model` dedicado antes da implementacao?
2. `Request` precisa de um tipo de `Session` mais rico que `SessionID string` ja na v1, ou isso pode esperar pela spec de persistence?
3. O default de `MaxSteps = 8` e adequado para paridade pratica com o Voltagent, ou deve ser ajustado apos fixtures de compatibilidade?
4. `ToolChoiceRequired` deve exigir "pelo menos uma tool call" ou "uma tool call bem-sucedida" para considerar o run valido?
5. O contrato publico de `types.Message` deve entrar antes desta implementacao para evitar alias temporarios em `pkg/agent`?
6. Memory deve persistir tambem o input do usuario quando o output for bloqueado, ou a v1 deve manter a regra mais simples de nao persistir nada em runs nao concluidos?
