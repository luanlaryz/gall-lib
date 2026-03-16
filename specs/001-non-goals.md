# Spec 001: Non-Goals

## Proposito

Este documento lista o que fica explicitamente fora do escopo da v1 de `gaal-lib`. Ele existe para evitar deriva de escopo e para reforcar que a biblioteca cobre apenas o nucleo inspirado no Voltagent, sem absorver a camada operacional de VoltOps.

## Fora do escopo v1

Os itens abaixo nao devem ser implementados na v1:

1. Qualquer recurso de VoltOps.
2. Control plane, dashboard, paineis operacionais ou interfaces web de administracao.
3. Execucao remota gerenciada, runners hospedados ou infraestrutura de orquestracao operacional.
4. Observabilidade hospedada, tracing centralizado, log aggregation ou telemetria SaaS.
5. Multi tenancy, organizacoes, RBAC, billing, quotas ou governanca operacional.
6. Deploy tooling, release orchestration, autoscaling, filas distribuidas ou job scheduling operacional.
7. Secret management hospedado, configuracao remota ou dependencia de servicos externos de operacao.
8. Recursos enterprise ou de plataforma que nao facam parte do core conceitual do framework.
9. Paridade de API textual, visual ou estrutural quando ela nao for necessaria para paridade funcional.
10. Suporte amplo a todos os provedores e integracoes antes da estabilizacao do nucleo.
11. Compatibilidade com SDKs de outras linguagens como objetivo primario da v1.
12. Ferramentas de migracao automatica a partir de projetos Voltagent existentes.

## Regra de decisao

Se uma proposta depender de operacao de plataforma, superficie administrativa, servicos hospedados ou qualquer capacidade classificada como VoltOps, ela deve ser rejeitada do escopo v1 e tratada como fora do alvo deste repositorio.
