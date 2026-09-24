---
paths:
  - "**/*"
---

# Segurança Git

Práticas para operações Git seguras neste workspace. As branches protegidas são `master`, `develop` e `release/**`; o fluxo git-flow completo está em `CONTRIBUTING.md`.

## Hooks de Git

- Deixe os hooks de `pre-commit` e `pre-push` (Lefthook) rodarem em cada operação — eles são a rede de segurança do commit verde.
- Quando um hook falhar, diagnostique e corrija a causa raiz (lint, typecheck, teste) antes de tentar de novo.
- Pular hooks com `--no-verify` exige pedido explícito do usuário — a falha sinaliza um problema real a resolver, não a silenciar. Sem esse pedido, corrija o que o hook apontou.

## Force push

- Em branches de feature, quando precisar reescrever o histórico local, use `git push --force-with-lease` (mais seguro que `--force`, pois aborta se o remoto tiver commits que você ainda não viu) e apenas com autorização explícita do usuário.
- Force push em `master`, `develop` e `release/**` é proibido — reescrever o histórico dessas branches compartilhadas descarta o trabalho de outras pessoas de forma irreversível. Promova mudanças por merge/PR, conforme o fluxo git-flow.

## Revisão antes do commit

- Rode `git diff` e `git diff --staged` e leia as mudanças antes de cada commit.
- Adicione arquivos específicos com `git add <arquivo>` para controlar exatamente o que entra no commit; prefira isso a `git add .` ou `git add -A`, que arrastam arquivos não intencionais.
- Confirme que nenhum arquivo sensível — `.env`, credenciais, chaves, tokens — está no stage. Segredos versionados vazam de forma irreversível no histórico; mantenha-os fora do commit e cobertos pelo `.gitignore`.

## Testes

- Quando um teste falhar, corrija o código de produção sob teste para que o comportamento esperado volte a valer.
- Alterar um teste apenas para fazê-lo passar é proibido — isso mascara a regressão em vez de corrigi-la. Se o teste estiver genuinamente errado, explique o motivo antes de ajustá-lo.

## Renomeação e movimentação de arquivos

- Antes de renomear ou mover um símbolo ou arquivo, busque todas as referências no workspace (`pnpm nx graph`, busca textual, consumidores das libs).
- Depois de mover, revise todos os imports afetados — inclusive os que o compilador pode não acusar: mocks de teste, imports dinâmicos e re-exports.
