---
id: SPEC-95AHV4D4
slug: dmpf-tags-modulo-go-consumo
title: DMPF KRN-14 — Tags de módulo Go separadas da release do produto e consumo dos módulos fora do workspace
stage: building
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
`replace` versionado só no `go.work`. Rastreio: `KRN-14`.

## Contexto

### Problema

Nenhum projeto fora deste workspace consegue usar os módulos Go:

- Nenhum dos 20 `go.mod` declara `require` para módulo irmão; a resolução
  depende do `go.work`. `GOWORK=off go list ./...` em
  `libs/backend/go/dmpf-application` falha em `dmpf-domain` e `dmpf-ports`.
- Alguns `go.mod` nem declaram as dependências externas que importam:
  `dmpf-contracts` importa `google.golang.org/protobuf`, `dmpf-app` e
  `bookings/provider` importam `pgx` e `protobuf`, e nenhum dos três tem
  `require`. Só compilam porque outro membro do `go.work` requer esses módulos.
- O `nx release` gera tags `{projectName}@{version}` (`nx.json`, bloco
  `release`), como `dmpf-domain-go@0.1.0`. O proxy Go só resolve módulo em
  subdiretório por tag `libs/backend/go/dmpf-domain/v0.1.0`
  (go.dev/ref/mod, "Mapping versions to commits"). O ADR-030 registra a lacuna
  como consequência negativa.

### Estado atual medido

| Ponto | Evidência |
| ----- | --------- |
| Única tag do repositório: `dmpf@0.1.0`, anotada, criada à mão | `CONTRIBUTING.md:80-84`, `bom/README.md:166-170` |
| `nx-release.yml` só usa `--first-release` se `git tag -l '*@*'` vier vazio; `dmpf@0.1.0` já casa | `.github/workflows/nx-release.yml:112-116` |
| `nx-publish-libs.yml` dispara em `**@*`, o que inclui `dmpf@*` | `.github/workflows/nx-publish-libs.yml:11` |
| Commit de release assinado como `gitea-actions[bot]` | `.github/workflows/nx-release.yml:65-66` |
| CI define `GOPRIVATE=github.com`, com comentário de "host privado" | `.github/actions/setup-go/action.yml:26-32` |
| BOM lê a versão de módulo do `package.json` (`0.0.0` em todos) | `libs/backend/go/dmpf-conformance/bom/registry.go:37` |
| Validador do BOM compara strings, não confere tag git | `libs/backend/go/dmpf-conformance/bom/validate.go:113-114` |
| Generator `bounded-context` emite `go.mod` só com `module` e `go` | `tools/dmpf-plugin/src/generators/bounded-context/files/module/go.mod__tmpl__` |
| Nome do projeto Nx diverge do diretório (`dmpf-domain-go` em `dmpf-domain`, `bookings-app-go` em `bookings/app`) | `libs/backend/go/**/project.json` |
| Repositório `github.com/mateusmacedo/dmpf` público e vazio; clone local sem remote | `gh api repos/mateusmacedo/dmpf` (`size: 0`) |

### Restrições do Nx 23.1.0 (verificadas na fonte instalada)

- `releaseTag.pattern` aceita só `{version}`, `{projectName}` e
  `{releaseGroupName}` (`nx/dist/src/config/nx-json.d.ts:514-518`), é resolvido
  por **grupo** e cada projeto pertence a um único grupo
  (`nx/dist/src/command-line/release/config/config.js:813`). Não há padrão de
  tag por projeto no `project.json`.
- O resolver `git-tag` escapa as partes literais do padrão
  (`release/utils/git.js:117`), então `libs/backend/go/dmpf-domain/v{version}`
  casa com tags existentes.
- `updateDependents` atravessa grupos (`release/utils/release-graph.js:412-416`).
- O grafo Nx já tem as arestas entre módulos Go derivadas dos imports
  (ex.: `dmpf-application-go` → `dmpf-domain-go`, `dmpf-ports-go`,
  `dmpf-testkit-go`), inclusive as de teste.

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
  só quebra para quem consome por tag — é o defeito que o dmpf tem hoje.

