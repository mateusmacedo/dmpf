---
id: SPEC-95AHV4D4
slug: dmpf-tags-modulo-go-consumo
title: DMPF KRN-14 — Tags de módulo Go separadas da release do produto e consumo dos módulos fora do workspace
stage: done
priority: P2
depends_on: [SPEC-JPP31095]
ticket_url: null
subtask_urls: []
created: 2026-09-14
---

# SPEC-95AHV4D4: DMPF KRN-14 — Tags de módulo Go separadas da release do produto e consumo dos módulos fora do workspace

## Resumo

Tornar cada módulo Go do workspace consumível de dois jeitos: como lib
independente, por `go get <module path>@vX.Y.Z`, e a partir de um clone local,
como base para validações ou projetos inteiros. Para isso, separar os dois
contextos de tag que hoje se misturam — a release do produto DMPF
(`dmpf@X.Y.Z`, que certifica um BOM) e a versão de cada módulo Go
(`libs/backend/go/<módulo>/vX.Y.Z`, o formato que o toolchain Go exige) —, fazer
o Nx Release gerar as duas no CI e declarar no `go.mod` de cada módulo tudo o
que ele importa. A mecânica segue o modelo já em uso no repositório `golibs` da
organização: versão no `package.json` privado, `require` versionado dos irmãos e
`replace` versionado só no `go.work`. Rastreio: `KRN-14` / `ARQ-550`.

