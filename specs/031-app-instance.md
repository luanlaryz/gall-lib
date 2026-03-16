# Spec 031: App Instance

## 1. Objetivo

Este documento define o equivalente em Go ao `VoltAgent` como entrypoint da aplicacao em `gaal-lib`.

O objetivo da spec e:

- estabelecer `pkg/app` como composition root publica da biblioteca
- especificar como a aplicacao e inicializada e como o runtime interno e exposto de forma controlada
- definir registries deterministas para descoberta de agents e workflows
- definir a hierarquia de defaults globais e como eles podem ser sobrescritos
- especificar integracao com logger, observability hooks locais, adapters de server e casos serverless
- definir o contrato de graceful shutdown orientado por `context.Context`

Esta spec complementa a `Spec 020` e a `Spec 030`.

Ficam fora do escopo deste documento:

- implementacao concreta de HTTP, gRPC, CLI, fila ou qualquer outro transporte
- infraestrutura remota, control plane, observabilidade hospedada ou qualquer capacidade de VoltOps
- detalhes internos do motor de workflow alem do contrato minimo necessario para registro e lookup

## 2. Responsabilidades do App

O `App` deve ser responsavel por:

1. centralizar a configuracao global do runtime da biblioteca
2. construir, registrar e expor agents e workflows de forma deterministica
3. aplicar defaults globais antes da materializacao do runtime
4. ser a unica ponte publica entre `pkg/*` e `internal/runtime`
5. expor accessors seguros para adapters de server e cenarios serverless
6. coordenar startup, lifecycle state e graceful shutdown
7. fornecer logger e observability hooks globais coerentes para os componentes gerenciados

O `App` nao deve ser responsavel por:

- expor endpoints de rede diretamente
- definir a API publica detalhada de `workflow`
- impor um framework de servidor especifico
- esconder erros de bootstrap, duplicidade de registro ou falha de shutdown

## 3. API publica proposta

### 3.1 Pacotes publicos envolvidos

O contrato principal desta spec deve viver em `pkg/app`.

Os contratos consumidos por `App` dependem de:

- `pkg/app`
- `pkg/agent`
- `pkg/guardrail`
- `pkg/workflow`
- `pkg/logger`
- `pkg/memory`
- `pkg/types`

### 3.2 Tipos principais exportados por `pkg/app`

```go
package app

type App struct {
    // opaco e seguro para uso concorrente
}

type Config struct {
    Name     string
    Defaults Defaults
}

type Defaults struct {
    Metadata        types.Metadata
    Logger          logger.Logger
    ShutdownTimeout time.Duration
    Agent           AgentDefaults
    Workflow        WorkflowDefaults
}

type AgentDefaults struct {
    MaxSteps            int
    Metadata            types.Metadata
    Engine              agent.Engine
    Memory              memory.Store
    WorkingMemory       memory.WorkingMemoryFactory
    InputGuardrails     []guardrail.Input
    StreamGuardrails    []guardrail.Stream
    OutputGuardrails    []guardrail.Output
    Hooks               []agent.Hook
}

type WorkflowDefaults struct {
    Metadata types.Metadata
    Hooks    []workflow.Hook
    History  workflow.HistorySink
    Retry    workflow.RetryPolicy
}

type Option func(*options) error

func New(cfg Config, opts ...Option) (*App, error)

func (a *App) Name() string
func (a *App) State() State
func (a *App) Start(ctx context.Context) error
func (a *App) EnsureStarted(ctx context.Context) error
func (a *App) Run(ctx context.Context) error
func (a *App) Shutdown(ctx context.Context) error

func (a *App) Logger() logger.Logger
func (a *App) Defaults() DefaultsSnapshot
func (a *App) Runtime() Runtime
func (a *App) Agents() AgentRegistry
func (a *App) Workflows() WorkflowRegistry
```

### 3.3 Tipos auxiliares minimos de `pkg/app`

```go
type State string

const (
    StateCreated  State = "created"
    StateStarting State = "starting"
    StateRunning  State = "running"
    StateStopping State = "stopping"
    StateStopped  State = "stopped"
)

type DefaultsSnapshot struct {
    Metadata        types.Metadata
    ShutdownTimeout time.Duration
    Agent           AgentDefaultsSnapshot
    Workflow        WorkflowDefaultsSnapshot
}

type AgentDefaultsSnapshot struct {
    MaxSteps int
    Metadata types.Metadata
    HasEngine bool
    HasMemory bool
}

type WorkflowDefaultsSnapshot struct {
    Metadata types.Metadata
    HasHistory bool
    HasRetry   bool
}

type Runtime interface {
    State() State
    Logger() logger.Logger
    Defaults() DefaultsSnapshot
    ResolveAgent(name string) (*agent.Agent, error)
    ResolveWorkflow(name string) (workflow.Workflow, error)
    ListAgents() []agent.Descriptor
    ListWorkflows() []workflow.Descriptor
}
```

