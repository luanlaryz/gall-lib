# Refinement checklist

Use esta checklist para transformar pedido vago em spec fechada.

## Campos obrigatorios
- problema observavel
- objetivo
- escopo
- fora de escopo
- spec ou dominio relacionado
- contratos afetados
- impacto em modulos
- riscos
- trade-offs
- estrategia de testes
- criterios de aceitacao

## Perguntas base
1. Qual comportamento observavel precisa mudar?
2. Qual spec ou modulo atual parece governar isso?
3. O que entra e o que fica explicitamente fora?
4. Ha mudanca de API publica, contrato interno ou apenas runtime?
5. Quais arquivos ou areas sao provavelmente afetados?
6. Quais falhas, cancelamentos, limites ou invariantes precisam ser preservados?
7. Como validar por teste que a mudanca foi feita corretamente?
8. Quais criterios de aceitacao tornam a mudanca verificavel?
9. Quais riscos e trade-offs a mudanca introduz?
10. A mudanca cabe em spec existente ou cria novo dominio?

## Gate de fechamento
A spec so esta fechada quando todos os campos obrigatorios estiverem materialmente respondidos.