### Comportamento verificado no spike (2026-09-15)

Spike num clone descartável, sem remote, com Nx 23.1.0 e go1.27.1:

- Release real sem push com um grupo por lib e padrão literal: cria as 19 tags
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
  só em `dmpf-domain` sobe 18 dos 19 módulos.
- O `projectChangelogs` da raiz gera `CHANGELOG.md` na raiz de cada módulo Go.
- Consumidor por tag: `go get` pelo bare local com `insteadOf` e
  `protocol.file.allow=always` passa, e o `go.sum` traz o módulo e o irmão.
- Consumidor por clone local: compila com `go.work` que resolve os irmãos; com
  `require` de versão sem tag e sem `replace`, reprova com `module lookup disabled`.
- Workspace com `require <irmão> v0.1.0` sem tag: sem `replace`, o build falha
  com `module lookup disabled`; com `replace <irmão> => ./dir` sem versão no
  `go.work`, o Go recusa ("workspace module … is replaced at all versions"); com
  `replace <irmão> v0.1.0 => ./dir`, os 20 módulos compilam e os testes passam.
- `go list -m -json all` no workspace roda sem rede e resolve o módulo de cada
  dependência externa.

### Fontes normativas

- `docs/adr/030-granularidade-modulo-go-e-bom.md` — um módulo Go por lib; `package.json` privado
- `docs/adr/034-fronteira-de-uow-em-go.md` — addendum de 2026-09-12 que cria a ARQ-550
- `docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md` — BOM e tag do produto
- `docs/adr/043-migracao-para-github-licenca-e-autoria.md` — host GitHub
- `bom/README.md` — schema `dmpf/bom@1`

<constraints>
- [P0] Tag de módulo Go publicada é imutável: nunca mover, recriar ou fazer force push de `libs/backend/go/*/v*`. Correção sai em versão nova ou por `retract`, porque proxy e checksum DB guardam o conteúdo para sempre.
- [P0] `dmpf@0.1.0` e `bom/dmpf/0.1.0.json` permanecem intocados.
- [P0] Uma família de tag nunca dispara workflow da outra.
- [P0] `replace` de irmão vive só no `go.work`, versionado e apontando para o diretório do módulo no workspace; `go.mod` não declara `replace`.
- [P0] Não quebrar `nx affected`, cache do Nx nem os gates DMPF existentes (dependency, cell, shared kernel, generator, conformance, BOM).
- [P1] Nenhum plugin Nx de terceiros novo e nenhum `VersionActions` próprio: a versão vem do `package.json` pelas version actions do `@nx/js`.
- [P1] Dentro do workspace, build e teste continuam sem rede para módulos irmãos.
</constraints>

## Requisitos

### Funcionais

**A. Nx Release gera tags de módulo Go**

- [ ] **[P0] Um release group por lib Go**: declarar em `nx.json` um grupo por projeto `tag:stack:go` + `tag:type:lib`, com nome igual ao do projeto, `releaseTag.pattern` literal `<projectRoot>/v{version}`, as version actions padrão do `@nx/js`, `currentVersionResolver: "git-tag"`, `fallbackCurrentVersionResolver: "disk"` e `updateDependents: "always"`. O padrão é literal porque o nome do projeto não coincide com o diretório.
- [ ] **[P0] Grupo npm preservado**: manter `@mateusmacedo/dmpf-plugin` e demais libs `stack:node` num grupo com `{projectName}@{version}`.
- [ ] **[P0] Primeira release em `v0.1.0`**: toda lib Go recebe `v0.1.0` na primeira execução; depois, o versionamento é independente por Conventional Commits, e todo projeto afetado pelo commit no grafo Nx, inclusive por aresta de teste, sobe ao menos patch. O release atualiza o `package.json` e não reescreve `go.mod`.
- [ ] **[P1] `package.json` privado como fonte da versão**: o manifesto dos módulos Go permanece; BOM e evidência seguem lendo a versão dele.

