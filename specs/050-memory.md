# Spec 050: Memory

## 1. Objetivo

Este documento define a camada de `memory` da `gaal-lib`.

Os objetivos desta spec sao:

- estabelecer o contrato publico de memoria conversacional e working memory em `pkg/memory`
- definir a fronteira entre memoria conversacional de `agent` e historico de execucao de `workflow`
- especificar o modelo de dados minimo observado pelo runtime e pelos adapters de storage
- definir comportamento de defaults, persistencia, ausencia de provider configurado e extensibilidade futura
- preservar paridade conceitual com o core do Voltagent sem introduzir dependencia de plataforma hospedada

Esta spec complementa a `Spec 000`, a `Spec 010`, a `Spec 020`, a `Spec 030` e a `Spec 031`.

Quando houver mais detalhe sobre memory neste documento, ele prevalece sobre referencias resumidas em specs mais gerais.

Ficam fora do escopo desta spec:

- banco vetorial hospedado, busca semantica remota ou qualquer dependencia obrigatoria de servico externo
- memoria global compartilhada entre aplicacoes sem chave conversacional explicita
- persistencia automatica de historico de execucao de workflows via `pkg/memory`
- replay operacional, dashboards, auditoria hospedada ou qualquer capacidade de VoltOps

## 2. Tipos de memoria

### 2.1 Memoria conversacional de agent

`Memoria conversacional` e o estado duravel ou compartilhado entre multiplos runs de um `Agent`, associado a uma conversa identificada por `SessionID`.

Ela existe para:

- restaurar contexto conversacional observavel entre runs
- reintroduzir mensagens e records persistidos antes da primeira chamada ao modelo
- permitir continuidade entre execucoes sem acoplamento a infraestrutura especifica

Ela nao existe para:

- registrar cada transicao interna do runtime
- substituir historico de workflow
- servir como camada obrigatoria de busca semantica na v1

### 2.2 Working memory

`Working memory` e o estado efemero de uma unica execucao.

Ela existe para:

- acumular mensagens efetivas do run
- registrar tool calls, resultados intermediarios e artifacts internos
- isolar estado temporario por run, sem compartilhamento implicito entre execucoes concorrentes

Por default, a working memory e sempre local ao run, em memoria de processo, e descartada ao final da execucao.

### 2.3 Historico de execucao de workflow

`Workflow execution history` nao faz parte da memoria conversacional de agent.

A separacao normativa e:

- memoria conversacional pertence ao dominio de continuidade de conversa e usa `pkg/memory`
- historico de workflow pertence ao dominio de execucao observavel de workflows e usa `pkg/workflow`, em especial `workflow.HistorySink`
- historico de workflow nao deve ser carregado automaticamente em prompts, snapshots de `memory.Store.Load` ou contexto conversacional do `Agent`
- memoria conversacional nao deve ser usada como substituto generico de auditoria de workflow

Quando um workflow precisar de estado temporario durante a execucao, esse estado e working memory do proprio run, nao historico duravel.

### 2.4 Storage em memoria vs storage persistente

Esta spec reconhece duas classes de backend para memoria conversacional:

- `in-memory store`: implementa o contrato de storage sem durabilidade entre reinicios do processo; adequado para testes, desenvolvimento e execucao efemera
- `persistent store`: implementa o mesmo contrato com durabilidade fora do ciclo de vida do processo; adequado para continuidade entre execucoes independentes

A diferenca entre essas classes esta na durabilidade, nao no contrato publico.

### 2.5 Comportamento sem provider configurado

Quando nenhum provider de memoria conversacional estiver configurado:

- o `Agent` continua valido e executavel
- nenhuma chamada a `memory.Store.Load` ou `memory.Store.Save` ocorre
- a working memory continua obrigatoria e deve existir por run
- `SessionID` deixa de ser obrigatorio para fins de memory
- nao existe persistencia implicita em arquivo local, banco local ou cache oculto

Em outras palavras: ausencia de provider de memory significa "sem memoria conversacional persistente", nao erro de configuracao global.

## 3. Modelo de dados

O contrato principal deve viver em `pkg/memory`.

