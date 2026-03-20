# Spec 220: Conversation Persistence

## 1. Objetivo

Adicionar a `gaal-lib` uma camada pluggable de persistencia conversacional que permita ao usuario escolher o backend de armazenamento, incluindo arquivos e bancos de dados, preservando o fallback atual in-memory e a ergonomia do runtime.

Esta spec existe para fechar o gap de conversation persistence identificado no diagnostico da biblioteca (Spec 200) e registrado na feature matrix (Spec 010) como P1, status `Nao iniciado`, risco Alto.

---

## 2. Motivacao

A `gaal-lib` ja possui memoria funcional em-processo, suficiente para:

- demos locais
- working memory basica
- fluxo de agent dentro do mesmo processo

Mas ainda nao possui persistencia conversacional duravel e configuravel.

O Voltagent oferece conversation persistence como parte do nucleo, permitindo backends alternativos. Para atingir paridade funcional, a `gaal-lib` precisa de persistencia com as seguintes caracteristicas:

- backend selecionavel pelo usuario
- contrato estavel e extensivel
- implementacao padrao simples
- suporte explicito a arquivos
- suporte explicito a bancos via adapter
- integracao limpa com `App`, `Agent` e `Memory`

A interface `memory.Store` ja existe em `pkg/memory` e define o contrato canonico de persistencia conversacional. Esta spec nao cria um contrato paralelo. Ela define implementacoes concretas, regras de integracao e evidencias obrigatorias para fechar o gap.

---

## 3. Escopo

### Dentro do escopo

- reafirmacao de `memory.Store` como contrato canonico de persistencia
- implementacao de backend file-based obrigatorio
- manutencao do backend in-memory existente
- extensibilidade para bancos de dados por adapter
- integracao com `App`, `Agent` e `Memory`
- hierarquia de defaults e overrides
- serializacao e formato de dados do file backend
- exemplos de configuracao e uso
- testes de persistencia com restart
- example/demo usando persistencia real

### Fora do escopo

- engine ORM propria
- migracoes complexas de banco de dados
- replicacao distribuida
- search vetorial ou indexing avancado
- storage multi-tenant complexo
- observabilidade hospedada
- encryption at rest
- migrations engine
- query language custom
- transactional saga
- sync distribuido
- painel visual de conversas

---

## 4. Requisitos de design

### 4.1 Adapter-first

A persistencia deve ser modelada pela interface `memory.Store` ja existente. Nenhuma implementacao concreta pode ser acoplada ao runtime. Novos backends sao implementacoes dessa interface.

### 4.2 Backend selecionavel

O usuario deve poder escolher o backend de persistencia por configuracao explicita:

- via `app.Config.Defaults.Agent.Memory` para default global
- via `agent.WithMemory(store)` para override por agent
- via factory que recebe `AgentDefaults.Memory` no bootstrap

### 4.3 File-based obrigatorio

Deve existir ao menos uma implementacao baseada em arquivos para comprovar que o sistema e realmente pluggable e para oferecer persistencia duravel sem dependencia de banco de dados.

### 4.4 Banco de dados via adapter

A arquitetura deve permitir implementar backends de banco de dados sem reescrever a camada de memoria. Qualquer tipo que implemente `memory.Store` e um backend valido.

### 4.5 In-memory preservado

O comportamento in-memory atual (`InMemoryStore`) deve continuar disponivel como fallback/default. Nenhuma mudanca breaking pode ser introduzida nesse provider.

### 4.6 Contrato claro de identidade

A persistencia usa `SessionID` como chave publica canonica, conforme definido na Spec 050. Chaves auxiliares como `user_id` e `conversation_id` podem ser usadas internamente pelo adapter para namespacing, mas nao alteram a semantica publica.

### 4.7 Semantica de leitura/escrita explicita

A ordem normativa de leitura e escrita e herdada da Spec 030 e da Spec 050:

1. input guardrails executam
2. `Store.Load` carrega historico existente
3. working memory e criada e populada
4. loop do agent executa
5. output guardrails executam
6. `Store.Save` persiste o estado aprovado
7. `EventAgentCompleted` sinaliza sucesso

