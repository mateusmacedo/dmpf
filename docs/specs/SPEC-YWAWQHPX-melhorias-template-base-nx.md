---
id: SPEC-YWAWQHPX
slug: melhorias-template-base-nx
title: Melhorias no template base NX (tooling, docs, CI/CD, infra)
stage: done
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-07-19
---

# SPEC-YWAWQHPX: Melhorias no template base NX (tooling, docs, CI/CD, infra)

## Resumo

Aplicar 11 melhorias no `dmpf` para elevá-lo a um template de
monorepo pronto para produção, alinhado aos padrões do monorepo `jusdocs`.
Como mantenedor do template, quero tooling, documentação de comunidade,
CI/CD e infra padronizados para que todo projeto derivado herde uma base
consistente e completa.

## Contexto

- **Problema**: o template está incompleto. Falta automação de dependências
  (Renovate), centralização de versões (pnpm catalog), documentação de
  comunidade (CONTRIBUTING, LICENSE, CODEOWNERS, SECURITY, CODE_OF_CONDUCT),
  CI funcional (workflows estão desabilitados como `.bck`), infra Docker
  (plugin `@nx/docker` configurado sem nenhum Dockerfile) e pinagem de
  versão de Node/pnpm.
- **Impacto**: projetos criados a partir do template começam com governança,
  automação e docs prontas — reduz setup manual e drift entre repositórios.
- **Inspiração**: monorepo `jusdocs` (`/home/mateus/work/jusdocs/repositories/jusdocs`),
  fonte de dados declarada no doc de origem.
- **Links relevantes**:
  - `dmpf-improves.md` — doc de origem (11 melhorias); rascunho local, fora do versionamento e não citável como fonte
  - `docs/adr/001-baseline-monorepo.md` — baseline do monorepo
  - `docs/adr/002-nx-task-configuration.md` — config de tasks e cache
  - `docs/nx-reference/tasks.md` — referência de tasks
- **Referências externas**:
  - Contributor Covenant 2.1 — <https://www.contributor-covenant.org/version/2/1/code_of_conduct/>
  - Renovate `config:recommended` — <https://docs.renovatebot.com/presets-config/>
  - pnpm catalogs — <https://pnpm.io/catalogs>

