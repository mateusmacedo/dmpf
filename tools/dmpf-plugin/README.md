# @mateusmacedo/dmpf-plugin

Plugin Nx que leva o DMPF a qualquer workspace Nx, dentro ou fora deste
monorepo (ADR-060). Ele é a âncora de versão do produto: carrega o
`versions.json` com a tag do kernel, a versão do `dmpf-conformance`, o Go e os
pins das ferramentas, e traz os generators `init` e `bounded-context`, os
executors dos gates, as migrations, os templates, os scripts e os ativos de IA.

Projeto Nx `@mateusmacedo/dmpf-plugin`, tags `type:lib`, `scope:shared`,
`stack:node`. É TypeScript e fica fora do universo do verificador DMPF: o
plugin não é unidade, o que ele gera é.

Dois modos, lidos de `tooling.mode` no `dmpf.json` da raiz:

| Modo | Onde | Ferramentas Go | Scripts |
| --- | --- | --- | --- |
| `local` | este platform | `go run ./tools/dmpf-conformance/cmd/<ferramenta>` | `tools/dmpf-plugin/scripts/` |
| `version` | consumidor | `go run github.com/mateusmacedo/dmpf/tools/dmpf-conformance/cmd/<ferramenta>@<conformance>` | `node_modules/@mateusmacedo/dmpf-plugin/scripts/` |

## Instalação num workspace externo

O pacote é publicado no GitHub Packages. O passo a passo — token, `.npmrc`,
`nx add`, `init`, primeiro contexto, gates e atualização — está em
[`docs/guides/dmpf-consumo-externo.md`](../../docs/guides/dmpf-consumo-externo.md).

```bash
pnpm nx add @mateusmacedo/dmpf-plugin
pnpm nx g @mateusmacedo/dmpf-plugin:init --modulePrefix=github.com/<org>/<repo>
```

## `versions.json`

Schema `dmpf/versions@1`, na raiz do pacote. Generators, executors, migrations
e o workflow de CI leem as versões daqui; nenhum template fixa versão.

| Campo | O que fixa |
| --- | --- |
| `kernel` | tag dos módulos `libs/backend/go/*` que os contextos exigem |
| `conformance` | tag do `tools/dmpf-conformance` (`conformance`, `modsync`, `infrasync`) |
| `go.directive`, `go.image` | diretiva do `go.work` e imagem de build dos contextos |
| `buf`, `protocGenGo` | CLI Buf e plugin `protoc-gen-go` dos contratos |
| `golangciLint`, `govulncheck` | lint e varredura de vulnerabilidades |
| `nxGo` | versão do `@nx-go/nx-go` que o `init` instala |
| `workflowRef` | SHA do commit-fonte da release, que fixa o workflow de CI reutilizável |

O arquivo não é editado à mão: `scripts/sync-versions.mjs` lê os pins das
fontes do platform e, com `--target <versão> --workflow-ref <sha>`, grava a
versão da release e o commit-fonte; `--check` reprova divergência entre as
fontes, o `versions.json` e a versão das migrations.

## Generator `init`

