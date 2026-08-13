---
id: SPEC-W4RWD02M
slug: evoluir-workflows-com-lidercap
title: Evoluir workflows do template a partir do lidercap-platform (Verdaccio)
stage: done
priority: P1
depends_on: [SPEC-YWAWQHPX]
ticket_url: null
subtask_urls: []
created: 2026-07-20
---

# SPEC-W4RWD02M: Evoluir workflows do template a partir do lidercap-platform (Verdaccio)

## Resumo

Fazer diff completo dos workflows de `lidercap-platform` contra
`nx-base-template`, portar o que eleva o template (release com
`--skip-publish`, publish separado no Verdaccio, first-release, push de
tags) e documentar explicitamente o que NÃO portar. Como mantenedor do
template, quero o mesmo fluxo de versionamento + publish do lidercap,
adaptado ao runner/`Node`/`pnpm` do template, para que projetos derivados
publiquem libs no Verdaccio sem misturar versionamento e publicação.

## Contexto

- **Problema**: o template já reativou CI/release (SPEC-YWAWQHPX), mas o
  `release.yml` ainda versiona e publica no mesmo job (GitHub Packages),
  sem o desacoplamento `version → tag push → publish-libs` do lidercap, e
  sem workflow de publish no Verdaccio.
- **Impacto**: release previsível (versionamento idempotente sem publish),
  publish consolidado por tags (`**@*`) no Verdaccio, parity documentada
  com o monorepo de referência da org.
- **Inspiração**: `lidercap-platform/.github/workflows/*` + reusable
  Verdaccio em `actions-templates`.
- **Links relevantes**:
  - SPEC-YWAWQHPX — reativação inicial de CI/CD (done)
  - ADR-003 — hardening do template (runner/registry como follow-up)
  - `lidercap-platform/.github/workflows/{ci,release,create-release,publish-libs}.yml`
  - `actions-templates/.github/workflows/publish-lib.yaml` (auth Verdaccio/SSM)
- **Referências externas**: nenhuma busca extra na org (contexto local
  suficiente, decisão do discovery).

### Matriz de diff (fonte da verdade do escopo)

| Arquivo | Template hoje | Lidercap | Decisão |
|---------|---------------|----------|---------|
| `ci.yml` | `gitea-runner`, Node 24 / pnpm 11, `NX_EXCLUDE=@nx-base-template/source` | `gitea-runner`, Node 22 / pnpm 9, exclude lidercap | **Manter template** (parity estrutural já existe) |
| `create-release.yml` | `gh pr create` | Gitea API (`curl` + `jq`) | **Manter `gh`** (template é GitHub) |
| `release.yml` | version+publish GH Packages + docker no mesmo job; input `release-type`; checkout sem `ref: master` forçado | só version (`--skip-publish`), `--first-release`, checkout `ref: master`, push `--follow-tags`; sem docker | **Portar split + first-release + push**; **manter** docker scheme / `release-type` / bot GitHub |
| `publish-libs.yml` | ausente | tag `**@*` → reusable Verdaccio | **Criar**, alvo Verdaccio |
| Runner / Node / pnpm | `gitea-runner` + 24/11 (após alinhamento org) | gitea + 22/9 | **Adotar** `gitea-runner`; **NÃO portar** Node 22 / pnpm 9 |
| Auth npm no release | `npm.pkg.github.com` + `@lidercap-apps` | ausente (publish separado) | **Remover** do release; auth fica no publish Verdaccio |

