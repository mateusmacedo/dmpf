---
id: SPEC-H1A190Y8
slug: dmpf-generator-bounded-context
title: DMPF KRN-12.2 — Plugin local e generator bounded-context (guarda-chuva)
stage: building
priority: P2
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-SJ66880S, SPEC-XMNBMY50]
ticket_url: null
subtask_urls: []
created: 2026-09-08
---

# SPEC-H1A190Y8: DMPF KRN-12.2 — Plugin local e generator bounded-context (guarda-chuva)

## Resumo

Segunda sub-spec de `SPEC-8HWBWJCB` (`KRN-12`). Entrega o primeiro plugin
local Nx do workspace, `tools/dmpf-plugin`, com o generator `bounded-context`,
que produz o **esqueleto determinístico** de um bounded context DMPF em Go —
cinco módulos na convenção de `KRN-01`, tags 3D mais `layer:*`, entradas no
`go.work`, manifesto `dmpf/units@1` com `block` e `bounded_context`
**declarados por opção** — e o **harness de agentes** que, a partir de uma spec
de bounded context em template próprio, escreve o código de negócio nos cinco
blocos (AI SDD), com os gates do repositório como juiz. A prova mecânica `tools/dmpf-generator-check.sh` gera num worktree
descartável, commita em dois atos (classificação; código), roda a cadeia e o
verificador com `--base`, e reprova se um byte gerado mudar.

Esta spec é o **guarda-chuva**: fixa o plugin, o esqueleto dos módulos, a
prova do esqueleto e o que já existe. O código de negócio é produzido pelo
harness:

| Sub-spec | Entrega | Estado |
| --- | --- | --- |
| `SPEC-VDP9XX65` (KRN-12.2h) | Harness de agentes: template de spec de bounded context, agente, skill (golden path), rules, command, golden `bookings` commitado, prova de regressão local, CI, guia e docs | ativa; depende desta e de `SPEC-XMNBMY50` |
| `SPEC-8FSD8505` (KRN-12.2a) | DSL de domínio + motor determinístico | **deferida** (2026-09-09) |
| `SPEC-VZ16X0MS` (KRN-12.2b) | Templates por agregado | **deferida** |
| `SPEC-F7S5B6KV` (KRN-12.2c) | Integração por contrato gerado | **deferida** |

`SPEC-XMNBMY50` (shared kernel) é pré-requisito normativo: sem ela, qualquer
contexto fora de `dmpf-kernel` que importe o kernel reprova com `DMPF-D002`.
Esta spec só fecha (`stage: done`) quando `SPEC-VDP9XX65` fechar.

Como usuário de uma squad, quero escrever a spec do meu bounded context,
rodar um command e obter um contexto que o verificador aprova, com a regra de
negócio escrita — restando a mim o rito de classificação e a revisão do PR.

## Contexto

- **Problema**: `tools/generators/` e `tools/executors/` eram `.gitkeep`;
  `nx.json` só tinha plugins npm; `@nx/plugin` e `@nx/devkit` não eram
  dependências diretas. Criar um bounded context era copiar cinco
  `project.json`, cinco `go.mod`, cinco `dmpf-units.json` e editar o `go.work`
  à mão — com o risco de tag faltando ou unidade classificada por nome de
  diretório. A primeira iteração desta spec provou o plugin e a prova, mas fixava um
  domínio de exemplo (`Request`/`Fulfillment`) em todo contexto gerado e
  expôs que a norma vigente impede um contexto novo de consumir o kernel; a
  segunda (generator orientado ao domínio) somou 24 achados em duas revisões
  externas e foi deferida em favor do harness de agentes.
- **Impacto**: um comando produz a estrutura e o código por agregado; o rito
  de classificação continua humano; o CI prova, a cada mudança do generator,
  que o artefato é aprovado.
- **Inspiração**: `docs/guides/dmpf-manifesto.md` (o gesto que o generator
  automatiza); `tools/dmpf-cell-check.sh` (prova em worktree descartável); os
  14 `project.json` existentes (o formato exato dos targets); o próprio fluxo
  spec-driven deste repositório (agentes, skills e rules em `.claude/`).
