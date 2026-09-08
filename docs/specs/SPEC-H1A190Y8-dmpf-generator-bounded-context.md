---
id: SPEC-H1A190Y8
slug: dmpf-generator-bounded-context
title: DMPF KRN-12.2 — Plugin local e generator bounded-context
stage: backlog
priority: P2
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-SJ66880S]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-546
subtask_urls: []
created: 2026-09-08
---

# SPEC-H1A190Y8: DMPF KRN-12.2 — Plugin local e generator bounded-context

## Resumo

Segunda sub-spec de `SPEC-8HWBWJCB` (`KRN-12`). Entrega o primeiro plugin local
Nx do workspace, `tools/dmpf-plugin`, com o generator `bounded-context`, que
scaffolda um bounded context conforme: cinco módulos Go na convenção de
`KRN-01`, as três tags 3D mais `layer:*`, as entradas no `go.work`, o manifesto
`dmpf/units@1` com `block` e `bounded_context` **declarados por opção**, e o
esqueleto de testes dos kits de `KRN-11`. A prova mecânica
`tools/dmpf-generator-check.sh` gera num worktree descartável, commita em dois
atos (classificação; código), roda a cadeia e o verificador com `--base`, e
reprova se um byte gerado mudar.

Como usuário de uma squad, quero rodar um comando e obter uma estrutura que o
verificador aprova sem eu tocar em nada além do rito de classificação.

## Contexto

- **Problema**: `tools/generators/` e `tools/executors/` são `.gitkeep`;
  `nx.json` só tem plugins npm; `@nx/plugin` e `@nx/devkit` não são
  dependências diretas. Criar um bounded context é copiar cinco `project.json`,
  cinco `go.mod`, cinco `dmpf-units.json` e editar o `go.work` à mão — com o
  risco de tag faltando ou unidade classificada por nome de diretório.
- **Impacto**: um comando produz a estrutura; o rito de classificação continua
  humano; o CI prova, a cada mudança do generator, que o artefato é aprovado.
- **Inspiração**: `docs/guides/dmpf-manifesto.md` (o gesto que o generator
  automatiza); `tools/dmpf-cell-check.sh` (prova em worktree descartável); os
  14 `project.json` existentes (o formato exato dos targets).
- **Links relevantes**:
  - `SPEC-8HWBWJCB` — "sem edição manual" definido; catálogo do que o generator
    não pode tocar
  - `SPEC-MQA5HAXF` — `KRN-01`, a convenção de módulo
  - `SPEC-WTAXFV8B` — `KRN-02`, o verificador e o baseline
  - `SPEC-SJ66880S` — `KRN-11`, os kits que o esqueleto de testes invoca
  - ADR-012, ADR-028, ADR-030, ADR-031, ADR-034
  - `nx_docs` "Local Generators" — `nx add @nx/plugin`, `nx g
    @nx/plugin:plugin tools/<nome>`, `nx g @nx/plugin:generator`

### Divergências entre o ticket e o repositório

| O ticket diz | O repositório tem | O que esta spec adota |
| --- | --- | --- |
| "Generator Nx em `tools/`" | nenhum plugin local; `@nx/plugin`/`@nx/devkit` ausentes do `package.json` raiz; `tsconfig.base.json` com `paths: {}` | Plugin criado por `nx g @nx/plugin:plugin tools/dmpf-plugin` após `pnpm add -D -w @nx/plugin@23.1.0 @nx/devkit@23.1.0` (entradas no `catalog:` **e** no `package.json` raiz; `pnpm-lock.yaml` muda) |
| "um módulo por bloco" | `.golangci.yml` seleciona por sufixo `*-domain`, `*-ports`, `*-application`, `*-contracts` | `<ctx>-{domain,ports,application,provider-postgres,app}`; `contract` recusado (fonte em `contracts/`) |
| "aprovado pelo verificador, sem edição manual" | sem `--base` → "não verificado" (`check.go:188`); autorização lida em commits `base..HEAD` (`check.go:206-214`); `DMPF-T002` se classificação e código no mesmo commit | A prova commita duas vezes no worktree e verifica com `--base <head-inicial>`; "sem edição" é `git diff --exit-code` depois da cadeia |
| — | `CI`, `DMPF_PG_DSN`, `DMPF_KAFKA_BROKERS` definidos no `env` do job (`ci.yml:26-36`); `tb.Env` falha sob `CI` sem infra (`tb.go:59-61`) | A prova tem duas fases no CI: estrutural (antes da infra: `fmt-check`, `vet`, `build`, `lint`, verificador) e `test-race` dos módulos gerados (depois de Postgres e Redpanda subirem) |
| `private: true` exclui do release | ADR-030: `private` exclui **publicação**, não versionamento; `release.projects` = `tag:type:lib` | O plugin é `type:lib` e **é versionado** pelo `nx release` (tem `package.json` real); não é publicado (`private`). Registrado no BOM pela sub-spec 4 |