**B. `go.mod` declarados e workspace resolvido pelo `go.work`**

- [ ] **[P0] `require` de irmãos**: todo `go.mod` de projeto Go (libs e `dmpf-reference-go`) declara `require <irmão> vX.Y.Z` para cada irmão importado, inclusive por teste e sob build tag. A versão de um `require` novo é a maior tag de release `<dir>/vX.Y.Z` alcançável do irmão; sem tag, `v0.1.0`, a versão da primeira release. `require` existente não é reescrito.
- [ ] **[P0] Dependências externas declaradas**: todo `go.mod` declara cada módulo externo que importa; um `require` novo usa a versão selecionada no workspace.
- [ ] **[P0] `replace` versionado no `go.work`**: para cada par (irmão, versão) requerido em algum `go.mod`, o `go.work` declara `replace <irmão> <versão> => ./<dir>`, num bloco único.
- [ ] **[P0] Ferramenta `dmpf-modsync`**: `--write` adiciona os `require` faltantes e regrava o bloco `replace` do `go.work`; `--check` reprova import sem `require`, `replace` em `go.mod` e bloco `replace` do `go.work` divergente dos `require`. Roda sem rede e entra no CI sem condição `go_affected`.

**C. Release do produto DMPF**

- [ ] **[P0] Workflow `dmpf-release.yml`**: `workflow_dispatch` com input `release` (semver), só em `master`, que cria e publica apenas a tag anotada `dmpf@<semver>` depois de passar em todos os gates abaixo.
- [ ] **[P0] Gates do workflow**: `bom/dmpf/<semver>.json` existe e declara `tag: dmpf@<semver>`; `dmpf-bom --release <semver>` passa; a evidência regenerada é idêntica a `bom/evidence/<semver>/` (reaproveitar `dmpf-evidence.yml` como `workflow_call`); a tag `dmpf@<semver>` ainda não existe.
- [ ] **[P0] Tag Go no BOM**: o `dmpf-bom` ganha `DMPF-B012`, que reprova entrada `subject: kernel` cuja tag `<projectRoot>/v<versão>` está ausente ou não é ancestral do commit alvo. A regra vale para releases a partir de `0.2.0`; o BOM `0.1.0`, com módulos em `0.0.0`, fica isento.
- [ ] **[P1] Rito manual removido**: `CONTRIBUTING.md` e `bom/README.md` passam a apontar o `dmpf-release.yml` como único caminho da tag do produto.

**D. Separação de gatilhos no CI**

- [ ] **[P0] `nx-publish-libs.yml`**: gatilho de tags `**@*` com exclusão `!dmpf@*`.
- [ ] **[P0] `--first-release` por família**: `nx-release.yml` detecta primeira release pelos padrões da própria família (`libs/backend/go/*/v*` para Go, `*@*` sem `dmpf@*` para npm), nunca por `dmpf@*`.
- [ ] **[P0] Push só das tags do release**: `nx-release.yml` publica o commit de release e as tags criadas na execução, sem tocar em `dmpf@*`.
- [ ] **[P1] `GOPRIVATE` removido**: retirar `GOPRIVATE=github.com` do `setup-go`, já que o repositório é público.
- [ ] **[P2] Identidade do commit de release**: trocar `gitea-actions[bot]` pela identidade de automação do host GitHub.

**E. Consumo**

- [ ] **[P1] Guia de consumo**: documentar o consumo por tag (`go get <module path>@vX.Y.Z`) e por clone local (`go.work` do consumidor com `use` do módulo e dos irmãos que ele requer, ou `replace` versionado para o clone).

**F. Generator e documentação**

- [ ] **[P0] Generator `bounded-context`**: registrar o release group de cada módulo gerado em `nx.json` e rodar `dmpf-modsync --write` depois da escrita em disco; o golden `bookings` segue consistente com o `--check`.
- [ ] **[P1] ADR-044**: registrar a separação das famílias de tag, os grupos com padrão literal, o `require` versionado com `replace` no `go.work`, o gate `dmpf-modsync`, a cascata de versão e a versão inicial, superseding a decisão do ADR-034 (sem `require` entre irmãos) e a parte do ADR-030 sobre esquema de tag.
- [ ] **[P1] `AGENTS.md`**: atualizar Tooling, Git e release e Comandos com o novo fluxo e a ferramenta.

