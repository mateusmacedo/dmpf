---
paths:
  - "**/*"
---

# Processo e qualidade

Orientações de processo para apoiar qualidade e rastreabilidade. Adapte ao workflow do time.

## Features novas

Toda feature, módulo ou serviço novo exige uma spec correspondente em `docs/specs/` antes de a implementação começar:

- Confirme que a spec (ou documento de design) já existe antes de escrever código.
- Escreva a spec antes de implementar quando ela ainda não existir; peça a criação ao usuário sempre que o escopo depender de decisões dele.

O formato do artefato (spec, PRD, RFC, ADR) depende do processo do projeto. Bugs simples e manutenção rotineira ficam isentos de spec.

## Plano antes de implementar

Mudanças que tocam 2 ou mais arquivos exigem um plano breve aprovado antes da execução:

- Apresente o plano com o que será feito, os arquivos afetados e os riscos ou tradeoffs.
- Aguarde aprovação explícita do usuário antes de prosseguir.

## Refatorações amplas

Toda refatoração exige um plano documentado antes da execução:

- Documente o plano com motivação, escopo, arquivos impactados e estratégia de validação.
- Confirme o plano com o usuário antes de aplicar mudanças, sobretudo quando o impacto cruza camadas.

## Exceções

Estas situações dispensam spec ou plano prévio e podem ser executadas direto:

- Correção de 1 linha (typo ou bug óbvio).
- Tarefa em arquivo único com instrução clara e específica.
- Pedido explícito do usuário ("pode fazer direto", "sem plano").

Bug simples e manutenção rotineira seguem isentos de spec.

## Validação

Após qualquer alteração, rode a cadeia de validação adotada pelo projeto. Tipicamente:

1. Lint.
2. Typecheck.
3. Testes.

Verifique quais passos rodam em hooks automáticos (pré-commit, pré-push) e quais dependem de execução manual. O build de distribuição costuma ser executado de forma explícita antes de releases.
