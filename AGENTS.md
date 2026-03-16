# AGENTS.md

## 1. Project mission

- `gaal-lib` existe para construir uma biblioteca em Go inspirada no nucleo do Voltagent.
- O objetivo principal e atingir paridade funcional com o framework base do Voltagent.
- A API publica deve ser idiomatica em Go, mesmo quando o conceito de origem vier do Voltagent.
- VoltOps esta explicitamente fora do escopo.
- O repositorio segue Spec Driven Development rigoroso.

## 2. Source of truth

- As specs em `specs/` sao a fonte de verdade para requisitos por modulo.
- `specs/000-compatibility-target.md` define o alvo de compatibilidade e as regras globais de paridade.
- `specs/001-non-goals.md` define o que nao pode entrar no escopo.
- `specs/010-feature-matrix.md` deve ser usada como checklist de cobertura, prioridade, risco e paridade.
- `specs/020-repository-architecture.md` define as fronteiras entre `pkg/`, `internal/`, `test/` e `examples/`.
- Specs especificas de modulo, como `specs/030-agent.md` e `specs/031-app-instance.md`, governam o contrato daquele modulo.
- Se houver conflito entre implementacao existente e spec aprovada, a spec prevalece.

## 3. Required execution workflow

- Sempre ler as specs relevantes antes de implementar, refatorar ou revisar comportamento.
- Identificar explicitamente qual spec e qual secao cobrem a tarefa antes de alterar codigo.
- Nunca implementar feature fora da spec sem registrar a lacuna.
- Se a spec nao existir, estiver incompleta ou ambigua para a tarefa, parar a implementacao e registrar o gap de especificacao.
- Nunca alterar API publica sem atualizar a spec correspondente.
- Toda feature implementada deve ser rastreavel para pelo menos uma spec.
- Toda mudanca de comportamento deve incluir atualizacao ou adicao de testes.
- Toda mudanca relevante deve ser validada contra a feature matrix como checklist de cobertura.

## 4. Architectural rules

- Manter separacao clara entre `pkg` e `internal`.
- `pkg/` contem contratos publicos estaveis e pontos de extensao importaveis.
- `internal/` contem runtime, wiring, validacao, state machine e detalhes operacionais nao exportados.
- Nao expor tipos de `internal/*` em assinaturas exportadas, erros publicos ou exemplos.
- Preferir interfaces pequenas e composicao.
- Evitar acoplamento ciclico entre pacotes.
- Evitar dependencias externas desnecessarias.
- Nao introduzir dependencia arquitetural em servicos hospedados para o runtime funcionar localmente.
- `pkg/app` e a unica ponte publica permitida para `internal/runtime`, conforme a arquitetura do repositorio.

## 5. Public API rules

- API publica deve ser pequena, intencional e orientada a contratos observaveis.
- Priorizar `context.Context` como fronteira padrao para execucao, cancelamento e lifecycle.
- Nomes, tipos e assinaturas em Go nao precisam copiar o Voltagent literalmente.
- Diferencas em relacao ao Voltagent so sao aceitaveis quando forem idiomaticas em Go e estiverem explicitadas na spec correspondente ou na feature matrix.
- Nunca alterar API publica sem atualizar a spec correspondente e os testes de contrato afetados.
- Sempre documentar erros observaveis, comportamento de cancelamento e ordem de execucao quando fizerem parte do contrato.
- Preferir construtores claros, options objetivas e tipos exportados apenas quando necessario.

## 6. Testing and conformance rules

- Sempre adicionar ou atualizar testes ao mudar comportamento.
- Testes de conformidade devem ser tratados como parte do contrato, nao como opcionais.
- Toda implementacao deve ter pelo menos um caminho de teste de compatibilidade conceitual ou comportamental.
- Casos de sucesso, falha, cancelamento e limites observaveis devem ser cobertos quando a spec exigir.
- Testes em `pkg/*` devem validar contratos publicos.
- Testes em `internal/*` devem validar detalhes de runtime e corretude operacional.
- Suites em `test/conformance` devem usar apenas a superficie publica em `pkg/*`.
- Se uma mudanca nao puder ser coberta por teste imediatamente, o gap deve ser declarado explicitamente com justificativa tecnica.

## 7. Spec compliance rules

- Nunca implementar feature fora da spec sem registrar a lacuna.
- Nunca assumir comportamento nao documentado quando a spec exigir semantica observavel.
- Se o codigo divergir da spec, corrigir o codigo ou atualizar a spec antes de seguir.
- Se a implementacao precisar de comportamento novo, primeiro atualizar ou propor a spec.
- Toda PR, patch ou entrega deve deixar claro qual spec foi atendida e quais criterios de aceitacao foram cobertos.
- Quando uma feature for parcialmente implementada, registrar explicitamente o que ficou pendente e qual secao da spec ainda nao foi atendida.

## 8. Voltagent parity rules

- O objetivo e paridade funcional, nao copia textual de API.
- Comparar comportamento por entrada, saida, efeitos colaterais controlados, erros e ordem de operacoes relevantes.
- Diferencas em relacao ao Voltagent so sao aceitaveis quando forem idiomaticas em Go e estiverem explicitadas na spec ou na feature matrix.
- Nao aceitar divergencia "conveniente" apenas para simplificar a implementacao.
- Nao introduzir nada de VoltOps.
- Quando houver diferenca intencional de comportamento, registrar a justificativa e o impacto na paridade.

## 9. Documentation rules

- Atualizar documentacao relevante sempre que a mudanca alterar comportamento observavel, API publica ou fluxo operacional importante.
- Nunca alterar API publica sem atualizar a spec correspondente.
- Usar `README.md` para contexto geral do projeto, nao para substituir specs normativas.
- Manter a feature matrix atual quando uma feature mudar de status, prioridade pratica ou cobertura.
- Documentar gaps restantes, riscos e divergencias intencionais.
- Relacionar codigo novo ou alterado com a spec correspondente de forma rastreavel.

## 10. Expected output format for code tasks

- Explicar quais specs foram lidas e quais secoes governaram a mudanca.
- Explicar quais arquivos foram alterados.
- Explicar as decisoes tomadas e os tradeoffs relevantes.
- Explicar quais testes foram adicionados, atualizados ou executados.
- Explicar gaps restantes, limitacoes conhecidas e o que ficou fora da entrega.
- Se a tarefa nao puder ser implementada por falta de spec, dizer isso explicitamente e apontar o gap de especificacao.
- Se houver divergencia intencional em relacao ao Voltagent, apontar a spec ou item da feature matrix que autoriza a diferenca.
