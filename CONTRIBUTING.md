# Guia de Contribuição

Obrigado por contribuir com este template. Este documento descreve o fluxo de
trabalho, os padrões de código e como enviar mudanças.

## Pré-requisitos

- Node.js `24` (ver `.nvmrc`)
- pnpm `11` (habilite via `corepack enable`)

```bash
corepack enable
pnpm install
```

O `pnpm install` instala as dependências e **tenta** registrar os hooks de git via
Lefthook (script `prepare`). O script é tolerante a falha (`lefthook install ||
true`), para não quebrar a instalação em ambiente sem o binário — nesse caso os
hooks ficam inativos. Para conferir e instalar explicitamente:

```bash
pnpm lefthook install
```

## Fluxo de trabalho

O guia `docs/guides/development-workflow.md` detalha o fluxo completo do dia a
dia (branches, commits, pull requests e validação); esta seção é o resumo.

Trabalhe sempre fora de `master` e `develop`.

1. Crie uma branch seguindo o padrão `tipo/descricao-curta`
   (ex: `feat/notificacoes-webhook`, `fix/lint-minha-lib`).
2. Faça commits atômicos seguindo Conventional Commits (ver abaixo).
3. Rode a validação local antes de abrir o PR.
4. Abra o Pull Request contra `develop` descrevendo o que muda e por quê.
   `master` recebe o trabalho pela branch `release/X.Y.Z`, não diretamente.

PRs que tocam apenas arquivos Markdown não disparam o pipeline, por causa do
`paths-ignore` configurado no `ci.yml`.

### Modelo develop → release (anti-drift)

1. Branches de trabalho abrem PR para `develop` (validação).
2. Depois de validadas, a **mesma** branch (mesmo tip / mesma árvore) é mergeada
   em `release/X.Y.Z`.
3. Antes de abrir PR para `develop`, a branch de trabalho faz merge de
   `release/X.Y.Z` nela — a release pode já ter updates aprovados de outras
   features.

**Regra dura anti-drift:** não promover para `release` uma árvore diferente da
que entrou em `develop`. Evite um segundo PR "paralelo" com re-resolução de
conflitos, cherry-picks reescritos ou tip mais novo com conteúdo divergente da
mesma feature. Se `develop` e `release` receberem o mesmo trabalho com SHAs ou
conteúdos diferentes, merges futuros de `release` nas branches de trabalho
reabrem o mesmo conflito.

Na prática: o tip mergeado em `release` deve ser o tip (ou o merge commit) que já
foi validado em `develop`, sem reeditar os mesmos arquivos "de outro jeito".

O `create-release.yml` automatiza a criação da branch `release/X.Y.Z`: ele calcula
o próximo número a partir das releases já mergeadas em `master`, deriva o
incremento dos commits em `origin/master..origin/develop` e abre o PR de release
pela API do Gitea.

## Conventional Commits

As mensagens de commit seguem o padrão [Conventional Commits](https://www.conventionalcommits.org):

```text
<tipo>(<escopo>): <descrição no imperativo>
```

Tipos comuns: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`.
O escopo é o projeto Nx afetado (ex: `minha-lib`, `workspace`).

Exemplo: `feat(minha-lib): adicionar helper de agrupamento`

O versionamento e o changelog são derivados desses commits pelo Nx Release.

## Validação local

Prefira sempre rodar as tarefas via `nx`:

```bash
pnpm nx affected -t lint typecheck test build   # apenas o que mudou
pnpm nx run-many -t lint typecheck test build    # tudo
```

O hook de `pre-commit` roda `biome check --write` nos arquivos em stage; o de
`pre-push` roda `lint`, `typecheck`, `test` e `build` nos projetos afetados.

## Padrões de código

- **Lint e formatação**: Biome (`biome.json`). Rode `pnpm format` para formatar.
- **TypeScript**: Project References com `nodenext` (ver `tsconfig.base.json`).
- **Tags obrigatórias**: todo `project.json` declara uma tag de cada dimensão
  (`type:`, `scope:`, `stack:`) — ver `README.md`.
- **Testes**: Jest com SWC.

## Reportar problemas

Abra uma issue descrevendo o comportamento esperado, o observado e os passos
para reproduzir. Para vulnerabilidades de segurança, siga o `SECURITY.md`.
