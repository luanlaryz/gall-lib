# Spec 040: Tools and Toolkits

## 1. Objetivo

Este documento define o sistema de `tools` e `toolkits` da `gaal-lib`.

Os objetivos desta spec sao:

- estabelecer um contrato publico idiomatico em Go para tools invocaveis pelo runtime
- definir como toolkits agrupam e distribuem conjuntos coerentes de tools
- especificar schema de entrada e de saida, incluindo o subconjunto validado na v1
- definir regras de execucao, validacao, contexto operacional, registro e resolucao
- preparar compatibilidade conceitual com tools e toolkits do Voltagent sem copiar sua API textual

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020` e a `Spec 030`.

Quando houver mais detalhe sobre comportamento de tools neste documento, ele prevalece sobre referencias resumidas em specs mais gerais.

Ficam fora do escopo desta spec:

- catalogos remotos de tools
- discovery hospedado, marketplace ou qualquer capacidade de VoltOps
- retries implicitos, fallback automatico ou paralelizacao obrigatoria de tool calls
- plugins binarios dinamicos ou carregamento de codigo externo em runtime

## 2. Conceitos principais

- `Tool`: capacidade invocavel pelo runtime por meio de um contrato observavel de nome, descricao, schema de entrada, schema de saida e execucao.
- `Toolkit`: agrupamento declarativo de tools relacionadas, normalmente compartilhando namespace, dependencias ou proposito.
- `Input schema`: contrato normativo usado para validar os argumentos recebidos pela tool antes da execucao.
- `Output schema`: contrato normativo usado para validar o resultado estruturado retornado pela tool apos a execucao.
- `Local name`: nome proprio da tool, sem namespace.
- `Effective name`: nome final usado para registro e resolucao. E calculado a partir do namespace do toolkit quando existir.
- `Standalone tool`: tool registrada diretamente, sem passar por toolkit.
- `Contributed tool`: tool exposta por um toolkit.
- `Registry`: componente publico responsavel por registrar, listar e resolver tools e toolkits de forma deterministica.
- `Operational context`: informacoes de execucao visiveis para a tool, como `RunID`, `SessionID`, `AgentID`, metadata e cancelamento via `context.Context`.

Compatibilidade conceitual com Voltagent:

- `gaal-lib` preserva o modelo mental de "tool como capacidade invocavel" e "toolkit como agrupamento reutilizavel".
- A divergencia idiomatica em Go e deliberada: a composicao ocorre por interfaces, registries e structs, nao por APIs dinamicas de runtime.
- O `Agent` continua consumindo uma lista plana de `tool.Tool`, conforme a `Spec 030`; `Toolkit` e um mecanismo de composicao e distribuicao, nao um segundo motor de execucao.

## 3. Interface de Tool

O contrato principal deve viver em `pkg/tool`.

API publica proposta:

```go
package tool

