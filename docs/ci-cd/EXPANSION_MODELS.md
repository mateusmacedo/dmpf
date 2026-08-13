# Modelos de expansão CI/CD

O modelo de referência é o **Modelo A** (um CD EKS monorepo para `dev` e `hmg`).
Este documento descreve **para onde expandir** sem misturar esses caminhos com o
padrão vigente.

Visão geral e decisão: [README.md](./README.md).
Operação do baseline: [DEPLOY_APPLICATIONS.md](./DEPLOY_APPLICATIONS.md).

> Como no restante deste diretório, valores entre `<` e `>` são placeholders.

---

## 1. Organização dos manifestos (A → B → C)

| Modelo | Status | Resumo | Gatilho típico de adoção |
| --- | --- | --- | --- |
| **A — Monorepo único** | Referência | `detect-apps` + um job `deploy-<app>` por app, com `project`/`ecr` literais | Ponto de partida |
| **B — Manifesto por app** | Expansão | Um workflow (ou par dev-hmg/prod) por app, nomes fixos | Isolamento por time/owner; cadências muito diferentes |
| **C — Híbrido** | Expansão madura | Orquestrador `detect-apps` chama `workflow_call` locais por app | O arquivo único do Modelo A ficou frágil (N apps ou regras por app) |

### 1.1 Permanecer no Modelo A quando

- Há poucas apps deployáveis.
- Mudança em `libs/` deve redeployar o conjunto.
- O número de jobs `deploy-<app>` ainda é pequeno o bastante para caber em um só
  arquivo sem virar ruído.

### 1.2 Migrar para o Modelo B quando

- Uma app precisa de gatilhos, approvals ou variáveis radicalmente diferentes.
- Você quer `paths:` restritos por pasta, sem orquestrador.
- O custo de N arquivos YAML é menor que o de um único arquivo com N jobs.

Esqueleto mental:

```text
.github/workflows/cd-<app-a>-dev-hmg.yml  →  deploy-eks (project/ecr fixos)
.github/workflows/cd-<app-b>-dev-hmg.yml  →  deploy-eks (project/ecr fixos)
# libs/: workflow orquestrador opcional que dispara os dois
```

### 1.3 Migrar para o Modelo C quando

- Você quer detectar apps automaticamente **e** manter `project`/`ecr`
  encapsulados por app.
- Deseja reutilizar o mesmo "workflow local" da app em produção e em disparo
  manual.

Esqueleto mental:

```text
cd-dev-hmg.yml  →  detect-apps → um job por app → uses: ./.github/workflows/deploy-<app>.yml
.github/workflows/deploy-<app-a>.yml   (workflow_call, project/ecr fixos)
.github/workflows/deploy-<app-b>.yml
```

> Atenção ao adotar o Modelo C no Gitea: o `act_runner` não injeta
> `strategy.matrix` vinda de expressão em job reutilizável (`uses:`) — a mesma
> limitação que fez o Modelo A usar um job explícito por app. O orquestrador do
> Modelo C precisa chamar cada `deploy-<app>.yml` em um job próprio, não por
> matriz.

### 1.4 Critérios de decisão (resumo)

```text
Precisa isolar app/time/cadência?     → B
Arquivo A ficou complexo / N apps?    → C
Caso contrário                        → permanecer em A
```

Não misture os Modelos A e B no mesmo ambiente sem documentar qual workflow é a
fonte da verdade.

---

## 2. Expansão de ambiente: produção

O baseline cobre apenas `dev` e `hmg`. Produção é expansão explícita.

| Item | Padrão sugerido (`actions-templates`) |
| --- | --- |
| Exemplo | `examples/eks-prod-monorepo.yaml` |
| Gatilho | `push` de tags `v*`, ou o fluxo de release do monorepo |
| `version` | `github.ref_name` |
| `environment` | `prod` |
| `path_project` | `<projeto>/prod` |
| Detect | frequentemente `shared_dirs: ""` (só apps tocadas na tag) — valide a política |

Pré-requisitos:

1. Manifestos em `EKS/environment/<projeto>/prod/<project>/`.
2. Parâmetros do ambiente de produção e recursos de secret alinhados.
3. Proteções de environment e aprovação manual, se a organização exigir.
4. Documentar se a promoção é "mesmo SHA de hmg" ou "tag de release".

Arquivo típico: `.github/workflows/cd-prod.yml` (Modelo A) ou workflows por app
(Modelos B/C).

---

## 3. Expansão de apps

Para cada app nova no monorepo:

1. Cumprir o checklist em
   [DEPLOY_APPLICATIONS.md](./DEPLOY_APPLICATIONS.md#checklist--app-pronta-para-cd).
2. Adicionar o job `deploy-<app>` no Modelo A **ou** criar manifesto no Modelo B/C.
3. Atualizar as tabelas deste diretório na mesma PR.
4. Se o arquivo do Modelo A crescer demais, avaliar a §1.3.

---

## 4. Expansão operacional (sem novo deploy de imagem)

| Necessidade | Template / exemplo | Notas |
| --- | --- | --- |
| Alterar parâmetros de ConfigMap/GitOps | `actions-templates` → `change-parameters.yaml` | `workflow_dispatch` + JSON; path no repositório de infraestrutura |
| Migrations de banco | `run-migrations.yaml` | Secrets de banco explícitos; só quando houver comando de migration |
| Lint SQL em PR | `sql-ci.yaml` / `sql-lint.yaml` | Paths de migrations SQL |

Esses fluxos **complementam** o CD; não substituem o Modelo A.

---

## 5. Expansão de plataforma (fora de EKS)

Regra: **uma plataforma por manifesto**.

| Plataforma | Exemplos em `actions-templates` | Quando |
| --- | --- | --- |
| ECS | `ecs-dev-hmg*.yaml`, `ecs-prod*.yaml` | O serviço já nasce ou migra para ECS |
| On-premises (Portainer) | `on-premises-*.yaml` | Deploy via stack Portainer |
| Lambda | `deploy-lambda.yaml` | Functions |
| S3 + CloudFront | `deploy-s3-cloudfront.yaml` | Front estático |

Não adicione jobs de ECS ou on-premises no mesmo YAML do CD EKS do Modelo A.

---

## 6. Expansão de libs / registry

Já existe: `.github/workflows/publish-libs.yml`. É independente do CD, disparado
por tags do Nx Release (`**@*`). Mantenha separado do `cd-dev-hmg.yml`.

---

## 7. Checklist antes de expandir

- [ ] O baseline do Modelo A está estável em `dev` e `hmg` para as apps atuais?
- [ ] A expansão está descrita neste doc (qual seção: 1–6)?
- [ ] Há PR correspondente no repositório de infraestrutura, se o runtime mudar?
- [ ] `docs/ci-cd/README.md` e
      [DEPLOY_APPLICATIONS.md](./DEPLOY_APPLICATIONS.md) foram atualizados na
      mesma mudança?
- [ ] Evitou-se um segundo "caminho oficial" não documentado para o mesmo
      ambiente ou app?

---

## 8. Referências

| Fonte | URL / uso |
| --- | --- |
| actions-templates (examples) | https://gitea.lidercap.com.br/lidercap-apps/actions-templates/src/branch/main/examples |
| lidercap-infra | https://gitea.lidercap.com.br/lidercap-apps/lidercap-infra |
| Exemplo de CD monorepo | https://gitea.lidercap.com.br/lidercap-apps/actions-templates/src/branch/main/examples/eks-dev-hmg-monorepo.yaml |
