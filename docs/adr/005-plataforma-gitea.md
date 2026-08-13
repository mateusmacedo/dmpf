# ADR-005: A plataforma de hospedagem é Gitea

## Status

Aceito — 2026-07-30. Implementa SPEC-4NR9KKS8.

Supersede parcialmente o [ADR-004](./004-workflows-verdaccio-release.md): apenas
os itens sobre host de hospedagem e uso do binário `gh`. As demais decisões do
ADR-004 (split version/publish, alvo Verdaccio, runner `gitea-runner`) seguem
válidas.

## Contexto

O ADR-004 afirma, na matriz do que não foi portado, que a API do Gitea não seria
adotada no `create-release` porque o "template [está] hospedado no GitHub", e por
isso manda "manter `gh pr create`". Na seção de decisão ele reforça que o runner
`gitea-runner` foi adotado apenas por paridade de label e que isso "não implica
API Gitea nem troca de host GitHub".

O remoto do repositório contradiz essa premissa:

```text
$ git remote -v
origin	git@gitea.lidercap.com.br:lidercap-apps/nx-base-template.git (fetch)
origin	git@gitea.lidercap.com.br:lidercap-apps/nx-base-template.git (push)
```

O template está hospedado em Gitea desde antes do ADR-004, que registrou uma
premissa de host que já não correspondia à realidade. A consequência prática é
concreta: o binário `gh` fala o protocolo da API do GitHub e não opera contra um
servidor Gitea. O passo "Open pull request" do `create-release.yml` falharia em
execução real — um defeito latente, porque o workflow só roda por
`workflow_dispatch` e ninguém o havia exercitado.

## Decisão

- **A plataforma é Gitea** (`gitea.lidercap.com.br`, organização
  `lidercap-apps`). Toda automação que precise falar com a plataforma usa a API
  do Gitea, não a do GitHub.
- **`create-release.yml` abre PR via API do Gitea**: `jq` monta o corpo JSON e
  `curl -fsS -X POST` chama
  `${GITHUB_SERVER_URL}/api/v1/repos/${GITHUB_REPOSITORY}/pulls`, autenticando
  com `Authorization: token ${GITEA_TOKEN}`. O checklist de release no corpo do
  PR é preservado. As variáveis `GITHUB_SERVER_URL` e `GITHUB_REPOSITORY` são
  populadas pelo `act_runner` do Gitea, que mantém compatibilidade de nomes com
  o GitHub Actions.
- **O binário `gh` não é usado** em nenhum workflow.
- **Itens do ADR-004 que esta decisão substitui**:

| Item do ADR-004 | Situação |
|-----------------|----------|
| "API Gitea no create-release / Template hospedado no GitHub; manter `gh pr create`" | Substituído: a API do Gitea passa a ser usada |
| "Runner `gitea-runner` [...] não implica API Gitea nem troca de host GitHub" | Substituído: o host sempre foi Gitea |
| "Manter no template: [...] `create-release.yml` via `gh`" | Substituído: `gh` sai do workflow |

## Consequências

- O `create-release.yml` deixa de depender do `gh` estar instalado no runner e
  passa a depender de `jq` e `curl`, ambos presentes nas imagens usadas pelo
  `act_runner`.
- O escopo do token continua sendo a variável de risco: `secrets.GITHUB_TOKEN`
  precisa permitir a criação de pull request no repositório. Isso só se confirma
  em execução real do workflow — a decisão assume o contrato documentado da API
  do Gitea, não uma execução observada.
- Ficam registradas duas inconsistências remanescentes com a plataforma, ambas
  fora do escopo desta entrega e candidatas a um follow-up próprio:
  - `nx.json` define `release.changelog.projectChangelogs.createRelease:
    "github"`, o que faz o Nx tentar publicar releases pela API do GitHub;
  - `nx.json` define `release.docker.registryUrl: "ghcr.io"` e o
    `release.yml` faz login nesse mesmo registry, enquanto o Gitea oferece
    registry de contêiner próprio.
- Projetos gerados a partir deste template herdam a automação já compatível com
  a plataforma real da organização.

## Referências

- Spec: `SPEC-4NR9KKS8` (em `docs/specs/`)
- [ADR-004](./004-workflows-verdaccio-release.md) — parcialmente superseded
- [ADR-003](./003-template-hardening.md) — follow-up de portabilidade de registry
- `.github/workflows/create-release.yml`
