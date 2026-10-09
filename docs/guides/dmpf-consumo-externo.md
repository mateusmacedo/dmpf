# Consumir o DMPF num workspace Nx de fora

Um workspace Nx fora deste monorepo usa o DMPF por versão: o plugin
`@mateusmacedo/dmpf-plugin` vem do GitHub Packages, e o kernel e o
`dmpf-conformance` vêm do proxy Go, pela tag que o `versions.json` do plugin
fixa (ADR-060). Este guia vai do acesso ao registry à atualização de versão. O
que cada generator, executor e migration faz está em
[`tools/dmpf-plugin/README.md`](../../tools/dmpf-plugin/README.md).

## Pré-requisitos

- Workspace Nx 23.x com pnpm: o plugin declara `@nx/devkit` `^23.0.0` como
  peer.
- Go na versão do campo `go.directive` do `versions.json` do plugin; o `init`
  grava essa diretiva no `go.work`.
- `bash`, `git` e `jq`, que os scripts do plugin usam, e Docker para a infra
  local.

## 1. Acesso ao GitHub Packages

O registry npm do GitHub Packages exige token até para instalar pacote
público. Fora do GitHub Actions, use um personal access token (classic) com o
escopo `read:packages`.

O registry do escopo vai no `.npmrc` da raiz do workspace, versionado:

```ini
@mateusmacedo:registry=https://npm.pkg.github.com
```

A credencial vai no `.npmrc` do seu usuário (`~/.npmrc`), nunca no do
workspace:

```ini
//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}
```

```bash
export NODE_AUTH_TOKEN=<token>
```

Desde a 11.5.3, o pnpm ignora `${...}` em credencial do `.npmrc` do repositório
e avisa `Ignored project-level auth setting` (GHSA-3qhv-2rgh-x77r): o arquivo vem
com o checkout, e expandir a variável deixaria um repositório desviar o token
para outro registry. No `.npmrc` de usuário a variável continua valendo, e o
arquivo não guarda o token. No CI, o workflow do DMPF (passo 5) preenche
`NODE_AUTH_TOKEN` com o `GITHUB_TOKEN` do job, que baixa pacote público de
qualquer repositório, e grava a credencial num `.npmrc` de usuário próprio.

## 2. Instalar e inicializar

```bash
pnpm nx add @mateusmacedo/dmpf-plugin
pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/<org>/<repo>
```

O `nx add` instala o pacote e roda o `init` sem opções, que grava o prefixo
reservado `example.com/change-me`. A segunda chamada troca pelo prefixo real
sem `--force`: os arquivos que o `init` gerou e ninguém editou são
reescritos, decididos pelo hash em `dmpf.rendered.json`.

O `init` grava o `dmpf.json` com `tooling.mode: version`, o `go.work`, o Go
setup do `nx.json`, o `@nx-go/nx-go`, o `.golangci.yml`, o esqueleto estático
de `infra/`, o baseline vazio, os ativos de IA em `.claude/` e `.agents/` e o
chamador de CI em `.github/workflows/dmpf-ci.yml`. Versione todos, inclusive o
`dmpf.rendered.json`.

## 3. Primeiro contexto

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> --boundedContext <ctx>
pnpm nx run-many -t tidy
```

O generator cria o módulo do contexto e o de contrato em
`<appsDir>/<name>`, roda o `modsync --write` com os `require` do kernel na
versão do `versions.json` e o `infrasync --write`, e imprime o comando que
regrava o baseline. Rode-o: unidades novas mudam a classificação.

```bash
CONFORMANCE="$(jq -r .conformance node_modules/@mateusmacedo/dmpf-plugin/versions.json)"
go run "github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/conformance@${CONFORMANCE}" --root . --write-baseline
```

O código de negócio dos blocos e o `.proto` do contrato ficam com o autor do
contexto. Com os ativos de IA do `init`, `/dmpf-new-context SPEC-<id>` escreve
o contexto a partir da spec e para em qualquer gate normativo. Os `go.mod`
exigem o kernel por tag, sem `replace`.

## 4. Gates locais

São os mesmos que o CI roda no job de gates:

```bash
DMPF=github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd
go run "${DMPF}/conformance@${CONFORMANCE}" --root .
go run "${DMPF}/modsync@${CONFORMANCE}" --root . --check
go run "${DMPF}/infrasync@${CONFORMANCE}" --root . --check
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" \
  DMPF_KERNEL_DDL="$(go mod download -json github.com/mateusmacedo/dmpf/libs/backend/go/postgres | jq -r .Dir)" \
  bash node_modules/@mateusmacedo/dmpf-plugin/scripts/dmpf-context-check.sh --phase structural
