# gaal-lib

`gaal-lib` e uma biblioteca em Go inspirada no nucleo do Voltagent. O objetivo e oferecer uma base idiomatica em Go para construcao de agentes e fluxos relacionados, com foco em paridade funcional conceitual e comportamental com o core do framework de referencia.

## Visao

O projeto existe para traduzir os conceitos centrais do Voltagent para Go sem copiar mecanicamente a API original. A prioridade e manter equivalencia de comportamento nos cenarios suportados, com uma implementacao que respeite convencoes da linguagem, facilite testes de compatibilidade e permita evolucao guiada por especificacoes.

## Escopo

Na fase inicial, `gaal-lib` cobre a fundacao necessaria para:

- definir o alvo de compatibilidade com o core do Voltagent
- estabelecer regras claras de spec driven development
- preparar a base de automacao, CI e validacao do projeto
- organizar o terreno para futuras implementacoes de agentes, ferramentas, contexto de execucao, memoria e fluxos centrais
- permitir testes de paridade comportamental contra casos de referencia do Voltagent

## Nao-escopo

Este repositorio nao implementa nem pretende absorver recursos de VoltOps. Isso inclui, de forma explicita:

- plataforma operacional e control plane
- dashboards, observabilidade hospedada e recursos de operacao remota
- deploy, execucao distribuida, multi tenancy, billing e governanca operacional
- qualquer dependencia arquitetural em servicos de operacao do ecossistema VoltOps

Tambem estao fora do escopo imediato desta fase inicial:

- implementacao concreta da biblioteca
- suporte amplo a integracoes antes da definicao do nucleo
- features nao cobertas por especificacoes aprovadas

## Roadmap Inicial

1. Fundacao do repositorio, modulo Go, specs basicas e pipeline de CI.
2. Mapeamento do core do Voltagent para conceitos nativos de Go.
3. Definicao de cenarios de paridade e contratos observaveis.
4. Implementacao incremental guiada por specs e testes de compatibilidade.
5. Expansao do suporte somente apos estabilizacao do nucleo.

## Modo de Trabalho

O projeto segue Spec Driven Development rigoroso:

- toda feature nasce de especificacao
- toda implementacao precisa de criterio de aceitacao observavel
- toda divergencia de comportamento deve ser documentada
- toda ampliacao de escopo precisa preservar a exclusao explicita de VoltOps
