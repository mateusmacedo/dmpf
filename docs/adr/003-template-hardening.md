# ADR-003: Hardening do template (tooling, docs, CI/CD, infra)

## Status

Aceito — 2026-07-19. Implementa SPEC-YWAWQHPX.

## Contexto

O template base carecia de automação de dependências, centralização de versões,
documentação de comunidade, CI ativo e infra Docker. Os workflows de CI/release
estavam desabilitados (`.bck`) e não havia pinagem de Node/pnpm. O objetivo foi
elevar o template a uma base pronta para produção, alinhada aos padrões do
monorepo de referência `jusdocs`.

## Decisão

- **Versões centralizadas via pnpm `catalog:`** no `pnpm-workspace.yaml`. A
  migração foi *version-preserving* (as 37 versões literais do `package.json`
  foram movidas para o catalog sem alteração), isolando a mudança de automação
  de qualquer upgrade.
- **Pinagem Node 24 + pnpm 11**: `packageManager: pnpm@11.14.0`,
  `engines { node: ^24, pnpm: ^11 }`, `.nvmrc`/`.node-version` = `24`.
  O pnpm 11 (`strictDepBuilds`) exigiu declarar `@swc/core`, `lefthook` e `nx`
  em `allowBuilds`.
- **Renovate** (`renovate.json`) com `config:recommended` +
  `helpers:pinGitHubActionDigests`, base `master`.
- **Biome migrado para o formato 2.x** (`assist.actions.source.organizeImports`,
  `files.includes`) — a config estava no formato 1.9 inválido para o CLI 2.4.16.
- **TypeScript**: `ignoreDeprecations: "6.0"` no `tsconfig.base.json` para o
  `baseUrl` (deprecado em TS 6.0/7.0), destravando emit e typecheck.
- **Docs de comunidade** criadas do zero (não existem no `jusdocs`, repo privado):
  `LICENSE` (MIT), `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant
  2.1), `SECURITY.md`, `.github/CODEOWNERS` — com placeholders para o consumidor.
- **CI/CD**: workflows `.bck` reativados ajustando apenas os pins de Node/pnpm,
  mantendo runner e organização originais (portabilidade fica como follow-up).
- **Infra Docker**: `.dockerignore`, um `Dockerfile` de referência e um
  `docker-compose.yml` de dev local (Postgres + Redis, por profile).

## Consequências

- Dependências passam a ser atualizadas automaticamente (Renovate) e versionadas
  em um único ponto (catalog), reduzindo drift entre projetos.
- `pnpm-workspace.yaml` e `pnpm-lock.yaml` entraram em `sharedGlobals` do
  `nx.json`: mudanças de dependências invalidam o cache de todos os projetos —
  comportamento correto, com custo de cache explícito.
- Os workflows reativados assumem runner self-hosted e organização específica;
  consumidores do template precisam ajustar runner, registry e branch model.
- Itens deixados como follow-up (escoteiros): hardening de flags do tsconfig/biome,
  portabilidade dos workflows para runners hospedados, consistência do campo
  `private` do generator `shared-lib` vs libs publicáveis, smoke test multi-stack,
  e remoção dos `.npmrc`/`package-lock.json` órfãos em `libs/shared/*`.

## Referências

- Spec: `docs/specs/SPEC-YWAWQHPX-melhorias-template-base-nx.md`
- ADR-001 (baseline) e ADR-002 (configuração de tasks)