### Fontes normativas

| Fonte | O que fixa |
| --- | --- |
| ADR-012 | `block` e `bounded_context` declarados, nunca inferidos |
| FND-10 §5.2 `AUT-01` A3; RFC §10.2 T4/T6; ADR-028 | Criar unidade é ato regulado; baseline em commit próprio; `DMPF-T001`/`T002` |
| ADR-031 | O gate não conserta o próprio insumo; `--write-baseline` é comando separado |
| ADR-030 | Todo módulo Go tem `package.json` `private`; `private` não exclui versionamento |
| ADR-034 | Sem `require` de irmão; resolução pelo `go.work` |
| `AGENTS.md` §Convenções | Tags 3D + `layer:*`; cinco targets Go via `nx:run-commands`; `lint` vem de `@nx-go/nx-go:lint` |
| FND-09 `KIT-*` (ADR-040) | Kits por camada; `t.Skip` nomeando variável fora de `CI` |
| `nx_docs` "Local Generators" | Estrutura do plugin; `schema.json`; `Tree`, `generateFiles`, `formatFiles` |

<constraints>
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório: os dois vêm de opção obrigatória e vão literalmente ao `dmpf-units.json` (ADR-012).
- [P0] NUNCA regravar `tools/dmpf-baseline/units-baseline.json` pelo generator: a prova o faz num worktree descartável, em commit próprio, e a squad o faz pelo rito (`AUT-01`, ADR-031).
- [P0] NUNCA produzir `project.json` com target que o plugin infere (`lint`) nem sem os cinco targets no formato dos módulos existentes (`AGENTS.md`).
- [P0] NUNCA deixar a prova passar sem `git diff --exit-code` sobre os arquivos gerados depois da cadeia: silêncio não é aprovação.
- [P1] Toda opção do generator é validada antes do primeiro `tree.write`; falha não deixa arquivo.
- [P1] A saída do generator termina com a instrução do `--write-baseline` e a frase "ato de classificação (AUT-01)"; a prova reprova se ela faltar.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Bootstrap do plugin**: `pnpm add -D -w @nx/plugin@23.1.0
  @nx/devkit@23.1.0` (versões no `catalog:` e referências `catalog:` no
  `package.json` raiz; `pnpm-lock.yaml` atualizado; `allowBuilds` inalterado),
  depois `pnpm nx g @nx/plugin:plugin tools/dmpf-plugin --name
  @lidercap-apps/dmpf-plugin --linter none --unitTestRunner jest`. Resultado:
  `tools/dmpf-plugin/{package.json, project.json, generators.json, tsconfig*.json,
  jest.config.ts, src/index.ts}`, tags `["type:lib", "scope:shared",
  "stack:node"]`, `private: true`, entrada em `tsconfig.base.json` `paths` e
  em `tsconfig.json` `references`. O plugin **é** versionado pelo `nx release`
  (`type:lib`); não é publicado.
  - Edge case: `pnpm nx run-many -t lint,typecheck,test,build
    --exclude=@nx-base-template/source` verde; `biome ci .` verde.