- **Links relevantes**:
  - `SPEC-8HWBWJCB` — "sem edição manual" definido; catálogo do que o generator
    não pode tocar
  - `SPEC-MQA5HAXF` — `KRN-01`, a convenção de módulo
  - `SPEC-WTAXFV8B` — `KRN-02`, o verificador e o baseline
  - `SPEC-SJ66880S` — `KRN-11`, os kits que os testes gerados invocam
  - `SPEC-XMNBMY50` — shared kernel (pré-requisito das sub-specs b e c)
  - ADR-012, ADR-017, ADR-028, ADR-030, ADR-031, ADR-034, ADR-041 (addendum
    2026-09-09)
  - `nx_docs` "Local Generators" — `nx g @nx/plugin:plugin tools/<nome>`, `nx
    g @nx/plugin:generator`

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta spec adota |
| --- | --- | --- |
| "Generator Nx em `tools/`" | nenhum plugin local; `@nx/plugin`/`@nx/devkit` ausentes; `tsconfig.base.json` com `paths: {}` | Plugin criado por `nx g @nx/plugin:plugin tools/dmpf-plugin`; `@nx/plugin` e `@nx/devkit` 23.1.0 no `catalog:` e no `package.json` raiz; resolução pelo workspace pnpm (`tools/*`), sem `paths` |
| "um módulo por bloco" | `.golangci.yml` seleciona por sufixo `*-domain`, `*-ports`, `*-application` | `<ctx>-{domain,ports,application,provider-postgres,app}`; `contract` só como fonte `.proto` (sub-spec c), nunca `gen/go` |
| "esqueleto de testes dos kits" | kits de `KRN-11` prontos | Testes gerados por agregado e por invariante declarada (sub-spec b), rodando de fato |
| "aprovado pelo verificador, sem edição manual" | sem `--base` → "não verificado"; autorização lida em `base..HEAD`; `DMPF-T002` se classificação e código no mesmo commit; **C2 reprova todo contexto fora de `dmpf-kernel` que importe o kernel** | A prova commita duas vezes e verifica com `--base`; a aprovação depende do shared kernel (`SPEC-XMNBMY50`) |
| — | `CI`, `DMPF_PG_DSN`, `DMPF_KAFKA_BROKERS` no `env` do job; `tb.Env` falha sob `CI` sem infra | Prova em duas fases, as duas antes da infra: `structural` e `self-test`. O esqueleto não tem código de negócio para integrar; a integração com infra é do golden `bookings` (`SPEC-VDP9XX65`) |
| `private: true` exclui do release | ADR-030: `private` exclui **publicação**, não versionamento | O plugin é `type:lib`, **versionado** pelo `nx release` e não publicado — `@nx/js` nem cria o target `nx-release-publish` para pacote privado |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| ADR-012 | `block` e `bounded_context` declarados, nunca inferidos |
| ADR-017 + `SPEC-XMNBMY50` | C2: contexto só importa outro por superfície pública ou shared kernel |
| FND-10 §5.2 `AUT-01` A3; RFC §10.2 T4/T6; ADR-028 | Criar unidade é ato regulado; baseline em commit próprio; `DMPF-T001`/`T002` |
| ADR-031 | O gate não conserta o próprio insumo; `--write-baseline` é comando separado |
| ADR-030 | Todo módulo Go tem `package.json` `private`; `private` não exclui versionamento |
| ADR-034 | Sem `require` de irmão; resolução pelo `go.work` |
| `AGENTS.md` §Convenções | Tags 3D + `layer:*`; cinco targets Go via `nx:run-commands`; `lint` de `@nx-go/nx-go:lint` |
| `nx_docs` "Local Generators" | Estrutura do plugin; `schema.json`; `Tree`, `generateFiles` |

