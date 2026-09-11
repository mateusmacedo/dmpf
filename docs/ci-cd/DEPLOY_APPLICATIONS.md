# Deploy de aplicações (EKS / Modelo A)

Como habilitar o CD de uma app, mapear nomes GitOps e provisionar ConfigMap e
secrets externos.

Baseline: **Modelo A** ([README.md](./README.md)). Expansões (B, C, produção,
outras plataformas): [EXPANSION_MODELS.md](./EXPANSION_MODELS.md).

> Este é um **guia de adoção**. Todo valor entre `<` e `>` é placeholder e precisa
> ser preenchido com os dados do seu projeto. Contas de nuvem, paths de parâmetro,
> namespaces e nomes de secret **não** são versionados neste repositório — eles
> ficam nas variáveis do repositório do projeto e no repositório de
> infraestrutura.

## Mapa canônico app → GitOps

Nomenclatura: sufixo terminal `-bff` (BFF) ou `-api` (API). Nunca `-bff-api`.

Preencha uma linha por app deployável:

| Pasta monorepo | Projeto Nx | Dockerfile | `project` / `ecr` | `path_project` |
| --- | --- | --- | --- | --- |
| `apps/<app>` | `@<org>/<app>` | `apps/<app>/Dockerfile` | `<app>` | `<projeto>/<env>` |

Convenções do modelo de referência:

- O nome do repositório de imagem é igual ao nome do `project` GitOps.
- O nome do repositório de imagem **não** carrega sufixo de ambiente
  (`-dev` / `-hmg`); o ambiente aparece no path GitOps, não no nome da imagem.

## Checklist — app pronta para CD

1. [ ] `apps/<app>/Dockerfile` com contexto na **raiz** do monorepo
       (`docker build -f apps/<app>/Dockerfile .`). Use
       `infra/docker/Dockerfile.node.example` como ponto de partida.
2. [ ] `pnpm nx prune <app>` funciona no Dockerfile (target `prune` declarado no
       projeto — ver o exemplo em `infra/docker/`).
3. [ ] Repositório de imagem criado (ou criação idempotente pelo template).
4. [ ] Pasta `EKS/environment/<projeto>/<env>/<project>/` com `kustomization.yaml`
       e Deployment, no repositório de infraestrutura.
5. [ ] ConfigMap e/ou secret externo alinhados ao schema de configuração da app e
       ao seu `.env.example` (seção abaixo).
6. [ ] Novo job `deploy-<app>` no `cd-dev-hmg.yml`, copiado do bloco de exemplo
       comentado no próprio arquivo.
7. [ ] Linha atualizada na tabela acima.
8. [ ] Smoke por `workflow_dispatch` → `dev`, depois `hmg`.
9. [ ] Seção de deploy no `apps/<app>/README.md` apontando para este doc.

## Como o Modelo A resolve nomes

`detect-apps` devolve paths `apps/<pasta>`, **não** o nome GitOps. A tradução para
`project`/`ecr` é feita **literalmente em cada job** `deploy-<app>`, no `with:`:

```yaml
  deploy-<app>:
    needs: [detect]
    if: always() && (contains(needs.detect.outputs.apps, 'apps/<app>') || inputs.app == 'apps/<app>')
    with:
      project: <app>
      ecr: <app>
      dockerfile: apps/<app>/Dockerfile
```

Não há job `resolve` nem `strategy.matrix`, e isso é deliberado: o `act_runner` do
Gitea não injeta matriz vinda de expressão em job reutilizável (`uses:`). Cada app
nova entra como um job novo — o custo é um bloco por app, e o ganho é um workflow
que roda na plataforma real. O `cd-dev-hmg.yml` traz esse bloco comentado, pronto
para copiar.

Inputs típicos do `deploy-eks`:

| Input | Valor |
| --- | --- |
| `version` | `github.sha` (dev/hmg) |
| `environment` | `dev` \| `hmg` |
| `path_project` | `<projeto>/${{ environment }}` |
| `docker_context` | `.` |
| `dockerfile` | `apps/<app>/Dockerfile` |
| `aws_account_id` | `vars.AWS_ACCOUNT_ID` |

### Pré-requisito — variáveis do repositório