### 3.4 Registro declarativo no bootstrap

O `App` deve suportar dois modos de registro no bootstrap:

1. instancia pronta
2. factory declarativa que participa da hierarquia de defaults globais

API minima:

```go
type AgentRegistry interface {
    Register(agent *agent.Agent) error
    RegisterFactory(factory AgentFactory) error
    Resolve(name string) (*agent.Agent, error)
    List() []agent.Descriptor
}

type WorkflowRegistry interface {
    Register(workflow workflow.Workflow) error
    RegisterFactory(factory WorkflowFactory) error
    Resolve(name string) (workflow.Workflow, error)
    List() []workflow.Descriptor
}

type AgentFactory interface {
    Name() string
    Build(ctx context.Context, defaults AgentDefaults) (*agent.Agent, error)
}

type WorkflowFactory interface {
    Name() string
    Build(ctx context.Context, defaults WorkflowDefaults) (workflow.Workflow, error)
}
```

Opcoes minimas:

```go
func WithAgents(agents ...*agent.Agent) Option
func WithAgentFactories(factories ...AgentFactory) Option
func WithWorkflows(workflows ...workflow.Workflow) Option
func WithWorkflowFactories(factories ...WorkflowFactory) Option
func WithLogger(log logger.Logger) Option
func WithDefaults(defaults Defaults) Option
func WithAppHooks(hooks ...Hook) Option
func WithServers(servers ...Server) Option
func WithServerlessHooks(hooks ...ServerlessHook) Option
```

### 3.5 Contratos minimos consumidos pelo App

O `App` precisa apenas dos contratos minimos abaixo para interagir com `agent` e `workflow` nesta fase:

```go
package agent

type Descriptor struct {
    Name string
    ID   string
}
```

```go
package workflow

type Request struct{}

type Response struct{}

type Workflow interface {
    Name() string
    Run(ctx context.Context, req Request) (Response, error)
}

type Event struct {
    Type string
}

type Hook interface {
    OnEvent(ctx context.Context, event Event)
}

type HistoryEntry struct {
    Kind string
}

type HistorySink interface {
    Append(ctx context.Context, entry HistoryEntry) error
}

type RetryPolicy interface {
    Next(attempt int, err error) (time.Duration, bool)
}

type Descriptor struct {
    Name string
    ID   string
}
```

O restante da API de `workflow` deve ser detalhado em spec propria.

### 3.6 Observacoes normativas da superficie publica

- `New` apenas valida configuracao, prepara registries e mantem a aplicacao em `StateCreated`.
- `Start` materializa factories pendentes, sela registries e sobe o runtime.
- `EnsureStarted` deve ser idempotente e e o ponto recomendado para adapters serverless.
- `Run` deve ser um helper de conveniencia equivalente a `Start`, espera por `ctx.Done()` e depois executa `Shutdown`.
- `Shutdown` deve ser idempotente.
- `Runtime()` deve retornar uma view somente-leitura do runtime.
- `Agents()` e `Workflows()` podem expor operacoes de registro apenas enquanto o `App` estiver em `StateCreated`.

## 4. Registro de agents

O registro de agents precisa suportar descoberta deterministica por nome e erro explicito em caso de ambiguidade.

Regras obrigatorias:

- Cada agent deve possuir `Name` logico unico dentro do `App`.
- O registry deve rejeitar duplicidade por nome no momento do registro.
- `Register` com instancia pronta registra o agent como ele foi construido; defaults globais nao podem mutar essa instancia.
- `RegisterFactory` registra um provider declarativo que so e materializado em `Start` ou `EnsureStarted`.
- Se existir um agent pronto e uma factory com o mesmo nome, o bootstrap deve falhar.
- `Resolve` deve aceitar nome logico exato.
- `Resolve` de agent ausente deve retornar erro classificavel com `errors.Is`.
- `List` deve ser ordenado deterministicamente por nome.
- Apos `Start`, o registry de agents fica selado para novas insercoes.

Descoberta e materializacao:

1. `New` coleta agentes prontos e factories.
2. `Start` ou `EnsureStarted` materializa todas as factories em ordem deterministica por nome.
3. Agents resultantes entram no registry final e passam a ser resolvidos por `Runtime.ResolveAgent`.
4. Falha de uma unica factory impede a transicao para `StateRunning`.

## 5. Registro de workflows

O registro de workflows segue a mesma semantica basica do registro de agents, mas aplicado ao dominio de workflow.

Regras obrigatorias:

- Cada workflow deve possuir `Name` logico unico dentro do `App`.
- Duplicidade de nome deve falhar no bootstrap.
- `Register` com workflow pronto nao pode receber mutacao implicita de defaults globais.
- `RegisterFactory` participa da hierarquia de defaults globais e e materializado no startup.
- `Resolve` deve retornar erro explicito para workflow ausente.
- `List` deve ser ordenado deterministicamente por nome.
- Apos `Start`, o registry de workflows fica selado.

Interacao com o runtime:

- `Runtime.ResolveWorkflow` deve ser a forma canonica de descoberta por adapters.
- `App.Workflows().Resolve` pode ser usado antes de `Start` apenas para workflows prontos ja registrados.
- Workflows materializados por factory so sao garantidos apos `Start` bem-sucedido.

## 6. Hierarquia de defaults globais

O `App` deve suportar defaults globais composicionais e previsiveis.

### 6.1 Defaults suportados

Os defaults globais minimos da v1 sao:

- metadata global da aplicacao
- logger global
- timeout padrao de graceful shutdown
- defaults de agent
- defaults de workflow

Dentro de `AgentDefaults`, a v1 deve suportar ao menos:

- `MaxSteps`
- metadata
- `Engine`
- `Memory`
- `WorkingMemory`
- input guardrails
- stream guardrails
- output guardrails
- hooks

Dentro de `WorkflowDefaults`, a v1 deve suportar ao menos:

- metadata
- hooks
- history sink
- retry policy

### 6.2 Ordem de precedencia

A ordem normativa de precedencia deve ser:

1. override explicito por request ou execucao
2. configuracao local do agent ou workflow
3. defaults globais do `App`
4. defaults built-in do pacote

Exemplos normativos:

- se `Request.MaxSteps` for definido, ele prevalece sobre `AgentDefaults.MaxSteps`
- se um agent factory fornecer `Memory` proprio, ele prevalece sobre `Defaults.Agent.Memory`
- se um workflow factory nao fornecer `Retry`, ele herda `Defaults.Workflow.Retry`
- se nenhum logger for fornecido, o `App` usa `logger.Nop()`
- se nenhum `Defaults.Agent.Engine` for fornecido, o `App` injeta o engine interno padrao apenas para factories; instancias prontas registradas por `Register` continuam imutaveis

### 6.3 Regras de merge

- campos escalares usam estrategia "ultimo valor nao-zero vence"
- slices de hooks e guardrails usam merge por append ordenado
- metadata usa merge com sobrescrita de chaves mais especificas sobre chaves mais globais
- instancias prontas registradas por `Register` nao participam de merge estrutural; apenas accessors globais do `App` as observam

### 6.4 Momento de aplicacao

- defaults globais sao congelados ao final de `New`
- factories enxergam o snapshot de defaults congelado no momento de `Start`
- defaults nao podem mudar depois que o runtime entra em `StateRunning`
- o built-in de `ShutdownTimeout` na primeira versao funcional e `30s`

## 7. Integracao com logger

`App` deve ser a origem do logger global da aplicacao.

Regras obrigatorias:

- o logger global deve ser configuravel em `Config.Defaults.Logger` ou `WithLogger`
- se nenhum logger for informado, o default deve ser `logger.Nop()`
- `App.Logger()` deve sempre retornar uma instancia nao-nula
- startup, shutdown, falhas de bootstrap, duplicidade de registro e falhas de factories devem ser logaveis atraves desse logger
- adapters de server e serverless que recebam `Runtime` devem conseguir acessar o mesmo logger global
- agents e workflows criados por factory podem herdar esse logger indiretamente via defaults ou hooks, mas `App` nao deve reescrever internamente loggers privados de instancias prontas

## 8. Integracao com observability hooks locais

`App` deve suportar hooks locais de observabilidade para lifecycle e eventos de bootstrap.

API minima:

```go
type Hook interface {
    OnEvent(ctx context.Context, event Event)
}

type Event struct {
    Type     EventType
    AppName  string
    Time     time.Time
    Err      error
    Metadata types.Metadata
}

type EventType string

const (
    EventAppStarting          EventType = "app.starting"
    EventAppStarted           EventType = "app.started"
    EventAppStopping          EventType = "app.stopping"
    EventAppStopped           EventType = "app.stopped"
    EventAgentRegistered      EventType = "app.agent_registered"
    EventWorkflowRegistered   EventType = "app.workflow_registered"
    EventBootstrapFailed      EventType = "app.bootstrap_failed"
)
```

Semantica obrigatoria:

- hooks do `App` observam lifecycle e bootstrap; nao substituem hooks de `agent` nem de `workflow`
- hooks devem ser executados na ordem de registro
- panics em hooks devem ser recuperados e nao podem derrubar o processo
- hooks nao podem bloquear indefinidamente o shutdown; devem respeitar o `ctx`
- events de bootstrap devem usar a mesma metadata global da aplicacao, acrescida de campos especificos do evento

## 9. Integracao com server e serverless

O `App` deve oferecer integracao controlada com adapters de server e cenarios serverless sem acoplar o core a um transporte.

### 9.1 Contrato para servers long-lived

```go
type Server interface {
    Name() string
    Start(ctx context.Context, rt Runtime) error
    Shutdown(ctx context.Context) error
}
```

Regras obrigatorias:

- `WithServers` registra componentes long-lived gerenciados pelo lifecycle do `App`
- `Server.Start` e chamado apenas depois de registries materializados e runtime em estado consistente
- se qualquer `Server.Start` falhar, o `App` deve abortar o startup e iniciar rollback do que ja foi iniciado
- `Server.Shutdown` deve ser chamado em ordem reversa da inicializacao
- o `App` nao deve depender de nenhum pacote concreto de transporte para cumprir este contrato

### 9.2 Contrato para cenarios serverless

```go
type ServerlessHook interface {
    OnColdStart(ctx context.Context, rt Runtime) error
    OnInvokeStart(ctx context.Context, target Target) (context.Context, error)
    OnInvokeDone(ctx context.Context, target Target, err error)
}

type Target struct {
    Kind string
    Name string
}
```

Regras obrigatorias:

- `WithServerlessHooks` registra hooks voltados a cold start e invocacoes curtas
- `OnColdStart` deve ser disparado na primeira transicao bem-sucedida para runtime pronto em contexto serverless
- `EnsureStarted` e a API recomendada para cold start seguro
- `OnInvokeStart` e `OnInvokeDone` nao executam por conta propria; precisam ser chamados pelo adaptador serverless ao envolver uma invocacao
- `App` nao deve assumir existencia de processo residente no modo serverless

### 9.3 Fronteira de escopo

- esta spec define apenas o contrato local de integracao
- implementacoes concretas de HTTP, gRPC ou Lambda-like continuam fora do escopo desta fase

## 10. Runtime accessors

O `App` deve expor accessors estaveis para que adapters e codigo de composicao descubram capacidades registradas sem importar `internal/*`.

Regras obrigatorias:

- `Runtime()` retorna uma view somente-leitura do runtime e nunca o runtime interno concreto
- `Runtime.ResolveAgent(name)` retorna o agent materializado associado ao nome
- `Runtime.ResolveWorkflow(name)` retorna o workflow materializado associado ao nome
- `Runtime.ListAgents()` e `Runtime.ListWorkflows()` retornam snapshots ordenados deterministicamente
- `App.Agents()` e `App.Workflows()` podem ser usados para bootstrap antes do startup
- depois do startup, a forma recomendada de descoberta em adapters e `Runtime()`
- accessors devem ser seguros para concorrencia

## 11. Graceful shutdown

O `App` deve oferecer desligamento cooperativo, previsivel e seguro.

### 11.1 Ordem normativa de shutdown

Ao receber `Shutdown(ctx)` em `StateRunning`, a ordem minima deve ser:

1. transicionar para `StateStopping`
2. emitir `EventAppStopping`
3. impedir novos startups ou novos registros
4. sinalizar cancelamento para componentes long-lived do runtime
5. chamar `Shutdown` dos `Server` em ordem reversa da inicializacao
6. aguardar operacoes inflight do runtime terminarem ou o `ctx` expirar
7. flush final de hooks/logging locais que respeitem `ctx`
8. transicionar para `StateStopped`
9. emitir `EventAppStopped`

### 11.2 Regras de seguranca

