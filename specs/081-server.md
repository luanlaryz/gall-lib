# Spec 081: Server

## 1. Objetivo

Este documento define a abstracao de `server` e `serverless` para expor uma `App` da `gaal-lib`.

Os objetivos desta spec sao:

- definir o contrato local para servidores long-lived gerenciados pelo lifecycle de `App`
- definir o contrato local para cenarios serverless e invocacoes curtas
- detalhar o ciclo de `start/stop` e sua ordem observavel
- definir a semantica local de `health` e `readiness` sem acoplamento a framework HTTP
- formalizar a integracao com registry de `agents` e `workflows` via `Runtime`
- detalhar graceful shutdown, draining e rollback de startup parcial

Esta spec complementa a `Spec 010`, a `Spec 020` e a `Spec 031`.

Compatibilidade com a `Spec 010`:

- `HTTP server abstraction` e `Serverless abstraction` continuam capacidades opcionais e locais
- esta spec detalha o contrato local dessas abstracoes, sem puxar escopo de plataforma hospedada
- a spec nao muda por si so a prioridade ou o status da matriz

Ficam fora do escopo desta spec:

- implementacao concreta de HTTP, gRPC, WebSocket, SSE, Lambda-like ou plataforma de cloud especifica
- autenticacao de endpoints, rate limiting, middlewares de transporte e serializacao de payloads
- autoscaling, load balancing, ingress, control plane ou qualquer capacidade de VoltOps
- deploy, configuracao remota, dashboards operacionais ou telemetria hospedada

## 2. Principios normativos

1. `App` continua sendo a unica ponte publica entre adapters e `internal/runtime`, conforme a `Spec 020`.
2. A abstracao de server deve depender apenas de contratos publicos, em especial `pkg/app`.
3. Nenhum contrato desta spec pode exigir processo residente para o caso serverless.
4. `Runtime` e a view somente-leitura canonica para descoberta de capacidades registradas.
5. Startup e shutdown devem ser cooperativos, idempotentes e orientados por `context.Context`.
6. Health e readiness devem ser conceitos locais e independentes de framework.
7. Implementacoes concretas continuam opcionais e nao devem contaminar o core com detalhes de transporte.

## 3. Escopo arquitetural

### 3.1 Onde o contrato vive

Nesta fase, os contratos normativos centrais continuam compativeis com `pkg/app`:

- `Server`
- `ServerlessHook`
- `Runtime`
- `Target`
- `Start`
- `EnsureStarted`
- `Shutdown`

Observacao arquitetural:

- `pkg/server` passa a poder oferecer helpers publicos aditivos para probes, invocacao curta e adapters de transporte sem mover os contratos centrais de `pkg/app`
- esta spec nao exige mover os contratos ja existentes de `pkg/app`
- qualquer evolucao para `pkg/server` deve preservar compatibilidade observavel com os nomes publicos ja adotados

### 3.2 Componentes reconhecidos

Esta spec reconhece tres papeis:

1. `App`: composition root, lifecycle owner e fonte de `Runtime`
2. `Server`: componente long-lived gerenciado por `App`
3. `Serverless adapter`: adaptador de invocacao curta que usa `EnsureStarted`, `OnInvokeStart` e `OnInvokeDone`

## 4. Abstracao de server

### 4.1 Contrato minimo

O contrato minimo para um servidor long-lived permanece:

```go
package app

type Server interface {
    Name() string
    Start(ctx context.Context, rt Runtime) error
    Shutdown(ctx context.Context) error
}
```

### 4.2 Responsabilidades de `Server`

Um `Server` deve ser responsavel por:

- iniciar seu listener, loop de consumo ou mecanismo equivalente
- usar `Runtime` para resolver `agents` e `workflows`
- expor, quando aplicavel, probes locais de `health` e `readiness`
- cooperar com `Shutdown(ctx)` sem depender de encerramento abrupto do processo

Um `Server` nao deve ser responsavel por:

- construir ou mutar `App`
- acessar `internal/*`
- manter registries paralelos de `agents` ou `workflows`
- assumir controle do lifecycle global da aplicacao fora do proprio componente

### 4.3 Regras normativas de `Server.Start`

1. `Server.Start` recebe um `Runtime` ja consistente para leitura.
2. `Runtime.ResolveAgent` e `Runtime.ResolveWorkflow` sao as formas canonicas de descoberta durante a execucao.
3. `Server.Start` nao deve registrar novos `agents` ou `workflows`.
4. `Server.Start` deve honrar cancelamento e deadline de `ctx`.
5. Falha de `Server.Start` deve abortar o startup do `App`.
6. Se `Server.Start` falhar, o `App` deve iniciar rollback dos servidores ja iniciados.