<constraints>
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório nem do arquivo de definição: os dois vêm de opção obrigatória e vão literalmente ao `dmpf-units.json` (ADR-012).
- [P0] NUNCA regravar `tools/dmpf-baseline/units-baseline.json` pelo generator: a prova o faz num worktree descartável, em commit próprio, e a squad o faz pelo rito (`AUT-01`, ADR-031).
- [P0] NUNCA produzir `project.json` com target que o plugin infere (`lint`) nem sem os cinco targets no formato dos módulos existentes (`AGENTS.md`).
- [P0] NUNCA deixar a prova passar sem `git diff --exit-code` e `git status --porcelain` vazio sobre o worktree depois da cadeia: silêncio não é aprovação.
- [P1] Toda opção é validada antes do primeiro `tree.write`; falha não deixa arquivo.
- [P1] A saída do generator termina com a instrução do `--write-baseline` e a frase "ato de classificação (AUT-01)"; a prova reprova se ela faltar.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Bootstrap do plugin**: `@nx/plugin` e `@nx/devkit` 23.1.0 no
  `catalog:` e no `package.json` raiz (`allowBuilds` inalterado); `pnpm nx g
  @nx/plugin:plugin tools/dmpf-plugin --name @mateusmacedo/dmpf-plugin
  --importPath @mateusmacedo/dmpf-plugin --linter none --unitTestRunner jest
  --e2eTestRunner none --tags type:lib,scope:shared,stack:node`. Resultado:
  `tools/dmpf-plugin/{package.json,generators.json,tsconfig*.json,
  jest.config.cts,.spec.swcrc,README.md,src/}`, `private: true`, `version
  0.0.0`, entrada em `tsconfig.json` `references`; targets `typecheck`, `test`
  e `build` **inferidos** pelos plugins (`@nx/js/typescript`, `@nx/jest`) —
  nenhum redeclarado. `generators.json` aponta `factory`/`schema` para
  `./src/…`: o Nx carrega o generator do fonte, sem `build`.
  - Edge case: `pnpm nx run-many -t lint,typecheck,test,build
    --exclude=@mateusmacedo/dmpf-source` verde; `biome ci .` verde.
- [x] **[P0] Esqueleto dos módulos por bloco**: para cada bloco pedido, o
  generator produz `<directory>/<name>-<sufixo>` com `go.mod` (`module
  github.com/mateusmacedo/dmpf/<directory>/<ctx>-<sufixo>`,
  `go <versão lida do go.work>`, sem `require` — workspace-only),
  `project.json` (`name` `<ctx>-<sufixo>-go`, quatro tags, cinco targets
  `fmt-check`, `vet`, `build`, `test-race`, `govulncheck` no formato dos
  módulos existentes; `test-race` com `cache: false` e `-count=1 -p 1
  -tags=integration` só em `provider-postgres` e `app`; `dependsOn` do provider
  no `app`), `package.json` (`@mateusmacedo/<ctx>-<sufixo>-go`, `0.0.0`,
  `private`), `dmpf-units.json` (`schema: dmpf/units@1`; unidade
  `<ctx>/<sufixo>` com `block`, `bounded_context`, `include` de todos os
  packages, `public_integration_surface: false`, `external`, `exceptions:
  []`), `README.md` e `doc.go`. `go-work.ts` insere os `use` em ordem
  lexicográfica e lê a versão `go`. `formatFiles` **não** é chamado; valores
  em JSON entram por `JSON.stringify`.

  | Sufixo | Bloco | `layer:*` | `external` |
  | --- | --- | --- | --- |
  | `<ctx>-domain` | `domain` | `layer:domain` | `[]` |
  | `<ctx>-ports` | `port` | `layer:domain` | `[]` |
  | `<ctx>-application` | `application` | `layer:services` | `[]` |
  | `<ctx>-provider-postgres` | `provider` | `layer:providers` | pgx `io.storage` + protobuf `wire.codec` (copiados de `dmpf-provider-postgres`) |
  | `<ctx>-app` | `app` | `layer:apps` | `[]` |

  - Opções: `name` (posicional, `^[a-z][a-z0-9-]*$`), `--bounded-context`
    (obrigatório, sem default, mesmo `pattern`), `--blocks` (default os
    cinco; `contract` recusado; subconjunto precisa fechar as dependências
    entre blocos), `--directory` (default `libs/backend/go`, relativo, sem
    `..`); `--dry-run` é flag do Nx. O `name` entra literal (kebab-case nos diretórios e projetos; sem separador
    no package Go raiz), sem pluralização. Toda validação antes do primeiro
    `tree.write`.
  - O **conteúdo Go por agregado** desses módulos é das sub-specs a, b e c.