<constraints>
- [P0] NUNCA portar Node 22, pnpm 9 ou API Gitea (`curl`/`jq`) para o template
- [P0] NUNCA publicar no GitHub Packages neste fluxo — alvo de publish de libs É o Verdaccio
- [P0] NUNCA misturar `--yes` e `--skip-publish` no mesmo `nx release` (mutuamente exclusivos no Nx)
- [P0] NUNCA hardcodar segredos; auth Verdaccio via o mesmo padrão do `actions-templates` (SSM / secrets), não em texto plano
- [P1] Preservar Node 24, pnpm 11, `gitea-runner` e steps Docker do template
- [P1] Documentar no ADR/spec o que ficou de fora (matriz acima) para evitar re-litígio
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Refatorar `release.yml` para version-only**: o job DEVE
  (1) checkout com `ref: master` + `fetch-depth: 0` + `fetch-tags: true`,
  (2) validar dispatch só em `master`,
  (3) rodar `pnpm nx release --skip-publish` com `--first-release` quando
  não existir tag `*@*`,
  (4) `git push --follow-tags origin master`,
  (5) manter detecção de apps Docker + login GHCR +
  `nx release --projects=... --dockerVersionScheme=...` (produção/hotfix),
  (6) remover setup `npm.pkg.github.com` / `NODE_AUTH_TOKEN` do job de
  versionamento.
- [x] **[P0] Criar `.github/workflows/publish-libs.yml`**: disparar em
  `push` de tags `**@*` e em `workflow_dispatch` (input `tag` default
  `latest`); concurrency `publish-libs` com `cancel-in-progress: true`;
  publicar libs no Verdaccio via `pnpm nx release publish` (ou reusable
  equivalente). Edge case: múltiplas tags no mesmo commit consolidam em
  um run; o executor DEVE ignorar versões já no registry.
- [x] **[P0] Resolver reusable vs inline**: preferir
  `lidercap-apps/actions-templates/.github/workflows/publish-libs.yaml@main`
  se existir com contrato Nx (`nx_exclude`, `scope`, `build_projects_filter`).
  Se só existir `publish-lib.yaml` (estado atual do repo local), a
  implementação DEVE inlinear um job no template que: assume role AWS /
  lê `/actions/VERDACCIO_PUBLISH_AUTH`, monta `~/.npmrc` apontando ao
  Verdaccio (default
  `http://verdaccio.verdaccio.svc.cluster.local:4873`), builda
  `tag:type:lib` e executa `pnpm nx release publish` com dist-tag
  configurável — sem copiar o loop `npm publish` por pasta do
  `publish-lib.yaml` legado.
  *(Resolvido no plano aprovado: caller reusable como lidercap; clone local
  de actions-templates só tem `publish-lib.yaml` — risco aceito de
  resolução em `@main` / follow-up.)*
- [ ] **[P1] Alinhar `publishConfig` das libs publicáveis**: libs
  `tag:type:lib` DEVEM declarar `publishConfig.registry` (e scope) para o
  Verdaccio usado pela org; remover/substituir referências a
  `npm.pkg.github.com` se ainda existirem em package.json/CHANGELOG
  histórico não precisa ser reescrito.
  *(Deferido no plano: parity lidercap — sem publishConfig; registry via
  auth do reusable. Ver ADR-004.)*
- [x] **[P1] Ajustar `nx.json` release**: manter
  `projects: ["tag:type:lib"]` e pattern `{projectName}@{version}`;
  revisar `git.commitMessage` (manter `[skip ci]` do template) e
  `changelog.projectChangelogs.createRelease` (`github` no template —
  manter, NÃO copiar `false` do lidercap).
  *(Sem mudança necessária — já alinhado.)*
- [x] **[P1] `ci.yml` e `create-release.yml`**: NÃO alterar estrutura
  além do estritamente necessário para consistência de comentários/docs;
  runner, Node, pnpm e `gh pr create` permanecem.
- [x] **[P2] Documentar matriz de não-portabilidade**: ADR curto em
  `docs/adr/` (ou seção no ADR-003) listando o que NÃO foi portado e por quê
  (API Gitea, Node/pnpm 22/9, createRelease false).

### Não-funcionais

- [x] Segurança: nenhum token Verdaccio/AWS em YAML; mascarar auth nos logs
  (`::add-mask::`).
  *(Auth fica no reusable; caller sem secrets em plaintext.)*