Prepara o workspace para o DMPF. Idempotente: uma segunda execução com as
mesmas opções não altera nada.

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:init [--modulePrefix=<path>] [--appsDir=apps/backend] [--force]
```

| Opção | Default | Regra |
| --- | --- | --- |
| `modulePrefix` | `example.com/change-me` | prefixo dos módulos Go; o valor reservado é gravado, mas o `bounded-context` o recusa |
| `npmScope` | escopo do `package.json` da raiz | escopo dos projetos gerados |
| `appsDir` | `apps/backend` | diretório das apps |
| `edge` | vazio | app que é a borda pública, cliente gRPC confiável dos contextos |
| `spiffeTrustDomain` | `dmpf` | trust domain da PKI local |
| `composeProfile` | `dmpf` | profile do Compose e prefixo dos serviços das apps |
| `composeProject` | nome do `package.json` da raiz, sem escopo | projeto Compose, com os sufixos `-local` e `-testinfra` |
| `imageRegistry` | `ghcr.io/<owner>` quando o prefixo é do GitHub | registry das imagens |
| `bufModule` | `buf.build/<owner>` quando o prefixo é do GitHub | owner dos módulos Buf |
| `force` | — | sobrescreve os arquivos do `init` editados à mão |

Opção omitida mantém o valor do `dmpf.json` anterior. O `init` escreve:

- `dmpf.json` (schema `dmpf/workspace@1`) com o layout e o `tooling.mode`;
- `go.work` com a diretiva de `versions.json`;
- no `nx.json`, só o que falta: o namedInput `go`, o `deploy-env`, os
  `targetDefaults` do `@nx-go/nx-go` e o plugin; e o `@nx-go/nx-go` no
  `package.json`;
- `.golangci.yml`, `.env.example`, o esqueleto estático de `infra/local`,
  `infra/test`, `infra/observability` e `infra/k8s/base`, e o baseline vazio
  em `tools/dmpf-baseline/`;
- os ativos de IA: as skills `dmpf-bounded-context` e `dmpf-testkit`, o agente
  `dmpf-context-author`, a rule `dmpf-bounded-context` e o comando
  `dmpf-new-context`;
- em modo `version`, `.github/workflows/dmpf-ci.yml`, que chama o
  `dmpf-go-ci.yml` deste repositório no `workflowRef` e declara a versão do
  plugin;
- `dmpf.rendered.json`, com o hash de cada arquivo gerenciado que escreveu.

Os templates estão em `templates/init/` e `templates/ai/`. Os de
`templates/init/seed/` (o `docker-compose.yml` local, o provisionamento do
Grafana e o baseline) só são criados: outros generators e o verificador os
estendem. Os demais são
gerenciados: o `init` os reescreve enquanto o hash no `dmpf.rendered.json`
bater com o arquivo, o que cobre a troca do prefixo reservado pelo real e a
troca de versão do plugin. Arquivo editado não é sobrescrito sem `--force`; o
`init` lista cada um e sai antes de escrever qualquer coisa. Os arquivos de
infra gerados a partir dos contextos são do `infrasync`, que o `init` roda
depois do flush.

## Generator `bounded-context`

Cria o **esqueleto** de um bounded context DMPF em Go — um módulo por contexto,
um package por bloco da arquitetura (`domain`, `port`, `application`,
`provider`, `app`; ADR-045) — em `<appsDir>/<name>`, porque um contexto de
negócio é uma app (`type:app`) e em `libs/backend/go` fica só o kernel de reuso
(ADR-046). O código de negócio dos blocos e o `.proto` não são gerados: o autor
do contexto os escreve a partir da spec, sobre este esqueleto
(`/dmpf-new-context`).

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> \
  --boundedContext <ctx> \
  [--blocks domain,port,application,provider,app] \
  [--directory <appsDir>] \
  [--grpcPort <porta>] \
  [--serviceName company.<name>.service.v1.<Name>Service] \
  [--dry-run]
```

| Opção | Obrigatória | Default | Regra |
| --- | --- | --- | --- |
| `name` (posicional) | sim | — | `^[a-z][a-z0-9-]*$`; pasta do módulo, nome do projeto Nx e do pacote npm privado; os packages Go levam o nome do bloco |
| `--boundedContext` | sim | — | `^[a-z][a-z0-9-]*$`; valor literal de `bounded_context` nos manifestos, nunca derivado do nome nem do diretório (ADR-012) |
| `--blocks` | não | os cinco | o subconjunto precisa fechar as dependências entre blocos |
| `--directory` | não | `appsDir` do `dmpf.json` | outro diretório é recusado |
| `--grpcPort` | não | a próxima livre depois das declaradas | porta já declarada é recusada |
| `--serviceName` | não | `company.<name sem hífens>.service.v1.<Name>Service` | nome qualificado do serviço gRPC |
| `--dry-run` | não | — | flag do próprio Nx: lista e não escreve |

Antes da primeira escrita, o generator recusa: workspace sem `dmpf.json` ou
com o prefixo reservado (com a instrução do `init --modulePrefix`), sem o
`infra/local/docker-compose.yml` do `init`, e opções inválidas. Quando aborta,
nada muda. Duas execuções com as mesmas opções produzem bytes idênticos.

### O que é gerado