Erros de `Load` impedem qualquer geracao do modelo. Erros de `Save` invalidam o run mesmo que a resposta final ja exista.

---

## 5. Modelo funcional esperado

A implementacao deve suportar, no minimo, este fluxo:

1. um agent recebe interacao com `SessionID`
2. o runtime carrega historico existente do backend configurado via `Store.Load`
3. a execucao ocorre normalmente (modelo, tools, guardrails)
4. as novas mensagens sao persistidas via `Store.Save`
5. apos reinicio do processo, o historico reaparece se o backend persistente estiver configurado
6. se nenhum backend persistente estiver configurado, o modo in-memory continua funcionando sem durabilidade

O passo 5 e o diferenciador critico entre `InMemoryStore` e backends persistentes. Ele deve ser comprovavel por teste executavel.

---

## 6. Requisitos funcionais

### RF-01 — Contrato canonico de persistencia

`memory.Store` em `pkg/memory` e o contrato canonico. Nao deve ser criada interface paralela. Todos os backends implementam esta interface:

```go
type Store interface {
    Load(ctx context.Context, sessionID string) (Snapshot, error)
    Save(ctx context.Context, sessionID string, delta Delta) error
}
```

### RF-02 — In-memory provider

O provider `InMemoryStore` existente em `pkg/memory` deve continuar funcionando sem alteracoes breaking. Ele permanece como default quando nenhum backend persistente e configurado.

### RF-03 — File provider

Deve existir um provider baseado em arquivos que implemente `memory.Store` com durabilidade entre reinicios do processo.

### RF-04 — Database extensibility

Deve ser possivel implementar provider de banco de dados via adapter externo, sem mudar a API publica central. Qualquer struct que implemente `memory.Store` e valida.

### RF-05 — App integration

O `App` deve poder aplicar um backend persistente como default global para todos os agents via `Config.Defaults.Agent.Memory`, conforme a hierarquia de defaults definida na Spec 031.

### RF-06 — Agent override

Deve ser possivel override por agent via `agent.WithMemory(store)`. O override prevalece sobre o default do `App`, conforme a hierarquia de precedencia da Spec 050:

1. configuracao local do `Agent`
2. defaults globais do `App`
3. defaults built-in do pacote (nenhum store)

### RF-07 — Restart proof

Deve existir evidencia executavel de que a persistencia sobrevive a restart quando backend persistente e usado. O teste deve:

1. criar agent com file store configurado
2. executar um run com mensagens
3. simular restart (nova instancia de store apontando para o mesmo diretorio)
4. executar novo run com o mesmo `SessionID`
5. verificar que o historico do run anterior esta presente

### RF-08 — Error handling

Erros de leitura/escrita devem ser observaveis e previsiveis:

- erros de `Load` devem propagar `agent.ErrorKindMemory`
- erros de `Save` devem propagar `agent.ErrorKindMemory`
- erros de I/O do file backend (permissao, disco cheio, path invalido) devem ser encapsulados e classificaveis
- cancelamento de contexto durante `Load` ou `Save` deve ser honrado

---

## 7. API publica e contrato

### 7.1 Contrato existente preservado

O contrato `memory.Store` ja existe e esta estavel:

```go
package memory

type Store interface {
    Load(ctx context.Context, sessionID string) (Snapshot, error)
    Save(ctx context.Context, sessionID string, delta Delta) error
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
```

### 7.2 FileStore proposto

O file-based backend deve viver em `pkg/memory` como implementacao de referencia persistente:

```go
type FileStore struct {
    Dir string
}

func NewFileStore(dir string) (*FileStore, error)
func (s *FileStore) Load(ctx context.Context, sessionID string) (Snapshot, error)
func (s *FileStore) Save(ctx context.Context, sessionID string, delta Delta) error
```

`NewFileStore` deve:

- validar que `dir` nao esta vazio
- criar o diretorio se nao existir (com `os.MkdirAll`)
- retornar erro se o diretorio nao puder ser criado ou acessado

### 7.3 Construtor auxiliar para uso rapido

Para conveniencia e DX, o pacote pode oferecer:

```go
func NewFileStoreOrPanic(dir string) *FileStore
```

Este helper e opcional e destinado a exemplos e demos.

---

## 8. File-based backend

### 8.1 Organizacao de arquivos

Cada sessao persistida deve corresponder a um unico arquivo dentro do diretorio base.

Convencao de nomeacao:

- o nome do arquivo deve ser derivado de `sessionID` por sanitizacao deterministica
- caracteres nao alfanumericos em `sessionID` devem ser substituidos por `_` ou codificados por hex
- a extensao deve ser `.json`
- colisoes de nomeacao por sanitizacao devem ser tratadas ou documentadas como limitacao

Exemplo: `sessionID = "user-123/conv-456"` resulta em `user-123_conv-456.json`.

### 8.2 Formato de serializacao

O formato de serializacao obrigatorio para o file backend e JSON.

O payload serializado deve representar o estado conversacional completo equivalente a um `Snapshot`:

```json
{
  "messages": [...],
  "records": [...],
  "metadata": {...}
}
```

Regras:

- `messages` deve preservar ordem cronologica
- `records` deve preservar ordem de insercao
- `metadata` deve incluir todas as chaves persistidas
- o formato deve ser legivel por humanos para facilitar debugging

### 8.3 Escrita atomica

A escrita no file backend deve usar a estrategia write-rename para minimizar corrupcao:

1. serializar o payload em memoria
2. escrever para arquivo temporario no mesmo diretorio
3. fazer `os.Rename` do temporario para o arquivo final

Se `Rename` falhar, o `Save` deve retornar erro. O arquivo temporario deve ser limpo em caso de falha.

### 8.4 Leitura

Regras de leitura:

- sessao ausente (arquivo inexistente) retorna `Snapshot{}` sem erro
- arquivo vazio retorna `Snapshot{}` sem erro
- arquivo com JSON invalido retorna erro encapsulado
- cancelamento de contexto durante leitura deve ser honrado

### 8.5 Concorrencia

O `FileStore` deve ser seguro para uso concorrente dentro do mesmo processo:

- `sync.RWMutex` protege operacoes de leitura e escrita
- nao e obrigatorio suportar acesso concorrente entre processos diferentes na v1
- se acesso entre processos for relevante no futuro, file locks podem ser adicionados sem quebrar o contrato

### 8.6 Permissoes e erros de I/O

Erros de I/O devem ser encapsulados com informacao suficiente para diagnostico:

- path do arquivo afetado
- operacao tentada (read/write/rename)
- causa original do erro

---

## 9. Backends minimos obrigatorios

### Obrigatorios nesta fase

| Backend | Pacote | Durabilidade | Status |
| --- | --- | --- | --- |
| in-memory | `pkg/memory` | nao | existente |
| file-based | `pkg/memory` | sim | novo |

### Arquiteturalmente suportados

| Backend | Pacote provavel | Observacao |
| --- | --- | --- |
| sqlite | `pkg/memory/sqlite` ou externo | adapter via `memory.Store` |
| postgres | externo | adapter via `memory.Store` |
| mysql | externo | adapter via `memory.Store` |
| redis | externo | adapter via `memory.Store` |
| outros | externo | qualquer implementacao de `memory.Store` |

Nao e obrigatorio implementar backends de banco nesta fase, mas a arquitetura deve deixa-los claramente suportaveis.

---

## 10. Decisoes obrigatorias de modelagem

A implementacao deve documentar explicitamente:

1. **Contrato da persistence interface**: `memory.Store` e o contrato canonico. Nao existe interface separada de persistence.

2. **Operacoes minimas do adapter**: `Load(ctx, sessionID)` e `Save(ctx, sessionID, delta)`. Nenhuma outra operacao e obrigatoria na v1.

3. **Serializacao no file backend**: JSON com estrutura equivalente a `Snapshot`. Messages preservam ordem cronologica. Records preservam ordem de insercao.

4. **Acionamento no runtime**: `Load` e chamado apos input guardrails e antes da primeira chamada ao modelo. `Save` e chamado apos output guardrails e antes de `EventAgentCompleted`. Ambos sao condicionais a existencia de `Store` configurado.

