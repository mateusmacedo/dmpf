# ADR-001: Baseline do Monorepo Multistack

**Status:** Aceito
**Data:** 2026-04-16

## Contexto

Precisamos de um workspace Nx capaz de hospedar apps Express, Fastify, NestJS,
Next.js e Angular no mesmo repositório, compartilhando libs TypeScript puras,
libs NestJS, libs React/Next e libs Angular.

## Decisão

Criar um baseline a partir do preset `--preset=ts` (neutro), adicionando todos
os plugins necessários de forma controlada, em vez de herdar convenções de um
preset focado em framework (nest, next, angular).

### Fundamentos técnicos escolhidos

| Decisão                                           | Justificativa                                                                               |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `preset=ts` como ponto de partida                 | Menor acoplamento inicial; tsconfig correto gerado de início                                |
| `module: "nodenext"` no tsconfig.base             | ESM moderno, compatível com Node.js LTS                                                     |
| `composite: true` + `emitDeclarationOnly`         | TypeScript Project References — typecheck incremental                                       |
| `emitDecoratorMetadata` **fora** do tsconfig.base | Evita conflito com Angular compiler; cada app NestJS configura no próprio tsconfig.app.json |
| `customConditions` **fora** do tsconfig.base      | Evita conflito cross-stack; configurado por projeto quando necessário                       |
| pnpm-workspace com `apps/*`, `libs/**`, `tools/*` | Todos os pacotes são workspace packages reais (`workspace:^`)                               |
| `@nx/enforce-module-boundaries` com tagging 3D    | Governança por `type:`, `scope:`, `stack:`                                                  |
| SWC como transformador de testes                  | Velocidade superior ao ts-jest; suporte a decorators via `.spec.swcrc`                      |
| `project.json` externos (não inline)              | Melhor para generators automatizados e diffs legíveis                                       |
| `@nx/docker` no nx.json                           | Targets docker:build e docker:run disponíveis para qualquer app Node                        |

## Consequências

- Cada nova stack adicionada deve seguir o roadmap de fases documentado
- `emitDecoratorMetadata` e `experimentalDecorators` são responsabilidade de cada app NestJS
- Angular e Next.js entram na Fase 2 (após validação das stacks Node)
- Todos os projetos nascem com tags obrigatórias (type, scope, stack)
- ESLint com module boundaries é enforced em CI desde o primeiro commit

## Addendum — 2026-07-30

Este addendum **não altera** o status **Aceito** nem reescreve a decisão histórica
acima. Ele registra o estado real do workspace, para que o ADR-001 continue sendo
a referência de *baseline de plataforma* sem induzir o leitor a confundir roadmap
com realidade.

### O que permanece válido (baseline de plataforma)

- Ponto de partida `preset=ts`, `module: "nodenext"`, TypeScript Project
  References (`composite` + `emitDeclarationOnly`).
- pnpm workspace (`apps/*`, `libs/**`, `tools/*`) com pacotes reais.
- SWC para testes (`.spec.swcrc`), plugins Nx sob demanda, `@nx/docker`
  disponível no `nx.json`.
- Intenção multistack (Express, Fastify, NestJS, Next.js, Angular, Go) como
  **visão de plataforma** — plugins instalados no workspace não implicam apps
  dessas stacks já existentes.

### Estado atual do workspace

| Dimensão | Realidade hoje |
| -------- | -------------- |
| Runtime / package manager | Node `^24` (`engines`); `pnpm@11.14.0` via `packageManager`; Nx `23.1.0` |
| Apps executáveis | Nenhuma. `apps/{backend,frontend,serverless}` são destinos vazios |
| Libs | `@lidercap-apps/shared-types` e `@lidercap-apps/shared-utils`, ambas em `libs/shared/` |
| Lint / format | **Biome** (`biome ci` / `biome lint`) no CI e no lefthook — **não** ESLint/Prettier |
| Testes | Jest 30 + SWC, com `passWithNoTests` no preset |
| Tags 3D | `type:` / `scope:` / `stack:` presentes nas duas libs; `@nx-base-template/source` é exceção conhecida |
| Documentação de entrada | `README.md`, `CONTRIBUTING.md`, `docs/onboarding.md`, `AGENTS.md` |

Fonte de verdade de projetos: `pnpm nx show projects`. Pastas vazias sob `apps/` e
`libs/` são destinos de roadmap, não projetos Nx.

### Drift em relação ao texto histórico (consequências originais)

| Afirmação original (2026-04) | Situação hoje |
| ---------------------------- | ------------- |
| "ESLint com module boundaries é enforced em CI desde o primeiro commit" | **Não vigente.** Não existe config de ESLint nem dependência de ESLint no repositório; `targetDefaults.lint` executa `biome lint {projectRoot}`. |
| "`@nx/enforce-module-boundaries` com tagging 3D" (fundamento) | Parcial. As tags 3D **permanecem** como convenção de governança, mas o **enforcement** de boundaries não está no CI — é débito técnico explícito. |
| "Todos os projetos nascem com tags obrigatórias" | Meta mantida para apps/libs novos; `@nx-base-template/source` foge do padrão 3D completo. |
| Stacks Express/Fastify/Next/Angular como parte do baseline "suportado" | Continuam no **roadmap**. O que existe de concreto são as duas libs `shared`. |

### Débitos e próximos passos derivados deste ADR

1. **Reativar enforcement de boundaries** — via plugin ESLint do Nx ou gate
   equivalente compatível com o fluxo Biome. A decisão de tooling merece ADR
   próprio quando for atacada.
2. **Manter as tags 3D em todo projeto novo**, para que o `release.projects:
   tag:type:lib` continue selecionando corretamente.
3. **Materializar outras stacks** por fases documentadas, com geradores e tags
   nascendo no padrão 3D.

### Referências cruzadas

- [ADR-002](./002-nx-task-configuration.md) — tasks, cache, Biome em
  `targetDefaults`, tags.
- [ADR-003](./003-template-hardening.md) — hardening de tooling, docs, CI/CD e
  infra.
- [ADR-004](./004-workflows-verdaccio-release.md) — split version/publish e alvo
  Verdaccio.
- [ADR-005](./005-plataforma-gitea.md) — a plataforma de hospedagem é Gitea.
- `AGENTS.md`, `README.md`, `docs/onboarding.md`, `docs/nx-reference/tasks.md` —
  mapa operacional alinhado a este addendum.
