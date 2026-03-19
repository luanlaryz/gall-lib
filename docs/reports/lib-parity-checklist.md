# Library Parity Checklist

Date: 2026-03-19 (Spec 200: Library Parity Diagnosis)

## Ambiente

- [x] `go.mod` inspecionado: `go 1.26.1`, zero dependencias externas (apenas stdlib).
- [x] Toolchain compativel: `go version go1.26.1 darwin/arm64`.
- [x] `go test ./...` executado em 2026-03-19 sem cache (`-count=1`): **PASS** (0 falhas).

## Agent

- [x] Run sincrono: `Agent.Run(ctx, Request) (Response, error)` implementado em `pkg/agent/agent.go`, coberto por `TestRunSuccess` e 13 outros testes unitarios.
- [x] Stream: `Agent.Stream(ctx, Request) (Stream, error)` implementado, coberto por `TestStreamCloseCancelsRunAndIsIdempotent` e smoke `streaming_sse`.
- [x] Context/cancelamento: `TestRunCanceledByContext` confirma cancelamento cooperativo via `context.Context`.
- [x] Memory/tools/guardrails integraveis: options `WithMemory`, `WithTools`, `WithInputGuardrails`, `WithStreamGuardrails`, `WithOutputGuardrails` existem e sao exercitadas nos testes.

## App/Runtime

- [x] Registry de agents: `AgentRegistry` com `Register`, `RegisterFactory`, `Resolve`, `List`. Coberto por `TestAgentRegistryRejectsDuplicateReadyAgent` e `TestStartMaterializesFactoriesDeterministically`.
- [x] Registry de workflows: `WorkflowRegistry` com `Register`, `RegisterFactory`, `Resolve`, `List`. Coberto por `TestResolveWorkflowReturnsReadyWorkflow` e `TestWorkflowFactoryInheritsAppWorkflowDefaults`.
- [x] Defaults: `Defaults` com `AgentDefaults` e `WorkflowDefaults` propagados para factories. Coberto por `TestAgentFactoryInheritsAppMemoryDefaults` e `TestAgentFactoryInheritsAppStreamGuardrails`.
- [x] Start/shutdown: `App.Start()`, `App.Shutdown()`, `App.EnsureStarted()`, `App.Run()` implementados. Coberto por `TestAppHealthAndReadinessFollowLifecycle` e `TestShutdownCreatedMovesToStopped`.

## Tools

- [x] Tools existem: interface `Tool` com `Name`, `Description`, `InputSchema`, `OutputSchema`, `Call` em `pkg/tool/tool.go`. `Toolkit` com namespace e flattening.
- [x] Tool path testado: 11 testes unitarios em `pkg/tool/tool_test.go` cobrindo registry, validacao, invocacao, cancelamento e concorrencia.
- [x] Erros de tool testados: `TestInvokeRejectsInvalidInputWithoutExecuting`, `TestInvokeRejectsInvalidOutput`, `TestInvokeRespectsCancellationAndWrapsExecutionErrors`. Sentinels tipados em `pkg/tool/errors.go`.

## Orquestracao entre agents

- [ ] Agents-as-tools ou equivalente: nenhum mecanismo implementado em `pkg/` ou `internal/`. `StepKindInvoke` e reservado mas nao implementado.
- [ ] Demo/teste de coordenacao: nenhum exemplo, teste ou demo demonstra coordenacao entre agents.

## Memory

- [x] In-memory default: `InMemoryStore` implementado em `pkg/memory/in_memory_store.go`. Coberto por `memory_test.go` e smoke `text_run_with_memory`.
- [x] Stateless/disable semantics claras: `Request.SessionID` vazio com memory configurada gera `ErrInvalidRequest`. Sem memory, agent funciona stateless.
- [ ] Working memory avaliada: `InMemoryWorkingMemoryFactory` e `InMemoryWorkingSet` existem e funcionam para agent runs. Integracao com workflows pendente.
- [ ] Persistence semantics avaliadas: `Conversation persistence` esta `Nao iniciado` na feature matrix (P1). Apenas in-process; limpa no restart (documentado e testado por `TestDemoAppMemoryResetAfterRestart`).

## Guardrails

- [x] Input: `guardrail.Input` com `CheckInput`. Demo prova com `inputBlockGuardrail` (BLOCK_ME -> HTTP 422). Coberto por `guardrail_input_block`.
- [x] Output: `guardrail.Output` com `CheckOutput`. Demo prova com `outputTagGuardrail` (sufixo `[guardrail:ok]`). Coberto por `guardrail_output_tag_on_run`.
- [x] Streaming: `guardrail.Stream` com `CheckStream`. Demo prova com `streamDigitGuardrail` (digitos -> `*`). Coberto por `guardrail_stream_digit_redaction`.
- [x] Intervencao observavel: `ActionAllow`, `ActionBlock`, `ActionTransform`, `ActionDrop`, `ActionAbort` definidos. Drop e abort cobertos por testes unitarios em `internal/runtime/guardrail_stream_test.go`.

## Workflows

- [x] Chain: `workflow.Chain` com execucao sequencial de steps. Coberto por `TestRunExecutesSequentialStepsWithSharedState`.
- [x] Branching: `workflow.Branch` com `DecisionFunc`. Demo prova com `route_order`. Coberto por `TestBranchRoutesToNamedStep`.
- [x] Retries: `FixedRetryPolicy` e `RetryPolicy` interface. Coberto por `TestWorkflowRetryAppliesWhenStepHasNoOverride` e `TestStepRetryOverridesWorkflowRetry`.
- [x] Hooks/estado: `Hook` interface, `NewLoggingHook`, `State` compartilhado. Coberto por `TestHooksObserveLifecycleOrder`.
- [x] Demo/teste: workflow `order-processing` na demo com 5 steps, 5 subtests no smoke.

## HTTP/Server

- [x] Health/readiness: `GET /healthz` e `GET /readyz` na demo. `pkg/server.Health()`, `pkg/server.Ready()`, `pkg/server.Snapshot()`. Cobertos por smoke `healthz` e `readyz`.
- [x] Listagem/execucao: `GET /agents`, `POST /agents/{name}/runs`. Cobertos por smoke `agents` e `text_run_with_memory`.
- [x] Stream HTTP: `POST /agents/{name}/stream` com SSE. Coberto por smoke `streaming_sse`.
- [x] Erros cobertos: `404` agent inexistente, `400` request invalido, `405` metodo invalido, `422` guardrail block, `500` tool error. Cobertos por 6 subtests de erro no smoke.

## DX

- [x] README: `README.md` descreve visao, objetivos, escopo, roadmap e modo de trabalho.
- [x] Examples: 7 exemplos funcionais (`simple-tool`, `simple-toolkit`, `basic-agent`, `server-lifecycle`, `workflow-chain`, `stream-redaction`, `memory`).
- [x] Demo(s): `cmd/demo-app` com UI web embutida, 8 arquivos de composicao, `examples/demo-app/README.md`, `.env.example`, `demo.http`.
- [x] Documentacao coerente: specs rastreavies, relatorios de diagnostico por trilha, feature matrix atualizada.

## Resumo

- Itens marcados: 30/34
- Itens nao marcados: 4
  - Agents-as-tools (2 itens)
  - Working memory workflow integration (1 item)
  - Conversation persistence (1 item)
