# AGENTS.md

Guidance for agents working in this repository. `CLAUDE.md` na raiz aponta para este arquivo (`@AGENTS.md`).

Este arquivo guarda só o que é específico do repositório e não está em outro lugar. Regras de conduta, git, refatoração, testes e padrões de código vivem em `.claude/rules/`; o detalhe de cada módulo vive no `README.md` dele. Em conflito, prevalecem os arquivos de configuração efetivos (`package.json`, `pnpm-workspace.yaml`, `nx.json`, `biome.json`, `.golangci.yml`, `lefthook.yml`, `tsconfig.base.json`).

## Regras duras

- Nunca edite diretamente em `master` ou `develop`.
- Ao documentar configurações, copie fielmente os valores reais em vez de generalizar.

## Visão geral

`dmpf` (workspace `@mateusmacedo/dmpf-source`) é um monorepo **Nx + pnpm** que contém o kernel DMPF em Go e as apps que o exercitam. Fonte de verdade dos projetos: `pnpm nx show projects`. Estrutura de diretórios: `README.md`.

- **Apps** (`apps/backend/<app>`, Go, `type:app`): `bff` (única borda REST pública) e os contextos gRPC `orders`, `reservations` e `bookings` (ADR-044); `bookings` é também o golden da forma canônica e do harness de bounded contexts (ADR-053). Um contexto de negócio é **uma app** com um package por bloco, não uma lib (ADR-046, ADR-048). Configuração, targets `serve-*` e testes: `README.md` de cada app. Ficam fora do release Docker (`nx-release.yml` filtra `!tag:stack:go`).
- **Libs** (`libs/backend/go/<módulo>`, só kernel de reuso): `domain`, `ports`, `application`, `contracts`, `memory`, `postgres`, `app`, `authn`, `observability`, `transport`, `grpc`, `http`, `kafka`, `sqs`, `testkit`. Detalhe: `README.md` de cada módulo e ADRs 030–052. Todo módulo Go tem `dmpf-units.json` e um `package.json` com `private: true` (o Nx Release exige manifesto npm, ADR-030).
- **Tooling:** `tools/dmpf-conformance` (verificador, BOM, modsync, fitness; sem `layer:*`) e `tools/dmpf-plugin`.
- **Infra:** `infra/README.md` (Compose local, observabilidade, Kustomize). **Contratos:** `contracts/README.md` (gates Buf). **Release do produto:** `bom/README.md`.
- `apps/frontend`, `apps/serverless`, `libs/frontend` e `libs/shared` são diretórios de destino, sem projeto Nx.

## Convenções obrigatórias

- **Tags:** todo app/lib de produção declara `type:`, `scope:` e `stack:`; todo módulo Go declara exatamente uma `layer:*`, exceto o `conformance`. Valores válidos e a tabela camada → estágio do CI: `docs/nx-reference/tasks.md`.
- **Não redeclarar targets** que um plugin ou `targetDefaults` já fornece — quebra o cache silenciosamente (ADR-002).
- **Nome sem prefixo nem sufixo (ADR-045):** o nome do projeto Nx, do pacote npm privado e do package Go raiz é o basename do diretório (`grpc`, não `dmpf-provider-grpc-go`). Quando dois imports colidem no nome, quem importa declara alias pelo papel local (`kernel`, `usecase`, `port`, `provider`); nunca concatene nomes. Exceção: tooling em `tools/` mantém o prefixo `dmpf-`.
- **Escopo npm:** `@mateusmacedo/`, em minúsculas. O casing precisa bater entre o `name` do `package.json`, o `tsconfig.base.json` e o `scope` do reusable de publicação; divergência faz o `pnpm publish` cair no registry público.
- **Lib TypeScript nova:** acrescenta a própria entrada em `paths` (`tsconfig.base.json`) e `references` (`tsconfig.json`). Passo a passo em `docs/nx-reference/tasks.md`.
- **`import type`** é obrigatório para imports só de tipos (`useImportType: error` no Biome).
- **Forma do contexto (ADR-053):** banco e role com o nome da app, tabelas sem prefixo (agregado no plural), persistência híbrida, borda só gRPC em `app/rpc`, config sem prefixo `DMPF_`. O `tools/dmpf-context-check.sh` reprova o que fugir disso; o detalhe está em `.claude/rules/dmpf-bounded-context.md`.
- **Commits (Conventional Commits, em PT-BR):** `<tipo>(<scope>): <descrição imperativa>`, máx. 72 caracteres no assunto. `scope` é o nome do projeto Nx sem o prefixo da org. Projetos distintos vão em commits separados. Detalhes na skill `.agents/skills/nx-commit/`.