- `Shutdown` deve ser idempotente
- se chamado em `StateCreated`, `Shutdown` deve apenas mover para `StateStopped` sem erro
- se chamado em `StateStopped`, deve retornar `nil`
- `Run(ctx)` deve chamar `Shutdown` automaticamente quando `ctx.Done()` for fechado
- o timeout efetivo de shutdown deve ser o menor entre:
  - deadline do `ctx` recebido
  - `Defaults.ShutdownTimeout`, quando configurado
- falha de shutdown de um `Server` nao deve impedir tentativa de desligar os demais
- se o `ctx` expirar durante o shutdown, o erro retornado deve preservar `context.DeadlineExceeded`
- runs ja em execucao devem receber cancelamento cooperativo; o `App` nao deve forcar terminacao abrupta sem dar chance ao runtime de encerrar

### 11.3 Startup rollback

Se `Start` falhar depois de inicializar parcialmente o runtime:

1. o `App` deve entrar em fluxo de rollback
2. todo `Server` ja iniciado deve receber `Shutdown`
3. factories parcialmente materializadas nao devem permanecer visiveis em `Runtime()`
4. o estado final deve ser `StateStopped`

## 12. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `pkg/app` expuser `App`, `Config`, `Defaults`, `Runtime` e `State`
2. `New` inicializar o `App` sem subir runtime nem materializar factories
3. `Start` materializar registries, aplicar defaults e levar o runtime a `StateRunning`
4. `EnsureStarted` for idempotente e seguro para concorrencia
5. duplicidade de agent ou workflow produzir erro explicito e deterministico
6. `Runtime.ResolveAgent` e `Runtime.ResolveWorkflow` permitirem descoberta por nome sem acesso a `internal/*`
7. defaults globais seguirem a ordem de precedencia documentada
8. logger global estiver disponivel para runtime e adapters
9. hooks locais observarem bootstrap e lifecycle sem controlar o fluxo principal
10. `Shutdown` encerrar o runtime de forma cooperativa e idempotente
11. rollback de startup parcial limpar servidores e estado observavel

## 13. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao futura sao:

1. `New` com configuracao valida cria `App` em `StateCreated`
2. erro ao criar `App` sem `Config.Name`
3. logger default e `logger.Nop()` quando nenhum logger for informado
4. registro de dois agents com o mesmo nome falha
5. registro de dois workflows com o mesmo nome falha
6. `Start` materializa agent factories e workflow factories em ordem deterministica
7. falha de uma agent factory impede `StateRunning`
8. falha de uma workflow factory impede `StateRunning`
9. `ResolveAgent` encontra instancia pronta registrada no bootstrap
10. `ResolveWorkflow` encontra workflow pronto registrado no bootstrap
11. `Runtime.ListAgents()` retorna nomes ordenados deterministicamente
12. `Runtime.ListWorkflows()` retorna nomes ordenados deterministicamente
13. defaults globais de agent sao aplicados a uma factory que nao define valores locais
14. override local da factory prevalece sobre default global
15. override de request prevalece sobre default global materializado no agent
16. instancia pronta registrada por `Register` nao recebe mutacao de defaults globais
17. hooks de `App` recebem `app.starting` e `app.started` em ordem
18. panic em hook de `App` e recuperado sem derrubar o startup
19. `EnsureStarted` chamado concorrentemente sobe o runtime uma unica vez
20. `Shutdown` em `StateCreated` leva a `StateStopped` sem erro
21. `Shutdown` em `StateStopped` retorna `nil`
22. `Shutdown` em `StateRunning` chama `Server.Shutdown` em ordem reversa
23. falha de `Server.Start` dispara rollback dos servers ja iniciados
24. expiracao de contexto durante `Shutdown` preserva `context.DeadlineExceeded`
25. `Run(ctx)` inicia o app, espera cancelamento e executa shutdown automatico

## 14. Questoes em aberto

1. Os tipos `AgentRegistry` e `WorkflowRegistry` devem permanecer em `pkg/app` ou merecem promocao para `pkg/agent` e `pkg/workflow` em specs proprias?
2. `workflow.Workflow` precisa de suporte a streaming ja no contrato minimo consumido por `App`, ou isso deve esperar a spec dedicada de workflow?
3. `Defaults.Agent.Engine` deve ser obrigatorio para todas as agent factories, ou pode existir um engine padrao injetado por `pkg/app`?
4. `EnsureStarted` deve ser publico na v1 ou restrito a adapters, mantendo `Start` como unica API de lifecycle explicitamente documentada?
5. O contrato de `ServerlessHook` precisa carregar metadata mais rica de invocacao na v1, ou `Target` simples e suficiente?