API publica proposta:

```go
package memory

import (
    "context"

    "github.com/luanlima/gaal-lib/pkg/types"
)

type Record struct {
    Kind string
    Name string
    Data map[string]any
}

type Snapshot struct {
    Messages []types.Message
    Records  []Record
    Metadata types.Metadata
}

type Delta struct {
    Messages []types.Message
    Records  []Record
    Response *types.Message
    Metadata types.Metadata
}

type Store interface {
    Load(ctx context.Context, sessionID string) (Snapshot, error)
    Save(ctx context.Context, sessionID string, delta Delta) error
}

type WorkingMemoryFactory interface {
    NewRunState(ctx context.Context, agentID, runID string) (WorkingSet, error)
}

type WorkingSet interface {
    AddMessage(msg types.Message)
    AddRecord(record Record)
    Snapshot() Snapshot
}
```

### 3.1 Regras normativas do modelo

1. `Snapshot` representa a visao carregada no inicio do run. Ele deve ser tratavel como somente-leitura pelo consumidor.
2. `Delta` representa o estado observavel entregue ao adapter apos um run bem-sucedido. O nome `Delta` nao obriga o backend a usar estrategia append-only.
3. `Snapshot.Messages` e `Delta.Messages` devem preservar ordem cronologica observavel, do mais antigo para o mais recente.
4. `Snapshot.Records` e `Delta.Records` devem preservar ordem deterministica de insercao.
5. `Delta.Response`, quando presente, deve corresponder a resposta final ja aprovada por output guardrails.
6. `Metadata` deve ser tratada como snapshot defensivo, sem compartilhamento mutavel com o runtime.
7. `Record.Kind` e `Record.Name` devem ser ASCII e semanticamente estaveis; nomes reservados para runtime interno podem existir, mas nao criam obrigacao de exposicao publica adicional.
8. Artifacts internos de reasoning podem existir na working memory, mas nao devem ser persistidos por default, conforme a `Spec 041`.
9. Entradas de historico de workflow nao devem ser serializadas em `Snapshot.Records` ou `Delta.Records` por default.

### 3.2 Semantica observavel de `Delta`

Para a v1, o contrato observavel de persistencia e:

- o runtime entrega ao `Store.Save` o estado conversacional materializado para aquele run bem-sucedido
- o backend pode implementar isso como overwrite total, append com compactacao, event log interno ou outra estrategia equivalente
- independentemente da estrategia interna, um `Load` posterior da mesma sessao deve refletir o mesmo estado conversacional como se aquele run tivesse sido persistido exatamente uma vez

Esta flexibilidade e intencional para permitir adapters simples em memoria e adapters persistentes mais sofisticados sem ampliar a API publica minima.

## 4. Chaves de identidade

### 4.1 Chave canonica de memoria conversacional

Na v1, a chave publica canonica de memoria conversacional e `SessionID`.

Regras obrigatorias:

1. `SessionID` identifica a conversa para `memory.Store.Load` e `memory.Store.Save`.
2. Se houver `memory.Store` configurado, `SessionID` vazio torna `Run` e `Stream` invalidos, conforme a `Spec 030`.
3. Se nao houver `memory.Store` configurado, `SessionID` pode permanecer vazio sem invalidar o run.

### 4.2 Chaves auxiliares

As chaves abaixo sao relevantes, mas nao sao a identidade publica primaria do storage na v1:

- `RunID`: identifica uma execucao individual; serve para tracing, eventos e working memory, nao para lookup da conversa persistida
- `AgentID`: identifica qual agent executou o run; pode ser usado internamente pelo adapter para diagnostico ou namespacing, mas nao substitui `SessionID` como contrato publico
- `Workflow execution ID`: identifica uma execucao de workflow; nao deve ser usado como chave de memoria conversacional

### 4.3 Namespacing interno de adapters

Um adapter persistente pode compor uma chave interna mais rica, por exemplo com `app`, `agent` ou prefixos proprios, desde que:

- a semantica observavel continue sendo baseada em `SessionID`
- a mesma sessao seja resolvida de forma deterministica pelo mesmo adapter e pela mesma configuracao
- a composicao interna nao quebre a possibilidade de continuidade entre runs que deveriam compartilhar a mesma conversa