> **Achado da pesquisa (importante)**: no repo fonte `jusdocs` os arquivos
> `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `LICENSE`, `CODEOWNERS` e
> `SECURITY.md` NÃO existem (repo privado). Essas melhorias (3-7) serão
> criadas do zero por boas práticas, não copiadas. Já Biome, Jest+SWC,
> Lefthook, tsconfig e nx.json JÁ existem no template — a Melhoria 1 e a 8
> são, em parte, enriquecimento e não criação.

<constraints>
- [P0] NUNCA quebrar `nx affected` / cache: todo arquivo global novo relevante
  para build deve entrar em `sharedGlobals` de `nx.json`
- [P0] NUNCA commitar segredos: `.gitignore` DEVE ignorar `.env*` antes de
  qualquer outra mudança
- [P0] Versões centralizadas via pnpm `catalog:` NÃO podem alterar as versões
  já resolvidas hoje (migração deve ser version-preserving)
- [P1] Preservar o que já funciona (Biome, Jest, Lefthook, tsconfig) —
  enriquecer sem regressão
- [P1] Workflows reativados DEVEM passar `actionlint` e não referenciar
  segredos/infra inexistentes no template
</constraints>

## Requisitos

### Funcionais

**Fase 1 — Config & tooling** (Melhoria 1 restante + 8)

- [ ] **[P0] `.gitignore` enriquecido**: adicionar `.env*`, `*.log`,
  `*.tsbuildinfo`, `.turbo/`, artefatos Go (`**/bin/`, `__debug_bin*`),
  Terraform (`.terraform/`, `*.tfstate*`) e `.next` — modelo do `jusdocs`,
  filtrado ao que o template suporta.
- [ ] **[P0] pnpm catalog**: criar bloco `catalog:` em `pnpm-workspace.yaml`
  centralizando as versões hoje fixas no `package.json` raiz; converter as
  entradas do `package.json` para `catalog:` (version-preserving).
- [ ] **[P1] Renovate**: criar `renovate.json` com `extends: [config:recommended,
  helpers:pinGitHubActionDigests]`, `baseBranchPatterns: [master]`, agrupamento
  de GitHub Actions com schedule semanal.
- [ ] **[P1] `packageManager` + `engines`**: declarar `packageManager: pnpm@<versão atual>`
  e `engines` (node, pnpm) no `package.json` raiz.
- [ ] **[P1] Pinagem de Node**: criar `.nvmrc` (e/ou `.node-version`) com a
  versão de Node alvo.
- [ ] **[P2] Hardening tsconfig/biome** (opcional): adotar flags mais estritas
  do `jusdocs` (`noUncheckedIndexedAccess`, `verbatimModuleSyntax`,
  `moduleDetection: force`) e grupos de `organizeImports` do Biome — somente
  se sem regressão nas libs existentes.

**Fase 2 — Docs de comunidade** (Melhorias 2-7)

- [ ] **[P1] README enriquecido**: adicionar seções Quick Start, Pré-requisitos
  (Node/pnpm), Testes e Troubleshooting às seções existentes.
- [ ] **[P1] `CONTRIBUTING.md`**: fluxo de contribuição, Conventional Commits,
  branching, como abrir PR, como rodar lint/test/build via `nx`.
- [ ] **[P2] `CODE_OF_CONDUCT.md`**: Contributor Covenant 2.1.
- [ ] **[P1] `LICENSE`**: arquivo MIT (coerente com `license: MIT` do package.json).
- [ ] **[P1] `CODEOWNERS`**: em `.github/CODEOWNERS`, com regra default e por área.
- [ ] **[P1] `SECURITY.md`**: política de divulgação responsável e contato.

**Fase 3 — CI/CD workflows** (Melhoria 10)

- [ ] **[P0] Reativar CI**: converter `ci.yml.bck` em `ci.yml` funcional
  (jobs: branch-policy/format/checks com `nx affected` lint+typecheck+test+build),
  usando a composite action `actions/setup-node-pnpm`.
- [ ] **[P1] Reativar release**: converter `release.yml.bck` e
  `create-release.yml.bck` (Nx release, conventional commits) adaptados ao
  `defaultBase: master`.
- [ ] **[P2] Workflows de suporte**: `renovate.yml` (self-hosted/dispatch) e
  `actionlint.yml`.

**Fase 4 — Infra Docker** (Melhoria 11)

- [ ] **[P1] `.dockerignore`**: modelo do `jusdocs` (git, node_modules, dist,
  .env, coverage, .nx, plans).
- [ ] **[P1] Dockerfile de exemplo**: um Dockerfile multi-stage de referência
  para app Node/Nest, consumível pelo plugin `@nx/docker`.
- [ ] **[P1] `docker-compose.yml`**: compose de dev local (ex: postgres, redis)
  em `infra/local/`, com profiles.

**Fase 5 — Estrutura de pastas & generators** (Melhorias 9 e 8)

- [ ] **[P1] Modelo de pastas**: materializar os diretórios documentados no
  README que faltam fisicamente (`libs/backend/`, `libs/frontend/`,
  `libs/data-access/`) com `.gitkeep`; alinhar `apps/` e `tools/executors/`.
- [ ] **[P2] nx.json generators defaults**: adicionar defaults de generators
  quando aplicável ao template.

### Não-funcionais

- [ ] Compatibilidade: pnpm `catalog:` requer pnpm 9+; declarar em `engines`.
- [ ] Segurança: nenhum segredo hardcoded em workflows/compose; `.env*` ignorado.
- [ ] Robustez: `nx affected -t lint typecheck test build` verde após cada fase.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Config raiz / tooling | [x] | .gitignore, pnpm-workspace, renovate, package.json, tsconfig, biome, .nvmrc |
| Documentação | [x] | README, CONTRIBUTING, CODE_OF_CONDUCT, LICENSE, CODEOWNERS, SECURITY |
| CI/CD (.github) | [x] | workflows ci/release/create-release/renovate/actionlint |
| Infra | [x] | Dockerfile, docker-compose, .dockerignore |
| Estrutura de projetos | [x] | libs/apps/tools placeholders + nx.json generators |

## Localização de código

```
dmpf/
  .gitignore                         — enriquecer
  pnpm-workspace.yaml                — adicionar catalog:
  package.json                       — packageManager, engines, deps -> catalog:
  renovate.json                      — criar
  .nvmrc / .node-version             — criar
  tsconfig.base.json / biome.json    — hardening opcional
  README.md                          — enriquecer
  CONTRIBUTING.md                    — criar
  CODE_OF_CONDUCT.md                 — criar
  LICENSE                            — criar
  SECURITY.md                        — criar
  .github/CODEOWNERS                 — criar
  .github/workflows/ci.yml           — reativar de .bck
  .github/workflows/release.yml      — reativar de .bck
  .github/workflows/create-release.yml — reativar de .bck
  .github/workflows/{renovate,actionlint}.yml — criar (opcional)
  .dockerignore                      — criar
  infra/local/docker-compose.yml     — criar
  <app-exemplo>/Dockerfile           — criar (referência)
  libs/{backend,frontend,data-access}/.gitkeep — criar
  nx.json                            — generators defaults (opcional)
```

**Arquivos a modificar**:

- `pnpm-workspace.yaml` — adicionar `catalog:` sem alterar versões efetivas
- `package.json` — trocar versões literais por `catalog:`, add packageManager/engines
- `nx.json` — incluir `renovate.json`, `.nvmrc` em `sharedGlobals` se relevante ao cache
- `.gitignore` — remover linha duplicada eventual e adicionar novas entradas
- `README.md` — anexar seções sem remover as existentes

## Design

### Arquitetura

Mudanças são majoritariamente de configuração e documentação, sem código de
runtime. O modelo de referência é o `jusdocs`, adaptado ao escopo do template
(multi-stack Node/React/Angular/Go, `defaultBase: master`, escopo `@mateusmacedo`).

### Fluxo principal (execução por fase)

1. Fase 1 (config) — base de tooling; valida `pnpm install` + `nx affected` verde.
2. Fase 2 (docs) — arquivos de comunidade; sem impacto em build.
3. Fase 3 (CI/CD) — reativa workflows; valida via `actionlint`.
4. Fase 4 (infra) — Docker de referência; valida `docker build` do exemplo.
5. Fase 5 (estrutura) — placeholders e generators defaults.

### Pseudocódigo (migração para catalog — version-preserving)

```
para cada dep em package.json (dev+prod):
    versao_atual = dep.version
    catalog[dep.name] = versao_atual      // preserva a versão hoje resolvida
    dep.version = "catalog:"
