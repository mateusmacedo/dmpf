# @mateusmacedo/dmpf-plugin

Plugin Nx local do workspace com o generator `bounded-context`, que cria o
**esqueleto** de um bounded context DMPF em Go — um módulo por contexto, um
package por bloco da arquitetura (`domain`, `port`, `application`, `provider`,
`app`; ADR-045) — já na convenção dos módulos do kernel: tags 3D mais `layer:*`,
`dmpf-units.json` com `block` e `bounded_context` declarados por unidade,
`go.mod` sem `require` e a entrada no `go.work`. O código de negócio dos blocos não é gerado aqui: é escrito por
agentes a partir de uma spec de bounded context, pelo harness da
`SPEC-VDP9XX65`, sobre este esqueleto. Sub-spec 2 do KRN-12 (SPEC-H1A190Y8,
ARQ-546).

Projeto Nx `@mateusmacedo/dmpf-plugin`, tags `type:lib`, `scope:shared`,
`stack:node`. É TypeScript e fica fora do universo do verificador DMPF: o
plugin não é unidade, o que ele gera é.

## Uso

```bash
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context <name> \
  --bounded-context <ctx> \
  [--blocks domain,port,application,provider,app] \
  [--directory libs/backend/go] \
  [--dry-run]
```

| Opção | Obrigatória | Default | Regra |
| --- | --- | --- | --- |
| `name` (posicional) | sim | — | `^[a-z][a-z0-9-]*$`; nome da pasta do módulo, do projeto Nx e do pacote npm privado; os packages Go levam o nome do bloco |
| `--bounded-context` | sim | — | `^[a-z][a-z0-9-]*$`; valor literal de `bounded_context` nos manifestos, nunca derivado do nome nem do diretório (ADR-012) |
| `--blocks` | não | os cinco | subconjunto precisa fechar as dependências entre blocos; `contract` é recusado |
| `--directory` | não | `libs/backend/go` | relativo à raiz, sem `..` |
| `--dry-run` | não | — | flag do próprio Nx: lista e não escreve |

O `name` é usado literalmente em kebab-case como pasta do módulo e nome do
projeto; os packages Go são o nome de cada bloco, sem prefixo:
`order-fulfillment` gera o módulo `order-fulfillment` (projeto
`order-fulfillment`) com os packages `domain`, `ports`, `application`,
`provider` e `app`. Onde um arquivo importa o kernel e o contexto com o mesmo
nome de package, o import do kernel recebe alias pelo papel (`kernel`,
`usecase`, `port`). Não há pluralização automática; identificadores de tabela e
de rota são do código de negócio, escrito pelo harness.

## O que é gerado

O contexto vira **um** módulo `<directory>/<name>`, projeto Nx `<name>`, com
cinco arquivos na raiz: `README.md`, `go.mod` (workspace-only, sem `require`),
`project.json` (quatro tags e os cinco targets dos módulos existentes —
`fmt-check`, `vet`, `build`, `test-race`, `govulncheck`), `package.json`
(privado, `0.0.0`) e `dmpf-units.json` (`schema: dmpf/units@1`, uma unidade
`<ctx>/<sufixo>` por bloco, com `include` do package do bloco). Cada bloco vira
um package `<name>/<bloco>` com um `doc.go` compilável (godoc de três linhas).
A tag `layer:*` é a do bloco mais alto gerado; `test-race` leva
`-tags=integration` e `dependsOn` em `postgres` quando `provider` entra; o
`external` do manifesto é a união dos blocos. O `go.work` recebe um `use` em
ordem.

| Bloco | Package | Unidade | `layer:*` | `external` |
| --- | --- | --- | --- | --- |
| `domain` | `domain` | `<ctx>/domain` | `layer:domain` | `[]` |
| `port` | `ports` | `<ctx>/ports` | `layer:domain` | `[]` |
| `application` | `application` | `<ctx>/application` | `layer:services` | `[]` |
| `provider` | `provider` | `<ctx>/provider-postgres` | `layer:providers` | pgx `io.storage`, protobuf `wire.codec` (copiados de `postgres`) |
| `app` | `app` | `<ctx>/app` | `layer:apps` | `[]` |

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
go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline
```

O generator nunca regrava o baseline por conta própria (ADR-031). O módulo
gerado tem `package.json` e vira importer do pnpm — rode `pnpm install` depois
de gerar. O rito completo — manifesto, baseline, gate no CI — está em
`docs/guides/dmpf-manifesto.md`.

## Verificação

```bash
pnpm nx test @mateusmacedo/dmpf-plugin                 # unit tests do generator sobre Tree
pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context billing --bounded-context billing --dry-run
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
    ├── generator.ts                     # validação, generateFiles do módulo e de cada bloco, go.work, instrução final
    ├── identifiers.ts                   # padrão do name e do bounded_context
    ├── blocks.ts                        # dirName, unidade, layer, external, dependências entre blocos
    ├── go-work.ts                       # parse e inserção ordenada no bloco use (...)
    ├── manifest.ts                      # fragmentos do dmpf-units.json (unidades e external)
    ├── generator.spec.ts                # contrato: esqueleto, identificadores, recusas, determinismo
    ├── files/module/                    # os cinco templates EJS (__tmpl__) da raiz do módulo
    └── files/block/                     # o doc.go de cada bloco
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
