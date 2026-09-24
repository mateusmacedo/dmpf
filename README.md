# dmpf

Baseline de monorepo Nx multistack da organização — Go e TypeScript no mesmo
workspace, com pnpm e Nx Release por projeto. O workspace já vem preparado
para Express, Fastify, NestJS, Next.js e Angular, mas as únicas apps e libs
reais hoje são as do kernel DMPF, em Go: a topologia de referência e o golden
do harness de bounded contexts. Quem parte deste template cria as apps de
outra stack do zero.

Para as convenções completas do workspace — estrutura, comandos, tags,
git-flow, padrões de código — ver [`AGENTS.md`](AGENTS.md), a fonte canônica
para quem (humano ou agente) trabalha neste repositório.

## Stack

| Camada | Tecnologia |
| --- | --- |
| Orquestração | Nx `23.1.0`, workspace pnpm |
| Kernel DMPF | Go `1.26.6` (`libs/backend/go/`, `apps/backend/`, `tools/dmpf-conformance/`) |
| Tooling do workspace | TypeScript (`tools/dmpf-plugin`, scripts) |
| Runtime | Node.js `^24`, pnpm `11.14.0` |
| Lint/format | Biome `2.4.16` |
| Testes TS | Jest `30` + SWC |

## Setup

```bash
corepack enable           # ativa o pnpm na versão de packageManager
pnpm install               # instala deps e registra os git hooks (Lefthook)
pnpm nx run-many -t build  # build incremental de tudo
```

As versões de dependências TS são centralizadas no `catalog:` do
`pnpm-workspace.yaml` — cada `package.json` referencia `"catalog:"`. Passo a
passo completo, incluindo infraestrutura local (Postgres, Redpanda) e o
primeiro PR, em [`docs/onboarding.md`](docs/onboarding.md).

## Estrutura em alto nível

```text
dmpf/
├── apps/backend/          # BFF REST e os bounded contexts Go (bff, orders, reservations, bookings)
├── libs/backend/go/       # Kernel DMPF de reuso — módulos Go (domain, ports, application, providers, testkit, ...)
├── contracts/             # Fonte dos contratos Protobuf (Buf), neutra de stack
├── bom/                   # BOM da release do produto e a evidência que o certifica
├── infra/                 # Compose local, observabilidade, manifestos Kustomize
├── tools/                 # dmpf-plugin (generator), dmpf-conformance (verificador), scripts do workspace
├── docs/                  # ADRs, specs, regras de domínio, guias
└── .claude/, .agents/     # skills e agents para assistentes
```

Detalhe módulo a módulo no README de cada lib/app (`libs/backend/go/<módulo>/README.md`,
`apps/backend/<app>/README.md`), infraestrutura em [`infra/README.md`](infra/README.md),
contratos em [`contracts/README.md`](contracts/README.md) e o BOM da release em
[`bom/README.md`](bom/README.md).

## Comandos essenciais

```bash
# Em lote / afetados pela mudança
pnpm nx run-many -t build
pnpm nx affected -t lint,typecheck,test,build

# Um único projeto
pnpm nx test <projeto>
pnpm nx build <projeto>

# Grafo de dependências
pnpm nx graph

# Formatação (Biome — não Prettier)
pnpm biome format --write .
pnpm biome ci .
```

Guia prático de tasks Nx — incluindo a taxonomia completa de tags (`type:`,
`scope:`, `stack:`, `layer:`) — em
[`docs/nx-reference/tasks.md`](docs/nx-reference/tasks.md).

## Git e release

Fluxo git-flow (`master` / `develop` / `release/*`), convenção de commits e o
rito de release estão em [`CONTRIBUTING.md`](CONTRIBUTING.md).

## Troubleshooting

- **`ERR_PNPM_IGNORED_BUILDS`**: um pacote com script de build não está em
  `allowBuilds` no `pnpm-workspace.yaml`. Adicione o pacote com valor `true`.
- **Versão de Node/pnpm incompatível**: rode `corepack enable` e use a versão
  do `.nvmrc` (`nvm use`). O `engines` do `package.json` exige Node `^24` / pnpm `^11`.
- **Cache desatualizado do Nx**: rode com `--skip-nx-cache` ou `pnpm nx reset`.
- **Erro de dependência no lockfile**: rode `pnpm install` e confira o diff de
  `pnpm-lock.yaml`; para CI use `pnpm install --frozen-lockfile`.

## Documentação

| Onde | O quê |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | Convenções completas do workspace |
| [`docs/adr/`](docs/adr/README.md) | Decisões arquiteturais |
| [`docs/specs/`](docs/specs/README.md) | Especificações de features |
| [`docs/rules/`](docs/rules/README.md) | Regras de domínio (negócio, aplicação, produto) |
| [`docs/guides/development-workflow.md`](docs/guides/development-workflow.md) | Guia detalhado do fluxo de trabalho |
| [`docs/ci-cd/`](docs/ci-cd/README.md) | Adoção de CI/CD e deploy |
| [`docs/onboarding.md`](docs/onboarding.md) | Setup local e primeiro PR |