Antes do primeiro smoke, configure no repositório do projeto:

| Nome | Tipo | Conteúdo |
| --- | --- | --- |
| `AWS_ACCOUNT_ID` | repository/environment **var** | ID da conta de nuvem que hospeda o registry de imagens |

Sem essa variável, o job `deploy-eks` falha ao receber `aws_account_id` vazio.

Não versione secrets em claro. Credenciais de nuvem usam OIDC com role assumida
pelo template — nunca chave estática no repositório.

## Disparo manual

No workflow de CD (`workflow_dispatch`):

| Input | Exemplo |
| --- | --- |
| `environment` | `dev` ou `hmg` |
| `app` | `apps/<app>` — vazio significa "apps detectadas" |

Mudança apenas em `libs/`, com `shared_dirs: libs`, enfileira **todas** as apps que
tenham `Dockerfile`.

## Secrets externos

O modelo de referência usa o External Secrets Operator para materializar, como
Secret do Kubernetes, valores guardados no gerenciador de parâmetros da nuvem.

### Convenção de path dos parâmetros

```text
/<ambiente>/<project>-secret/<CHAVE>
```

| Pasta GitOps | Prefixo de ambiente |
| --- | --- |
| `dev` | `<prefixo-dev>` |
| `hmg` | `<prefixo-hmg>` |
| `prod` | `<prefixo-prod>` |

Definidos no repositório de infraestrutura, não aqui:

- o *secret store* de cluster consultado pelo operador (`<cluster-secret-store>`);
- o namespace de destino (`<namespace>`).

### Recurso `ExternalSecret` da app

Alvo: `EKS/environment/<projeto>/<env>/<project>/external-secret.yaml`

Uma linha por chave que a app exige:

| Chave | Referência remota |
| --- | --- |
| `<NOME_DA_CHAVE>` | `/<ambiente>/<project>-secret/<NOME_DA_CHAVE>` |

Valores não sensíveis (URLs, `PORT`, flags de instrumentação) vão no
`configmap.yaml` do mesmo diretório, não no recurso de secret.

Se a organização mantiver credenciais compartilhadas entre apps — licença de APM,
por exemplo — elas ficam em um recurso próprio, referenciado pelo Deployment, e
não são duplicadas no recurso da app.

### Checklist de secrets (por app × ambiente)

1. [ ] Parâmetros criados no gerenciador, com tipo cifrado para segredos.
2. [ ] `external-secret.yaml` criado e incluído no `kustomization.yaml`.
3. [ ] Deployment com `envFrom` apontando para ConfigMap e Secret da app.
4. [ ] Smoke: o pod sobe e a validação de configuração da app não acusa chave
       faltando.
5. [ ] Owner e política de rotação documentados para cada chave sensível.

## Troubleshooting

| Sintoma | Verificação |
| --- | --- |
| `apps=[]` no detect | O commit não tocou `apps/*` nem `libs/*`, ou a app não tem `Dockerfile` |
| App detectada não faz deploy | Não existe job `deploy-<app>` para esse path; adicione o bloco no `cd-dev-hmg.yml` |
| `AWS_ACCOUNT_ID` vazio | Configure a var no repositório do projeto |
| Deploy com `project` errado | `project`/`ecr` do job `deploy-<app>` desatualizados |
| Docker build "Dockerfile not found" | Confira `docker_context: "."` e `dockerfile: apps/<app>/Dockerfile` |
| Build falha em registry privado | Avalie `setup_npmrc: true` no `deploy-eks` (o baseline começa sem) |
| Pod em CrashLoop por configuração | Chave faltando no ConfigMap ou no secret externo; confira a sincronização do operador |
| Imagem não atualiza no cluster | Tag no `kustomization.yaml` e o Application do ArgoCD em `EKS/applications/<projeto>/` |
| Porta ou probe divergente | Alinhe `containerPort` e probes com a imagem publicada |

## Referências

- Templates reutilizáveis: https://github.com/mateusmacedo/actions-templates
- Repositório de infraestrutura: https://github.com/mateusmacedo/infra-reference
- Expansões: [EXPANSION_MODELS.md](./EXPANSION_MODELS.md)
- Dockerfile de referência: `infra/docker/Dockerfile.node.example`