Dois módulos Go. O do contexto, `<appsDir>/<name>` (projeto `<name>`), com
`README.md`, `go.mod` (só `module` e `go`), `project.json`, `package.json`
privado e `dmpf-units.json` (uma unidade `<ctx>/<sufixo>` por bloco). O de
contrato, `<appsDir>/<name>/contract` (projeto `<name>-contract`,
`layer:contract`), com `buf.yaml`, `buf.gen.yaml`, `proto/` vazio para o autor
e os targets Buf. O generator também gera `deploy/` (Compose, `.env.example`,
`infra.json` e Kustomize por ambiente), registra os dois módulos no `go.work`
(abrindo o bloco `use` se não houver), inclui o `deploy/compose.yml` no
`infra/local/docker-compose.yml` e põe o contrato num release group próprio
no `nx.json`.

| Bloco | Package | Unidade | `layer:*` | `external` |
| --- | --- | --- | --- | --- |
| `domain` | `domain` | `<ctx>/domain` | `layer:domain` | `[]` |
| `port` | `ports` | `<ctx>/ports` | `layer:domain` | `[]` |
| `application` | `application` | `<ctx>/application` | `layer:services` | `[]` |
| `provider` | `provider` | `<ctx>/provider-postgres` | `layer:providers` | pgx `io.storage`, protobuf `wire.codec` (copiados de `postgres`) |
| `app` | `app/` + `app/rpc/` + `cmd/` | `<ctx>/app` | `layer:apps` | `[]` |
| `app` (companion) | `appkit/` | `<ctx>/appkit` | `layer:apps` | `[]` |
| `app` (companion) | `distkit/` | `<ctx>/distkit` | `layer:apps` | `[]` |

A tag `layer:*` do projeto é a do bloco mais alto gerado. `domain`, `port` e
`application` recebem um `doc.go`; o `provider` recebe também `schema.sql` e
`schema.go`, que embute o DDL para o composition root aplicar por
`postgres.Migrate` (ADR-053). O bloco `app`, quando selecionado (o default),
gera a forma canônica dos contextos, e a raiz do contexto **não recebe código
Go** (ADR-048): `app/config.go` (`Defaults`, `FromEnv`, `Validate`, sem prefixo
`DMPF_`), `app/wiring.go` (borda só gRPC com `kernelgrpc.ServerInterceptors`,
migrate da outbox e do schema, relay), `app/telemetry.go`, `app/catalog.go`,
`app/rpc/service.go`, o binário em `cmd/main.go` (`--role api|relay`), o
`Dockerfile` e os kits `appkit/` e `distkit/`. Os targets e a forma exata de
cada arquivo estão fixados em `src/generators/bounded-context/generator.spec.ts`.

Depois do flush, o callback roda o `modsync --write` e, com o bloco `app`, o
`infrasync --write`, porque ambos leem o disco, não a Tree. Em modo `version`,
o `modsync` recebe `--require` dos módulos do kernel que os templates importam,
na versão do `versions.json`: o primeiro contexto de um consumidor importa o
kernel antes de qualquer `go get`.

### Depois de gerar

```bash
pnpm nx run-many -t tidy                                  # go.sum dos módulos novos
go run <conformance> --root . --write-baseline            # unidades novas mudam a classificação
```

O generator imprime o comando do baseline com o `<conformance>` do modo e nunca
o regrava sozinho (ADR-031). O rito completo — manifesto, baseline e gate no CI
— está em `docs/guides/dmpf-manifesto.md`.

## Executors

Os scripts de `scripts/` rodam como executors, com o layout do `dmpf.json` e as
versões do `versions.json` no ambiente.

| Executor | O que faz |
| --- | --- |
| `go-tidy` | `go mod tidy` do módulo com os `replace` do `go.work` aplicados só durante o tidy (ADR-047) |
| `test-env` | roda um comando de teste com o `.env.example` da raiz preenchendo o que o ambiente não declara |
| `test-infra` | sobe ou derruba a infra dos testes de integração; no CI é no-op |
| `buf` | CLI Buf na versão do `versions.json` |
| `buf-gate` | gates Buf de um contrato: `warmup`, `lint`, `pins`, `generate-check` e `breaking` |
| `gate-check` | prova que o depguard e o forbidigo reprovam o que devem em cada módulo |
| `context-check` | prova que todo bounded context segue a forma canônica (ADR-053); o `self-test` só roda em modo `local` |

