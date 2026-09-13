# dmpf

Baseline de monorepo Nx multistack — Express, Fastify, NestJS, Next.js e Angular no mesmo workspace.

## Pré-requisitos

- Node.js `24` (ver `.nvmrc` / `.node-version`)
- pnpm `11` (habilite via `corepack enable`)

## Quick Start

```bash
corepack enable          # ativa o pnpm na versão de packageManager
pnpm install             # instala deps e registra os git hooks (Lefthook)
pnpm nx run-many -t build # build incremental de tudo
```

As versões de dependências são centralizadas no `catalog:` do
`pnpm-workspace.yaml` — cada `package.json` referencia `"catalog:"`.

## Stack suportada

| Stack                     | Tipo        | Plugin Nx                     |
| ------------------------- | ----------- | ----------------------------- |
| Node.js / TypeScript puro | libs        | `@nx/js`                      |
| Express                   | apps        | `@nx/express` + `@nx/webpack` |
| Fastify                   | apps        | `@nx/node` + `@nx/webpack`    |
| NestJS                    | apps + libs | `@nx/nest` + `@nx/webpack`    |
| Next.js                   | apps + libs | `@nx/next`                    |
| Angular                   | apps + libs | `@nx/angular`                 |

## Estrutura de diretórios

```
dmpf/
├── apps/                        # Aplicações executáveis
│   └── <stack>-<name>/
│   └── <stack>-<name>-e2e/
├── libs/
│   ├── shared/                  # Libs agnósticas de framework (scope:shared)
│   │   ├── utils/               # Utilitários TypeScript puros
│   │   ├── types/               # Tipos e interfaces compartilhados
│   │   └── testing/             # Fixtures e helpers de teste
│   ├── backend/                 # Libs de servidor (scope:backend)
│   │   ├── domain/              # Lógica de domínio
│   │   ├── nest/                # Módulos NestJS reutilizáveis
│   │   └── infra/               # Adapters (DB, cache, queue)
│   ├── frontend/                # Libs de UI (scope:frontend)
│   │   ├── ui/                  # Componentes React/Next.js
│   │   └── angular/             # Componentes Angular
│   └── data-access/
│       └── api-client/          # Client HTTP compartilhado
├── tools/
│   ├── generators/              # Nx generators do workspace (vazio)
│   └── executors/               # Nx executors customizados
├── docs/
│   └── adr/                     # Architecture Decision Records
├── .github/workflows/ci.yml     # CI com nx affected
├── nx.json
├── tsconfig.base.json           # TypeScript Project References (nodenext)
├── pnpm-workspace.yaml          # apps/*, libs/**, tools/* + catalog de versões
├── biome.json                   # Biome (lint + formatação)
├── jest.config.ts               # Jest root
├── jest.preset.js               # Preset SWC
└── .spec.swcrc                  # Config SWC para testes (suporta decorators)
```

## Convenção de tags (obrigatória em todo project.json)

Cada projeto deve declarar uma tag de cada dimensão:

```json
{
  "tags": ["type:app", "scope:backend", "stack:nest"]
}
```

| Dimensão | Valores                                                 |
| -------- | ------------------------------------------------------- |
| `type:`  | `app`, `lib`, `e2e`                                     |
| `scope:` | `shared`, `backend`, `frontend`                         |
| `stack:` | `node`, `express`, `fastify`, `nest`, `next`, `angular` |

## Comandos principais

```bash
# Rodar testes de uma lib
pnpm nx test @mateusmacedo/minha-lib

# Rodar lint apenas nos projetos afetados pelo último commit
pnpm nx affected -t lint

# Typecheck incremental de tudo
pnpm nx run-many -t typecheck

# Build incremental de tudo
pnpm nx run-many -t build

# Ver grafo de dependências
pnpm nx graph

# Gerar nova shared lib
pnpm nx g @nx/js:lib libs/shared/minha-lib \
  --importPath=@mateusmacedo/minha-lib \
  --bundler=tsc --unitTestRunner=jest --linter=none \
  --tags=type:lib,scope:shared,stack:node
```

## Adicionar uma nova app

```bash
# Express
pnpm nx g @nx/express:app my-api --directory=apps/my-api

# NestJS
pnpm nx g @nx/nest:app my-nest-api --directory=apps/my-nest-api

# Next.js
pnpm nx g @nx/next:app my-web --directory=apps/my-web

# Angular
pnpm nx g @nx/angular:app my-angular --directory=apps/my-angular
```

Sempre adicione as tags obrigatórias no `project.json` gerado.

## CI/CD

O pipeline em `.github/workflows/ci.yml` executa sobre os projetos **afetados**
pela mudança (`nx affected`), na ordem:

1. `format:check` — Biome (`biome ci`)
2. `lint` — Biome
3. `typecheck` — TypeScript Project References
4. `test` — Jest + SWC
5. `build` — build incremental
6. `e2e` — testes E2E

## Testes

Os testes usam Jest com transform via SWC (`.spec.swcrc`).

```bash
# Testar uma lib específica
pnpm nx test @mateusmacedo/minha-lib

# Testar apenas os projetos afetados
pnpm nx affected -t test

# Testar tudo
pnpm nx run-many -t test
```

## Troubleshooting

- **`ERR_PNPM_IGNORED_BUILDS`**: um pacote com script de build não está em
  `allowBuilds` no `pnpm-workspace.yaml`. Adicione o pacote com valor `true`.
- **Versão de Node/pnpm incompatível**: rode `corepack enable` e use a versão
  do `.nvmrc` (`nvm use`). O `engines` do `package.json` exige Node `^24` / pnpm `^11`.
- **Cache desatualizado do Nx**: rode com `--skip-nx-cache` ou `pnpm nx reset`.
- **Erro de dependência no lockfile**: rode `pnpm install` e confira o diff de
  `pnpm-lock.yaml`; para CI use `pnpm install --frozen-lockfile`.

## Release do produto DMPF

O kernel DMPF é liberado como produto por uma tag anotada `dmpf@<semver>`, com
o BOM em `bom/dmpf/<semver>.json` e a evidência de execução que o certifica em
`bom/evidence/<semver>/`. O schema, o validador `dmpf-bom` e a geração da
evidência estão em [`bom/README.md`](bom/README.md); o rito da tag, em
[`CONTRIBUTING.md`](CONTRIBUTING.md).

## Decisões arquiteturais

Consulte `docs/adr/` para o registro de decisões.
ADR-001 cobre a escolha do baseline e os fundamentos técnicos.