import (
    "context"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Tool interface {
    Name() string
    Description() string
    InputSchema() Schema
    OutputSchema() Schema
    Call(ctx context.Context, call Call) (Result, error)
}

type Schema struct {
    Type                 string
    Description          string
    Properties           map[string]Schema
    Items                *Schema
    Required             []string
    Enum                 []string
    AdditionalProperties *bool
}

type Call struct {
    ID        string
    ToolName  string
    RunID     string
    SessionID string
    AgentID   string
    Input     map[string]any
    Metadata  types.Metadata
}

type Result struct {
    Value    any
    Content  string
    Metadata types.Metadata
}

type Descriptor struct {
    Name         string
    LocalName    string
    Description  string
    Toolkit      string
    Namespace    string
    InputSchema  Schema
    OutputSchema Schema
}
```

Regras normativas da interface:

1. `Name()` deve retornar o `local name` da tool, nao o `effective name`.
2. `Description()` deve ser obrigatoria e descrever o comportamento observavel da tool.
3. `InputSchema()` e `OutputSchema()` devem ser contratos completos e imutaveis apos o registro.
4. `Call` deve honrar `context.Context` para cancelamento, deadline e tracing local.
5. `Call` nao deve assumir acesso a `internal/*`, `pkg/app` ou detalhes do runtime.
6. A tool deve ser segura para uso concorrente depois de registrada.
7. A tool nao deve aplicar retry implicito por conta propria quando isso alterar a semantica observavel sem documentacao explicita.
8. A tool deve tratar `Call.Input` e `Call.Metadata` como snapshots imutaveis.

## 4. Interface de Toolkit

`Toolkit` deve ser um contrato de composicao em `pkg/tool`.

API publica proposta:

```go
package tool

type Toolkit interface {
    Name() string
    Description() string
    Namespace() string
    Tools() []Tool
}

type ToolkitDescriptor struct {
    Name        string
    Description string
    Namespace   string
    ToolCount   int
}
```

Regras normativas da interface:

1. `Name()` identifica logicamente o toolkit para diagnostico, docs e listagem.
2. `Description()` deve descrever o proposito do conjunto.
3. `Namespace()` e opcional, mas quando fornecido participa do calculo do `effective name`.
4. `Tools()` deve retornar um conjunto deterministico e semanticamente equivalente entre chamadas repetidas.
5. `Tools()` nao pode conter `nil`.
6. Um toolkit pode compartilhar dependencias entre suas tools, mas deve encapsular essa composicao sem expor tipos de `internal/*`.
7. O toolkit nao executa tool calls; ele apenas contribui tools para registro e resolucao.

Divergencia intencional em relacao ao Voltagent:

- `gaal-lib` modela toolkit como primitive de composicao em `pkg/tool`, em vez de exigir uma camada publica separada para execucao.
- Esta diferenca e autorizada pela `Spec 000` e pela `Spec 020`, desde que a semantica conceitual de agrupamento e distribuicao seja preservada.

## 5. Regras de naming e conflitos

### 5.1 Regras de naming

- `local name` de tool deve obedecer ao regex `^[a-z][a-z0-9_-]{0,63}$`.
- `namespace` de toolkit, quando fornecido, deve obedecer ao mesmo regex.
- `Toolkit.Name()` tambem deve obedecer ao mesmo regex.
- Nomes devem ser ASCII lowercase. Espacos, acentos, ponto e camelCase nao sao validos.
- O `effective name` deve ser calculado assim:
  - standalone tool: `effective name = local name`
  - tool via toolkit com namespace: `effective name = namespace + "." + local name`
  - tool via toolkit sem namespace: `effective name = local name`

### 5.2 Regras de conflito

- Duas standalone tools com o mesmo `effective name` devem falhar no registro.
- Duas tools dentro do mesmo toolkit com o mesmo `local name` devem falhar no registro do toolkit.
- Dois toolkits com o mesmo `Name()` devem falhar no registro.
- Duas tools de toolkits diferentes que colidam no mesmo `effective name` devem falhar no registro.
- Uma standalone tool nao pode sobrescrever silenciosamente uma tool contribuidora de toolkit, nem o inverso.
- Nao pode haver auto-renomeacao, sufixo numerico ou fallback implicito para resolver conflitos.
- Resolucao deve ser sempre por match exato do `effective name`.

### 5.3 Consequencias observaveis

- Todo conflito de nome e erro de configuracao ou de registro, nunca erro tardio de execucao.
- O runtime do `Agent` nao deve receber listas ambiguas de tools.
- Toda mensagem, evento e erro observavel deve usar o `effective name`.

## 6. Input/output contracts

### 6.1 Modelo de schema

A v1 garante validacao de um subconjunto de JSON Schema conceitualmente compativel com tool calling moderno:

- `type`
- `description`
- `properties`
- `items`
- `required`
- `enum`
- `additionalProperties`

Qualquer keyword adicional pode ser preservada por adaptadores futuros, mas nao faz parte da garantia minima de validacao da v1 sem nova spec.

### 6.2 Regras do input

- `InputSchema().Type` deve ser obrigatoriamente `object`.
- `Call.Input` deve ser um mapa JSON-like, composto apenas por `nil`, `bool`, `string`, numeros, `[]any` e `map[string]any`.
- Nao ha coercao implicita de tipos na v1.
- Campos marcados como `required` devem existir no input antes da execucao.
- Se `additionalProperties` nao for informado em um schema `object`, o runtime deve tratar como `false`.
- `required` nao pode conter nomes duplicados nem campos ausentes em `properties`.

### 6.3 Regras do output

- `OutputSchema().Type` deve ser explicitamente definido.
- `Result.Value` e o valor canonico retornado pela tool e o unico alvo de validacao de output.
- `Result.Value` deve ser JSON-like sob as mesmas restricoes do input.
- `Result.Content` e uma projecao textual opcional do resultado para consumo humano e para reinjecao no contexto do modelo.
- Se `Result.Content` vier vazio e `Result.Value` existir, o runtime pode serializar `Result.Value` de forma deterministica para compor a mensagem de role `tool`.
- `Result.Metadata` e observavel, mas nao substitui `Result.Value` como contrato de saida.

### 6.4 Regras de validacao

- Validacao de input ocorre antes de `Call`.
- Validacao de output ocorre depois de `Call` e antes da propagacao do resultado ao `Agent`.
- Input invalido impede a execucao da tool.
- Output invalido invalida a tool call, mesmo quando `Call` retornou `nil` error.
- O runtime nao deve mutar o input validado nem o output validado ao registrar `ToolCallRecord`, exceto por copia defensiva.

## 7. Contexto operacional

O contexto operacional de uma tool e composto por `context.Context` e pelo envelope `Call`.

Garantias obrigatorias:

1. `context.Context` e a fonte de verdade para cancelamento e deadline.
2. `Call.ID` deve ser nao vazio no momento da execucao; se o modelo ou adaptador nao fornecer um id, o runtime deve gerar um identificador deterministico por run.
3. `Call.ToolName` deve carregar o `effective name` resolvido da tool executada.
4. `Call.RunID`, `Call.SessionID` e `Call.AgentID` devem refletir o run corrente quando disponiveis.
5. `Call.Metadata` deve carregar a metadata efetiva do run, incluindo `trace_id`, `span_id` e `parent_span_id` quando existirem.
6. Tools nao podem depender da ordem interna de eventos do runtime alem do que esta observavel na `Spec 030`.
7. Tools devem ser preparadas para uso concorrente em multiplos runs, mesmo que uma instancia de `Agent` use execucao sequencial de tool calls dentro de um unico run na v1.
8. Nenhuma tool pode exigir servico hospedado de VoltOps para funcionar localmente.

Relacao com a `Spec 030`:

- O `Agent` continua executando tool calls de forma sequencial por run na v1.
- Falhas de tool encerram o run na v1.
- Cancelamento deve interromper novas tool calls o mais cedo possivel.

## 8. Erros e validacao

`pkg/tool` deve expor erros classificaveis e compativeis com `errors.Is` e `errors.As`.

API minima proposta:

```go
package tool

type ErrorKind string

const (
    ErrorKindInvalidTool    ErrorKind = "invalid_tool"
    ErrorKindInvalidToolkit ErrorKind = "invalid_toolkit"
    ErrorKindInvalidSchema  ErrorKind = "invalid_schema"
    ErrorKindInvalidInput   ErrorKind = "invalid_input"
    ErrorKindInvalidOutput  ErrorKind = "invalid_output"
    ErrorKindNameConflict   ErrorKind = "name_conflict"
    ErrorKindNotFound       ErrorKind = "not_found"
    ErrorKindExecution      ErrorKind = "execution"
    ErrorKindCanceled       ErrorKind = "canceled"
)

type Error struct {
    Kind        ErrorKind
    Op          string
    ToolName    string
    ToolkitName string
    CallID      string
    Cause       error
}

func (e *Error) Error() string
func (e *Error) Unwrap() error
```

Sentinels minimos:

- `ErrInvalidTool`
- `ErrInvalidToolkit`
- `ErrInvalidSchema`
- `ErrInvalidInput`
- `ErrInvalidOutput`
- `ErrNameConflict`
- `ErrToolNotFound`

Semantica obrigatoria:

- erro de naming invalido, schema invalido, tool nil ou toolkit nil deve falhar antes do registro efetivo
- input invalido deve retornar erro classificavel como `invalid_input` sem executar `Call`
- output invalido deve retornar erro classificavel como `invalid_output`
- erro retornado por `Call` deve ser encapsulado como `execution`
- `errors.Is(err, context.Canceled)` e `errors.Is(err, context.DeadlineExceeded)` devem continuar funcionando
- o `Agent` pode encapsular erros de tool como `agent.ErrorKindTool`, mas o erro original de `pkg/tool` deve permanecer acessivel por `errors.As`

## 9. Registro e resolucao

`pkg/tool` deve oferecer helpers publicos para registro e resolucao sem obrigar o usuario a importar `internal/*`.

API minima proposta:

```go
package tool

type Registry interface {
    Register(tools ...Tool) error
    RegisterToolkits(toolkits ...Toolkit) error
    Resolve(name string) (Tool, error)
    List() []Descriptor
    ListToolkits() []ToolkitDescriptor
}
```

Regras normativas:

1. `Register` deve validar naming, schemas, nulidade e conflitos antes de tornar uma tool resolvivel.
2. `RegisterToolkits` deve validar o toolkit e todas as suas tools contribuidas como uma operacao atomica.
3. Se uma tool de toolkit falhar na validacao, nenhuma tool daquele toolkit deve ser registrada parcialmente.
4. `Resolve` deve usar apenas `effective name`.
5. `Resolve` deve retornar `ErrToolNotFound` quando a tool nao existir.
6. `List` deve retornar `Descriptor` ordenado lexicograficamente por `Name`.
7. `ListToolkits` deve retornar `ToolkitDescriptor` ordenado lexicograficamente por `Name`.
8. O registry deve manter a associacao entre `Descriptor.Toolkit` e a origem do registro para diagnostico e observabilidade.
9. O registry nao deve mutar as instancias registradas alem do necessario para copia defensiva de metadata ou descriptors.
10. O `Agent` pode consumir o resultado plano de um registry, mas nao precisa conhecer `Toolkit` diretamente.

## 10. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao futura sao:

1. Registro bem-sucedido de standalone tool valida.
2. Erro ao registrar tool com `Name()` vazio.
3. Erro ao registrar tool com nome fora do regex permitido.
4. Erro ao registrar tool com `Description()` vazia.
5. Erro ao registrar tool com `InputSchema().Type != "object"`.
6. Erro ao registrar tool com `OutputSchema().Type` vazio.
7. Erro ao registrar toolkit com `Name()` invalido.
8. Erro ao registrar toolkit contendo `nil` tool.
9. Erro ao registrar toolkit com tools locais duplicadas.
10. Erro ao registrar conflito entre standalone tool e tool contribuidora de toolkit.
11. Resolucao bem-sucedida de tool namespaced por `namespace.local_name`.
12. `Resolve` falhando com `ErrToolNotFound` para nome ausente.
13. Validacao de input rejeitando campo ausente em `required`.
14. Validacao de input rejeitando tipo divergente sem coercao implicita.
15. `Call` recebendo `Call.ID`, `RunID`, `SessionID`, `AgentID` e metadata efetivos.
16. `Call` respeitando cancelamento por `context.Context`.
17. Erro retornado pela tool sendo propagado como erro classificavel de execucao.
18. Output valido sendo aceito e preservado em `ToolCallRecord`.
19. Output invalido sendo rejeitado mesmo quando `Call` retorna `nil` error.
20. `List` retornando tools em ordem lexicografica deterministica.
21. Uso concorrente da mesma tool registrada sem race nem corrupcao observavel.
22. Registro atomico de toolkit: falha em uma tool impede registro parcial do conjunto.

## 11. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `pkg/tool` expuser `Tool`, `Toolkit`, `Schema`, `Call`, `Result`, `Descriptor`, `ToolkitDescriptor`, `Registry` e `Error`.
2. O contrato suportar schema de entrada e saida observaveis e validaveis.
3. O runtime conseguir validar input antes da execucao e output apos a execucao.
4. Naming e conflito de nomes forem tratados de forma deterministica e antecipada.
5. O `Agent` puder continuar consumindo uma lista plana de tools sem importar `internal/*`.
6. Toolkits puderem agrupar tools sem perder rastreabilidade de origem, namespace e effective name.
7. Cancelamento e deadline propagarem corretamente ate a execucao da tool.
8. Erros de `pkg/tool` permanecerem classificaveis mesmo quando encapsulados pelo `Agent`.
9. Nenhuma parte da solucao exigir dependencia de VoltOps ou servico hospedado para funcionar localmente.
10. Suites de contrato e conformidade cobrirem pelo menos os cenarios listados na secao 10.

Validacao contra a `Spec 010`:

- `Tool` so pode sair de `Parcial` quando schema rico, validacao e erros classificados estiverem implementados e testados.
- `Toolkit` so pode sair de `Nao iniciado` quando registro, conflito, resolucao e suites obrigatorias desta spec estiverem implementados e testados.

## 12. Questoes em aberto

1. O subconjunto de JSON Schema da v1 deve incluir `oneOf` e `default`, ou isso deve ficar explicitamente fora ate haver demanda real de paridade?
2. `Toolkit.Namespace()` deve permanecer opcional ou a v1 deve exigir namespace para todo toolkit para reduzir ambiguidade?
3. O registry deve viver inteiramente em `pkg/tool` ou parte da composicao de toolkits deve subir para `pkg/app` em uma spec futura?
4. `Result.Content` deve ser apenas opcional ou a v1 deve exigir uma representacao textual explicita para toda tool usada por `Agent`?
5. O `Agent` deve expor no futuro uma option de mais alto nivel para receber `Toolkit` diretamente, ou a flattening explicita por registry e suficiente para a v1?