## Migrations

```bash
pnpm nx migrate @mateusmacedo/dmpf-plugin@<nova>
pnpm nx migrate --run-migrations
```

| Migration | O que faz |
| --- | --- |
| `sync-init` | reaplica os templates do `init` e o Go setup do `nx.json`; arquivo editado não é sobrescrito e vai para o log |
| `update-versions` | sobe o `require` do kernel nos `go.mod`, os pins de `govulncheck`, `golangci-lint` e `@nx-go/nx-go` e a diretiva do `go.work` |

As migrations editam só a Tree e devolvem em `nextSteps` os comandos que
precisam rodar depois (`pnpm install`, `pnpm nx run-many -t tidy`). Cada uma
tem uma entrada só no `migrations.json`, na versão do pacote, mantida pelo
`sync-versions`: ela leva qualquer versão anterior à saída do `init` atual.

## Verificação

```bash
pnpm nx run-many -t typecheck,build,test -p @mateusmacedo/dmpf-plugin
bash tools/dmpf-generator-check.sh --phase structural   # contexto gerado num worktree descartável
bash tools/dmpf-generator-check.sh --phase self-test    # vetores negativos
bash tools/external-consumer.sh                         # workspace Nx de fora, por versão
```

O `dmpf-generator-check.sh` gera o esqueleto num worktree sobre `HEAD`, faz os
commits do rito com os hooks ativos, roda a cadeia Go e o verificador e exige o
worktree limpo; `DMPF_GENERATOR_CHECK_WORKING_TREE=1` aplica o working tree num
commit efêmero. O `external-consumer.sh` cria um workspace Nx vazio fora da
árvore, instala o plugin por `pnpm pack`, resolve o kernel e o `conformance`
do PR por um proxy `file://` (`tools/kernel-file-proxy.sh`) e roda `nx add`,
`init`, `bounded-context` e os gates do consumidor; é o job
`external-consumer` do CI.

## Desenvolvimento

```text
tools/dmpf-plugin/
├── generators.json · executors.json · migrations.json   # código em ./dist, schemas em ./src
├── versions.json                                          # âncora de versões
├── scripts/                                               # scripts dos executors e sync-versions.mjs
├── templates/
│   ├── init/{managed,seed,version}/                       # o que o init escreve
│   ├── ai/                                                # ativos de IA
│   └── bounded-context/{module,block,application,provider,app,deploy,contract}/
└── src/
    ├── generators/{init,bounded-context}/
    ├── executors/<executor>/
    ├── migrations/{sync-init,update-versions}/
    ├── lib/                                               # dmpf.json, versions.json, caminhos e execução de scripts
    └── testing/                                           # helpers de teste, fora do build
```

No platform, o Nx carrega generators e executors direto da fonte, sem build. O
pacote publicado usa o `dist` e leva `templates/`, `scripts/` e
`versions.json` pelo campo `files`. A versão segue o release group `npm`
(ADR-047) e a publicação vai para `https://npm.pkg.github.com`.

Os ativos de IA, a infra genérica e o `.golangci.yml` do platform são saída do
`init`: edite o template, não a cópia. O CI reprova a diferença
(`init --dry-run` sem mudança).

## Referências

- `docs/adr/060-o-dmpf-tem-consumidor-externo.md` — o DMPF consumido por versão fora do monorepo.
- `docs/guides/dmpf-consumo-externo.md` — passo a passo do consumidor.
- `docs/specs/SPEC-H1A190Y8-dmpf-generator-bounded-context.md` — spec do generator de bounded context.
- `docs/specs/SPEC-VDP9XX65-dmpf-harness-bounded-context.md` — harness que escreve o código de negócio sobre o esqueleto.
- `docs/adr/012-classificacao-por-metadado-declarado.md` — `block` e `bounded_context` declarados, nunca inferidos.
- `docs/adr/048-layout-canonico-de-bounded-context.md` — layout canônico do bloco `app`.
- `docs/adr/053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md` — forma do contexto.
- `docs/guides/dmpf-manifesto.md` — manifesto, baseline e gate de conformidade.