## 5. Interface de storage adapter

O contrato minimo do adapter de storage e:

```go
type Store interface {
    Load(ctx context.Context, sessionID string) (Snapshot, error)
    Save(ctx context.Context, sessionID string, delta Delta) error
}
```

### 5.1 Responsabilidades de `Load`

- carregar o estado conversacional associado a `sessionID`
- retornar `Snapshot{}` e `nil` quando a sessao ainda nao existir
- honrar cancelamento e deadline vindos de `context.Context`
- nao mutar dados recebidos nem compartilhar referencias mutaveis com o runtime

### 5.2 Responsabilidades de `Save`

- persistir o estado conversacional aprovado para `sessionID`
- honrar cancelamento e deadline vindos de `context.Context`
- tratar `delta` como snapshot imutavel entregue pelo runtime
- falhar explicitamente quando nao conseguir garantir a persistencia requerida pelo backend

### 5.3 Regras normativas do adapter

1. O adapter deve ser seguro para uso concorrente depois de configurado.
2. `Load` nao deve falhar apenas porque a sessao nao existe; ausencia de sessao e caso normal.
3. `Save` so deve ser chamado em runs bem-sucedidos e apos output guardrails.
4. O adapter nao deve exigir importacao de `internal/*`.
5. O adapter nao deve introduzir dependencia obrigatoria de servico hospedado para o runtime funcionar localmente.
6. Erros do adapter devem ser propagados como falha de memory pelo runtime do `Agent`.
7. Se o backend suportar escrita atomica, ela deve ser preferida.
8. Se o backend nao suportar escrita atomica, a estrategia adotada deve manter resultado deterministico ou falhar explicitamente.

### 5.4 Adapter em memoria

Um adapter em memoria e aceitavel na v1 desde que:

- implemente o mesmo contrato `Store`
- preserve isolamento por `SessionID`
- seja deterministico dentro do mesmo processo
- deixe claro que nao oferece durabilidade entre reinicios

### 5.5 Adapter persistente

Um adapter persistente e aceitavel na v1 desde que:

- implemente o mesmo contrato `Store`
- forneca durabilidade fora do ciclo de vida do processo
- mantenha compatibilidade observavel com a ordem de `Load` e `Save` definida pela `Spec 030`
- nao misture historico de workflow com memoria conversacional por default

## 6. Estrategia de persistencia

### 6.1 Ordem obrigatoria no runtime do agent

A ordem normativa, herdada e detalhada a partir da `Spec 030`, e:

1. validar request e executar input guardrails
2. carregar memoria conversacional via `Store.Load`, quando houver provider configurado
3. criar working memory isolada para o run
4. executar o loop do agente
5. executar output guardrails
6. persistir via `Store.Save`, quando houver provider configurado
7. somente depois sinalizar sucesso terminal do run

### 6.2 Regras de persistencia observavel

1. Apenas runs bem-sucedidos podem produzir persistencia conversacional.
2. Falha em `Load` impede qualquer chamada ao modelo.
3. Falha em `Save` invalida o run, mesmo que a resposta final ja exista.
4. Output bloqueado impede persistencia.
5. Cancelamento por `context.Context` impede persistencia de sucesso.
6. Stream abortado pelo consumidor impede `agent.completed` e nao deve ser tratado como persistencia bem-sucedida.
7. O backend nao deve persistir reasoning artifacts por default.
8. O backend nao deve persistir historico de workflow por default por meio de `memory.Store`.

### 6.3 Estrategias internas permitidas

Um adapter pode usar internamente:

- snapshot completo
- append log com compactacao posterior
- estrutura chave-valor com payload serializado
- banco relacional, documento local ou outra persistencia equivalente

O que permanece obrigatorio e o contrato observavel, nao o formato fisico.

### 6.4 Ausencia de provider

Se nao houver `Store` configurado no agent nem herdado via defaults do `App`:

- o runtime deve pular as etapas de `Load` e `Save`
- o run deve depender apenas da working memory efemera
- nenhuma persistencia implicita deve ser criada automaticamente