- [ ] **[P0] Generator `bounded-context`** (`pnpm nx g @nx/plugin:generator
  tools/dmpf-plugin/src/generators/bounded-context`), invocado por
  `pnpm nx g @lidercap-apps/dmpf-plugin:bounded-context <name>
  --bounded-context <ctx> [--blocks domain,port,application,provider,app]
  [--directory libs/backend/go] [--dry-run]`.
  - `schema.json`: `name` (posicional, `^[a-z][a-z0-9-]*$`, obrigatório),
    `boundedContext` (obrigatório, sem default, sem derivação de `name`),
    `blocks` (array, default os cinco; valor `contract` recusado),
    `directory` (default `libs/backend/go`), `dryRun`.
  - Validação antes de escrever: nome existente em disco ou no `go.work`
    aborta; `boundedContext` vazio aborta com "bounded_context é declarado,
    nunca inferido (ADR-012)"; `blocks` com `contract` aborta apontando
    `contracts/`.
  - Por bloco, `generateFiles` a partir de `files/<bloco>/`:

    | Sufixo | Bloco | `layer:*` | `external` | Esqueleto de teste |
    | --- | --- | --- | --- | --- |
    | `<ctx>-domain` | `domain` | `layer:domain` | `[]` | `domainkit.Run` sobre uma UPR placeholder (`placeholder_test.go`) |
    | `<ctx>-ports` | `port` | `layer:domain` | `[]` | compilação (`doc_test.go`) |
    | `<ctx>-application` | `application` | `layer:services` | `[]` | `serviceskit` com `Ledger` e `Decide` |
    | `<ctx>-provider-postgres` | `provider` | `layer:providers` | `pgx/v5` `io.storage` (copiado de `dmpf-provider-postgres`) | `providerkit.UnitOfWork` sob build tag `integration`, `tb.Env("DMPF_PG_DSN")` |
    | `<ctx>-app` | `app` | `layer:apps` | `[]` | `appkit`-style harness placeholder sob `integration` |

    Cada módulo recebe: `go.mod` (`module gitea.lidercap.com.br/lidercap-apps/lidercap-platform/<directory>/<ctx>-<sufixo>`,
    `go <versão lida do go.work>`, sem `require`), `project.json` (`name`
    `<ctx>-<sufixo>-go`, quatro tags, cinco targets `fmt-check`, `vet`,
    `build`, `test-race`, `govulncheck` com os mesmos comandos dos módulos
    existentes; `test-race` com `cache: false` só no `provider-postgres` e no
    `app`), `package.json` (`@lidercap-apps/<ctx>-<sufixo>-go`, `0.0.0`,
    `private`), `dmpf-units.json` (`schema: dmpf/units@1`; unidade
    `<ctx>/<sufixo>` com `block`, `bounded_context`, `include` = import path,
    `public_integration_surface: false`, `external`, `exceptions: []`),
    `README.md` com a tabela de unidades, `doc.go` com godoc de três linhas.
  - `go-work.ts` insere os `use` em ordem lexicográfica; `formatFiles`.
  - Saída final: os caminhos criados e a instrução "Unidades novas são ato de
    classificação (AUT-01). Regrave o baseline em commit próprio: `go run
    ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root .
    --write-baseline`".
  - Edge case: `--dry-run` lista e não escreve; duas execuções com as mesmas
    opções em `Tree`s limpos produzem bytes idênticos.
- [ ] **[P0] Prova mecânica `tools/dmpf-generator-check.sh`**, padrão de
  `dmpf-cell-check.sh`, com `--phase structural|integration|self-test`:
  1. `git worktree add` descartável em `develop`; `HEAD0=$(git rev-parse
     HEAD)`.
  2. Roda o generator para `genproof --bounded-context genproof`; captura a
     saída e reprova se a instrução do baseline faltar.
  3. Commit 1 (classificação): `dmpf-units.json` dos cinco módulos +
     `--write-baseline` → `chore(genproof): classificar unidades`.
  4. Commit 2 (código): todo o resto gerado + `go.work` → `feat(genproof):
     scaffold`.
  5. **Fase `structural`** (antes da infra no CI): `fmt-check`, `vet`,
     `build`, `lint` nos cinco projetos; `dmpf-conformance --root . --base
     $HEAD0`; `git diff --exit-code` (nada mudou desde o commit 2).
  6. **Fase `integration`** (depois da infra): `test-race` nos cinco; `git
     diff --exit-code` de novo.
  7. **Fase `self-test`** (vetores negativos, roda sempre): (a) edita um
     arquivo gerado depois do commit 2 → a prova reprova nomeando-o; (b)
     classificação e código no mesmo commit → verificador emite `DMPF-T002` e
     a prova reprova; (c) generator sem a instrução do baseline (simulado por
     variável) → reprova.
  8. Remove o worktree sempre (`trap`).
  - `ci.yml`: `structural` e `self-test` no bloco "Gates DMPF" (depois de
    `dmpf-cell-check.sh`); `integration` depois de Postgres e Redpanda, antes
    do estágio 3.
