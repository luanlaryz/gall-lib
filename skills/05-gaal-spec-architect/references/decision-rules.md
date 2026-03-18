# Decision rules

## Preferir amendar spec existente
Escolha este caminho quando:
- a mudanca pertence claramente ao mesmo modulo ja governado
- altera comportamento ja previsto parcialmente
- adiciona regra, contrato ou criterio de aceite dentro do mesmo dominio
- manter no mesmo documento melhora a rastreabilidade

## Criar nova spec ligada a anterior
Escolha este caminho quando:
- a mudanca introduz novo bounded context
- a mudanca cria novo contrato principal
- a extensao deixaria a spec atual longa, confusa ou com responsabilidade misturada
- ha impacto transversal que merece governanca propria

## Regra pratica
1. Procure a spec canonica do modulo.
2. Se houver aderencia forte de escopo, amende a spec existente.
3. Se a aderencia for fraca ou houver novo dominio, crie nova spec com referencia cruzada.
4. Nunca duplique regras normativas ja existentes; referencie-as.