5. **Falhas de persistence**: `Load` com erro impede geracao do modelo. `Save` com erro invalida o run. Erros sao propagados como `agent.ErrorKindMemory`.

6. **Working memory vs persistence**: working memory e run-scoped e efemera. Persistence e session-scoped e duravel. Working memory continua obrigatoria com ou sem `Store`. A working memory pode ser a fonte do material entregue ao `Store.Save`, mas nem tudo na working memory sera persistido (reasoning artifacts sao filtrados, conforme Spec 041).

7. **Defaults do App e overrides do Agent**: `App.Config.Defaults.Agent.Memory` define o store global. `agent.WithMemory(store)` sobrescreve o global. Instancias prontas registradas por `Register` nao recebem mutacao de defaults. Factories recebem o default via `AgentDefaults.Memory` no `Build`.

---

## 11. Integracao com App, Agent e Memory

### 11.1 Wiring via App

O `App` aplica o backend de persistencia atraves da hierarquia de defaults:

```go
app.New(app.Config{
    Name: "my-app",
    Defaults: app.Defaults{
        Agent: app.AgentDefaults{
            Memory: memory.MustNewFileStore("./data/conversations"),
        },
    },
}, app.WithAgentFactories(myFactory))
```

Neste cenario, todo agent construido via factory herda o `FileStore` como backend.

### 11.2 Override por Agent

Um agent pode sobrescrever o backend global:

```go
agent.New(agent.Config{
    Name:  "ephemeral-agent",
    Model: myModel,
}, agent.WithMemory(&memory.InMemoryStore{}))
```

### 11.3 Hierarquia de precedencia

Conforme Spec 050 e Spec 031:

1. `agent.WithMemory(store)` — override local do agent
2. `AgentDefaults.Memory` — default global do `App`
3. nenhum store — built-in (sem persistencia conversacional)

### 11.4 Relacao com Working Memory

A persistencia nao substitui working memory:

- working memory continua obrigatoria por run, com ou sem `Store`
- working memory default e `InMemoryWorkingMemoryFactory`
- o runtime entrega ao `Store.Save` o estado materializado a partir da working memory, filtrado por `filterPersistedRecords`

### 11.5 Relacao com SessionID

- quando `Store` esta configurado, `SessionID` vazio no request invalida o run (conforme Spec 030)
- quando `Store` nao esta configurado, `SessionID` pode ser vazio
- `SessionID` e a chave publica para `Load` e `Save`

### 11.6 Relacao com historico de workflow

Conforme Spec 050:

- historico de workflow usa `workflow.HistorySink`, nao `memory.Store`
- `Store.Load` nao deve devolver historico de workflow
- `Store.Save` nao deve receber historico de workflow

---

## 12. Casos de teste obrigatorios

Os testes minimos obrigatorios para a implementacao sao:

1. `FileStore.Load` retornando `Snapshot{}` sem erro quando a sessao nao existe (arquivo ausente).
2. `FileStore.Save` seguido de `FileStore.Load` retornando o estado persistido corretamente.
3. `FileStore.Save` preservando ordem cronologica de mensagens apos Load.
4. `FileStore.Save` preservando Records com Kind, Name e Data apos Load.
5. `FileStore.Save` preservando Metadata apos Load.
6. `FileStore.Save` com escrita atomica: arquivo corrompido a meio nao deixa lixo observavel.
7. `FileStore.Load` com arquivo JSON invalido retornando erro classificavel.
8. `FileStore.Load` honrando cancelamento de contexto.
9. `FileStore.Save` honrando cancelamento de contexto.
10. `NewFileStore` criando diretorio automaticamente quando nao existe.
11. `NewFileStore` retornando erro com diretorio invalido ou inacessivel.
12. Teste de restart: dois runs independentes com `FileStore` apontando para o mesmo diretorio, segundo run observando historico do primeiro via `Load`.
13. `FileStore` seguro para uso concorrente: multiplos goroutines executando `Load` e `Save` sem race condition.
14. Integracao: agent com `FileStore` configurado executando `Run` com persistencia end-to-end.
15. Integracao: `App` com `Defaults.Agent.Memory` definido como `FileStore`, factory herdando o default.
16. Integracao: agent com `WithMemory` sobrescrevendo o default do `App`.
17. `InMemoryStore` continuando funcional sem alteracoes breaking apos introducao do `FileStore`.
18. `FileStore.Save` sobrescrevendo sessao existente com novo estado completo.