## 7. Working memory

### 7.1 Contrato minimo

Toda execucao deve possuir um `WorkingSet`, criado por `WorkingMemoryFactory`.

API minima:

```go
type WorkingMemoryFactory interface {
    NewRunState(ctx context.Context, agentID, runID string) (WorkingSet, error)
}

type WorkingSet interface {
    AddMessage(msg types.Message)
    AddRecord(record Record)
    Snapshot() Snapshot
}
```

### 7.2 Responsabilidades da working memory

A working memory deve registrar ao menos:

- mensagens carregadas da memoria conversacional, quando houver
- mensagens efetivas do request apos input guardrails
- mensagens e records produzidos por tool calls
- artifacts internos do runtime que precisem existir apenas durante o run
- resposta final antes da persistencia

### 7.3 Regras normativas

1. Toda execucao de `Agent` deve possuir working memory, com ou sem `Store`.
2. A working memory default deve ser efemera, local ao run e isolada por execucao.
3. Falha em `NewRunState` deve falhar o run antes da primeira geracao do modelo.
4. A working memory pode conter mais dados do que o que sera persistido em `Store.Save`.
5. O fato de um artifact existir na working memory nao implica direito de persistencia.
6. Working memory de agent e historico de workflow continuam conceitos distintos.
7. Um runtime de workflow pode reutilizar a abstracao de working memory internamente no futuro, mas isso nao promove historico de workflow para `pkg/memory`.

### 7.4 Relacao com a memoria conversacional

- working memory e sempre run-scoped
- memoria conversacional e session-scoped
- working memory pode ser a fonte do material entregue ao `Store.Save`
- quando nao houver provider configurado, a working memory continua sendo a unica memoria do run

## 8. Busca futura e extensibilidade

A v1 nao exige interface publica de busca em `pkg/memory`.

Mesmo assim, a camada deve ser desenhada para extensao futura sem quebra do contrato basico.

Capacidades candidatas para evolucao posterior:

- busca textual ou semantica sobre memoria conversacional
- filtros por `Record.Kind`, janela temporal ou metadata
- compactacao, sumarizacao e politicas de retencao
- versionamento otimista ou compare-and-swap
- namespaces mais ricos do que `SessionID`

Regras normativas para essa evolucao:

1. Qualquer busca futura deve operar sobre memoria conversacional explicitamente, nao sobre historico de workflow por default.
2. Extensoes futuras nao devem quebrar o contrato minimo de `Store`.
3. Adapters avancados podem expor capacidades adicionais por interfaces opcionais ou subpacotes futuros, nunca por acoplamento a `internal/*`.
4. Busca futura nao deve virar requisito obrigatorio para o caso basico de memoria local em v1.

## 9. Hierarquia de defaults

### 9.1 Ordem de precedencia

A hierarquia normativa de defaults para memory e:

1. override explicito por request ou execucao, quando aplicavel ao contexto
2. configuracao local do `Agent`
3. defaults globais do `App`
4. defaults built-in do pacote

Aplicado a memory na v1:

- `Request.SessionID` define a identidade da conversa, mas nao cria um store por si so
- `agent.WithMemory` prevalece sobre `app.Defaults.Agent.Memory`
- `agent.WithWorkingMemory` prevalece sobre `app.Defaults.Agent.WorkingMemory`
- se factory de agent nao fornecer store, ela pode herdar `app.Defaults.Agent.Memory`
- se nenhum store for definido em nivel algum, o built-in e "sem memoria conversacional persistente"
- se nenhuma working memory factory for definida em nivel algum, o built-in e uma working memory efemera em memoria de processo

### 9.2 Regras de merge

- stores usam estrategia "ultimo valor nao-nulo vence"
- working memory factories usam estrategia "ultimo valor nao-nulo vence"
- metadata segue merge por sobrescrita de chaves mais especificas sobre chaves mais globais
- instancias prontas de `Agent` registradas diretamente no `App` nao recebem mutacao estrutural retroativa de defaults

### 9.3 Separacao de defaults entre agent e workflow