- [ ] **[P1] Guia de composição** `docs/guides/dmpf-composicao.md`: seis
  passos — gerar, classificar (baseline em commit próprio), escrever UPR e caso
  de uso, cabear o composition root copiando `dmpf-reference`, subir Postgres e
  Redpanda locais, rodar cadeia e verificador — cada um com a regra normativa e
  o arquivo que o exemplifica. Seção "Divergir do golden path" com ponteiro
  para a sub-spec 3. Entrada no índice de `docs/dmpf/README.md`.
- [ ] **[P1] Documentação**: `AGENTS.md` (seção Comandos ganha o generator e a
  prova; seção "Diretórios" descreve `tools/dmpf-plugin`);
  `docs/nx-reference/tasks.md` (generator); `README.md` do plugin; addendum no
  ADR-041 com as decisões desta spec.

### Não-funcionais

- [ ] Determinismo: mesmas opções → bytes idênticos; `formatFiles` é a única
  transformação após os templates.
- [ ] Feedback rápido: generator em menos de 5 s; fase `structural` em menos de
  3 min no runner.
- [ ] Conformidade: os cinco módulos gerados passam pelo verificador com
  `--base`; o próprio plugin não é unidade DMPF (TypeScript, fora do universo).
- [ ] Cadeia verde do workspace com o plugin incluído; `biome ci` passa.
- [ ] Dependências novas: só `@nx/plugin` e `@nx/devkit` (npm), na versão de
  `nx`.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | O que muda |
| --- | --- | --- |
| `domain` … `app` | [ ] | Nada no kernel; o generator produz módulos novos por bloco |
| Workspace | [x] | `tools/dmpf-plugin` (novo), `tools/dmpf-generator-check.sh` (novo), `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml`, `tsconfig.base.json`, `tsconfig.json`, `ci.yml`, docs |

## Localização de código

```text
tools/dmpf-plugin/                                    — NOVO plugin local (@lidercap-apps/dmpf-plugin); type:lib scope:shared stack:node
  package.json, project.json, generators.json, tsconfig.json, tsconfig.lib.json, tsconfig.spec.json, jest.config.ts, README.md
  src/index.ts
  src/generators/bounded-context/schema.json          — name, boundedContext, blocks, directory, dryRun
  src/generators/bounded-context/schema.d.ts
  src/generators/bounded-context/generator.ts         — validate → generateFiles por bloco → go.work → formatFiles → instrução do baseline
  src/generators/bounded-context/blocks.ts            — tabela sufixo × bloco × layer × external × template
  src/generators/bounded-context/go-work.ts           — parse/insert ordenado em use (...); leitura da versão go
  src/generators/bounded-context/generator.spec.ts    — Tree em memória: arquivos, tags, manifesto, go.work, recusas, dry-run, determinismo
  src/generators/bounded-context/files/{domain,ports,application,provider-postgres,app}/ — templates __tmpl__
tools/dmpf-generator-check.sh                         — NOVO: --phase structural|integration|self-test
package.json                                          — MODIFICAR: devDependencies @nx/plugin, @nx/devkit (catalog:)
pnpm-workspace.yaml                                   — MODIFICAR: catalog @nx/plugin, @nx/devkit 23.1.0
pnpm-lock.yaml                                        — MODIFICAR (pnpm install)
tsconfig.base.json, tsconfig.json                     — MODIFICAR: paths, references
.github/workflows/ci.yml                              — MODIFICAR: 3 passos do generator-check
docs/guides/dmpf-composicao.md                        — NOVO
docs/dmpf/README.md, docs/nx-reference/tasks.md, AGENTS.md — MODIFICAR
docs/adr/041-*.md                                     — MODIFICAR: addendum
```

## Design

### Arquitetura