escrever catalog: em pnpm-workspace.yaml
rodar pnpm install e conferir lockfile sem downgrade de versões
```

## Decisões técnicas

- **LICENSE = MIT**: coerente com `license: "MIT"` já declarado no
  `package.json`. Alternativa descartada: Apache-2.0 (sem indício de preferência).
- **CODE_OF_CONDUCT = Contributor Covenant 2.1**: padrão de mercado, adotado
  por default. Confirmar contato de report na Fase 2.
- **baseBranch do Renovate/workflows = `master`**: `nx.json` usa
  `defaultBase: master`. Diferente do `jusdocs` (`main`/`develop`).
- **catalog version-preserving**: migrar sem bump para isolar a mudança de
  automação da mudança de versões. Alternativa descartada: adotar versões do
  `jusdocs` (Nx 22.7.2 etc.) — geraria upgrade acoplado e risco de regressão.
- **Docker de exemplo único**: um Dockerfile de referência (Node/Nest) em vez
  de um por stack — mantém o template enxuto; stacks adicionais ficam fora.

## Regras relacionadas

- `docs/adr/001-baseline-monorepo.md` — tagging 3D, SWC, nodenext
- `docs/adr/002-nx-task-configuration.md` — 3 camadas de config de tasks
- CLAUDE.md do projeto — usar `nx` para tasks, prefixar com `pnpm`

## Verificação e testes

### Critérios de aceite

- [x] `.gitignore` ignora `.env*` (verificável: `git check-ignore .env`)
- [x] `pnpm-workspace.yaml` tem `catalog:` e `package.json` usa `catalog:`;
  `pnpm install` mantém lockfile sem downgrade de versões
- [x] `renovate.json` válido (`npx --yes renovate-config-validator`)
- [x] `package.json` tem `packageManager` e `engines`; `.nvmrc` presente
- [x] README com seções Quick Start, Pré-requisitos, Testes, Troubleshooting
- [x] Existem CONTRIBUTING.md, CODE_OF_CONDUCT.md, LICENSE, SECURITY.md e
  `.github/CODEOWNERS`
- [ ] `.github/workflows/ci.yml` ativo e passa `actionlint`
- [ ] Existe `.dockerignore`, um Dockerfile de referência e `docker-compose.yml`;
  `docker build` do exemplo conclui
- [ ] Diretórios `libs/{backend,frontend,data-access}` existem
- [ ] `nx affected -t lint typecheck test build` verde ao final
- [ ] Validação do projeto passando (lint, typecheck, test)

### Cenários de teste (mínimo 3)

```
DADO o template após a Fase 1
QUANDO rodo `pnpm install` e `git check-ignore .env`
ENTÃO o lockfile não sofre downgrade E `.env` é ignorado

DADO o CI reativado (Fase 3)
QUANDO abro um PR contra master
ENTÃO os jobs de lint/typecheck/test/build rodam via nx affected e passam

DADO renovate.json recém-criado (Fase 1)
QUANDO rodo `npx renovate-config-validator`
ENTÃO a validação passa sem erro
```

<critical_constraints>

- [P0] `.gitignore` DEVE ignorar `.env*` — nenhum segredo commitável
- [P0] Migração para `catalog:` DEVE preservar versões efetivas (sem downgrade)
- [P0] Todo global novo que afeta build entra em `sharedGlobals` de `nx.json`
- [P1] Não regredir Biome/Jest/Lefthook/tsconfig existentes
- [P1] Workflows reativados passam `actionlint` sem segredos inexistentes
</critical_constraints>

## Escopo fora

- **Adoção das versões do jusdocs (Nx 22.7.2, React 19, Next 16, etc.)**:
  fora — a migração de catalog é version-preserving; upgrades ficam para spec
  futura de bump de dependências.
- **Dockerfile por stack (Go, Next, Angular)**: fora — apenas um exemplo de
  referência; stacks adicionais em specs próprias.
- **Módulos Terraform / infra cloud do jusdocs**: fora — o template cobre só
  infra local (docker-compose) e `.gitignore`/`.dockerignore` preparados.
- **commitlint / hook commit-msg**: fora — Lefthook já cobre pre-commit/pre-push;
  enforcement de Conventional Commits é melhoria separada.
- **Ativar `@nx/enforce-module-boundaries` via ESLint**: fora — incoerência
  documental (Biome é o linter real) tratada em spec própria.
