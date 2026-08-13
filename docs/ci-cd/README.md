# CI/CD — guia de adoção

Este diretório **não** descreve um pipeline ativo. Ele descreve o modelo de CI/CD
adotado pela organização e o que você precisa preencher para ligá-lo no projeto
gerado a partir deste template.

Tudo entre `<` e `>` é placeholder: substitua pelos valores do seu projeto antes
de usar. Nenhum identificador real de infraestrutura (conta, path de parâmetro,
namespace, nome de secret) é versionado aqui — e não deve passar a ser.

## O que já vem pronto e o que falta

| Item | Estado no template |
| --- | --- |
| CI em PR (`ci.yml`) | **Ativo** — Nx affected de lint/typecheck/test/build/e2e |
| Versionamento e publish de libs | **Ativo** — `release.yml`, `publish-libs.yml`, `create-release.yml` |
| CD (`cd-dev-hmg.yml`) | **Template** — só `workflow_dispatch`, parametrizado por placeholder |
| Manifestos GitOps | **Ausentes** — vivem no repositório de infraestrutura, não aqui |
| Variáveis e secrets de deploy | **Ausentes** — configurados no repositório do projeto |

Separação rígida: **CI não faz deploy**; **CD não substitui o CI**.

## Modelo de referência

| Campo | Valor de referência |
| --- | --- |
| Organização dos manifestos CD | **Modelo A** — um workflow monorepo: `detect-apps` mais um job `deploy-<app>` por app, com `project`/`ecr` literais |
| Plataforma | EKS, com GitOps reconciliado por ArgoCD |
| Ambientes iniciais | `dev` e `hmg` |
| Nomenclatura de app | Sufixo terminal canônico `-bff` ou `-api` (nunca `-bff-api`) |
| Templates reutilizáveis | [actions-templates](https://gitea.lidercap.com.br/lidercap-apps/actions-templates) (`deploy-eks`, `detect-apps`, …) |

Modelos B/C, CD de produção e outras plataformas estão em
[EXPANSION_MODELS.md](./EXPANSION_MODELS.md) — consulte antes de proliferar
workflows.

## Fluxo resumido

```text
PR  →  .github/workflows/ci.yml          (Nx affected: lint/typecheck/test/build/e2e)
push develop / workflow_dispatch
    →  .github/workflows/cd-dev-hmg.yml  (Modelo A: detect → deploy-eks, um job por app)
        →  build/push da imagem no registry
        →  atualiza a tag da imagem no repositório de infraestrutura (Kustomize)
        →  ArgoCD reconcilia o cluster
```

## Documentos deste diretório

| Doc | Conteúdo |
| --- | --- |
| [DEPLOY_APPLICATIONS.md](./DEPLOY_APPLICATIONS.md) | Como habilitar o CD de uma app, mapa GitOps, secrets externos e troubleshooting |
| [EXPANSION_MODELS.md](./EXPANSION_MODELS.md) | Modelos de expansão (B, C, produção, ECS/on-prem, …) |

## Fontes externas

| Repo | URL |
| --- | --- |
| actions-templates | https://gitea.lidercap.com.br/lidercap-apps/actions-templates |
| lidercap-infra | https://gitea.lidercap.com.br/lidercap-apps/lidercap-infra |

Paths GitOps típicos, no repositório de infraestrutura:

```text
EKS/environment/<projeto>/<env>/<project>/
EKS/applications/<projeto>/<env>-<project>.yaml
```

## Gatilhos CD (Modelo A)

| Evento | Ambiente | Comportamento |
| --- | --- | --- |
| `push` em `develop` | default `dev` | Detecta apps alteradas (`apps/` + `libs/` → todas com Dockerfile) e faz deploy |
| `workflow_dispatch` | choice `dev` \| `hmg` | Opcional: input `app` (`apps/<app>`); vazio = detect |

No template, o gatilho de `push` vem **desligado**: o `cd-dev-hmg.yml` nasce com
apenas `workflow_dispatch`, para que nenhum projeto novo faça deploy acidental
antes de a infraestrutura existir.

## Permissões típicas do manifesto CD

```yaml
permissions:
  id-token: write
  contents: read
```

Secrets de pipeline: `secrets: inherit`. Variáveis de **runtime** do pod vêm de
ConfigMap e de secrets externos definidos no repositório de infraestrutura — ver
[DEPLOY_APPLICATIONS.md](./DEPLOY_APPLICATIONS.md#secrets-externos).

## Outros workflows do monorepo

| Workflow | Papel |
| --- | --- |
| `.github/workflows/ci.yml` | CI em PR |
| `.github/workflows/publish-libs.yml` | Publish de libs (Nx Release → Verdaccio) |
| `.github/workflows/release.yml` / `create-release.yml` | Versionamento e criação da branch de release |

Decisões relacionadas: [ADR-004](../adr/004-workflows-verdaccio-release.md) (split
version/publish) e [ADR-005](../adr/005-plataforma-gitea.md) (a plataforma é
Gitea).