### Não-funcionais

- [ ] **Reprodutibilidade**: o mesmo commit gera sempre as mesmas tags; `dmpf-modsync --write` é idempotente.
- [ ] **Sem rede**: `dmpf-modsync --check` e o build do workspace não dependem de rede para módulos do próprio repositório.
- [ ] **Compatibilidade**: Nx 23.1.0, Go `1.26.6` (piso do `go.work`), `@nx-go/nx-go` 4.0.0 sem troca.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Configuração Nx | `nx.json` (`release.groups`) |
| Módulos Go | `go.mod`/`go.sum` dos 20 projetos; bloco `replace` do `go.work` |
| Verificador DMPF | `dmpf-conformance` (`modsync`, `cmd/dmpf-modsync`, `DMPF-B012`) |
| Generator | `tools/dmpf-plugin` (`bounded-context`) e golden `bookings` |
| CI/CD | `ci.yml`, `nx-release.yml`, `nx-publish-libs.yml`, `dmpf-evidence.yml`, novo `dmpf-release.yml`, `setup-go` |
| Documentação | ADR-044, `AGENTS.md`, `CONTRIBUTING.md`, `bom/README.md`, guia de consumo |

## Localização de código

```text
nx.json                                                  release.groups
go.work, go.work.sum                                     bloco replace versionado
apps/backend/dmpf-reference/go.mod                       require de irmãos e externos
libs/backend/go/**/go.mod, go.sum                        require de irmãos e externos
libs/backend/go/dmpf-conformance/modsync/                novo
libs/backend/go/dmpf-conformance/cmd/dmpf-modsync/       novo
libs/backend/go/dmpf-conformance/bom/validate.go         DMPF-B012
tools/dmpf-plugin/src/generators/bounded-context/generator.ts
.github/workflows/ci.yml                                 DMPF modsync gate
.github/workflows/nx-release.yml                         first-release por família, identidade, push
.github/workflows/nx-publish-libs.yml                    !dmpf@*
.github/workflows/dmpf-evidence.yml                      workflow_call
.github/workflows/dmpf-release.yml                       novo
.github/actions/setup-go/action.yml                      sem GOPRIVATE
docs/adr/044-tags-de-modulo-go-e-consumo-fora-do-workspace.md   novo
AGENTS.md, CONTRIBUTING.md, bom/README.md, docs/guides/dmpf-composicao.md
```

## Design

### Arquitetura

```text
                 merge release/X.Y.Z → master
                              │
                      nx-release.yml
                              │
          ┌───────────────────┴───────────────────┐
   grupos Go (1 por lib)                     grupo npm
   @nx/js (package.json)                     @nx/js
   tag <root>/vX.Y.Z                          tag <projeto>@X.Y.Z
   go.mod intocado                            dispara nx-publish-libs.yml
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

```text
module github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application

go 1.26.6

require (
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports v0.1.0
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit v0.1.0
)
```

```text
go 1.26.6

use (
	./libs/backend/go/dmpf-application
	./libs/backend/go/dmpf-domain
	...
)