- [x] **[P0] Base da prova mecânica** `tools/dmpf-generator-check.sh`, padrão de
  `dmpf-cell-check.sh`, com `--phase structural|self-test`:
  1. `git worktree add --detach` descartável em `HEAD` — nunca em `develop`;
     `ln -s` do `node_modules` da raiz e do plugin; `NX_DAEMON=false`;
     `HEAD0=$(git -C "$WT" rev-parse HEAD)`. Com `node_modules` compartilhado
     por symlink, a prova exporta `pnpm_config_verify_deps_before_run=false`,
     nunca define `CI`, e reprova se qualquer link de `node_modules/*` ou
     `node_modules/@*/*` da raiz sair do repositório — sem isso o pnpm
     reinstala dentro do worktree e reescreve o `node_modules` real (incidente
     verificado; ADR-041). Variável de desenvolvimento
     `DMPF_GENERATOR_CHECK_WORKING_TREE=1` aplica o working tree num commit
     efêmero para validar o plugin antes de ele estar commitado.
  2. Roda o generator; captura a saída e reprova se a instrução do baseline
     faltar.
  3. Checagem sem escrita (`biome ci`, `gofmt -l`) e manifesto SHA-256 de
     todo arquivo gerado, fora do worktree — o `biome check --write` do
     pre-commit corrigiria o arquivo e o `git diff` posterior não acusaria.
  4. Commit 1 (classificação): `dmpf-units.json` + `--write-baseline` →
     `chore(genproof): classificar unidades`.
  5. Commit 2 (código): o resto + `go.work` → `feat(genproof): scaffold`.
     Identidade de automação por `git -c`; hooks do Lefthook ativos.
  6. **Fase `structural`**: `fmt-check`, `vet`, `build`, `lint`;
     `dmpf-conformance --root . --base $HEAD0`; manifesto; `git diff
     --exit-code`; `status --porcelain` vazio.
    7. **Fase `self-test`** (quatro vetores: edição pós-commit; classificação
     misturada → `T002`; instrução do baseline ausente; JSON fora do padrão
     Biome) sobre o esqueleto; não há fase `integration` para o esqueleto —
     módulos sem código de negócio não têm o que integrar; o golden `bookings`
     (`SPEC-VDP9XX65`) é quem roda `test-race` com infra no CI.
  8. Remove o worktree sempre (`trap`); nunca `rm`.
- [ ] **[P0] Harness de agentes** (`SPEC-VDP9XX65`): template de spec de
  bounded context, agente, skill, rules, command e golden `bookings`. Esta
  spec fecha quando ela fechar.
- [x] **[P1] CI**: `structural` e `self-test` no bloco Gates DMPF do `ci.yml`,
  os dois **sem** `if: steps.go_affected` — o artefato que a prova mede é escrito
  pelo generator, e mudança só no TypeScript do plugin não toca projeto
  `stack:go`.
- [ ] **[P1] Documentação**: guia, `AGENTS.md`, `tasks.md`, README e addendum do
  ADR-041 pela `SPEC-VDP9XX65`, única dona desses arquivos.

### Não-funcionais

- [x] Determinismo do esqueleto: mesmas opções → bytes idênticos; sem
  `formatFiles`; `JSON.stringify` em JSON. O código de negócio (harness) não
  promete bytes idênticos — promete passar nos gates.
- [ ] Feedback rápido: generator em menos de 5 s; fase `structural` em menos
  de 3 min no runner. Medido local: `structural` com os cinco blocos entre 31 s
  e 1 min 04 s conforme o cache do Nx; falta a medição no runner.
