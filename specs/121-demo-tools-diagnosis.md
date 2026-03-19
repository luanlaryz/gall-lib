# Spec 121: Demo V2 Tools Diagnosis

## 1. Objetivo

Diagnosticar se a expansão de tools da demo realmente prova a trilha de tools da `gaal-lib` de forma executável, didática e coerente com a arquitetura.

---

## 2. Escopo

O diagnóstico deve avaliar:

- existência das tools
- wiring correto
- evidência de uso pelo agent
- tratamento de erro
- cobertura de testes
- documentação
- impacto na DX da demo

---

## 3. Critérios de avaliação

### PASS
A trilha de tools está funcional, observável e bem documentada.

### PARTIAL
As tools existem e funcionam, mas a evidência ou a documentação ainda são fracas.

### FAIL
A demo não prova a trilha de tools de forma confiável.

---

## 4. Checklist obrigatório

- [ ] ao menos 2 tools existem
- [ ] agent consegue acioná-las
- [ ] há evidência observável de uso
- [ ] há pelo menos 1 caso de erro
- [ ] há teste cobrindo a trilha
- [ ] README/documentação cobre o uso
- [ ] `demo.http` cobre o uso

---

## 5. Saída obrigatória

- relatório curto em `docs/reports/demo-tools-report.md`
- decisão: `PASS`, `PARTIAL` ou `FAIL`
- gaps nomeados
- próximos passos sugeridos

---

## 6. Pergunta principal

"A demo v2 já prova tools como uma capacidade real da `gaal-lib`?"