replace (
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-domain v0.1.0 => ./libs/backend/go/dmpf-domain
	github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports v0.1.0 => ./libs/backend/go/dmpf-ports
	...
)
```

| Quem resolve | Como chega ao irmão |
| ------------ | ------------------- |
| Build dentro do workspace | `use` + `replace` versionado do `go.work` |
| Projeto externo por `go get` | `require` + tag publicada |
| Projeto externo com clone local | `go.work` do consumidor com `use` do módulo e dos irmãos requeridos, ou `replace` versionado para o clone |
| `GOWORK=off` no repositório | só com as tags publicadas |

### `dmpf-modsync`

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
| Geração da tag Go | Um grupo por lib com padrão literal e as version actions do `@nx/js` | `VersionActions` Go próprio (código a mais sem ganho, dado o `package.json` mantido); renomear os projetos para casar `{projectName}` (quebra o sufixo `-go` do ADR-030 e o aninhamento `bookings/*`); `@naxodev/gonx`; script pós-`nx release` |
| Tag do produto | `dmpf-release.yml` com gates | Rito manual com gate só no `dmpf-bom` (mantém passo humano sujeito a erro) |
| Irmãos no `go.mod` | `require` versionado + `replace` versionado no `go.work`, como no `golibs` | `replace` relativo no `go.mod` com `tidy-check` sob `GOWORK=off` e prova hermética de consumo (validado no spike, mas o custo só compra consumo antes da primeira tag); sem `require` (consumo por tag não reproduzível) |
| Sync e gate | Ferramenta Go `dmpf-modsync` com `--write` e `--check` | Script shell de `replace` como o do `golibs`, sem gate (não detecta import sem `require`) |
| Reescrita de `require` no release | Não reescrever: `require` fica na versão mínima | Reescrever pelo release (exige `VersionActions` próprio) |
| Cascata de versão | Aceitar a cascata do Nx pelo grafo bruto | Só dependentes de produção por `readDependencies` (refutado no spike); version plans (muda o fluxo de PR e não foi verificado); arestas de teste fora do grafo (quebra `nx affected`) |
| Versão inicial | `v0.1.0` na próxima release; `require` de irmão sem tag já nasce em `v0.1.0`; `dmpf@0.2.0` é o primeiro BOM com tags Go | `v0.0.0` no `require` (a primeira release publicaria `go.mod` apontando para versão inexistente) |
| Corte do `DMPF-B012` | Releases a partir de `0.2.0` | Regra retroativa (reprovaria o BOM `0.1.0`, que é imutável) |
| Publicação no GitHub | Pré-requisito operacional fora da spec | Incluir push e proteções como requisito (mistura operação de repositório com mecânica de release) |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| `require` na versão mínima: um módulo passa a usar API nova de um irmão sem subir o `require`, e o consumidor por tag compila contra a versão antiga | Aceito, como no `golibs`; subir o `require` no mesmo PR da mudança e registrar no guia de consumo |
| Cascata de versão pelo grafo bruto: um commit em `dmpf-testkit` sobe 14 módulos | Aceita por decisão; remover as arestas de teste para `dmpf-testkit` reduz a cascata e fica fora desta spec |
| `require` novo sem `replace` no `go.work` quebra o build do workspace sem rede | `dmpf-modsync --write` regrava o bloco; `--check` no CI reprova a divergência |
| Consumidor local antes das tags publicadas precisa resolver todos os irmãos | Guia de consumo com `use` dos irmãos ou `replace` versionado |
| `release.docker` da raiz roda `docker:build` antes de todo `nx release` | Decidir no ajuste do `nx-release.yml` se a config sai da raiz ou se o pre-version passa a ser explícito |
| Criação da tag com barras no nome não segue o mesmo caminho do resolver | Validado no spike; o critério de aceite conta as tags numa release real em clone descartável |
| Golden `bookings` diverge do generator | Regenerar pelo generator e rodar `dmpf-modsync --check` e `dmpf-harness-check.sh --phase self-test` |

## Verificação e testes

### Critérios de aceite

- [ ] `pnpm nx release 0.1.0 --first-release --skip-publish`, num clone descartável sem remote, cria 19 tags `libs/backend/go/<módulo>/v0.1.0`, nenhuma tag `*-go@*`, e o grupo npm com o padrão atual.
- [ ] `dmpf-modsync --check` passa na árvore e reprova import sem `require`, `replace` em `go.mod` e bloco `replace` do `go.work` divergente.
- [ ] Build, vet e testes dos 20 projetos Go passam no workspace com `GOPROXY=off`.
- [ ] `dmpf-bom` reprova com `DMPF-B012` um BOM de teste de release `0.2.0` ou posterior cuja versão de módulo não tem tag Go ancestral, e mantém o BOM `0.1.0` verde.
- [ ] `nx-publish-libs.yml` não dispara no push de `dmpf@*` e `dmpf-release.yml` não roda fora de `master`.
- [ ] Os gates DMPF existentes e `pnpm nx affected -t lint,typecheck,test,build` seguem verdes.
- [ ] Generator `bounded-context` gera contexto novo com `require`, `replace` no `go.work` e release group, e o `self-test` do harness passa.
- [ ] Após o push operacional do repositório e da release: um repositório fora deste workspace faz `go get github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application@v0.1.0` contra `github.com` (dependente do pré-requisito operacional).

### Cenários de teste

**Happy path — release independente**
- DADO `dmpf-domain-go` em `v0.1.0` e um commit `feat` só em `dmpf-domain`
- QUANDO `nx-release.yml` roda
- ENTÃO nasce `libs/backend/go/dmpf-domain/v0.2.0`, os projetos afetados pelo commit no grafo Nx sobem patch com tag própria, nenhum `go.mod` muda e nenhuma tag `dmpf@*` é criada

**Happy path — consumo por clone local**
- DADO um projeto fora do repositório com `go.work` que faz `use` de `dmpf-application` e dos irmãos que ele requer
- QUANDO roda `GOPROXY=off go build ./...`
- ENTÃO compila resolvendo `dmpf-domain` e `dmpf-ports` pelo clone, sem rede

**Edge case — primeira release com `dmpf@0.1.0` existente**
- DADO o repositório só com a tag `dmpf@0.1.0`
- QUANDO `nx-release.yml` roda
- ENTÃO a execução é tratada como primeira release das famílias Go e npm, e todas as libs Go recebem `v0.1.0`

**Erro — tag do produto sem tag Go**
- DADO `bom/dmpf/0.2.0.json` declarando `dmpf-ports` em `0.2.0` sem a tag `libs/backend/go/dmpf-ports/v0.2.0`
- QUANDO `dmpf-release.yml` roda com `release=0.2.0`
- ENTÃO o `dmpf-bom` reprova com `DMPF-B012` e nenhuma tag `dmpf@0.2.0` é criada

**Erro — import sem `require`**
- DADO `dmpf-ports` passando a importar um package de `dmpf-contracts` sem declarar o `require`
- QUANDO `dmpf-modsync --check` roda no CI
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
- [P0] Não quebrar `nx affected`, cache do Nx nem os gates DMPF existentes (dependency, cell, shared kernel, generator, conformance, BOM).
- [P1] Nenhum plugin Nx de terceiros novo e nenhum `VersionActions` próprio: a versão vem do `package.json` pelas version actions do `@nx/js`.
- [P1] Dentro do workspace, build e teste continuam sem rede para módulos irmãos.
</critical_constraints>

## Escopo fora

- **Publicar o repositório no GitHub**: configurar remote, fazer push de `master`/`develop` e aplicar proteções é operação de repositório, pré-requisito do último critério de aceite, não mecânica de release.
- **Cunhar `dmpf@0.2.0`**: a spec entrega o workflow e os gates; a release real acontece depois do push operacional.
- **Tag `v0.x` para `dmpf-reference-go`**: é `type:app`, composition root, não é consumida como lib; recebe `require` e entra no `replace` do `go.work`, sem release group.
- **Prova automatizada de consumo no CI**: sem bare clone, `insteadOf` nem consumidor hermético; o consumo é validado pelo spike e pelo guia.
- **Build com `GOWORK=off` antes das tags publicadas** e **reescrita automática de `require` no release**.
- **Major `v2+` e sufixo `/vN`**: nenhum módulo sai de `v0`; a regra entra quando houver o primeiro breaking change pós-`v1`.
- **Publicação npm dos módulos Go**: continuam fora do GitHub Packages, com o filtro `!tag:stack:go` preservado.
- **Kernel TypeScript**: as tags e o gate não se aplicam à futura contraparte TS.
- **Remover arestas de teste para `dmpf-testkit`**: reduziria a cascata de versão, mas mover os testes é outra tarefa.
