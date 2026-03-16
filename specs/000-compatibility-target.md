# Spec 000: Compatibility Target

## Objetivo do projeto

`gaal-lib` existe para construir uma biblioteca em Go inspirada no nucleo do Voltagent, cobrindo os recursos principais do framework sob uma abordagem rigorosa de Spec Driven Development. O projeto busca compatibilidade funcional com o core de referencia, excluindo de forma explicita qualquer recurso pertencente a VoltOps.

## Definicao de paridade funcional

Paridade funcional, para este projeto, significa:

- o mesmo cenario de uso suportado deve produzir comportamento observavel equivalente entre Voltagent core e `gaal-lib`
- equivalencia e medida por contratos de entrada, saida, efeitos colaterais controlados, ordem de operacoes relevantes e semantica de erro
- a API em Go nao precisa replicar nomes ou formatos identicos ao framework de referencia, desde que preserve o significado operacional
- divergencias intencionais so sao aceitas quando documentadas, justificadas e cobertas por especificacao

## Criterios de aceitacao globais

Os seguintes criterios valem para todo o repositorio:

1. Nenhuma feature entra sem especificacao aprovada.
2. Toda capacidade suportada precisa de criterio de aceitacao observavel.
3. Toda implementacao deve ter pelo menos um caminho de teste de compatibilidade conceitual ou comportamental.
4. O design publico deve ser idiomatico em Go sem quebrar o modelo mental do core do Voltagent.
5. Nao pode haver dependencia direta ou indireta de recursos de VoltOps para que a biblioteca funcione.
6. Casos de erro e limites de comportamento precisam ser especificados, nao inferidos informalmente.
7. Toda divergencia em relacao ao comportamento do Voltagent deve ser registrada em spec ou documento de decisao.

## Estrategia de compatibilidade conceitual e comportamental

### Compatibilidade conceitual

O projeto preserva os conceitos centrais do core do Voltagent, traduzindo-os para construtos idiomaticos de Go. Isso inclui, conforme avancar a implementacao:

- agentes e unidades de execucao
- modelos e provedores
- ferramentas e contratos de invocacao
- contexto de execucao, sessao, memoria e estado
- fluxos, handoffs e outros mecanismos centrais do framework

A estrategia nao busca copia literal da API. Ela busca manter o mesmo significado conceitual para que um usuario reconheca os mesmos blocos fundamentais e os mesmos limites de responsabilidade.

### Compatibilidade comportamental

A paridade sera validada por cenarios observaveis e reproduziveis:

- fixtures e casos de referencia derivados do comportamento esperado do Voltagent
- comparacao de saidas, eventos relevantes, erros e transicoes de estado
- testes deterministas para os caminhos principais
- documentacao explicita para qualquer comportamento ainda nao coberto ou deliberadamente divergente

O objetivo de compatibilidade e comportamental, nao cosmetico. O projeto aceitara diferencas de forma quando a substancia do comportamento permanecer equivalente e verificavel.