**A metade do `go.mod` já está entregue.** As fases 0 (spike) e 1 (`modsync` e
migração dos 19 `go.mod`) foram mergeadas na `develop` em `cf07b43` e seguem
verdes. O que resta é a metade da tag: os release groups do Nx, o workflow da
release do produto, o gate `DMPF-B012` e o generator. Ver
[Entregue nas fases 0 e 1](#entregue-nas-fases-0-e-1).

## Contexto

### Problema

O repositório publica uma única família de tag, e ela não serve ao toolchain Go:

- O `nx release` gera tags `{projectName}@{version}` (`nx.json:73-75`), como
  `domain@0.1.0`. O proxy Go só resolve módulo em subdiretório por tag
  `libs/backend/go/domain/v0.1.0` (go.dev/ref/mod, "Mapping versions to
  commits"). O ADR-030 registra a lacuna como consequência negativa.
- A tag do produto (`dmpf@0.1.0`) e as tags de módulo compartilham o mesmo
  espaço de gatilho: `nx-publish-libs.yml` dispara em `**@*`
  (`.github/workflows/nx-publish-libs.yml:8-11`), o que inclui `dmpf@*`.
- O rito da tag do produto é manual, com quatro comandos copiados à mão
  (`CONTRIBUTING.md:79-85`).

O terceiro sintoma — `go.mod` sem `require` de irmão, com a resolução dependendo
do `go.work` — **foi corrigido na fase 1** e não consta mais como problema.

### Entregue nas fases 0 e 1

Mergeado em `develop` por `cf07b43` (2026-09-15), antes dos refactors ADR-045 e
ADR-046, e **revalidado nesta retomada** contra a árvore atual:

| Entrega | Evidência de hoje |
| ------- | ----------------- |
| `modsync` (`--write`/`--check`), sem rede | `tools/dmpf-conformance/modsync/`, `cmd/modsync/` |
| Gate no CI, sem condição `go_affected` | `.github/workflows/ci.yml:171-172` |
| Gate verde na árvore atual | `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` → `dmpf-modsync: conforme` (exit 0) |
| `require` de irmãos e externos nos 19 `go.mod` | ex.: `libs/backend/go/testkit/go.mod:5-16` (8 irmãos + 2 externos) |
| Bloco `replace` versionado no `go.work` | `go.work:25-40`, 14 pares (um por irmão efetivamente requerido; `sqs` não é importado por ninguém e por isso não aparece) |

Os dois refactors posteriores **não regrediram a fase 1**: o ADR-045 renomeou os
módulos e o ADR-046 moveu o `conformance` para `tools/`, e o `--check` continua
conforme, com o step do CI já apontando para o caminho novo.

### Estado atual medido

Medido em 2026-09-17, na `develop` em `07bf69e`. Substitui a tabela da redação
original, escrita contra o layout anterior ao ADR-045/046.

| Ponto | Evidência |
| ----- | --------- |
| Única tag do repositório: `dmpf@0.1.0`, anotada, criada à mão | `git tag -l` (1 tag); `CONTRIBUTING.md:79-85` |
| `release.projects` e `releaseTag.pattern` inalterados: uma família só, no formato npm | `nx.json:70-75` |
| `nx-release.yml` só usa `--first-release` se `git tag -l '*@*'` vier vazio; `dmpf@0.1.0` já casa | `.github/workflows/nx-release.yml:112-114` |
| `nx-publish-libs.yml` dispara em `**@*`, o que inclui `dmpf@*` | `.github/workflows/nx-publish-libs.yml:8-11` |
| Commit de release assinado como `gitea-actions[bot]`, apesar da migração para o GitHub (ADR-043) | `.github/workflows/nx-release.yml:65-66` |
| CI define `GOPRIVATE=github.com`, com comentário de "host privado" | `.github/actions/setup-go/action.yml:30-32` |
| BOM lê a versão de módulo do `package.json`; os 15 manifestos Go estão em `0.0.0` | `tools/dmpf-conformance/bom/registry.go:37` |
| Validador do BOM compara a tag do produto por string e não confere tag de módulo | `tools/dmpf-conformance/bom/validate.go:113` |
| Generator `bounded-context` emite `go.mod` só com `module` e `go` | `tools/dmpf-plugin/src/generators/bounded-context/files/module/go.mod__tmpl__` |
| **Nome do projeto Nx agora é o basename do diretório** (`domain` em `libs/backend/go/domain`) | ADR-045; `libs/backend/go/*/project.json` |
| Contexto de negócio virou app de módulo único; sumiu o aninhamento `bookings/*` | ADR-046; `apps/backend/bookings` |
| Repositório sem remote configurado; nada publicado | `git branch -r` (vazio) |

#### Inventário Go de hoje

| Classe | Quantidade | Onde | Entra em release group? |
| ------ | ---------- | ---- | ----------------------- |
| Libs do kernel | 14 | `libs/backend/go/<nome>` | sim |
| Tooling (`conformance`) | 1 | `tools/dmpf-conformance` | sim — é `type:lib` e o `testkit` o requer (`libs/backend/go/testkit/go.mod:14`) |
| Apps | 4 | `apps/backend/<nome>` | não (`type:app`) |
| **Total de `go.mod`** | **19** | — | 15 versionáveis |

A redação original falava em 19 grupos e 20 `go.mod`, contagem do layout
anterior. Os números acima a substituem.

### Restrições do Nx 23.1.0 (verificadas na fonte instalada)

- `releaseTag.pattern` aceita só `{version}`, `{projectName}` e
  `{releaseGroupName}` (`nx/dist/src/config/nx-json.d.ts:514-518`), é resolvido
  por **grupo** e cada projeto pertence a um único grupo
  (`nx/dist/src/command-line/release/config/config.js:813`). Não há padrão de
  tag por projeto no `project.json`.
- O resolver `git-tag` escapa as partes literais do padrão
  (`release/utils/git.js:117`), então um padrão com prefixo literal casa com
  tags existentes.
- `updateDependents` atravessa grupos (`release/utils/release-graph.js:412-416`).
- O grafo Nx já tem as arestas entre módulos Go derivadas dos imports
  (ex.: `application` → `domain`, `ports`, `memory`, `testkit`), inclusive as de
  teste.

**Consequência nova do ADR-045**: como o nome do projeto passou a ser o basename
do diretório, `libs/backend/go/{projectName}/v{version}` resolve as 14 libs num
**único grupo**. A redação original precisava de um grupo por lib, com padrão
literal, justamente porque os nomes divergiam dos diretórios (`domain-go` em
`dmpf-domain`). Essa restrição deixou de existir.

### Referência na organização: `golibs`

O repositório `golibs` (15 módulos Go em `packages/<nome>`, 53 tags) resolve a
mesma necessidade com menos peças, verificado em 2026-09-15:

- Uma config de release na raiz com `releaseTagPattern:
  "packages/{projectName}/v{version}"`, que funciona porque o nome de cada
  projeto é igual ao diretório. Versão no `package.json` privado de cada pacote,
  com as version actions do `@nx/js`.
- `go.mod` com `require` dos irmãos em versão publicada, sem `replace`. O release
  não reescreve o `require` (ex.: `goweb` requer `gocore v0.1.0` com `gocore`
  em `v0.1.1`).
- `replace` versionado só no `go.work`, gerado por
  `tools/sync-go-work-replaces.sh`. Consumo por tag documentado em
  `docs/CONSUMERS.md`; uso local por `go.work` do consumidor.
- Não há gate de `require`: import de irmão sem `require` compila no workspace e
  só quebra para quem consome por tag. Era o defeito do dmpf, fechado na fase 1
  pelo `modsync --check`.

Com o ADR-045, o dmpf passou a ter a mesma propriedade que faz a config do
`golibs` funcionar com um padrão só.

### Comportamento verificado no spike (2026-09-15)

Spike num clone descartável, sem remote, com Nx 23.1.0 e go1.27.1. Registrado
como histórico: a contagem de 19 tags é do layout anterior, e o mecanismo
observado continua valendo.

- Release real sem push com um grupo por lib e padrão literal: cria as tags
  `libs/backend/go/<módulo>/v0.1.0` e `@mateusmacedo/dmpf-plugin@0.1.0` no
  commit de release, sem nenhuma `*-go@*`.
- `nx release version` recusa o `release.git` da raiz; o release roda por
  `nx release --skip-publish`, como já faz o `nx-release.yml`.
- O `release.docker` da raiz injeta `npx nx run-many -t docker:build` antes de
  todo `nx release` (`release/config/config.js:188`, `release/version.js:190-194`).
- O dry-run não cria tag (`release/utils/git.js:363-365`); contar as tags exige
  release real em clone descartável.
- Os conventional commits associam commit a projeto por `filterAffected` sobre o
  grafo Nx bruto, com arestas de teste (`release/utils/shared.js:340`). Um `fix`
  só em `domain` sobe quase todos os módulos.
- O `projectChangelogs` da raiz gera `CHANGELOG.md` na raiz de cada módulo Go.
- Consumidor por tag: `go get` pelo bare local com `insteadOf` e
  `protocol.file.allow=always` passa, e o `go.sum` traz o módulo e o irmão.
- Consumidor por clone local: compila com `go.work` que resolve os irmãos; com
  `require` de versão sem tag e sem `replace`, reprova com `module lookup disabled`.
- Workspace com `require <irmão> v0.1.0` sem tag: sem `replace`, o build falha
  com `module lookup disabled`; com `replace <irmão> => ./dir` sem versão no
  `go.work`, o Go recusa ("workspace module … is replaced at all versions"); com
  `replace <irmão> v0.1.0 => ./dir`, os módulos compilam e os testes passam.
- `go list -m -json all` no workspace roda sem rede e resolve o módulo de cada
  dependência externa.

### Fontes normativas

- `docs/adr/030-granularidade-modulo-go-e-bom.md` — um módulo Go por lib; `package.json` privado
- `docs/adr/034-fronteira-de-uow-em-go.md:225` — addendum de 2026-09-12 que cria a ARQ-550
- `docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md` — BOM e tag do produto
- `docs/adr/043-migracao-para-github-licenca-e-autoria.md` — host GitHub
- `docs/adr/045-nomes-bare-e-contexto-em-modulo-unico.md` — nome do projeto = basename do diretório
- `docs/adr/046-libs-somente-kernel-de-reuso.md` — `conformance` em `tools/`; contextos como apps
- `bom/README.md` — schema `dmpf/bom@1`

<constraints>
- [P0] Tag de módulo Go publicada é imutável: nunca mover, recriar ou fazer force push de `libs/backend/go/*/v*`. Correção sai em versão nova ou por `retract`, porque proxy e checksum DB guardam o conteúdo para sempre.
- [P0] `dmpf@0.1.0` e `bom/dmpf/0.1.0.json` permanecem intocados.
- [P0] Uma família de tag nunca dispara workflow da outra.
- [P0] `replace` de irmão vive só no `go.work`, versionado e apontando para o diretório do módulo no workspace; `go.mod` não declara `replace`.
- [P0] O gate `modsync --check` entregue na fase 1 permanece verde em toda a entrega.
- [P0] Não quebrar `nx affected`, cache do Nx nem os gates DMPF existentes (dependency, cell, shared kernel, generator, conformance, BOM).
- [P0] O module path de um módulo é chave canônica e estável (RFC DMPF): nenhuma renomeação de diretório Go entra nesta spec.
- [P1] Nenhum plugin Nx de terceiros novo e nenhum `VersionActions` próprio: a versão vem do `package.json` pelas version actions do `@nx/js`.
- [P1] Dentro do workspace, build e teste continuam sem rede para módulos irmãos.
</constraints>

## Requisitos

### Funcionais

**A. Nx Release gera tags de módulo Go**

- [x] **[P0] Grupo único para as libs do kernel**: declarar em `nx.json` um grupo com os 14 projetos de `libs/backend/go/*` (seletor `directory:libs/backend/go/*` menos `!tag:type:app`, que faz toda lib nova entrar sozinha e barra app criada fora da convenção) e `releaseTag.pattern` `libs/backend/go/{projectName}/v{version}`. O padrão com `{projectName}` é possível porque o ADR-045 alinhou nome e diretório. As version actions do `@nx/js` (`@nx/js/src/release/version-actions`), `currentVersionResolver: "git-tag"`, `specifierSource: "conventional-commits"`, `fallbackCurrentVersionResolver: "disk"` e `updateDependents: "always"` são herdados do bloco `version` da raiz e **não** se declaram no grupo: `conventionalCommits: true` é um atalho que já define resolver e specifier, e declarar qualquer um dos dois ao lado dele é erro de configuração (`CONVENTIONAL_COMMITS_SHORTHAND_MIXED_WITH_OVERLAPPING_OPTIONS`).
- [x] **[P0] Grupo próprio para o `conformance`**: o projeto se chama `conformance` e mora em `tools/dmpf-conformance`, então o padrão do grupo é o literal `tools/dmpf-conformance/v{version}`. Ele é `type:lib` e o `testkit` o requer, logo precisa de tag publicável.
- [x] **[P0] Grupo npm preservado**: manter `@mateusmacedo/dmpf-plugin` e demais libs `stack:node` num grupo com `{projectName}@{version}`.
- [x] **[P0] `release.docker` fora da raiz**: remover o bloco `release.docker` do `nx.json`. Com ele na raiz, **todo** release group herda config de Docker (`config.js`, `shouldIncludeDockerConfig`), e o Nx então força `releaseTag.requireSemver = false` no grupo — por atribuição direta, que nenhuma config do usuário sobrescreve. Com `requireSemver: false`, `extractTagAndVersion` devolve o **primeiro** grupo de captura do padrão em vez do que casa semver: a tag `libs/backend/go/domain/v0.1.0` passa a ser lida como versão `"domain"`, e `@mateusmacedo/dmpf-plugin@0.1.0` como `"@mateusmacedo/dmpf-plugin"`. O defeito é anterior a esta spec — o grupo implícito de `develop` já resolve com `requireSemver: false` — e só não apareceu porque nunca houve segunda release de lib. O step `Release — docker apps` do `nx-release.yml` hoje não tem candidatos (`tag:type:app,!tag:stack:go` é vazio, porque toda app é Go); quando existir app não-Go, ela declara o `docker` no grupo de release próprio.
- [x] **[P0] Primeira release em `v0.1.0`**: toda lib Go recebe `v0.1.0` na primeira execução; depois, o versionamento é independente por Conventional Commits, e todo projeto afetado pelo commit no grafo Nx, inclusive por aresta de teste, sobe ao menos patch. O release atualiza o `package.json` e não reescreve `go.mod`.
- [x] **[P1] `package.json` privado como fonte da versão**: o manifesto dos módulos Go permanece; BOM e evidência seguem lendo a versão dele.

**B. `go.mod` declarados e workspace resolvido pelo `go.work`** — ✅ entregue na fase 1

- [x] **[P0] `require` de irmãos**: todo `go.mod` de projeto Go declara `require <irmão> vX.Y.Z` para cada irmão importado, inclusive por teste e sob build tag. A versão de um `require` novo é a maior tag de release `<dir>/vX.Y.Z` alcançável do irmão; sem tag, `v0.1.0`. `require` existente não é reescrito.
- [x] **[P0] Dependências externas declaradas**: todo `go.mod` declara cada módulo externo que importa; um `require` novo usa a versão selecionada no workspace.
- [x] **[P0] `replace` versionado no `go.work`**: para cada par (irmão, versão) requerido em algum `go.mod`, o `go.work` declara `replace <irmão> <versão> => ./<dir>`, num bloco único.
- [x] **[P0] Ferramenta `modsync`**: `--write` adiciona os `require` faltantes e regrava o bloco `replace` do `go.work`; `--check` reprova import sem `require`, `replace` em `go.mod` e bloco divergente. Roda sem rede e está no CI sem condição `go_affected`.

**C. Release do produto DMPF** — ✅ entregue na fase 3

- [x] **[P0] Workflow `dmpf-release.yml`**: `workflow_dispatch` com input `release` (semver), só em `master`, que cria e publica apenas a tag anotada `dmpf@<semver>` depois de passar em todos os gates abaixo.
- [x] **[P0] Gates do workflow**: `bom/dmpf/<semver>.json` existe e declara `tag: dmpf@<semver>`; `dmpf-bom --release <semver>` passa; a evidência regenerada é idêntica a `bom/evidence/<semver>/` (reaproveitar `dmpf-evidence.yml` como `workflow_call`); a tag `dmpf@<semver>` ainda não existe.
- [x] **[P0] Tag Go no BOM**: o `bom` ganha `DMPF-B012`, que reprova entrada `subject: kernel` cuja tag de módulo está ausente ou não é ancestral do commit alvo. A regra vale para releases a partir de `0.2.0`; o BOM `0.1.0`, com módulos em `0.0.0`, fica isento. O padrão da tag por projeto vem do grupo, então a regra resolve `libs/backend/go/<nome>/v<versão>` e `tools/dmpf-conformance/v<versão>`.
- [x] **[P1] Rito manual removido**: `CONTRIBUTING.md:79-85` e `bom/README.md` passam a apontar o `dmpf-release.yml` como único caminho da tag do produto.

**D. Separação de gatilhos no CI**

- [x] **[P0] `nx-publish-libs.yml`**: gatilho de tags `**@*` com exclusão `!dmpf@*`.
- [x] **[P0] `--first-release` por família, com versão explícita**: `nx-release.yml` detecta primeira release pelos padrões da própria família (`libs/backend/go/*/v*` e `tools/dmpf-conformance/v*` para Go, `*@*` sem `dmpf@*` para npm), nunca por `dmpf@*`, e passa a versão `0.1.0` explicitamente nessa execução. Sem a versão, o resolver cai no fallback `disk` (`0.0.0` do `package.json`) e o specifier de conventional commits decide por projeto — `0.0.1` para uns, nenhuma tag para outros —, o que descasaria dos `require` já gravados em `v0.1.0` na fase 1.
- [x] **[P0] Push só das tags do release**: `nx-release.yml` publica o commit de release e as tags criadas na execução, sem tocar em `dmpf@*`.
- [x] **[P1] `GOPRIVATE` removido**: retirar `GOPRIVATE=github.com` do `setup-go`, já que o repositório é público.
- [x] **[P2] Identidade do commit de release**: trocar `gitea-actions[bot]` pela identidade de automação do GitHub, fechando a pendência que o ADR-043 deixou.

**E. Consumo**

- [x] **[P1] Guia de consumo**: documentar o consumo por tag (`go get <module path>@vX.Y.Z`) e por clone local (`go.work` do consumidor com `use` do módulo e dos irmãos que ele requer, ou `replace` versionado para o clone).

**F. Generator e documentação**

- [x] **[P0] Generator `bounded-context`**: rodar `modsync --write` depois da escrita em disco (callback pós-flush); o golden `bookings` segue consistente com o `--check`. O generator **não** escreve release group: o contexto é sempre `type:app` (ADR-046) e uma lib nova em `libs/backend/go/` já é casada pelo seletor de diretório, então a condicional seria código morto. Quem garante que um contexto criado fora da convenção não seja versionado como lib é o `!tag:type:app` do grupo `go-libs`, não o generator — `--directory` aceita qualquer caminho relativo.
- [x] **[P1] ADR novo**: `docs/adr/047-tags-de-modulo-go-e-consumo-fora-do-workspace.md` registra a separação das famílias de tag, os grupos e padrões, o `require` versionado com `replace` no `go.work`, o gate `modsync`, a cascata de versão e a versão inicial. Supersede parcialmente o **ADR-041**, que é quem fixou o `go.mod` workspace-only (`041:101`) — não o ADR-034, que apenas registra ARQ-550/`KRN-14` como tema sucessor (`034:225`). Os números 044 a 046 foram consumidos; este ADR é o 047.
- [x] **[P1] `AGENTS.md`**: atualizar Tooling, Git e release e Comandos com o novo fluxo e a ferramenta.

### Não-funcionais

- [x] **Reprodutibilidade**: o mesmo commit gera sempre as mesmas tags; `modsync --write` é idempotente.
- [x] **Sem rede**: `modsync --check` e o build do workspace não dependem de rede para módulos do próprio repositório.
- [x] **Compatibilidade**: Nx 23.1.0, Go `1.26.6` (piso do `go.work`), `@nx-go/nx-go` sem troca.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Configuração Nx | `nx.json` (`release.groups`) |
| Verificador DMPF | `tools/dmpf-conformance` (`DMPF-B012` no `bom`) |
| Generator | `tools/dmpf-plugin` (`bounded-context`) e golden `bookings` |
| CI/CD | `nx-release.yml`, `nx-publish-libs.yml`, `dmpf-evidence.yml`, novo `dmpf-release.yml`, `setup-go` |
| Documentação | ADR novo, `AGENTS.md`, `CONTRIBUTING.md`, `bom/README.md`, guia de consumo |

Os módulos Go (`go.mod`, `go.sum`, bloco `replace` do `go.work`) e o `ci.yml`
saíram da lista: foram tocados e fechados na fase 1.

## Localização de código

```text
nx.json                                                  release.groups (go-libs, go-tools, npm)
tools/dmpf-conformance/bom/validate.go                   DMPF-B012
tools/dmpf-conformance/internal/rule/diagnostic.go       CodeB012 no catálogo
tools/dmpf-plugin/src/generators/bounded-context/generator.ts
.github/workflows/nx-release.yml                         first-release por família, identidade, push
.github/workflows/nx-publish-libs.yml                    !dmpf@*
.github/workflows/dmpf-evidence.yml                      workflow_call
.github/workflows/dmpf-release.yml                       novo
.github/actions/setup-go/action.yml                      sem GOPRIVATE
docs/adr/0NN-tags-de-modulo-go-e-consumo-fora-do-workspace.md   novo (número a resolver)
AGENTS.md, CONTRIBUTING.md, bom/README.md, docs/guides/dmpf-composicao.md
```

Entregue na fase 1, sem mudança prevista nesta retomada:

```text
tools/dmpf-conformance/modsync/           ✅
tools/dmpf-conformance/cmd/modsync/       ✅
go.work                                   ✅ bloco replace versionado
libs/backend/go/**/go.mod, apps/backend/**/go.mod   ✅ require de irmãos e externos
.github/workflows/ci.yml                  ✅ DMPF modsync gate
```

## Design

### Arquitetura

```text
                 merge release/X.Y.Z → master
                              │
                      nx-release.yml
                              │
          ┌───────────────────┼───────────────────┐
   grupo go-libs        grupo go-tools        grupo npm
   14 projetos           conformance           @nx/js
   @nx/js                @nx/js                tag <projeto>@X.Y.Z
   tag libs/backend/     tag tools/dmpf-       dispara nx-publish-libs.yml
   go/{projectName}/     conformance/vX.Y.Z
   vX.Y.Z
   go.mod intocado
          │
   commit de release + tags Go (push)
                              │
                 PR com bom/dmpf/<semver>.json
                              │
            dmpf-release.yml (workflow_dispatch)
   BOM válido → evidência idêntica → B012 (tags Go ancestrais)
                              │
                     tag dmpf@<semver> (push)
```

### Forma do `go.mod` e do `go.work`

Entregue na fase 1. Estado real de `libs/backend/go/application/go.mod`:

```text
module github.com/mateusmacedo/dmpf/libs/backend/go/application

go 1.26.6

require (
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/memory v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/ports v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/testkit v0.1.0
)
```

```text
go 1.26.6

use (
	./apps/backend/bff
	...
	./tools/dmpf-conformance
)

replace (
	github.com/mateusmacedo/dmpf/libs/backend/go/domain v0.1.0 => ./libs/backend/go/domain
	...
	github.com/mateusmacedo/dmpf/tools/dmpf-conformance v0.1.0 => ./tools/dmpf-conformance
)
```

| Quem resolve | Como chega ao irmão |
| ------------ | ------------------- |
| Build dentro do workspace | `use` + `replace` versionado do `go.work` |
| Projeto externo por `go get` | `require` + tag publicada |
| Projeto externo com clone local | `go.work` do consumidor com `use` do módulo e dos irmãos requeridos, ou `replace` versionado para o clone |
| `GOWORK=off` no repositório | só com as tags publicadas |

### `modsync`

Entregue na fase 1:

```text
Derive   imports de todos os .go (avaliação de tags do go mod tidy, menos ignore)
         → dono de cada import pela build list do workspace (maior prefixo)
         → require esperado: irmão (maior tag de release ou v0.1.0) e externo (versão selecionada)
         → bloco replace esperado: um (irmão, versão) por require de irmão presente
Write    go mod edit -require para o que falta; go.work regravado com o bloco replace
Check    import sem require; replace em go.mod; bloco replace do go.work ≠ esperado
```

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Geração da tag Go | Grupo único para as 14 libs com `libs/backend/go/{projectName}/v{version}`, mais um grupo literal para o `conformance` | **Um grupo por lib com padrão literal** — era a escolha da redação original e deixou de ser necessária com o ADR-045, que alinhou nome e diretório; `VersionActions` Go próprio (código a mais sem ganho, dado o `package.json` mantido); `@naxodev/gonx`; script pós-`nx release` |
| Diretório do `conformance` | Mantido em `tools/dmpf-conformance`, com padrão literal no grupo | Renomear para `tools/conformance` para casar `{projectName}` — o module path é chave canônica e estável pela RFC, e o ADR-046 já o regravou uma vez |
| Tag do produto | `dmpf-release.yml` com gates | Rito manual com gate só no `bom` (mantém passo humano sujeito a erro) |
| Irmãos no `go.mod` | `require` versionado + `replace` versionado no `go.work`, como no `golibs` — **entregue** | `replace` relativo no `go.mod` com `tidy-check` sob `GOWORK=off`; sem `require` (consumo por tag não reproduzível) |
| Sync e gate | Ferramenta Go `modsync` com `--write` e `--check` — **entregue** | Script shell de `replace` como o do `golibs`, sem gate (não detecta import sem `require`) |
| Reescrita de `require` no release | Não reescrever: `require` fica na versão mínima | Reescrever pelo release (exige `VersionActions` próprio) |
| Cascata de versão | Aceitar a cascata do Nx pelo grafo bruto | Só dependentes de produção por `readDependencies` (refutado no spike); version plans (muda o fluxo de PR e não foi verificado); arestas de teste fora do grafo (quebra `nx affected`) |
| Versão inicial | `v0.1.0` na próxima release; os `require` já nasceram em `v0.1.0` na fase 1; `dmpf@0.2.0` é o primeiro BOM com tags Go | `v0.0.0` no `require` (a primeira release publicaria `go.mod` apontando para versão inexistente) |
| Corte do `DMPF-B012` | Releases a partir de `0.2.0` | Regra retroativa (reprovaria o BOM `0.1.0`, que é imutável) |
| Publicação no GitHub | Pré-requisito operacional fora da spec | Incluir push e proteções como requisito (mistura operação de repositório com mecânica de release) |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| `require` na versão mínima: um módulo passa a usar API nova de um irmão sem subir o `require`, e o consumidor por tag compila contra a versão antiga | Aceito, como no `golibs`; subir o `require` no mesmo PR da mudança e registrar no guia de consumo |
| Cascata de versão pelo grafo bruto: um commit em `testkit` sobe quase todos os módulos | Aceita por decisão; remover as arestas de teste para `testkit` reduz a cascata e fica fora desta spec |
| `conformance` num grupo separado pode ficar fora de sincronia com o resto na primeira release | `--first-release` detecta as duas famílias Go; o critério de aceite conta as tags dos dois grupos na mesma execução |
| `release.docker` da raiz roda `docker:build` antes de todo `nx release` | Decidir no ajuste do `nx-release.yml` se a config sai da raiz ou se o pre-version passa a ser explícito |
| Criação da tag com barras no nome não segue o mesmo caminho do resolver | Validado no spike; o critério de aceite conta as tags numa release real em clone descartável |
| Golden `bookings` diverge do generator | Regenerar pelo generator e rodar `modsync --check` e `dmpf-harness-check.sh --phase self-test` |
| Regressão silenciosa da fase 1 durante esta entrega | `modsync --check` entra na validação de toda fase, não só na do CI |

## Verificação e testes

### Critérios de aceite

- [x] `pnpm nx release 0.1.0 --first-release --skip-publish`, num clone descartável sem remote, cria 14 tags `libs/backend/go/<módulo>/v0.1.0`, a tag `tools/dmpf-conformance/v0.1.0` e o grupo npm com o padrão atual, sem nenhuma tag `<projeto>@0.1.0` para projeto Go.
- [x] Uma **segunda** release no mesmo clone, depois de um commit `fix`, resolve a versão corrente **pela tag** (`Resolved the current version as 0.1.0 from git tag "libs/backend/go/<módulo>/v0.1.0"`) e não pelo nome do projeto — é o que prova que `requireSemver` continua `true` e que nenhuma config de Docker vazou para os grupos.
- [x] `modsync --check` passa na árvore e reprova import sem `require`, `replace` em `go.mod` e bloco `replace` do `go.work` divergente. *(gate já verde hoje; aqui vale como não-regressão)*
- [x] Build, vet e testes dos 19 módulos Go passam no workspace com `GOPROXY=off`.
- [x] `dmpf-bom` reprova com `DMPF-B012` um BOM de teste de release `0.2.0` ou posterior cuja versão de módulo não tem tag Go ancestral, e mantém o BOM `0.1.0` verde.
- [x] `nx-publish-libs.yml` não dispara no push de `dmpf@*` e `dmpf-release.yml` não roda fora de `master`.
- [x] Os gates DMPF existentes e `pnpm nx affected -t lint,typecheck,test,build` seguem verdes.
- [x] Generator `bounded-context` gera contexto novo e roda o `modsync --write` no callback pós-flush, deixando `require` e `replace` do `go.work` em dia; não escreve release group (o contexto é `type:app`, e o `!tag:type:app` do grupo `go-libs` é a guarda). O `self-test` do harness passa.
- [ ] Após o push operacional do repositório e da release: um repositório fora deste workspace faz `go get github.com/mateusmacedo/dmpf/libs/backend/go/application@v0.1.0` contra `github.com` (dependente do pré-requisito operacional).

### Cenários de teste

**Happy path — release independente**
- DADO `domain` em `v0.1.0` e um commit `feat` só em `libs/backend/go/domain`
- QUANDO `nx-release.yml` roda
- ENTÃO nasce `libs/backend/go/domain/v0.2.0`, os projetos afetados pelo commit no grafo Nx sobem patch com tag própria, nenhum `go.mod` muda e nenhuma tag `dmpf@*` é criada

**Happy path — consumo por clone local**
- DADO um projeto fora do repositório com `go.work` que faz `use` de `application` e dos irmãos que ele requer
- QUANDO roda `GOPROXY=off go build ./...`
- ENTÃO compila resolvendo `domain`, `ports`, `memory` e `testkit` pelo clone, sem rede

**Edge case — primeira release com `dmpf@0.1.0` existente**
- DADO o repositório só com a tag `dmpf@0.1.0`
- QUANDO `nx-release.yml` roda
- ENTÃO a execução é tratada como primeira release das três famílias, e todos os módulos Go `type:lib` recebem `v0.1.0`

**Edge case — `conformance` fora do padrão do grupo das libs**
- DADO o grupo `go-libs` com `libs/backend/go/{projectName}/v{version}` e o `conformance` em `tools/dmpf-conformance`
- QUANDO o release roda
- ENTÃO a tag do `conformance` é `tools/dmpf-conformance/v0.1.0`, e nenhuma tag `libs/backend/go/conformance/v*` é criada

**Erro — tag do produto sem tag Go**
- DADO `bom/dmpf/0.2.0.json` declarando `ports` em `0.2.0` sem a tag `libs/backend/go/ports/v0.2.0`
- QUANDO `dmpf-release.yml` roda com `release=0.2.0`
- ENTÃO o `dmpf-bom` reprova com `DMPF-B012` e nenhuma tag `dmpf@0.2.0` é criada

**Erro — import sem `require`** *(regressão da fase 1)*
- DADO `ports` passando a importar um package de `contracts` sem declarar o `require`
- QUANDO `modsync --check` roda no CI
- ENTÃO reprova nomeando o módulo e o `require` faltante

**Erro — tag de família errada disparando publicação**
- DADO o push de `dmpf@0.2.0`
- QUANDO o GitHub avalia os gatilhos
- ENTÃO `nx-publish-libs.yml` não roda

<critical_constraints>
- [P0] Tag de módulo Go publicada é imutável: nunca mover, recriar ou fazer force push de `libs/backend/go/*/v*`. Correção sai em versão nova ou por `retract`, porque proxy e checksum DB guardam o conteúdo para sempre.
- [P0] `dmpf@0.1.0` e `bom/dmpf/0.1.0.json` permanecem intocados.
- [P0] Uma família de tag nunca dispara workflow da outra.
- [P0] `replace` de irmão vive só no `go.work`, versionado e apontando para o diretório do módulo no workspace; `go.mod` não declara `replace`.
- [P0] O gate `modsync --check` entregue na fase 1 permanece verde em toda a entrega.
- [P0] Não quebrar `nx affected`, cache do Nx nem os gates DMPF existentes (dependency, cell, shared kernel, generator, conformance, BOM).
- [P0] O module path de um módulo é chave canônica e estável (RFC DMPF): nenhuma renomeação de diretório Go entra nesta spec.
- [P1] Nenhum plugin Nx de terceiros novo e nenhum `VersionActions` próprio: a versão vem do `package.json` pelas version actions do `@nx/js`.
- [P1] Dentro do workspace, build e teste continuam sem rede para módulos irmãos.
</critical_constraints>

## Escopo fora

- **Refazer a fase 1**: `modsync`, os `require` dos 19 `go.mod` e o bloco `replace` do `go.work` estão entregues e verdes; esta retomada só os protege contra regressão.
- **Publicar o repositório no GitHub**: configurar remote, fazer push de `master`/`develop` e aplicar proteções é operação de repositório, pré-requisito do último critério de aceite, não mecânica de release.
- **Cunhar `dmpf@0.2.0`**: a spec entrega o workflow e os gates; a release real acontece depois do push operacional.
- **Tag `v0.x` para as apps Go** (`bff`, `bookings`, `orders`, `reservations`): são `type:app`, não consumidas como lib; recebem `require` e entram no `replace` do `go.work`, sem release group.
- **Renomear `tools/dmpf-conformance`**: casaria `{projectName}` e dispensaria o grupo literal, mas muda o module path, que a RFC exige estável.
- **Prova automatizada de consumo no CI**: sem bare clone, `insteadOf` nem consumidor hermético; o consumo é validado pelo spike e pelo guia.
- **Build com `GOWORK=off` antes das tags publicadas** e **reescrita automática de `require` no release**.
- **Major `v2+` e sufixo `/vN`**: nenhum módulo sai de `v0`; a regra entra quando houver o primeiro breaking change pós-`v1`.
- **Publicação npm dos módulos Go**: continuam fora do GitHub Packages, com o filtro `!tag:stack:go` preservado.
- **Kernel TypeScript**: as tags e o gate não se aplicam à futura contraparte TS.
- **Remover arestas de teste para `testkit`**: reduziria a cascata de versão, mas mover os testes é outra tarefa.