- [x] Conformidade: os módulos gerados passam pelo verificador com `--base`
  (com o shared kernel); o plugin não é unidade DMPF.
- [x] Cadeia verde do workspace com o plugin incluído; `biome ci` passa.
- [x] Dependências npm novas: só `@nx/plugin` e `@nx/devkit`, na versão de
  `nx` (`23.1.0` no `catalog:`); o harness é Markdown.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain` … `app` | [ ] | Nada no kernel; o generator produz módulos novos por bloco |
| Workspace | [x] | `tools/dmpf-plugin` (novo), `tools/dmpf-generator-check.sh` (novo), `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml`, `tsconfig.json`, `ci.yml`, docs |
| Norma | [ ] | `SPEC-XMNBMY50` (shared kernel) — pré-requisito, fora desta spec |
| Harness e golden | [ ] | `SPEC-VDP9XX65` — agente, skill, rules, command, `bookings-*` |

## Localização de código

```text
tools/dmpf-plugin/                                    — plugin local (@mateusmacedo/dmpf-plugin); type:lib scope:shared stack:node
  package.json, generators.json, tsconfig.json, tsconfig.lib.json, tsconfig.spec.json, jest.config.cts, .spec.swcrc, README.md
    src/generators/bounded-context/schema.json          — name, boundedContext, blocks, directory
  src/generators/bounded-context/generator.ts         — validate → generateFiles por bloco → go.work → instrução do baseline
  src/generators/bounded-context/{identifiers,blocks,go-work,manifest}.ts
  src/generators/bounded-context/generator.spec.ts
    src/generators/bounded-context/files/**             — templates __tmpl__ do esqueleto (metadados por bloco; os de domain da 1ª iteração ficam como referência de forma)
tools/dmpf-generator-check.sh                         — --phase structural|self-test
package.json, pnpm-workspace.yaml, pnpm-lock.yaml     — @nx/plugin, @nx/devkit (catalog:)
tsconfig.json                                         — references
.github/workflows/ci.yml                              — structural + self-test no bloco Gates DMPF
docs/** e AGENTS.md                                    — SPEC-VDP9XX65
```

## Design

### Arquitetura

```text
  pnpm nx g @mateusmacedo/dmpf-plugin:bounded-context bookings --bounded-context resource-scheduling
   │ validate(options)  ── abort sem escrever ──►  recusa
   ▼
 blocks.ts ──► generateFiles por bloco (esqueleto) ──► go-work.ts ──► instrução do baseline
   └──► harness (SPEC-VDP9XX65): agente escreve o código de negócio sobre o esqueleto; gates julgam

 tools/dmpf-generator-check.sh (worktree descartável em HEAD)
   HEAD0 ── generator ── checks sem escrita + manifesto ── commit1(manifestos+baseline) ── commit2(código+go.work)
         ── structural: fmt/vet/build/lint + dmpf-conformance --base HEAD0 + manifesto + diff/status
         ── self-test: 4 vetores reprovando pelo motivo esperado
```

### Fluxo — do comando ao bounded context aprovado

1. A squad escreve a spec do bounded context (template próprio) e roda o
   command do harness; o generator produz o esqueleto — a validação aborta
   antes de escrever quando uma opção falha.
2. O agente escreve o código dos cinco blocos, o `.proto` e o OpenAPI, roda a
   cadeia e o verificador até passar, e imprime o rito humano.
3. A squad roda o rito Buf e commita o contrato; roda `--write-baseline` e
   commita só manifestos e baseline; commita o código; abre o PR.
4. `dmpf-conformance --root . --base <antes>` aprova: unidades criadas em
   commit próprio (`T4`/`T6`), unidades do kernel designadas como shared
   kernel, nada fora do `include`.

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| ADR-012 | `generator.spec.ts`: recusa sem `boundedContext`; manifesto leva o valor da opção | plugin `test` |
| C2 / shared kernel | prova com `--base`, com o kernel já designado no baseline — o contexto gerado importa o kernel e é aprovado; o vetor negativo (unidade não designada e outro bounded context → `D002`) é do `dmpf-shared-kernel-check.sh` | CI (esta spec + `SPEC-XMNBMY50`) |
| `AUT-01`, `T001`/`T002` | dois commits na prova; `self-test` mistura e reprova | CI |
| "Sem edição manual" | manifesto SHA-256 + `git diff --exit-code` + `status --porcelain`; `self-test` | CI |
| Tags 3D + `layer:*`, cinco targets, sem `lint` | `generator.spec.ts` compara com `dmpf-domain/project.json` | plugin `test` |
| ADR-034 (sem `require`) | `generator.spec.ts` lê o `go.mod` gerado | plugin `test` |
| Determinismo | `generator.spec.ts` gera duas vezes e compara bytes | plugin `test` |

## Decisões técnicas

- **Plugin local via `@nx/plugin:plugin`** porque é o caminho documentado do
  Nx 23 e dá `schema.json`, `--dry-run` e `Tree`. Alternativa descartada:
  script Node em `tools/generators/`, sem validação de schema nem dry-run.
- **Generator próprio, sem compor `@nx-go/nx-go:library`** porque o plugin
  infere targets e o workspace declara os cinco em `project.json`. Alternativa
  descartada: compor e sobrescrever, porque redeclarar target inferido quebra
  o cache (ADR-002).
- **`generators.json` aponta para `src/`, sem `build`** porque o Nx transpila
  plugin local do fonte; `dist/` exigiria build e cópia de assets antes de todo
  uso, inclusive na prova. Alternativa descartada: `@nx/js:tsc` explícito com
  `assets`, porque redeclara o `build` inferido.
- **Sem `formatFiles`** porque o Prettier não existe no workspace e o devkit o
  trata como no-op — a saída dependeria do ambiente.
- **Dois commits na prova** porque o verificador lê a autorização em
  `base..HEAD` e emite `T002` para classificação misturada com código.
- **Worktree em `HEAD`, checks sem escrita e manifesto SHA-256** porque o
  plugin não existe em `develop`, e o pre-commit corrige arquivo fora do
  padrão sem que o `git diff` posterior acuse.
- **Shared kernel como pré-requisito, não contorno** porque declarar o
  contexto gerado como `dmpf-kernel` apagaria a identidade de limite (ADR-017)
  e re-escopar para um esqueleto sem kernel cumpriria o critério 1 de
  ARQ-531 de forma trivial. Alternativas descartadas registradas no
  ADR-041 e na `SPEC-XMNBMY50`.
- **Harness de agentes, não generator orientado ao domínio nem mini contexto
  fixo** porque templates fixos produzem o mesmo código em todo contexto, e a
  DSL determinística que os substituiria somou 24 achados em duas revisões
  (sub-specs a/b/c, deferidas). O esqueleto continua determinístico; o código
  de negócio é do agente, julgado pelos gates (`SPEC-VDP9XX65`).
- **O plugin é versionado e não publicado** porque `release.projects` =
  `tag:type:lib` e `private: true` faz o `@nx/js` nem criar o target
  `nx-release-publish` (ADR-030).

## Regras relacionadas

- `SPEC-8HWBWJCB` — "sem edição manual"; o generator nunca toca os baselines.
- `docs/guides/dmpf-manifesto.md` — o rito que a prova reproduz.
- `AGENTS.md` — formato dos targets; `nx-generate` para scaffolding.
- `SPEC-XMNBMY50`, `SPEC-VDP9XX65`; sub-specs a/b/c deferidas como caminho avaliado.

## Verificação e testes

### Critérios de aceite

- [x] Plugin criado; `@nx/plugin` e `@nx/devkit` via `catalog:`; lockfile
  atualizado; cadeia do workspace verde com o plugin; `biome ci` verde.
- [x] O generator aborta sem escrever quando `boundedContext` falta, `name`
  existe, `blocks` inclui `contract` ou não fecha dependências, `directory`
  escapa; `--dry-run` não escreve; duas execuções → bytes idênticos.
- [x] `project.json` gerado tem os cinco targets no formato dos módulos e não
  redeclara `lint`; `go.mod` sem `require`; `go.work` em ordem.
- [x] A saída termina com a instrução do `--write-baseline` e "ato de
  classificação (AUT-01)".
- [x] Fase `structural` da prova chega ao commit 2 com hooks ativos e cadeia Go
  verde. Enquanto o shared kernel não existia, ela reprovava só no verificador,
  por `D002`; com `SPEC-XMNBMY50` designada no baseline, passa.
- [ ] O golden `bookings`, produzido pelo harness a partir da spec de
  bounded context, compila, passa na cadeia Go e é **aprovado** pelo
  verificador (critério 1 de ARQ-531) — `SPEC-VDP9XX65` + `SPEC-XMNBMY50`.
- [x] `structural` e `self-test` passam: `structural` com os cinco blocos gera
  31 arquivos, `dmpf-conformance` devolve `conforme`, o manifesto confere e `git
  diff`/`status --porcelain` saem vazios; `self-test` reprova nos quatro vetores
  pelo motivo esperado. Os dois passos estão no `ci.yml`; o verde no runner sai
  no PR.
- [ ] Guia, `AGENTS.md`, `tasks.md`, README e addendum do ADR-041 atualizados —
  `SPEC-VDP9XX65`.

### Cenários de teste

```text
DADO um Tree com o go.work atual
QUANDO name=checkout --bounded-context=sales --blocks domain,port,application,provider,app
ENTÃO cinco diretórios checkout-* existem, cada dmpf-units.json tem block do bloco e bounded_context "sales" literal, cada project.json tem quatro tags e cinco targets, go.work tem cinco use novos em ordem

DADO um Tree limpo
QUANDO name=billing sem --bounded-context
ENTÃO recusa contendo "ADR-012" e nenhuma mudança no Tree

DADO o worktree da prova com o plugin commitado e sem shared kernel no baseline
QUANDO --phase structural roda
ENTÃO commit 1 e commit 2 acontecem com hooks ativos, a cadeia Go passa e o verificador reprova com D002 nomeando <ctx>-domain → dmpf-kernel/domain

DADO o mesmo worktree com shared_kernels ["dmpf-kernel"] no baseline (SPEC-XMNBMY50)
QUANDO --phase structural roda
ENTÃO dmpf-conformance devolve "conforme", git diff --exit-code passa e status --porcelain é vazio
```

<critical_constraints>
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório nem do arquivo de definição (ADR-012).
- [P0] NUNCA regravar `tools/dmpf-baseline/units-baseline.json` pelo generator (`AUT-01`, ADR-031).
- [P0] NUNCA produzir `project.json` com target que o plugin infere nem sem os cinco targets no formato dos módulos existentes.
- [P0] NUNCA deixar a prova passar sem `git diff --exit-code` e `status --porcelain` vazio: silêncio não é aprovação.
- [P1] Toda opção é validada antes do primeiro `tree.write`.
- [P1] A saída termina com a instrução do `--write-baseline` e "ato de classificação (AUT-01)".
</critical_constraints>

## Escopo fora

- **Mudança normativa do shared kernel**: `SPEC-XMNBMY50`, com ADR próprio e
  mudança no verificador.
- **Mini contexto fixo `Request`/`Fulfillment`**: desenho da primeira
  iteração, descartado em 2026-09-09; os templates ficam como referência de
  forma.
- **Generator orientado ao domínio** (DSL, `--update`, inventário): sub-specs
  a/b/c, deferidas em 2026-09-09 em favor do harness.
- **Bloco `contract` além da fonte `.proto`**: `gen/go`, `buf generate`,
  baseline BUF-08 continuam do rito Buf.
- **Generator de composition root**: o binário com `--role api|relay|consumer`
  continua fora — `dmpf-reference` é o exemplo a copiar; `cmd/` não é gerado.
- **Executors Nx**: nenhum target novo precisa de executor; `nx:run-commands`
  cobre.
- **Portal do desenvolvedor**: FND-10 remete aos épicos de tooling.
