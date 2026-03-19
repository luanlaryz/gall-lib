# Refinement checklist

Use esta checklist para transformar pedido vago em pacote dual-spec fechado.

## Campos obrigatorios de construcao
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

## Campos obrigatorios de diagnostico
- sinais observaveis esperados
- sintomas de falha
- hipoteses principais
- logs relevantes
- metricas relevantes
- traces relevantes
- health checks ou consultas operacionais
- procedimento de troubleshooting
- criterio de confirmacao diagnostica

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
10. A mudanca cabe em spec existente ou cria nova trilha dual-spec?
11. Quais sinais em runtime indicam comportamento saudavel?
12. Quais logs, metricas ou traces ajudam a diferenciar sucesso, regressao e falha?
13. Como um operador confirma o diagnostico e reduz o espaco de busca?

## Gate de fechamento
O pacote de specs so esta fechado quando todos os campos obrigatorios de construcao e diagnostico estiverem materialmente respondidos.
