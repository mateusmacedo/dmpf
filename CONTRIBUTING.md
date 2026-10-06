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
o próximo número a partir das releases já mergeadas em `master` e deriva o
incremento dos commits em `origin/master..origin/develop`. A branch nasce igual a
`master`, e o GitHub não abre PR sem commits; por isso o `release-pr.yml` abre o PR
de release no primeiro push que leva commits à branch, e não duplica PR já aberto.

### Pré-release

Uma branch de release com sufixo de pré-release, como `release/1.0.0-rc.0`, fixa a
versão de todos os projetos versionados. No merge em `master`, o `nx-release.yml`
roda `nx release <versão>` em vez de derivar o incremento dos commits. O
`create-release.yml` só calcula versões sem sufixo, então essa branch é criada à
mão a partir de `master`.

Antes de promovê-la, suba para a mesma versão o `require` de cada irmão nos
`go.mod` e o `replace` correspondente no `go.work` (o `modsync --write` regrava o
`go.work` a partir dos `go.mod`). Sem isso, o consumidor de fora do workspace
recebe os irmãos na versão anterior, como registra o ADR-047.

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

O hook de `pre-commit` roda `biome check --write` e `gofmt -l` nos arquivos em
stage; o de `pre-push` roda `lint`, `typecheck`, `test`, `build`, `fmt-check`,
`vet` e `test-race` nos projetos afetados pelo que o push leva — a base é o que
o remoto já tem, e uma branch sem upstream compara com `origin/develop`.

## Padrões de código

- **Lint e formatação**: Biome (`biome.json`). Rode `pnpm format` para formatar.
- **TypeScript**: Project References com `nodenext` (ver `tsconfig.base.json`).
- **Tags obrigatórias**: todo `project.json` declara uma tag de cada dimensão
  (`type:`, `scope:`, `stack:`) — ver `README.md`.
- **Testes**: Jest com SWC.

## Reportar problemas

Abra uma issue descrevendo o comportamento esperado, o observado e os passos
para reproduzir. Para vulnerabilidades de segurança, siga o `SECURITY.md`.
