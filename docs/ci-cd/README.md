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
| Versionamento e publish de libs | **Ativo** — `nx-release.yml`, `nx-publish-libs.yml`, `create-release.yml` |
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
| Templates reutilizáveis | `detect-apps` e `publish-libs` internalizados em `.github/workflows/` (ADR-043); `deploy-eks` ainda referenciado do [actions-templates](https://github.com/mateusmacedo/actions-templates), a internalizar quando o CD for ligado |

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
| actions-templates | https://github.com/mateusmacedo/actions-templates |
| infra-reference | https://github.com/mateusmacedo/infra-reference |

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
| `.github/workflows/ci.yml` | CI em PR (Nx affected) |
| `.github/workflows/nx-release.yml` | Versionamento Nx Release (release groups `go-libs`, `go-tools`, `npm`); cunha as tags de módulo |
| `.github/workflows/nx-publish-libs.yml` | Publica libs npm no GitHub Packages (dispara no push de tag `**@*`, exceto `dmpf@*`), pelo reusable `publish-libs.yaml` |
| `.github/workflows/create-release.yml` | Cria a branch `release/<versão>` a partir de `develop` |
| `.github/workflows/dmpf-release.yml` | Cunha a tag do produto `dmpf@<release>` sob os gates de BOM e evidência (`workflow_dispatch`) |
| `.github/workflows/dmpf-verify.yml` | Gate de congruência horizontal do acervo `docs/dmpf/` |
| `.github/workflows/dmpf-evidence.yml` | Reproduz e compara a evidência publicada de uma release |
| `.github/workflows/dmpf-distributed.yml` | Camada distribuída da pirâmide de testes (build tag `distributed`) |

Decisões relacionadas: [ADR-004](../adr/004-workflows-verdaccio-release.md) (split
version/publish, parcialmente supersedido) e [ADR-043](../adr/043-migracao-para-github-licenca-e-autoria.md)
(a plataforma é GitHub, com registry, runner e reusables próprios — supersede
[ADR-005](../adr/005-plataforma-gitea.md)).