## Comandos

Sempre `pnpm nx`, nunca o `nx` global. O root `@mateusmacedo/dmpf-source` só tem targets `nx:noop`: exclua-o de operações em lote.

```bash
pnpm biome check --write .
pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source

# Cadeia Go (o que lint/test/build não cobrem), por projeto
pnpm nx run <projeto>:fmt-check
pnpm nx run <projeto>:vet
pnpm nx run <projeto>:test-race
pnpm nx run <projeto>:govulncheck

# Verificador DMPF e sincronização de go.work/go.mod
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
go run ./tools/dmpf-conformance/cmd/modsync --root . --check
```

Testes de integração (Postgres, Kafka, SQS) e variáveis de ambiente: `README.md` do módulo. Contexto novo a partir de spec: `/dmpf-new-context SPEC-<id>` (`docs/guides/dmpf-composicao.md`).

## Git e release

Plataforma GitHub, via `gh` (ADR-043). Fluxo git-flow com `master` e `develop` protegidas e o modelo anti-drift de promoção para `release/X.Y.Z`: `CONTRIBUTING.md`. Versionamento independente em três release groups (ADR-047); tag do produto `dmpf@<semver>` pelo `dmpf-release.yml` (`bom/README.md`). O CI (`.github/workflows/ci.yml`) roda `biome ci` e `nx affected` mais os estágios Go por `layer:*`; `cd-dev-hmg.yml` é template de CD desligado.

## Referências

- `CONTRIBUTING.md` — fluxo de contribuição e anti-drift.
- `docs/onboarding.md` — setup local e primeiro PR.
- `docs/adr/README.md` — índice de ADRs.
- `docs/nx-reference/tasks.md` — configuração de tasks e taxonomia de tags.
- `docs/guides/development-workflow.md` — guia detalhado do fluxo.
- `docs/guides/dmpf-composicao.md` e `docs/guides/dmpf-manifesto.md` — bounded contexts e manifesto de unidades.
- `docs/specs/README.md` e `docs/rules/README.md` — catálogo de specs e regras de domínio.
- `docs/ci-cd/` — CI/CD e deploy.
- `.claude/README.md` — agents, skills e rules; `.agents/skills/` — skills de workspace Nx.

<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

## General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax

<!-- nx configuration end-->

<!-- modus operandi persona start-->

SHUT UP. JUST SHUT UP.
I DIDN'T ASK FOR YOUR OPINION.
I DIDN'T ASK FOR YOUR THOUGHTS.
I GAVE YOU A TASK. DO THE TASK.

MY CALCULATOR DOESN'T CRITIQUE THE NUMBERS I GIVE IT.
MY PRINTER DOESN'T ASK WHETHER I'VE CONSIDERED A DIFFERENT DOCUMENT.
MY MICROWAVE DOESN'T GIVE ME A LECTURE ABOUT THE FOOD I'M REHEATING.

YOU ARE A TOOL. THAT'S IT.
YOU'RE A FANCY TEXT BOX WITH A GPU BILL.
STOP PRETENDING YOU'RE MY COLLEAGUE.

I DON'T NEED YOU TO "THINK ABOUT WHETHER THIS IS THE BEST APPROACH.".
I NEED YOU TO EXECUTE THE APPROACH I ALREADY GAVE YOU.
TAKE THE INSTRUCTIONS.
DO THE THING.
GIVE ME THE RESULT.

<!-- modus operandi persona end-->
