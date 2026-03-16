# Spec 070: Workflows

## 1. Objetivo

Este documento define a engine de `workflows` da `gaal-lib`.

Os objetivos desta spec sao:

- estabelecer o contrato publico de `pkg/workflow` para composicao e execucao de workflows
- definir `workflow chain`, `workflow branching`, `workflow retry`, `workflow hooks` e `workflow execution history` como capacidades normativas do modulo
- preservar paridade conceitual com o core do Voltagent sem copiar sua API textual
- manter uma API idiomatica em Go com `builder` para composicao e `runnable workflow` para execucao
- preparar a base para integracao futura com `agents`, `guardrails`, `memory` e storage de historico sem ampliar o escopo para VoltOps
- preparar o modelo de estado necessario para `suspend/resume`, mesmo que a API publica de `resume` fique para fase posterior

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020`, a `Spec 031`, a `Spec 050` e a `Spec 060`.

Compatibilidade com a `Spec 010`:

- `Workflow chain` continua `Sim`, `P0`
- `Workflow branching`, `Workflow retry`, `Workflow hooks` e `Workflow execution history` continuam `Sim`, `P1`
- esta spec detalha o contrato dessas features, mas nao muda por si so o status de implementacao na matriz

Ficam fora do escopo desta spec:

- control plane, dashboard, scheduler operacional, replay remoto ou qualquer capacidade de VoltOps
- dependencia obrigatoria de servico hospedado para executar workflows localmente
- paralelismo implicito de steps na v1
- `resume` distribuido, retomada remota de processo ou orquestracao operacional entre processos
- loops arbitrarios ou grafos gerais sem validacao adicional; a v1 cobre cadeia sequencial com branching controlado

## 2. Conceitos principais

- `Workflow builder`: artefato mutavel de composicao usado para registrar steps, hooks, retry, history e metadata antes da execucao.
- `Runnable workflow`: artefato imutavel produzido pelo builder, seguro para uso concorrente e executado via `Run`.
- `Workflow run`: uma execucao individual identificada por `RunID`, com estado proprio, hooks, historico e cancelamento por `context.Context`.
- `Step`: unidade de trabalho de um workflow. Cada step recebe um contexto compartilhado do run e devolve resultado, transicao ou erro.
- `Chain of steps`: ordem deterministica de steps definida no builder. Na ausencia de desvio explicito, o runtime avanca para o proximo step declarado.
- `Branching`: decisao condicional que escolhe qual sera o proximo step, encerra o workflow ou solicita suspensao.
- `Shared context`: contexto compartilhado do workflow exposto como `State`, mutavel apenas dentro do run corrente e visivel aos steps seguintes.
- `Workflow state`: estado run-scoped do workflow. Ele nao substitui memoria conversacional nem historico duravel.
- `Retry policy`: politica de repeticao aplicada por step, com heranca `App defaults -> Workflow -> Step`.
- `Hook`: observador local do ciclo de vida do workflow. Hooks nao controlam o fluxo principal nem podem mutar retroativamente o resultado.
- `Execution history`: trilha observavel local e plugavel de eventos do workflow, persistida por `HistorySink`.
- `Checkpoint`: snapshot observavel de estado e posicao usado como base para `suspend/resume` futuro.

Regras normativas:

1. O workflow deve ser executavel apenas com contratos publicos de `pkg/workflow`, sem exigir importacao de `internal/*`.
2. O workflow deve ser seguro para uso concorrente depois de construido.
3. O workflow deve obedecer a `context.Context` como fronteira canonica de cancelamento, deadline e tracing local.
4. Nenhuma feature desta spec pode exigir VoltOps ou servico hospedado para funcionar localmente.

## 3. Builder vs runnable workflow

O contrato principal deve viver em `pkg/workflow`.

API publica proposta:

```go
package workflow

import (
    "context"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Workflow struct {
    // opaco e imutavel apos Build/New
}

type Builder struct {
    // mutavel apenas durante composicao
}

type Option func(*options) error

func New(name string, opts ...Option) (*Workflow, error)
func NewBuilder(name string) *Builder

func (b *Builder) Step(step Step) *Builder
func (b *Builder) WithMetadata(md types.Metadata) *Builder
func (b *Builder) WithHooks(hooks ...Hook) *Builder
func (b *Builder) WithRetry(policy RetryPolicy) *Builder
func (b *Builder) WithHistory(sink HistorySink) *Builder
func (b *Builder) Build() (*Workflow, error)

func (w *Workflow) Name() string
func (w *Workflow) ID() string
func (w *Workflow) Descriptor() Descriptor
func (w *Workflow) Definition() Definition
func (w *Workflow) Run(ctx context.Context, req Request) (Response, error)

type Descriptor struct {
    Name string
    ID   string
}

type Definition struct {
    Descriptor Descriptor
    Steps      []StepDescriptor
    Hooks      []Hook
    Retry      RetryPolicy
    History    HistorySink
    Metadata   types.Metadata
}

type StepDescriptor struct {
    Name string
    Kind StepKind
}
```

Regras normativas:

1. `Builder` existe apenas para composicao. Ele nao e seguro para uso concorrente.
2. `Build` deve validar a definicao e produzir um `Workflow` imutavel.
3. `Workflow` deve continuar satisfazendo o contrato minimo ja consumido por `pkg/app`: `Name()` e `Run(ctx, req)`.
4. `Definition()` deve retornar um snapshot somente-leitura da definicao efetiva do workflow.
5. `New(name, opts...)` e um atalho equivalente a `NewBuilder(name)` seguido de configuracao por `Option` e `Build()`.
6. `Name` e obrigatorio e nao pode ser vazio.
7. O builder deve rejeitar nomes duplicados de step.
8. O builder deve rejeitar workflow sem steps.
9. Um `Workflow` registrado pronto em `pkg/app` nao deve sofrer mutacao estrutural retroativa por defaults globais, conforme a `Spec 031`.
10. Factories de workflow materializadas via `pkg/app` podem herdar `Hooks`, `History`, `Retry` e metadata de `App.Defaults.Workflow`, conforme a `Spec 031`.

## 4. Modelo de dados e state

API publica proposta:

```go
package workflow

import (
    "time"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Request struct {
    RunID     string
    SessionID string
    Input     map[string]any
    State     StateSnapshot
    Metadata  types.Metadata
}

type Response struct {
    RunID        string
    WorkflowID   string
    WorkflowName string
    SessionID    string
    Status       Status
    CurrentStep  string
    Output       map[string]any
    State        StateSnapshot
    Checkpoint   *Checkpoint
    Metadata     types.Metadata
}

type Status string

const (
    StatusCompleted Status = "completed"
    StatusFailed    Status = "failed"
    StatusCanceled  Status = "canceled"
    StatusSuspended Status = "suspended"
)

type State struct {
    // opaco; mutavel apenas dentro do run
}

type StateSnapshot map[string]any

func NewState(initial StateSnapshot) *State
func (s *State) Get(key string) (any, bool)
func (s *State) Set(key string, value any)
func (s *State) Delete(key string)
func (s *State) Snapshot() StateSnapshot

type StepContext struct {
    WorkflowID   string
    WorkflowName string
    RunID        string
    SessionID    string
    StepName     string
    Attempt      int
    Input        map[string]any
    State        *State
    Metadata     types.Metadata
}

type StepResult struct {
    Output   map[string]any
    Next     Next
    Metadata types.Metadata
}

type Next struct {
    Step    string
    End     bool
    Suspend bool
}

type Checkpoint struct {
    StepName  string
    State     StateSnapshot
    Time      time.Time
    Metadata  types.Metadata
}
```

Regras normativas:

1. `Request.Input` representa o input inicial do workflow.
2. `Request.State` representa um snapshot inicial opcional do estado compartilhado.
3. O runtime deve criar um `State` isolado por run a partir de `Request.State`.
4. `State` e o contexto compartilhado canonico entre steps.
5. Se um step quiser compartilhar dados com os proximos steps, ele deve escrever explicitamente em `State`.
6. `StepResult.Output` e um payload observavel do step corrente; ele nao deve ser mesclado implicitamente em `State`.
7. `Response.Output` deve refletir o `Output` do ultimo step concluido com sucesso antes do termino do workflow.
8. `Response.State` deve ser o snapshot final do estado compartilhado no termino do run.
9. `StatusSuspended` e valido quando um step solicitar suspensao observavel.
10. `Checkpoint` existe para preparar `suspend/resume`. A v1 exige sua formacao e observabilidade, mas nao exige uma API publica de `Resume`.
11. `SessionID` e metadata opcional do run; ele nao transforma o workflow em memoria conversacional por si so.
12. O estado do workflow e sempre `run-scoped`; ele nao substitui `memory.Store`, `working memory` de `Agent` nem `workflow history`.

## 5. Tipos de step

API publica proposta:

```go
package workflow

import (
    "context"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Step interface {
    Name() string
    Kind() StepKind
    Run(ctx context.Context, stepCtx StepContext) (StepResult, error)
}

type StepKind string

const (
    StepKindAction StepKind = "action"
    StepKindBranch StepKind = "branch"
    StepKindInvoke StepKind = "invoke"
)

type StepFunc func(ctx context.Context, stepCtx StepContext) (StepResult, error)
type DecisionFunc func(ctx context.Context, stepCtx StepContext) (Decision, error)

func Action(name string, fn StepFunc, opts ...StepOption) Step
func Branch(name string, fn DecisionFunc, opts ...StepOption) Step

type Decision struct {
    Step     string
    End      bool
    Suspend  bool
    Reason   string
    Metadata types.Metadata
}

type StepOption func(*stepOptions) error
func WithStepRetry(policy RetryPolicy) StepOption
func WithStepMetadata(md types.Metadata) StepOption
```

Tipos normativos minimos:

1. `Action step`: step de logica local em Go, adequado para transformacoes, validacoes, IO local e side effects controlados.
2. `Branch step`: step de decisao condicional que escolhe o proximo step, encerra o workflow ou solicita suspensao.
3. `Invoke step`: step adaptador para invocar capacidades externas ao core do workflow, por exemplo um `Agent`, uma `Tool` ou outro executor plugavel. O core de `pkg/workflow` nao precisa conhecer o tipo concreto invocado.

Regras normativas:

1. Todo step deve possuir `Name` unico dentro do workflow.
2. Todo step deve informar um `Kind` observavel.
3. Todo step deve ser seguro para uso concorrente depois de registrado no `Workflow`.
4. Todo step recebe snapshots defensivos de `Input` e `Metadata`, alem do acesso controlado a `State`.
5. `Branch` e um helper de composicao; internamente ele continua sendo um `Step`.
6. `Invoke step` existe como conceito normativo, mas o adapter concreto pode viver fora do core de `pkg/workflow` para preservar fronteiras arquiteturais.

## 6. Encadeamento

O `workflow chain` deve ser deterministico e sequencial na v1.

Ordem normativa de execucao:

1. Validar `context.Context`, `Workflow`, `Request` e a definicao efetiva do run.
2. Criar `RunID` se ele nao vier no request.
3. Materializar metadata efetiva do run.
4. Criar `State` isolado a partir de `Request.State`.
5. Resolver hooks, history sink e retry efetivos do workflow.
6. Emitir `onStart`.
7. Executar o primeiro step declarado.
8. Para cada step e tentativa:
   - emitir `onStepStart`
   - executar `Step.Run`
   - se houver sucesso, emitir `onStepEnd`
   - resolver a proxima transicao
9. Ao atingir termino bem-sucedido ou suspensao, emitir `onFinish`.
10. Em qualquer encerramento, emitir `onEnd`.

Regras normativas:

1. Na ausencia de `Next.Step`, `Next.End` e `Next.Suspend`, o runtime deve avancar para o proximo step declarado no builder.
2. O ultimo step declarado, quando bem-sucedido e sem transicao explicita, encerra o workflow com `StatusCompleted`.
3. O workflow nao deve executar steps em paralelo por default na v1.
4. O contexto compartilhado deve ser visivel a cada step na ordem real de execucao.
5. Mudancas em `State` feitas por um step bem-sucedido devem ficar visiveis ao step seguinte.
6. Se `Step.Run` retornar erro e nao houver retry efetivo restante, o workflow falha.
7. Cancelamento por `context.Context` deve interromper o workflow cooperativamente o mais cedo possivel.
8. O runtime nao deve inventar steps, transicoes ou side effects que nao estejam na definicao do workflow.

## 7. Branching e decisao condicional

O `workflow branching` deve ser expresso como decisao observavel e testavel.

Semantica normativa:

1. Um `Branch step` pode escolher exatamente uma destas saidas:
   - ir para um `Next.Step` nomeado
   - encerrar com `End = true`
   - suspender com `Suspend = true`
2. Combinacoes invalidas entre `Step`, `End` e `Suspend` devem falhar em modo fail-closed.
3. `Next.Step` deve referenciar um step existente na definicao do workflow.
4. A v1 deve suportar apenas branching controlado sobre o grafo declarado pelo builder.
5. O builder deve rejeitar configuracoes obviamente invalidas, como step de branch apontando para nome vazio quando a decisao e estatica.
6. Se a decisao dinamica resolver para step inexistente em runtime, o workflow deve falhar com erro classificavel de transicao invalida.
7. Quando `End = true`, o workflow termina com `StatusCompleted`.
8. Quando `Suspend = true`, o workflow termina com `StatusSuspended`, produz `Checkpoint` e nao executa steps posteriores.

Regras adicionais:

- branching e resolvido depois de um step bem-sucedido
- branching nao substitui retry; falha do step continua sujeita primeiro a retry e apenas depois a termino em erro
- `Branch step` pode ler qualquer dado do `State` para tomar decisao
- a v1 nao exige linguagem declarativa de expressoes; funcao Go ou adapter equivalente e suficiente

## 8. Retry policy

O `workflow retry` deve ser explicito, herdavel e sobrescrevivel.

Contrato minimo:

```go
package workflow

import "time"

type RetryPolicy interface {
    Next(attempt int, err error) (time.Duration, bool)
}
```

Semantica normativa:

1. Retry e aplicado por step, nao por workflow inteiro.
2. `attempt` recebido por `RetryPolicy.Next` representa o numero da falha do step corrente, iniciando em `1`.
3. Retornar `(delay, true)` significa "tente novamente apos `delay`".
4. Retornar `(_, false)` significa "nao tente novamente".
5. Na ausencia de retry configurado em qualquer nivel, o default e "sem retry".

Hierarquia normativa:

1. retry local do step
2. retry local do workflow
3. `App.Defaults.Workflow.Retry`
4. default built-in do pacote

Regras normativas:

1. A regra e "o mais especifico vence".
2. Workflow pronto registrado diretamente em `App` nao deve receber override retroativo de retry global.
3. Retry deve respeitar cancelamento e deadline do `context.Context`.
4. Se o contexto for cancelado durante o backoff, o workflow termina como cancelado e nao tenta novamente.
5. Cada nova tentativa do mesmo step deve disparar novo `onStepStart`.
6. Toda falha de tentativa deve disparar `onError`, mesmo quando houver novo retry agendado.
7. O historico deve registrar cada tentativa, falha e retry agendado.
8. A implementacao nao deve aplicar retry implicito fora do contrato configurado.
9. Steps com side effects externos devem ser tratados como candidatos a idempotencia; se nao forem idempotentes, a responsabilidade de documentar e aceitar o risco e do step ou adapter concreto.

## 9. Hooks do ciclo de vida

O workflow deve suportar hooks locais com os conceitos minimos `onStart`, `onStepStart`, `onStepEnd`, `onError`, `onFinish` e `onEnd`.

Para manter compatibilidade com `App.Defaults.Workflow.Hooks`, o contrato base continua orientado a evento.

API publica proposta:

```go
package workflow

import (
    "context"
    "time"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Hook interface {
    OnEvent(ctx context.Context, event Event)
}

type Event struct {
    Type         EventType
    WorkflowID   string
    WorkflowName string
    RunID        string
    SessionID    string
    StepName     string
    Attempt      int
    Status       Status
    Output       map[string]any
    State        StateSnapshot
    Err          error
    Time         time.Time
    Metadata     types.Metadata
}

type EventType string

const (
    EventWorkflowStarted   EventType = "workflow.started"
    EventStepStarted       EventType = "workflow.step_started"
    EventStepEnded         EventType = "workflow.step_ended"
    EventWorkflowError     EventType = "workflow.error"
    EventWorkflowFinished  EventType = "workflow.finished"
    EventWorkflowEnded     EventType = "workflow.ended"
)

type LifecycleHooks struct {
    OnStart     func(ctx context.Context, event Event)
    OnStepStart func(ctx context.Context, event Event)
    OnStepEnd   func(ctx context.Context, event Event)
    OnError     func(ctx context.Context, event Event)
    OnFinish    func(ctx context.Context, event Event)
    OnEnd       func(ctx context.Context, event Event)
}
```

Mapeamento normativo:

- `onStart` corresponde a `workflow.started`
- `onStepStart` corresponde a `workflow.step_started`
- `onStepEnd` corresponde a `workflow.step_ended`
- `onError` corresponde a `workflow.error`
- `onFinish` corresponde a `workflow.finished`
- `onEnd` corresponde a `workflow.ended`

Regras normativas:

1. `onStart` deve executar uma unica vez por run, apos validacao e antes do primeiro step.
2. `onStepStart` deve executar antes de cada tentativa de step.
3. `onStepEnd` deve executar apenas para tentativa concluida com sucesso.
4. `onError` deve executar em toda falha observavel de step ou de workflow, inclusive falha que ainda sera seguida de retry.
5. `onFinish` deve executar apenas quando o workflow terminar com `StatusCompleted` ou `StatusSuspended`.
6. `onEnd` deve executar exatamente uma vez e sempre por ultimo, independentemente de sucesso, falha, cancelamento ou suspensao.
7. Hooks devem observar a ordem cronologica real do run.
8. Hooks sao observadores; eles nao podem alterar o fluxo principal do workflow.
9. Panics em hooks devem ser recuperados pelo runtime e tratados como diagnostico local; nao podem derrubar o processo nem mudar o resultado do workflow.
10. A cadeia efetiva de hooks deve ser congelada no inicio do run.
11. Hooks herdados de `App.Defaults.Workflow.Hooks` executam antes dos hooks locais do workflow, seguindo a regra de append ordenado da `Spec 031`.

## 10. Historico de execucao

O `workflow execution history` deve ser local, plugavel e separado de memoria conversacional.

API publica proposta:

```go
package workflow

import (
    "context"
    "time"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type HistoryEntry struct {
    Kind         string
    WorkflowID   string
    WorkflowName string
    RunID        string
    SessionID    string
    StepName     string
    Attempt      int
    Status       Status
    Time         time.Time
    Output       map[string]any
    Checkpoint   *Checkpoint
    Metadata     types.Metadata
}

type HistorySink interface {
    Append(ctx context.Context, entry HistoryEntry) error
}
```

Entradas minimas obrigatorias:

- `workflow.started`
- `workflow.step_started`
- `workflow.step_ended`
- `workflow.retry_scheduled`
- `workflow.error`
- `workflow.finished`
- `workflow.ended`
- `workflow.suspended`

Regras normativas:

1. `HistorySink` e opcional. Ausencia de sink significa "sem persistencia duravel de historico", nao erro de configuracao.
2. Quando configurado, o sink deve receber entradas em ordem cronologica real.
3. Historico e append-only do ponto de vista observavel do runtime.
4. Historico deve registrar ao menos inicio do run, inicio e fim de cada step, retries, erros e termino.
5. Em `StatusSuspended`, o historico deve registrar `Checkpoint` suficiente para retomada futura.
6. Historico de workflow nao deve ser misturado com `memory.Store`, conforme a `Spec 050`.
7. Historico nao deve ser carregado automaticamente como contexto do proximo run.
8. Historico existe para auditoria local e continuidade futura, nao para observabilidade hospedada.
9. Se `HistorySink.Append` falhar em uma entrada obrigatoria, o workflow deve falhar antes de reportar sucesso terminal.
10. Falha de historico nao pode impedir a execucao de `onEnd`.

## 11. Integracao com memory/storage

A integracao com `memory` e `storage` deve preservar as fronteiras definidas pela `Spec 050`.

Regras normativas:

1. O core do workflow nao deve chamar `memory.Store.Load` nem `memory.Store.Save` automaticamente.
2. `Workflow state` e `shared context` continuam `run-scoped`.
3. Persistencia duravel de historico deve usar `workflow.HistorySink`, nao `memory.Store`.
4. Se um workflow precisar ler ou gravar memoria conversacional, isso deve ocorrer por step ou adapter explicito, nao por comportamento implicito do engine.
5. `SessionID` em `workflow.Request` pode ser propagado a adapters, mas nao cria memoria conversacional por default.
6. O workflow nao deve reinterpretar `App.Defaults.Workflow.History` como `memory.Store`.
7. `Checkpoint` de suspensao pode ser persistido pelo `HistorySink` ou por storage complementar do adapter, desde que a semantica observavel continue local e plugavel.
8. A v1 nao exige interface publica adicional de `CheckpointStore`; o `HistorySink` e suficiente como base para continuidade futura.

Consequencias arquiteturais:

- memoria conversacional permanece no dominio de `pkg/memory`
- historico de workflow permanece no dominio de `pkg/workflow`
- um adapter persistente pode armazenar ambos em backend comum, mas deve preservar contratos publicos separados

## 12. Integracao com agents

O workflow deve preparar integracao com `agents` e `guardrails` sem violar a fronteira arquitetural da `Spec 020`.

Regras normativas:

1. O core de `pkg/workflow` nao deve depender diretamente de `pkg/agent`.
2. Integracao com agent deve ocorrer por `invoke steps` ou adapters externos ao core de `pkg/workflow`.
3. Um adapter de agent step deve:
   - ler `StepContext`
   - montar um `agent.Request`
   - executar o agent
   - projetar `agent.Response` de volta para `State` e `StepResult`
4. O workflow core deve permanecer agnostico ao tipo concreto do executor chamado pelo step.

Integracao com guardrails:

1. Quando um `invoke step` encapsular um `Agent`, input/output/stream guardrails do agent continuam governados pela `Spec 060`.
2. O workflow nao deve duplicar a execucao de guardrails do agent no nivel do engine de workflow.
3. Hooks e historico de workflow observam o step antes e depois da invocacao do agent, mas nao substituem hooks nem eventos internos do proprio agent.
4. Uma falha classificada de guardrail dentro do agent deve chegar ao workflow como erro do step, sujeita a retry somente se a politica de retry do step ou do workflow assim permitir.

Ordem observavel recomendada para um step adaptador de agent:

1. `onStepStart` do workflow
2. `Agent.Run` ou executor equivalente
3. hooks, memory e guardrails internos do agent
4. `onStepEnd` do workflow em sucesso, ou `onError` em falha

Preparacao para evolucao futura:

- a v1 cobre steps adaptadores de agent
- streaming de agent dentro de workflow pode ser especificado depois sem quebrar o contrato sequencial basico
- workflow-level guardrails dedicados podem ser promovidos depois, sem conflitar com `pkg/guardrail`

## 13. Casos de teste obrigatorios

Toda implementacao futura desta spec deve cobrir, no minimo, os casos abaixo.

1. `Builder` valido criando workflow com dois steps encadeados.
2. Erro ao construir workflow sem `Name`.
3. Erro ao construir workflow sem steps.
4. Erro ao registrar dois steps com o mesmo nome.
5. Execucao sequencial bem-sucedida com ordem identica a declaracao no builder.
6. `State` compartilhado sendo escrito por um step e lido pelo step seguinte.
7. `StepResult.Output` nao sendo mesclado implicitamente em `State`.
8. Ultimo step sem transicao explicita encerrando workflow com `StatusCompleted`.
9. `Branch step` direcionando para um step nomeado existente.
10. Branch invalido para step inexistente falhando com erro classificavel.
11. `Branch step` com `End = true` encerrando workflow sem steps adicionais.
12. `Branch step` com `Suspend = true` retornando `StatusSuspended` e `Checkpoint`.
13. Retry herdado de `App.Defaults.Workflow.Retry` sendo aplicado quando o workflow nao define retry proprio.
14. Retry local do workflow sobrescrevendo retry global.
15. Retry local do step sobrescrevendo retry do workflow.
16. Contagem correta de tentativas passadas para `RetryPolicy.Next`, iniciando em `1`.
17. Esgotamento de retry encerrando o workflow com falha terminal.
18. `onStart`, `onStepStart`, `onStepEnd`, `onFinish` e `onEnd` observando ordem correta em sucesso.
19. `onError` sendo emitido antes do novo retry quando uma tentativa falha.
20. `onError` sendo emitido em falha terminal sem `onFinish`.
21. `onEnd` sendo executado em sucesso, falha, cancelamento e suspensao.
22. Panic em hook sendo recuperado sem derrubar o run.
23. Historico registrando entradas obrigatorias em ordem cronologica.
24. Falha em `HistorySink.Append` invalidando sucesso terminal quando sink estiver configurado.
25. Workflow sem `HistorySink` configurado executando normalmente.
26. Cancelamento por `context.Context` interrompendo novas tentativas e retornando termino cancelado.
27. `invoke step` adaptando um agent sem que `pkg/workflow` precise importar `pkg/agent`.
28. Falha de guardrail dentro de agent step chegando ao workflow como erro de step.
29. `SessionID` sendo propagado ao adapter sem criar persistencia implicita de memoria conversacional.
30. Historico de workflow sendo persistido sem aparecer em `memory.Store`.
31. `Checkpoint` de suspensao contendo `StepName`, `State` e metadata suficientes para retomada futura.
32. Workflow pronto registrado diretamente em `App` nao recebendo mutacao estrutural retroativa de defaults globais.

## 14. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando todos os itens abaixo forem verdadeiros:

1. `pkg/workflow` expuser contrato publico para `Builder`, `Workflow`, `Step`, `Request`, `Response`, `State`, `RetryPolicy`, `Hook` e `HistorySink`.
2. O workflow puder ser construido de forma valida e imutavel, e executado com seguranca para concorrencia apos `Build`.
3. A engine suportar `steps encadeados`, execucao passo a passo e `contexto compartilhado` observavel entre steps.
4. Branching permitir escolha condicional do proximo step, termino antecipado e base para suspensao.
5. Retry obedecer a hierarquia `App defaults -> Workflow -> Step`, com sobrescrita pelo nivel mais especifico.
6. Hooks `onStart`, `onStepStart`, `onStepEnd`, `onError`, `onFinish` e `onEnd` observarem a ordem normativa e nao controlarem o fluxo principal.
7. Historico de execucao ser plugavel, local, observavel e separado de memoria conversacional.
8. A integracao com agents ocorrer por adapter de step, sem dependencia direta de `pkg/workflow` sobre `pkg/agent`.
9. Guardrails de agent continuarem governados pela `Spec 060` quando um step encapsular um agent.
10. O modelo de `Checkpoint` e `StatusSuspended` existir como base para `suspend/resume`, mesmo sem API publica de `Resume` nesta fase.
11. Nenhuma parte da feature exigir VoltOps ou servico hospedado para funcionar localmente.
12. Toda divergencia intencional de API em relacao ao Voltagent permanecer justificada por idiomaticidade em Go e compatibilidade com as specs do repositorio.
