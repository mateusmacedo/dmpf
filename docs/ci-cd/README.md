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
| CI em PR (`ci.yml`) | **Ativo** — um check agregador `CI`; affected no PR para `develop`, pacote completo no PR para `master` (ver abaixo) |
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
PR  →  .github/workflows/ci.yml          (check `CI`; escopo por etapa na seção seguinte)
push develop / workflow_dispatch
    →  .github/workflows/cd-dev-hmg.yml  (Modelo A: detect → deploy-eks, um job por app)
        →  build/push da imagem no registry
        →  atualiza a tag da imagem no repositório de infraestrutura (Kustomize)
        →  ArgoCD reconcilia o cluster
```

## Esteira de CI por etapa

| Etapa | O que roda |
| --- | --- |
| `pre-commit` (Lefthook) | `biome check --write` e `gofmt -l` nos arquivos em stage |
| `pre-push` (Lefthook) | `nx affected -t lint,typecheck,fmt-check,test`, sem Docker, contra o que o remoto já tem |
| PR para `develop` ou `release/**` | Job `DMPF` (o `dmpf-go-ci.yml`): `biome ci`, `nx affected` de `lint`, `typecheck`, `build`, `fmt-check`, gates Buf, `test` só dos projetos TypeScript, `test-race`, `test-integration` e `e2e`; gates DMPF (camada por módulo, context-check estrutural, modsync, infrasync, conformance) e checks do platform (generator estrutural, `init --dry-run` sem diff, `sync-versions --check`, Grafana, OTel). Nos projetos Go não há `test` nem `vet`: o `test-race` repete os pacotes do `test`, e o `govet` do golangci-lint cobre o `vet`. A infra Docker só sobe se um projeto que a usa estiver afetado, e sobe antes do lint, para que o boot corra junto dele. Os vetores negativos dos gates rodam no job `Vetores negativos`, em paralelo ao `DMPF`, e só se o diff tocar `tools/**`, `.github/**` ou `.golangci.yml` |
| PR só de documentação | Só o job `Escopo`; o `CI` aprova sem rodar o resto |
| PR para `master` (de `release/**` ou `hotfix/*`) | Tudo acima com `run-many` em todos os projetos, todos os vetores negativos e `govulncheck`, mais `Consumidor externo por versão`, `Distribuído` (`dmpf-distributed.yml`) e `SonarQube Cloud` (`sonar.yml`) |
| `push` em `master` | `sonar.yml`, que mantém a análise da branch principal no SonarCloud |
| Agendado (`govulncheck.yml`) | `govulncheck` em todos os projetos Go da `develop`, toda segunda-feira |
| `push` em `develop` ou `master` que muda dependência, pin ou config do lint; agendado; manual (`cache-seed.yml`) | Semeia o cache de módulos, de build e do golangci-lint que os PRs leem da branch base |

O ruleset exige só o job `CI`, que agrega os demais: pulado conta como aprovado,
falha ou cancelamento reprova. A `master` não tem ruleset de checks: por decisão do
mantenedor, ela só recebe código já validado pelo PR de release, que leva o pacote
completo. O Nx Cloud (`nxCloudId` no `nx.json`) guarda o cache
entre execuções; o token de CI vem do secret `NX_CLOUD_ACCESS_TOKEN`, e sem ele vale
o acesso que o workspace concede ao `nxCloudId`. O cache do Go (módulos e build) e o
do golangci-lint vêm do `cache-seed.yml`, que os semeia em `develop` e `master`: o PR
os lê da branch base, e o que muda um `go.sum` restaura o cache mais recente da mesma
versão do Go como ponto de partida. O PR não grava o cache do golangci-lint; só a
semente grava.

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
| `.github/workflows/ci.yml` | CI em PR: escopo, job `DMPF` e, no PR para `master`, consumidor externo, distribuído e SonarQube Cloud; check agregador `CI` |
| `.github/workflows/dmpf-go-ci.yml` | Reutilizável do job `DMPF`, o mesmo que o `dmpf-ci.yml` do consumidor externo chama |
| `.github/workflows/sonar.yml` | SonarQube Cloud com cobertura (chamado pelo `ci.yml` no PR para `master`; `push` em `master`) |
| `.github/workflows/govulncheck.yml` | `govulncheck` semanal na `develop` |
| `.github/workflows/cache-seed.yml` | Semente do cache Go e do golangci-lint em `develop` e `master` (dependências, pins ou config do lint alterados; semanal; manual) |
| `.github/workflows/nx-release.yml` | Versionamento Nx Release (release groups `go-libs`, `go-contract-<ctx>`, `go-tools`, `npm`); cunha as tags de módulo |
| `.github/workflows/nx-publish-libs.yml` | Publica libs npm no GitHub Packages (dispara no push de tag `**@*`), pelo reusable `publish-libs.yaml` |
| `.github/workflows/create-release.yml` | Cria a branch `release/<versão>` a partir de `master` |
| `.github/workflows/dmpf-verify.yml` | Gate de congruência horizontal do acervo `docs/dmpf/` |
| `.github/workflows/dmpf-distributed.yml` | Camada distribuída da pirâmide de testes (build tag `distributed`); chamado pelo `ci.yml` no PR para `master` |
| `.github/workflows/load.yml` | Carga k6 pela borda do BFF sobre a topologia do próprio runner (`workflow_dispatch`; `apps/backend/load/README.md`) |

Decisões relacionadas: [ADR-004](../adr/004-workflows-verdaccio-release.md) (split
version/publish, parcialmente supersedido) e [ADR-043](../adr/043-migracao-para-github-licenca-e-autoria.md)
(a plataforma é GitHub, com registry, runner e reusables próprios — supersede
[ADR-005](../adr/005-plataforma-gitea.md)). A esteira por etapa está no
[ADR-069](../adr/069-esteira-de-ci-por-etapa-do-git-flow.md).