pnpm nx run-many -t buf-lint
```

A cadeia Go de cada projeto roda pelos targets que o generator declarou; os
testes de integração pedem a infra de testes do contexto:

```bash
pnpm nx run <name>:test-infra-up
pnpm nx run-many -t fmt-check,vet,test-race
```

O `conformance` sai com exit 2 e `NAO VERIFICADO` quando não consegue ler um
módulo do kernel, por exemplo sem rede e sem o módulo no cache: o gate nunca
aprova o que não leu.

## 5. CI

O `.github/workflows/dmpf-ci.yml` que o `init` escreve chama o
`dmpf-go-ci.yml` deste repositório fixado no `workflowRef` do `versions.json`,
o SHA do commit de onde a versão do plugin saiu, e declara a versão do plugin
para a qual foi escrito. O workflow roda dois jobs:

- **pirâmide Go**: os estágios por `layer:*`, com Postgres, Redpanda e floci
  para a integração, e os gates Buf no estágio de contrato;
- **gates DMPF**: camada única por módulo, gate de dependência, context-check,
  `modsync --check`, `infrasync --check`, `conformance` e `govulncheck`.

O chamador concede `packages: read`, que o workflow pede para instalar o
plugin. Se o plugin instalado divergir da versão do chamador, o workflow
reprova e indica as migrations. A base do `affected` é a branch default do
repositório.

## 6. Atualizar de versão

```bash
pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova>
pnpm install
pnpm nx migrate --run-migrations
```

As migrations do plugin reaplicam os templates do `init`, inclusive o
chamador de CI com o novo `workflowRef`, e sobem o `require` do kernel nos
`go.mod`, os pins de `govulncheck`, `golangci-lint` e `@nx-go/nx-go` e a
diretiva do `go.work`. Rode os comandos que elas devolvem ao fim (`pnpm
install` e `pnpm nx run-many -t tidy`) e os gates do passo 4.

Arquivo do `init` editado à mão não é sobrescrito: a migration o lista no log.
Compare com a versão nova e, depois de salvar o que quiser manter, sobrescreva
com `pnpm nx g @mateusmacedo/dmpf-plugin:init --force`.

## Solução de problemas

| Sintoma | Causa e saída |
| --- | --- |
| `bounded-context` recusa o prefixo `example.com/change-me` | o `init` rodou sem `--modulePrefix`; rode-o com o prefixo real |
| `init` lista arquivos editados e não escreve nada | os arquivos foram alterados depois do `init`; salve as mudanças e rode com `--force` |
| `pnpm install` responde 401 no `npm.pkg.github.com` | `NODE_AUTH_TOKEN` ausente ou sem `read:packages`, ou a credencial está no `.npmrc` do workspace (o pnpm 11.5.3+ a ignora com `Ignored project-level auth setting`); mova-a para o `~/.npmrc` |
| CI: `dmpf-plugin instalado (X) difere do chamador do CI (Y)` | o plugin subiu sem as migrations; rode o passo 6 |
| `conformance` com exit 2 e `NAO VERIFICADO` | um módulo do kernel não foi lido; confira a rede e o proxy Go |

## Referências

- [`docs/adr/060-o-dmpf-tem-consumidor-externo.md`](../adr/060-o-dmpf-tem-consumidor-externo.md) — a decisão.
- [`tools/dmpf-plugin/README.md`](../../tools/dmpf-plugin/README.md) — generators, executors, migrations e `versions.json`.
- [`docs/guides/dmpf-composicao.md`](dmpf-composicao.md) — como um contexto se compõe sobre o kernel.
- [`docs/guides/dmpf-manifesto.md`](dmpf-manifesto.md) — manifesto, baseline e gate de conformidade.