```text
 pnpm nx g @lidercap-apps/dmpf-plugin:bounded-context billing --bounded-context billing
   │ validate(options)  ── abort sem escrever ──►  (nome existente | boundedContext vazio | contract)
   ▼
 blocks.ts ──► generateFiles(files/<bloco>) × 5 ──► go-work.ts (use ordenado) ──► formatFiles
   │
   └──► stdout: caminhos + "Regrave o baseline em commit próprio: … --write-baseline (AUT-01)"

 tools/dmpf-generator-check.sh (worktree descartável)
   HEAD0 ── generator ── commit1(manifestos+baseline) ── commit2(código+go.work)
         ── structural: fmt/vet/build/lint + dmpf-conformance --base HEAD0 + git diff --exit-code
         ── integration (após infra): test-race + git diff --exit-code
         ── self-test: (a) edição pós-commit2 → reprova; (b) T002 → reprova; (c) sem instrução → reprova
```

### Fluxo — do comando ao bounded context aprovado

1. A squad roda o generator; a validação aborta antes de escrever quando uma
   opção falha.
2. Cinco módulos e o `go.work` são escritos; a saída termina com a instrução do
   baseline.
3. A squad executa `--write-baseline` e commita `chore(workspace): registrar
   as unidades de billing no baseline` — só manifestos e baseline.
4. A squad commita o código gerado.
5. `dmpf-conformance --root . --base <antes>` aprova: a criação das unidades
   tem commit próprio (`T4`/`T6`), e nenhum byte gerado foi tocado.

### Onde cada regra é provada

| Regra | Instrumento | Onde |
| --- | --- | --- |
| ADR-012 | `generator.spec.ts`: recusa sem `boundedContext`; manifesto leva o valor da opção | plugin `test` |
| `AUT-01`, `DMPF-T001`/`T002` | prova faz dois commits; self-test (b) mistura e reprova | CI |
| "Sem edição manual" | `git diff --exit-code` após a cadeia; self-test (a) | CI |
| Tags 3D + `layer:*` | `generator.spec.ts` conta quatro tags por `project.json`; guarda de cobertura do `ci.yml` | plugin `test`; CI |
| Cinco targets, sem `lint` redeclarado | `generator.spec.ts` compara `targets` com o `project.json` de `dmpf-domain` | plugin `test` |
| ADR-034 (sem `require`) | `generator.spec.ts` lê o `go.mod` gerado | plugin `test` |
| `KIT-*` (`t.Skip` fora de `CI`) | fase `integration` roda os esqueletos com infra | CI |
| Determinismo | `generator.spec.ts` gera duas vezes e compara bytes | plugin `test` |

## Decisões técnicas

- **Plugin local via `@nx/plugin:plugin`** porque é o caminho documentado do
  Nx 23 e dá `schema.json`, `--dry-run` e `Tree`. Alternativa descartada:
  script Node em `tools/generators/`, porque não tem validação de schema nem
  dry-run.
- **Generator próprio, sem compor `@nx-go/nx-go:library`** porque o plugin
  infere targets e o workspace declara os cinco em `project.json`; o artefato
  divergiria dos 14 módulos. Alternativa descartada: compor e sobrescrever,
  porque redeclarar target inferido quebra o cache (ADR-002).
- **Cinco blocos, `contract` recusado** porque a fonte de contrato é
  `contracts/`, neutra de stack, com rito Buf.
- **Dois commits na prova** porque o verificador lê a autorização em
  `base..HEAD` e emite `T002` para classificação misturada com código; a prova
  reproduz o rito exato que a squad segue. Alternativa descartada: `--base`
  sobre a árvore suja, porque o verificador não olha alterações pendentes.
- **Duas fases no CI** porque o job define `CI` e as variáveis de infra antes
  de os containers subirem, e `tb.Env` falha em vez de pular; rodar `test-race`
  dos gerados antes da infra falharia sempre. Alternativa descartada: apagar
  `CI` na fase estrutural, porque transformaria ausência de exercício em passe.
- **`git diff --exit-code` após commit** porque `git status --porcelain` mostra
  `??` para arquivo não rastreado antes e depois de uma edição. Alternativa
  descartada: hash manual dos arquivos, porque o commit já é o snapshot.
- **O plugin é versionado** porque `release.projects` = `tag:type:lib` e
  `private` só exclui publicação (ADR-030). Alternativa descartada: excluí-lo de
  `release.projects`, porque introduziria a primeira exceção ao filtro por tag.

## Regras relacionadas