---

## 13. Criterios de aceitacao

Esta spec sera considerada concluida quando:

1. `memory.Store` permanecer como contrato canonico sem interface paralela
2. `InMemoryStore` continuar funcionando sem alteracoes breaking
3. existir `FileStore` implementando `memory.Store` com durabilidade real
4. a arquitetura permitir bancos de dados por adapter sem mudanca na API publica
5. houver teste executavel cobrindo persistencia com restart (caso 12)
6. houver documentacao clara de configuracao do backend
7. houver example ou demo usando `FileStore` com persistencia real
8. `go test ./...` continuar passando com todos os testes existentes e novos

---

## 14. Evidencia obrigatoria

A implementacao deve deixar evidencia em:

| Artefato | Localizacao esperada |
| --- | --- |
| implementacao de `FileStore` | `pkg/memory/file_store.go` |
| testes do `FileStore` | `pkg/memory/file_store_test.go` |
| teste de restart | `pkg/memory/file_store_test.go` ou `test/conformance/memory/` |
| example/demo com persistencia | `examples/file-persistence/` ou equivalente |
| documentacao de configuracao | README ou doc.go com instrucoes de uso |
| nota de decisoes de design | esta spec e comentarios em codigo quando relevante |

---

## 15. Fora do escopo tecnico explicito

Os itens abaixo estao explicitamente fora desta spec:

- migrations engine para backends de banco
- query language custom sobre historico
- transactional saga entre multiplos stores
- sync distribuido entre instancias
- encryption at rest
- compactacao ou sumarizacao automatica de historico
- versionamento otimista ou compare-and-swap
- busca vetorial ou semantica
- painel visual de conversas
- file locks entre processos
- streaming de persistencia (flush por chunk)

---

## 16. Observacao normativa: dual-spec

Conforme o `AGENTS.md`, toda trilha nova deve possuir duas specs normativas e rastreaveis:

- **spec de construcao**: esta spec (220)
- **spec de diagnostico**: `specs/221-conversation-persistence-diagnosis.md`

A spec de diagnostico (221) ainda nao existe. Ela e necessaria antes de iniciar implementacao para definir sinais observaveis, sintomas de falha, hipoteses, troubleshooting e criterios de confirmacao.

Enquanto a spec 221 nao existir, a trilha de conversation persistence deve ser tratada como gap de especificacao aberto, mesmo que a spec de construcao esteja completa.

---

## 17. Questoes em aberto

1. A sanitizacao de `sessionID` para nome de arquivo deve usar hex encoding ou substituicao por `_`? Hex e mais seguro contra colisoes; `_` e mais legivel.
2. O `FileStore` deve suportar configuracao de permissoes de arquivo (file mode), ou `0644` e suficiente como default?
3. Deve existir um helper de migracao que converta dados de `InMemoryStore` para `FileStore` para facilitar transicao em cenarios de desenvolvimento?
4. O contrato de `Store` precisa de metodo `Close()` ou `Flush()` para backends que mantenham estado bufferizado? Na v1, a resposta e nao, mas backends futuros podem precisar.

---

## 18. Referencias cruzadas

| Spec | Relacao |
| --- | --- |
| Spec 000 | alvo de compatibilidade geral |
| Spec 010 | feature matrix: Conversation persistence como P1 |
| Spec 020 | arquitetura de repositorio e regras de dependencia |
| Spec 030 | contrato do Agent, integracao com memory, ordem de Load/Save |
| Spec 031 | App instance, hierarquia de defaults, wiring de Memory |
| Spec 050 | contrato de memory, Store, Snapshot, Delta, chaves de identidade |
| Spec 200 | diagnostico de paridade: conversation persistence como gap |