### 4.4 Regras normativas de `Server.Shutdown`

1. `Shutdown` deve ser idempotente do ponto de vista observavel.
2. `Shutdown` deve parar de aceitar novo trafego o mais cedo possivel.
3. `Shutdown` deve permitir drenagem cooperativa de operacoes em andamento, respeitando `ctx`.
4. Falha de `Shutdown` nao deve impedir tentativa de desligar os demais `Server`.
5. `Shutdown` nao deve exigir acesso mutavel a `Runtime`.

## 5. Abstracao de serverless

### 5.1 Contrato minimo

O contrato minimo para observacao de cold start e invocacoes curtas permanece:

```go
package app

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

### 5.2 Objetivo do contrato serverless

O contrato serverless existe para:

- preparar recursos locais na primeira transicao bem-sucedida para runtime pronto
- envolver invocacoes curtas com contexto adicional de observabilidade, tracing ou metricas locais
- permitir bootstrap seguro via `EnsureStarted`

Ele nao existe para:

- assumir processo residente
- obrigar servidor HTTP local
- exigir runtime especifico de cloud provider

### 5.3 Regras normativas de `OnColdStart`

1. `OnColdStart` deve ser disparado apenas apos registries materializados e runtime local pronto para uso.
2. `OnColdStart` deve ocorrer no maximo uma vez por instancia materializada de `App`.
3. Falha em `OnColdStart` deve ser tratada como falha de startup da aplicacao.
4. `OnColdStart` recebe `Runtime` somente-leitura, nunca o runtime interno concreto.

### 5.4 Regras normativas de invocacao curta

Adapters serverless devem seguir a ordem abaixo para cada invocacao:

1. chamar `App.EnsureStarted(ctx)`
2. chamar `OnInvokeStart(ctx, target)` de cada hook registrado, em ordem
3. usar o `context.Context` resultante para resolver e executar o alvo
4. chamar `OnInvokeDone(ctx, target, err)` ao final da invocacao

Regras adicionais:

1. `OnInvokeStart` pode enriquecer contexto de correlacao local, mas nao deve esconder cancelamento e deadline do contexto original.
2. `OnInvokeDone` deve sempre ser chamado, mesmo em erro de negocio do alvo.
3. O adaptador serverless pode delegar essa sequencia a um helper publico de `App` ou `pkg/server`, desde que a ordem observavel seja preservada.
4. `Target.Kind` e `Target.Name` devem identificar de forma observavel o alvo executado.

## 6. Ciclo start/stop

### 6.1 Ordem normativa de startup

O startup observavel de `App` com `Server` deve obedecer a ordem abaixo:

1. validar `ctx` e estado atual do `App`
2. transicionar de `created` para `starting`
3. emitir `app.starting`
4. materializar `AgentFactory` e `WorkflowFactory`
5. selar registries para novas insercoes
6. expor `Runtime` consistente para leitura
7. chamar `Server.Start` de cada server registrado, em ordem declarada
8. disparar `OnColdStart` dos hooks serverless, quando houver
9. transicionar para `running`
10. emitir `app.started`

Regras adicionais:

1. Se qualquer factory falhar, o startup falha antes de iniciar `Server`.
2. Se qualquer `Server.Start` falhar, o `App` deve executar rollback dos servers ja iniciados.
3. Se qualquer `OnColdStart` falhar, o `App` deve executar rollback dos servers ja iniciados.
4. Depois de `running`, `Runtime.ResolveAgent` e `Runtime.ResolveWorkflow` devem refletir apenas componentes materializados com sucesso.

### 6.2 Ordem normativa de shutdown

Ao receber `Shutdown(ctx)` em `StateRunning` ou `StateStarting`, a ordem minima deve ser:

1. transicionar para `stopping`
2. emitir `app.stopping`
3. tornar `readiness` negativa para novo trafego
4. propagar contexto de shutdown aos servers
5. chamar `Server.Shutdown` em ordem reversa da inicializacao
6. aguardar drenagem cooperativa ou expiracao de `ctx`
7. limpar referencia a servers iniciados
8. transicionar para `stopped`
9. emitir `app.stopped`

Regras adicionais:

1. `Shutdown` em `StateCreated` deve apenas mover para `StateStopped`.
2. `Shutdown` em `StateStopped` deve retornar `nil`.
3. `Shutdown` em `StateStopping` deve ser idempotente.
4. Se o `ctx` expirar durante o shutdown, o erro retornado deve preservar `context.DeadlineExceeded`.

## 7. Health e readiness

### 7.1 Conceitos normativos

Esta spec distingue:

- `health`: capacidade local de o processo e o adapter responderem a uma probe de vida
- `readiness`: capacidade local de aceitar novo trafego util para `agents` e `workflows`

### 7.2 Semantica minima por estado do `App`

| Estado do `App` | Health | Readiness |
| --- | --- | --- |
| `created` | saudavel | nao pronto |
| `starting` | saudavel | nao pronto |
| `running` | saudavel | pronto |
| `stopping` | saudavel enquanto o processo drena | nao pronto |
| `stopped` | nao saudavel para novo uso | nao pronto |

Regras normativas:

1. `readiness` deve ficar negativa antes do inicio efetivo do shutdown dos servers.
2. `health` pode permanecer positiva durante draining, desde que o processo ainda consiga responder a probes locais.
3. Falha de bootstrap terminal deve resultar em `health` negativa para aquela instancia parada.
4. No modo serverless, `readiness` e interpretada como "runtime pronto para a invocacao corrente" e nao exige endpoint residente.

### 7.3 Endpoints e probes

Para adapters HTTP-like, os endpoints padrao recomendados da v1 sao:

- `/healthz`
- `/readyz`

Regras normativas:

1. A semantica dos endpoints deve seguir os conceitos desta spec, independentemente do framework.
2. Um adapter nao HTTP pode expor probes equivalentes por outro mecanismo, desde que preserve a mesma semantica local.
3. Esta spec nao exige formato de payload especifico para probes alem da semantica de saudavel/pronto.
4. Quando um adapter expuser payload detalhado, ele nao deve depender de `internal/*`.

### 7.4 Readiness e draining

Durante graceful shutdown:

1. readiness deve ser a primeira probe a mudar para "nao pronto"
2. o adapter deve parar de aceitar novas requisicoes ou novas mensagens de entrada
3. operacoes inflight podem continuar ate terminar ou ate `ctx` expirar
4. health pode permanecer "saudavel" enquanto o processo ainda estiver drenando com controle

Observacao de implementacao:

- `App.Health()` e `App.Ready()` podem expor essa semantica diretamente para adapters locais
- `pkg/server` pode oferecer helpers como snapshots de probe sem introduzir dependencia em HTTP

## 8. Integracao com registry de agents e workflows

### 8.1 Descoberta via `Runtime`

`Runtime` continua sendo o contrato canonico para descoberta:

```go
package app

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

### 8.2 Regras normativas de integracao

1. Um `Server` nao deve manter copia mutavel dos registries; deve consultar `Runtime`.
2. `ResolveAgent` e `ResolveWorkflow` devem ser usados para lookup por nome logico.
3. `ListAgents` e `ListWorkflows` devem ser tratados como snapshots somente-leitura.
4. Adapters devem retornar erro explicito quando o alvo solicitado nao existir.
5. Apos startup bem-sucedido, a forma recomendada de descoberta e sempre `Runtime()`.

### 8.3 Fronteira entre `App`, `Server` e dominio

1. `Server` expoe transporte; ele nao redefine o contrato de `Agent` nem de `Workflow`.
2. Serializacao, headers, codigos HTTP ou detalhes equivalentes ficam no adapter concreto.
3. A execucao de negocio continua governada pelas specs de `Agent`, `Workflow`, `Memory` e `Guardrails`.

## 9. Graceful shutdown

### 9.1 Objetivo

Graceful shutdown significa:

- parar de aceitar novo trafego
- permitir termino cooperativo das operacoes inflight
- encerrar servers em ordem previsivel
- preservar logs e eventos de lifecycle

### 9.2 Regras normativas

1. `App` deve chamar `Server.Shutdown` em ordem reversa da inicializacao.
2. Falha de shutdown de um server nao deve impedir tentativa de desligar os demais.
3. Se um server suportar draining explicito, ele deve entrar em modo de nao aceitar novas requisicoes antes de interromper listeners.
4. Operacoes inflight devem observar cancelamento cooperativo do contexto quando o shutdown avancar.
5. O timeout efetivo de shutdown continua sendo o menor entre deadline do `ctx` recebido e `Defaults.ShutdownTimeout`, conforme a `Spec 031`.
6. O runtime nao deve forcar terminacao abrupta sem antes conceder oportunidade de encerramento cooperativo.

### 9.3 Startup rollback

Se `Start` falhar depois de inicializar parcialmente a aplicacao:

1. todo `Server` ja iniciado deve receber `Shutdown`
2. componentes parcialmente inicializados nao devem permanecer visiveis em `Runtime()` como runtime pronto
3. `App` deve terminar em `StateStopped`
4. `app.bootstrap_failed` deve permanecer observavel para diagnostico local

## 10. Regras para implementacoes futuras

Implementacoes futuras desta spec devem respeitar:

1. dependencia apenas de contratos publicos
2. `App` como composition root e dono do lifecycle
3. `Runtime` como view somente-leitura para lookup e logger
4. probes de `health` e `readiness` coerentes com o estado do `App`
5. serverless sem suposicao de processo residente
6. ausencia de dependencia de servico hospedado para execucao local

Observacao arquitetural:

- um futuro pacote `pkg/server` pode oferecer helpers de transporte, probe helpers e adapters reutilizaveis
- isso nao autoriza dependencias de `pkg/agent` ou `pkg/workflow` para `internal/*`

## 11. Criterios de aceitacao

Esta spec so pode ser considerada atendida quando:

1. existir um contrato publico coerente para `Server` long-lived e `ServerlessHook`
2. `Start`, `EnsureStarted` e `Shutdown` preservarem a ordem normativa de lifecycle documentada
3. `Runtime` puder ser usado por adapters para resolver `agents` e `workflows` sem acesso a `internal/*`
4. health e readiness estiverem definidos de forma local e independente de framework
5. readiness cair antes do inicio efetivo do draining em shutdown
6. startup parcial com falha executar rollback dos servers ja iniciados
7. graceful shutdown for cooperativo, idempotente e orientado por `context.Context`
8. cenarios serverless puderem usar `EnsureStarted`, `OnColdStart`, `OnInvokeStart` e `OnInvokeDone` sem processo residente obrigatorio
9. nenhuma parte da feature exigir VoltOps ou servico hospedado para funcionar localmente

## 12. Casos de teste obrigatorios

Toda implementacao futura desta spec deve cobrir, no minimo, os casos abaixo.

1. `Server.Start` recebendo `Runtime` capaz de resolver um agent registrado.
2. `Server.Start` recebendo `Runtime` capaz de resolver um workflow registrado.
3. falha de uma `AgentFactory` impedindo inicio de qualquer server.
4. falha de uma `WorkflowFactory` impedindo inicio de qualquer server.
5. falha do primeiro `Server.Start` abortando o startup.
6. falha de um `Server.Start` intermediario disparando rollback dos servers ja iniciados.
7. `Server.Shutdown` sendo chamado em ordem reversa da inicializacao.
8. `Shutdown` em `StateCreated` levando o app a `StateStopped` sem erro.
9. `Shutdown` em `StateStopped` retornando `nil`.
10. expiracao de contexto durante shutdown preservando `context.DeadlineExceeded`.
11. readiness retornando "nao pronto" em `created`.
12. readiness retornando "nao pronto" em `starting`.
13. readiness retornando "pronto" em `running`.
14. readiness retornando "nao pronto" em `stopping`.
15. health permanecendo positiva durante draining controlado antes do estado final `stopped`.
16. startup com sucesso expondo probes coerentes de `health` e `readiness`.
17. falha terminal de bootstrap resultando em probes coerentes para instancia parada.
18. `OnColdStart` sendo chamado no maximo uma vez por instancia de `App`.
19. falha em `OnColdStart` disparando rollback dos servers iniciados.
20. adaptador serverless chamando `EnsureStarted` antes da primeira invocacao.
21. `OnInvokeStart` podendo enriquecer o contexto da invocacao.
22. `OnInvokeDone` sendo chamado tanto em sucesso quanto em erro.
23. `Target.Kind` e `Target.Name` identificando corretamente o alvo resolvido.
24. adapter HTTP-like expondo `/healthz` e `/readyz` com semantica coerente com o estado do `App`.
25. adapter nao HTTP expondo probe equivalente sem quebrar a semantica local de health/readiness.

## 13. Questoes em aberto

1. `pkg/server` deve ser promovido logo na primeira implementacao desses adapters, ou a v1 deve manter apenas os contratos em `pkg/app` e deixar helpers de transporte para depois?
2. A v1 deve expor tipos publicos reutilizaveis para payload de probe, ou a semantica local de saudavel/pronto e suficiente nesta fase?
3. Readiness de `Server` multiplo deve ser a intersecao de todos os servers gerenciados pelo `App`, ou pode existir diferenciacao por adapter no futuro?
