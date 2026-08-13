# ADR-004: Split version/publish e alvo Verdaccio

## Status

Aceito — 2026-07-20. Implementa SPEC-W4RWD02M.

> **Addendum (2026-07-30)** — parcialmente superseded pelo
> [ADR-005](./005-plataforma-gitea.md). A premissa de que o template estaria
> hospedado no GitHub é falsa: `git remote -v` aponta para
> `gitea.lidercap.com.br`. Ficam substituídos os itens deste ADR sobre host de
> hospedagem e uso do `gh` — em especial "API Gitea no create-release / Template
> hospedado no GitHub; manter `gh pr create`" (matriz abaixo), a ressalva de que
> o runner `gitea-runner` "não implica API Gitea nem troca de host GitHub" e o
> item "manter no template: `create-release.yml` via `gh`". O texto original é
> preservado como registro histórico. As decisões de split version/publish e de
> alvo Verdaccio permanecem válidas.

## Contexto

A SPEC-YWAWQHPX reativou CI/release no template, mas o `release.yml` ainda
versionava e publicava no mesmo job (GitHub Packages). O monorepo de referência
`lidercap-platform` separa versionamento (`nx release --skip-publish` + push de
tags) da publicação no Verdaccio (`publish-libs.yml` via reusable em
`actions-templates`). Esta evolução alinha o template a esse modelo de release,
adotando o label de runner da org e preservando pins Node/pnpm e `gh` do
template.

Fecha o follow-up de portabilidade de registry mencionado no ADR-003, no recorte
Verdaccio (não cobre migração da plataforma para Gitea).

## Decisão

- **`release.yml` version-only**: checkout `ref: master`,
  `pnpm nx release --skip-publish` (com `--first-release` quando não há tags
  `*@*`), `git push --follow-tags origin master`. Removido setup
  `npm.pkg.github.com` / `NODE_AUTH_TOKEN` do job de versionamento. Sem
  `continue-on-error` no versionamento (falha real não deve empurrar tags).
- **`publish-libs.yml`**: dispara em tags `**@*` e `workflow_dispatch`; chama
  `lidercap-apps/actions-templates/.github/workflows/publish-libs.yaml@main`
  com Node 24 / pnpm 11, `nx_exclude: @nx-base-template/source`,
  `scope: @lidercap-apps`, `build_projects_filter: tag:type:lib`.
- **Runner `gitea-runner`**: adoção deliberada da org (parity de label com
  lidercap). Não implica API Gitea nem troca de host GitHub.
- **Manter no template**: Docker GHCR + `release-type`, bot GitHub,
  `create-release.yml` via `gh`, pins 24/11, `changelog.createRelease: github`,
  `[skip ci]` no commit de release.
- **Sem `publishConfig` nas libs**: parity com lidercap; registry vem do auth
  do reusable. O escopo dos pacotes (`@lidercap-apps`) precisa bater com o
  `scope` passado ao reusable — o Nx resolve o registry pelo escopo do pacote,
  e divergir faz o publish cair no registry público.

### Matriz do que NÃO foi portado

| Item do lidercap | Motivo de não portar |
|------------------|----------------------|
| Node 22 / pnpm 9 | Template pinado em 24/11 (ADR-003) |
| API Gitea no create-release | Template hospedado no GitHub; manter `gh pr create` |
| `createRelease: false` no changelog | Template mantém releases GitHub (`github`) |
| Publish inline / loop `npm publish` | Preferir reusable Nx; criar/alterar reusable em `actions-templates` é follow-up se `@main` falhar |
| Auth/publish GitHub Packages | Substituído por Verdaccio |

## Consequências

- Versionamento fica idempotente em relação ao registry: falha de publish não
  impede tags/changelog.
- Consumidores do template precisam do reusable `publish-libs.yaml` acessível
  em `lidercap-apps/actions-templates@main` e da role/SSM de auth Verdaccio
  configurada na org.
- Docker release permanece no mesmo workflow de versionamento (vantagem do
  template ausente no lidercap).
- CI estrutural (`ci.yml`) e create-release não mudam nesta entrega.

## Referências

- Spec: `docs/specs/SPEC-W4RWD02M-evoluir-workflows-com-lidercap.md`
- ADR-003 (hardening / follow-up de registry)
- Referência: `lidercap-platform/.github/workflows/{release,publish-libs}.yml`