- `SPEC-8HWBWJCB` — "sem edição manual"; o generator nunca toca o baseline.
- `docs/guides/dmpf-manifesto.md` — o rito que a prova reproduz.
- `AGENTS.md` — formato dos targets; `nx-generate` skill para scaffolding.

## Verificação e testes

### Critérios de aceite

- [ ] Um bounded context gerado, sem edição manual, compila, passa na cadeia Go
  e é **aprovado** pelo verificador, com uma tag de cada dimensão por projeto e
  a unidade declarada no manifesto (critério 1 do ticket) — provado por
  `dmpf-generator-check.sh` no CI, com `--base` e dois commits.
- [ ] O generator aborta sem escrever quando `boundedContext` falta, `name`
  existe ou `blocks` inclui `contract`; `--dry-run` não escreve.
- [ ] Os três vetores de `self-test` reprovam.
- [ ] A fase `integration` roda depois da infra e passa.
- [ ] Duas execuções produzem bytes idênticos.
- [ ] `@nx/plugin` e `@nx/devkit` no `package.json` raiz via `catalog:`;
  lockfile atualizado; cadeia do workspace verde.
- [ ] Guia de composição criado e indexado; `AGENTS.md` e `tasks.md`
  atualizados; addendum no ADR-041.

### Cenários de teste

**Generator (`generator.spec.ts`)**

- Dado um `Tree` com o `go.work` atual, quando `name=billing
  --bounded-context=billing`, então cinco diretórios existem, cada
  `project.json` tem quatro tags e os cinco targets, cada `dmpf-units.json`
  tem `block` do bloco e `bounded_context: billing`, o `go.work` tem cinco
  `use` novos em ordem, e nenhum `go.mod` tem `require`.
- Dado o mesmo `Tree`, quando sem `--bounded-context`, então aborta com a
  mensagem da ADR-012 e o `Tree` está limpo.
- Dado `libs/backend/go/billing-domain` existente, quando `name=billing`,
  então aborta antes de escrever.
- Dado `--blocks=domain,contract`, então aborta nomeando `contracts/`.
- Dado `--dry-run`, então lista e não escreve.
- Dado duas execuções em `Tree`s limpos, então bytes idênticos.
- Dado a execução, então a saída contém "--write-baseline" e "AUT-01".

**Prova (`tools/dmpf-generator-check.sh`)**

- Dado worktree limpo, quando `--phase structural`, então `fmt-check`, `vet`,
  `build`, `lint` e `dmpf-conformance --base HEAD0` passam, e `git diff
  --exit-code` é 0.
- Dado a infra no ar, quando `--phase integration`, então `test-race` dos
  cinco passa.
- Dado `--phase self-test`, quando um arquivo gerado é editado após o commit
  2, então reprova nomeando o arquivo; quando classificação e código vão no
  mesmo commit, então `DMPF-T002` e reprova; quando a instrução falta, então
  reprova.

<critical_constraints>
- [P0] NUNCA inferir `block` ou `bounded_context` do nome do diretório: os dois vêm de opção obrigatória e vão literalmente ao `dmpf-units.json` (ADR-012).
- [P0] NUNCA regravar `tools/dmpf-baseline/units-baseline.json` pelo generator: a prova o faz num worktree descartável, em commit próprio, e a squad o faz pelo rito (`AUT-01`, ADR-031).
- [P0] NUNCA produzir `project.json` com target que o plugin infere (`lint`) nem sem os cinco targets no formato dos módulos existentes (`AGENTS.md`).
- [P0] NUNCA deixar a prova passar sem `git diff --exit-code` sobre os arquivos gerados depois da cadeia: silêncio não é aprovação.
- [P1] Toda opção do generator é validada antes do primeiro `tree.write`; falha não deixa arquivo.
- [P1] A saída do generator termina com a instrução do `--write-baseline` e a frase "ato de classificação (AUT-01)"; a prova reprova se ela faltar.
</critical_constraints>

## Escopo fora

- **Bloco `contract` no generator**: `contracts/` com rito Buf.
- **Generator de app/composition root**: `dmpf-reference` é o exemplo a
  copiar; gerar apps é matéria do golden path.
- **Executors Nx**: nenhum target novo precisa de executor; `nx:run-commands`
  cobre.
- **Portal do desenvolvedor**: FND-10 remete aos épicos de tooling.
