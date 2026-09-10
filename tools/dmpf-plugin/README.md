# @lidercap-apps/dmpf-plugin

Plugin Nx local do workspace com o generator `bounded-context`, que cria o
**esqueleto** de um bounded context DMPF em Go — um módulo por bloco da
arquitetura (`domain`, `port`, `application`, `provider`, `app`) — já na
convenção dos módulos do kernel: tags 3D mais `layer:*`, `dmpf-units.json` com
`block` e `bounded_context` declarados, `go.mod` sem `require` e entradas no
`go.work`. O código de negócio dos blocos não é gerado aqui: é escrito por
agentes a partir de uma spec de bounded context, pelo harness da
`SPEC-VDP9XX65`, sobre este esqueleto. Sub-spec 2 do KRN-12 (SPEC-H1A190Y8,
ARQ-546).

Projeto Nx `@lidercap-apps/dmpf-plugin`, tags `type:lib`, `scope:shared`,
`stack:node`. É TypeScript e fica fora do universo do verificador DMPF: o
plugin não é unidade, o que ele gera é.

## Uso

```bash
pnpm nx g @lidercap-apps/dmpf-plugin:bounded-context <name> \
  --bounded-context <ctx> \
  [--blocks domain,port,application,provider,app] \
  [--directory libs/backend/go] \
  [--dry-run]
```

| Opção | Obrigatória | Default | Regra |
| --- | --- | --- | --- |
| `name` (posicional) | sim | — | `^[a-z][a-z0-9-]*$`; prefixo dos diretórios e origem do package Go raiz de cada módulo |
| `--bounded-context` | sim | — | `^[a-z][a-z0-9-]*$`; valor literal de `bounded_context` nos manifestos, nunca derivado do nome nem do diretório (ADR-012) |
| `--blocks` | não | os cinco | subconjunto precisa fechar as dependências entre blocos; `contract` é recusado |
| `--directory` | não | `libs/backend/go` | relativo à raiz, sem `..` |
| `--dry-run` | não | — | flag do próprio Nx: lista e não escreve |

O `name` é usado literalmente em kebab-case nos diretórios e nomes de projeto,
e sem separador como package Go raiz de cada módulo: `order-fulfillment` gera
`order-fulfillment-domain` com package `orderfulfillmentdomain`. Não há
pluralização automática; identificadores de tabela e de rota são do código de
negócio, escrito pelo harness.

## O que é gerado

Cada bloco vira um módulo `<directory>/<name>-<sufixo>`, projeto Nx
`<name>-<sufixo>-go`, com seis arquivos: `README.md`, `doc.go` (package raiz
compilável, godoc de três linhas), `go.mod` (workspace-only, sem `require`),
`project.json` (quatro tags e os cinco targets dos módulos existentes —
`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`), `package.json`
(privado, `0.0.0`) e `dmpf-units.json` (`schema: dmpf/units@1`, uma unidade
`<ctx>/<sufixo>` com `include` do package raiz). O `go.work` recebe os `use`
em ordem.

| Sufixo | Bloco | `layer:*` | `external` |
| --- | --- | --- | --- |
| `-domain` | `domain` | `layer:domain` | `[]` |
| `-ports` | `port` | `layer:domain` | `[]` |
| `-application` | `application` | `layer:services` | `[]` |
| `-provider-postgres` | `provider` | `layer:providers` | pgx `io.storage`, protobuf `wire.codec` (copiados de `dmpf-provider-postgres`) |
| `-app` | `app` | `layer:apps` | `[]` |

O `test-race` de `provider-postgres` e `app` sai com `cache: false` e
`-tags=integration`, e o do `app` depende do provider — como nos módulos
reais. O bloco `contract` fica fora: a fonte vive em `contracts/` pelo rito
Buf, e é o harness que escreve o `.proto` do contexto.

Toda validação corre antes da primeira escrita; quando o generator aborta,
nada muda no workspace. Duas execuções com as mesmas opções produzem bytes
idênticos: os templates saem no formato final e `formatFiles` não é chamado.

## Depois de gerar: classificar

A saída termina com a instrução do baseline. Unidades novas são ato de
classificação (AUT-01), então o baseline do verificador é regravado em commit
próprio, separado do código:

```bash
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --write-baseline
```

O generator nunca regrava o baseline por conta própria (ADR-031). O módulo
gerado tem `package.json` e vira importer do pnpm — rode `pnpm install` depois
de gerar. O rito completo — manifesto, baseline, gate no CI — está em
`docs/guides/dmpf-manifesto.md`.

## Verificação

```bash
pnpm nx test @lidercap-apps/dmpf-plugin                 # unit tests do generator sobre Tree
pnpm nx g @lidercap-apps/dmpf-plugin:bounded-context billing --bounded-context billing --dry-run
bash tools/dmpf-generator-check.sh --phase structural   # prova mecânica em worktree descartável
bash tools/dmpf-generator-check.sh --phase self-test    # vetores negativos
```

A prova gera o esqueleto num worktree sobre `HEAD`, checa `biome ci` e
`gofmt -l` sem escrita, guarda o SHA-256 de cada arquivo gerado, faz os dois
commits do rito (classificação, depois código) com os hooks ativos, roda a
cadeia Go e o verificador com `--base` e exige `git diff --exit-code` e
`git status --porcelain` vazio. Enquanto o plugin não estiver commitado,
`DMPF_GENERATOR_CHECK_WORKING_TREE=1` aplica o working tree num commit efêmero
do worktree. A prova nunca define `CI` e exporta
`pnpm_config_verify_deps_before_run=false`: com o `node_modules` da raiz
montado por symlink, o pnpm reinstalaria dentro do worktree e reescreveria os
links reais (ADR-041).

## Desenvolvimento

```text
tools/dmpf-plugin/
├── generators.json                      # coleção: aponta para src/, sem build
└── src/generators/bounded-context/
    ├── schema.json / schema.d.ts        # opções e validação de entrada
    ├── generator.ts                     # validação, generateFiles por bloco, go.work, instrução final
    ├── identifiers.ts                   # identificador Go derivado do name
    ├── blocks.ts                        # sufixo, layer, external, dependências entre blocos
    ├── go-work.ts                       # parse e inserção ordenada no bloco use (...)
    ├── manifest.ts                      # fragmentos do dmpf-units.json
    ├── generator.spec.ts                # contrato: esqueleto, identificadores, recusas, determinismo
    └── files/module/                    # os seis templates EJS (__tmpl__) parametrizados por bloco
```

`typecheck`, `test` e `build` são inferidos pelos plugins do workspace
(`@nx/js/typescript`, `@nx/jest/plugin`); o `lint` é o `biome ci .` global. O
Nx carrega o generator direto do fonte, então não há passo de build antes de
usá-lo. O plugin é versionado pelo `nx release` (`type:lib`) e não é publicado
(`private: true`), como os módulos Go (ADR-030).

## Referências

- `docs/specs/SPEC-H1A190Y8-dmpf-generator-bounded-context.md` — spec guarda-chuva.
- `docs/specs/SPEC-VDP9XX65-dmpf-harness-bounded-context.md` — harness que escreve o código de negócio sobre o esqueleto.
- `docs/specs/SPEC-XMNBMY50-dmpf-shared-kernel.md` — pré-requisito normativo para o contexto consumir o kernel.
- `docs/adr/041-sdk-de-referencia-generator-e-bom-certificado.md` — addendum de 2026-09-09.
- `docs/adr/012-classificacao-por-metadado-declarado.md` — `block` e `bounded_context` declarados, nunca inferidos.
- `docs/guides/dmpf-manifesto.md` — manifesto, baseline e gate de conformidade.
