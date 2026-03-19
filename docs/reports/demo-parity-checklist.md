# Demo Parity Checklist

Date: 2026-03-18 (closure audit — Spec 114)

Previous audit: 2026-03-17

## 9.1 Build e ambiente

- [x] `go.mod` foi inspecionado.
- [x] Versão de Go requerida foi registrada: `go 1.26.1`.
- [x] Ambiente atual foi comparado com a versão exigida: `go version go1.26.1 darwin/arm64`.
- [x] `go test ./...` foi tentado.
- [x] Bloqueios foram documentados: nenhum bloqueio encontrado.

## 9.2 Boot e lifecycle

- [x] [cmd/demo-app/main.go](/Users/luanlima/Documents/study/go/gaal-lib/cmd/demo-app/main.go) foi inspecionado.
- [x] A composição da demo foi identificada em `cmd/demo-app/` (`config.go`, `agent.go`, `server.go`).
- [x] `App.Start()` é usado.
- [x] `App.Shutdown()` é usado.
- [x] Sinais do sistema são tratados.
- [x] Base URL e endpoints são anunciados no boot.

## 9.3 Superfície HTTP

- [x] `GET /healthz` existe.
- [x] `GET /readyz` existe.
- [x] `GET /agents` existe.
- [x] `POST /agents/{name}/runs` existe.
- [x] `POST /agents/{name}/stream` existe.
- [x] Erro de método inválido existe.
- [x] Erro de agent não encontrado existe.
- [x] Erro de request inválido existe.

## 9.4 Runtime demonstrado

- [x] Há um agent da demo.
- [x] Há memória in-memory por default.
- [x] Há logger local.
- [x] O runtime usa registry real de agents.
- [x] O stream vem de `Agent.Stream`.
- [x] O run síncrono vem de `Agent.Run`.

## 9.5 DX e docs

- [x] README existe.
- [x] `.env.example` existe.
- [x] `demo.http` existe.
- [x] O README bate com os endpoints reais.
- [x] O README documenta lacunas.
- [x] O README descreve smoke manual.

## 9.6 Testes

- [x] Existe smoke test automatizado.
- [x] O smoke testa boot.
- [x] O smoke testa health.
- [x] O smoke testa readiness.
- [x] O smoke testa listagem.
- [x] O smoke testa text run.
- [x] O smoke testa streaming SSE.
- [x] O smoke testa `404` para agent inexistente.
- [x] O smoke testa `400` para request inválido.
- [x] O smoke testa `405` para método inválido.
- [x] O smoke testa limpeza de memória após reinício.
- [x] Lacunas de teste foram registradas no relatório.

## Observações de auditoria

- A checklist obrigatória da `Spec 110` foi satisfeita integralmente.
- Os gaps DPG-001, DPG-002 e DPG-003 foram resolvidos.
- O binário da demo não importa `internal/*`, conforme a `Spec 100`.
- A cobertura de smoke agora inclui o caminho principal, erros HTTP observáveis, streaming SSE e prova de limpeza de memória após reinício do processo.
- Ver [docs/reports/demo-parity-report.md](/Users/luanlima/Documents/study/go/gaal-lib/docs/reports/demo-parity-report.md) para scorecard, severidades e decisão de aptidão.

## Closure audit (Spec 114) — 2026-03-18

A checklist da `Spec 110` foi reaplicada integralmente em 2026-03-18 como parte da `Spec 114: Demo V1 Closure Diagnosis`.

### Evidência executável fresca

- `go test ./... -v -count=1` executado sem cache em 2026-03-18: **PASS** (0 falhas).
- `test/smoke` executou `TestDemoApp` (11 subtests) e `TestDemoAppMemoryResetAfterRestart`: todos PASS.
- Ambiente: `go version go1.26.1 darwin/arm64`, `go.mod` declara `go 1.26.1`.

### Resultado da reaplicação

- Todos os 39 itens da checklist (9.1–9.6) permanecem marcados [x].
- Nenhum item regrediu ou mudou de status.
- Nenhum gap novo identificado.
- Os 3 gaps anteriores (DPG-001, DPG-002, DPG-003) permanecem RESOLVED.

### Decisão

A checklist confirma o status `APT for base demo parity` sem reservas.