`App.Defaults.Agent.Memory` e `App.Defaults.Agent.WorkingMemory` pertencem ao dominio de `Agent`.

`App.Defaults.Workflow.History` pertence ao dominio de workflow.

Regras obrigatorias:

- defaults de workflow nao devem ser reinterpretados como defaults de memory conversacional
- um `workflow.HistorySink` nao deve ser promovido implicitamente a `memory.Store`
- ausencia de `Defaults.Agent.Memory` nao deve ser compensada por `Defaults.Workflow.History`

## 10. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao futura sao:

1. `Agent` sem provider de memoria conversacional executando com sucesso e usando apenas working memory efemera.
2. Erro de request invalido quando existe `memory.Store` configurado e `SessionID` vem vazio.
3. `Store.Load` retornando sessao ausente como `Snapshot{}` sem erro.
4. `Store.Load` sendo chamado depois dos input guardrails e antes da primeira chamada ao modelo.
5. `Store.Save` sendo chamado depois dos output guardrails e antes do sinal terminal de sucesso.
6. Falha em `Store.Load` impedindo qualquer geracao do modelo.
7. Falha em `Store.Save` invalidando o run.
8. Output guardrail bloqueando a resposta e impedindo `Store.Save`.
9. Cancelamento por contexto impedindo persistencia de sucesso.
10. Stream abortado pelo consumidor nao produzindo `agent.completed` nem persistencia de sucesso.
11. Working memory default existindo mesmo quando nenhum `Store` esta configurado.
12. `WorkingMemoryFactory.NewRunState` falhando e abortando o run antes da primeira chamada ao modelo.
13. Artifacts internos de reasoning permanecendo apenas na working memory por default.
14. Adapter em memoria preservando isolamento por `SessionID` dentro do mesmo processo.
15. Adapter persistente mantendo continuidade observavel entre duas execucoes independentes da mesma sessao.
16. `app.Defaults.Agent.Memory` sendo herdado por agent construido via factory.
17. `agent.WithMemory` sobrescrevendo `app.Defaults.Agent.Memory`.
18. `app.Defaults.Agent.WorkingMemory` sendo herdado quando o agent factory nao define working memory propria.
19. Historico de workflow sendo escrito em `workflow.HistorySink` sem aparecer em `memory.Store`.
20. `memory.Store.Load` nao devolvendo historico de workflow como parte do snapshot conversacional.

## 11. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. `pkg/memory` expuser contratos publicos claros para `Store`, `Snapshot`, `Delta`, `WorkingMemoryFactory` e `WorkingSet`.
2. A fronteira entre memoria conversacional de `Agent`, working memory e historico de execucao de workflow estiver documentada e refletida na implementacao.
3. O `Agent` suportar execucao correta com e sem provider de memoria conversacional configurado.
4. A ordem observavel de `Load`, `Run` e `Save` estiver implementada e testada.
5. Adapters em memoria e persistentes puderem coexistir sob o mesmo contrato publico.
6. Working memory continuar obrigatoria por run, independentemente de persistencia.
7. Nenhum comportamento exigir servico hospedado para a biblioteca funcionar localmente.
8. Qualquer extensao futura de busca ou capacidades avancadas preservar o contrato minimo da v1.

## 12. Questoes em aberto

1. A chave publica de memoria deve continuar sendo apenas `SessionID string` na v1, ou um tipo mais rico de identidade conversacional merece entrar cedo?
2. O contrato de `Delta` deve continuar representando estado materializado de sucesso, ou vale promover um tipo mais explicito para evitar ambiguidade entre patch e snapshot?
3. A camada publica de memory precisa de versionamento otimista antes da primeira implementacao persistente nao trivial?
4. Devemos definir uma taxonomia normativa de `Record.Kind` na v1, ou isso pode permanecer flexivel enquanto as suites de conformidade amadurecem?
5. Workflows vao precisar de uma abstracao publica propria para working state, distinta da working memory de `Agent`, antes da spec detalhada de `workflow`?
6. Busca futura deve viver em `pkg/memory`, em subpacote dedicado ou em interface opcional descoberta por type assertion?