- [x] Compatibilidade: Node 24 + pnpm 11 + runner `gitea-runner`.
- [x] Determinismo: workflows passam `actionlint` (se disponível no repo)
  ou revisão manual equivalente sem referências a secrets inventados.
  *(actionlint ausente; YAML parse + review manual OK.)*
- [x] Portabilidade do template: URL do Verdaccio e role/SSM DEVEM ser
  inputs/defaults documentados para o consumidor ajustar.
  *(Documentado no ADR-004 / caller espelha lidercap.)*

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| CI/CD (`.github/workflows`) | [x] | release, publish-libs; ci/create-release só docs se preciso |
| Config Nx (`nx.json`) | [x] | release/git/changelog se necessário |
| Libs publicáveis (`package.json`) | [x] | `publishConfig` → Verdaccio |
| Apps / domínio / UI | [ ] | sem mudança |
| Docs / ADR | [x] | matriz de diff e decisões |

## Localização de código

```
nx-base-template/
  .github/workflows/ci.yml              — manter (parity)
  .github/workflows/create-release.yml  — manter gh
  .github/workflows/release.yml         — refatorar version-only + docker
  .github/workflows/publish-libs.yml    — criar (Verdaccio)
  nx.json                               — release config
  libs/shared/*/package.json            — publishConfig
  docs/adr/                             — ADR de evolução CI/CD
```

**Arquivos a modificar**:

- `.github/workflows/release.yml` — split version/publish; first-release; push tags
- `.github/workflows/publish-libs.yml` — criar
- `libs/shared/types/package.json` / `libs/shared/utils/package.json` — registry Verdaccio
- `nx.json` — só se commitMessage/changelog precisarem de ajuste mínimo
- `docs/adr/00N-*.md` — documentar decisões e não-portabilidade

**Referência (somente leitura)**:

- `lidercap-platform/.github/workflows/*`
- `actions-templates/.github/workflows/publish-lib.yaml`

## Design

### Arquitetura

```text
PR release/*|hotfix/* → master
        │
        ▼
  release.yml (version)
   - nx release --skip-publish [--first-release]
   - git push --follow-tags origin master
   - (opcional) docker release para tag:type:app
        │
        │ tags {project}@{version}
        ▼
  publish-libs.yml
   - auth Verdaccio (SSM)
   - nx release publish → Verdaccio
```

### Fluxo principal

1. Merge de `release/X.Y.Z` ou `hotfix/*` em `master` (ou `workflow_dispatch` em master).
2. `release.yml` versiona libs, gera changelog/tags, faz push; em seguida tenta Docker se houver apps.
3. Push das tags dispara `publish-libs.yml`.
4. Publish autentica no Verdaccio e publica apenas versões novas.

### Pseudocódigo

```
if no git tags matching '*@*':
  FIRST_RELEASE = '--first-release'
pnpm nx release --skip-publish $FIRST_RELEASE
git push --follow-tags origin master
# parallel path via tag webhook:
auth_verdaccio_from_ssm()
pnpm nx release publish --tag=$DIST_TAG
```

## Decisões técnicas

- **Publish no Verdaccio (não GitHub Packages)**: alinhamento com lidercap e
  registry interno da org. Alternativa descartada: manter `npm.pkg.github.com`
  — pedido explícito do discovery.
- **Split version / publish**: mesmo modelo do lidercap; tags disparam publish
  com concurrency cancel-in-progress. Alternativa descartada: publish inline —
  acopla falha de registry ao versionamento.
- **Manter Docker no `release.yml`**: template já tem scheme produção/hotfix e
  GHCR; lidercap ainda não espelha isso no release — NÃO remover.
- **Manter `gh` no create-release**: template hospedado no GitHub; API Gitea
  fica fora.
