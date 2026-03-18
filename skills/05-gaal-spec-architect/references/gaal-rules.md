# Gaal-lib operating rules

Use este arquivo como resumo operacional do `AGENTS.md`.

## Source of truth
- `specs/` governa requisitos por modulo.
- `specs/000-compatibility-target.md` define compatibilidade e regras globais.
- `specs/001-non-goals.md` define exclusoes de escopo.
- `specs/010-feature-matrix.md` funciona como checklist de cobertura, prioridade, risco e paridade.
- `specs/020-repository-architecture.md` governa fronteiras entre `pkg/`, `internal/`, `test/` e `examples/`.
- Specs especificas, como `030-agent`, `050-memory` e `070-workflows`, governam contratos de modulo.
- Se implementacao e spec divergirem, a spec prevalece.

## Workflow obrigatorio
1. Ler specs relevantes antes de propor implementacao.
2. Identificar spec e secao antes de alterar codigo.
3. Nao implementar feature fora da spec.
4. Se a spec estiver ausente, incompleta ou ambigua, parar e registrar gap.
5. Toda mudanca de comportamento exige teste.
6. Toda entrega deve dizer qual spec foi atendida e quais criterios de aceitacao foram cobertos.

## Regras arquiteturais
- `pkg/` contem contratos publicos estaveis.
- `internal/` contem runtime e detalhes operacionais nao exportados.
- Nao expor `internal/*` em API publica.
- Preferir interfaces pequenas e composicao.
- Evitar dependencias externas desnecessarias.
- `pkg/app` e a ponte publica para `internal/runtime`.

## Regras de saida para tarefas de codigo
Sempre exigir que a implementacao final informe:
- specs lidas e secoes usadas
- arquivos alterados
- decisoes e trade-offs
- testes adicionados, atualizados ou executados
- gaps restantes
- divergencias intencionais em relacao ao Voltagent