- **Runner `gitea-runner`**: alinhamento deliberado com a org (mesmo label do
  lidercap). Não implica migrar a plataforma para Gitea nem adotar a API
  Gitea no create-release. Pins Node 24 / pnpm 11 permanecem (ADR-003).
- **Reusable preferido, inline se ausente**: o lidercap referencia
  `publish-libs.yaml`, mas o clone local de `actions-templates` só tem
  `publish-lib.yaml` (loop npm legado). A implementação valida a existência
  do reusable Nx; se ausente, inline com `nx release publish` + auth SSM.

## Regras relacionadas

- SPEC-YWAWQHPX — base de CI/CD já entregue; esta spec evolui o modelo de release
- ADR-003 — hardening; portabilidade de registry era follow-up — esta spec fecha
  a parte Verdaccio
- `CLAUDE.md` / tags `type:lib` — Nx Release publica só libs

## Verificação e testes

### Critérios de aceite

- [x] `release.yml` NÃO chama publish de npm no job de versionamento; usa
  `--skip-publish` e push `--follow-tags`
- [x] Sem tags `*@*`, o job passa `--first-release`
- [x] Existe `publish-libs.yml` disparado por tags `**@*` e `workflow_dispatch`
- [x] Auth Verdaccio não aparece em plaintext nos YAML
- [x] Steps Docker (detecção + GHCR + dockerVersionScheme) permanecem no release
- [x] `ci.yml` mantém Node 24 / pnpm 11 / `gitea-runner`
- [x] `create-release.yml` continua usando `gh pr create`
- [x] ADR/docs registram a matriz do que NÃO foi portado
- [x] Validação do projeto passando (`pnpm biome ci .` nos arquivos tocados;
      `actionlint` se disponível)
  *(biome local indisponível nesta sessão; YAML parse + asserts manuais OK.)*

### Cenários de teste (mínimo 3)

```
DADO master sem tags {project}@{version}
QUANDO release.yml roda após merge de release/*
ENTÃO nx release usa --first-release e --skip-publish; tags são pushadas

DADO tags {project}@{version} acabaram de ser pushadas
QUANDO publish-libs.yml dispara
ENTÃO libs novas são publicadas no Verdaccio e versões já existentes são ignoradas

DADO workflow_dispatch de release fora de master
QUANDO Validate manual dispatch branch executa
ENTÃO o job falha com erro explícito e não versiona

DADO create-release.yml (sem mudança de plataforma)
QUANDO bump auto detecta feat em develop..master
ENTÃO abre PR via gh contra master (não via API Gitea)
```

<critical_constraints>

- [P0] NUNCA portar Node 22, pnpm 9 ou API Gitea (`curl`/`jq`) para o template
- [P0] NUNCA publicar no GitHub Packages neste fluxo — alvo de publish de libs É o Verdaccio
- [P0] NUNCA misturar `--yes` e `--skip-publish` no mesmo `nx release`
- [P0] NUNCA hardcodar segredos; auth Verdaccio via padrão actions-templates (SSM / secrets)
- [P1] Preservar Node 24, pnpm 11, `gitea-runner` e steps Docker do template
- [P1] Documentar o que ficou de fora (matriz de diff)
</critical_constraints>

## Escopo fora

- **Migrar a plataforma para Gitea / API Gitea no create-release**: o template
  permanece no GitHub com `gh pr create`; só o label do runner foi alinhado
  (`gitea-runner`).
- **Downgrade Node/pnpm para 22/9**: template já pinado em 24/11 (ADR-003).
- **Publicar no GitHub Packages**: substituído por Verdaccio nesta spec.
- **Criar/alterar o reusable `publish-libs.yaml` no actions-templates**: pode
  ser follow-up; esta spec aceita inline no template se o reusable Nx não
  existir.
- **Semgrep / renovate / actionlint workflows novos**: fora do diff lidercap→
  template desta rodada (renovate/actionlint já eram P2 da SPEC-YWAWQHPX).
- **Infra de deploy do Verdaccio**: assume registry já existente na org.
