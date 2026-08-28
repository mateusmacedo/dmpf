---
id: SPEC-BG2K2W30
slug: destravar-release-gitea-actions
title: Destravar o pipeline de release do template no Gitea Actions
stage: done
priority: P1
depends_on: []
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-462
subtask_urls: []
created: 2026-08-11
---

# SPEC-BG2K2W30: Destravar o pipeline de release do template no Gitea Actions

## Resumo

O workflow `Release` versiona as libs compartilhadas corretamente, mas não consegue empurrar o resultado para o remoto: o Gitea recusa as três refs (`master` e as duas tags de versão) porque o token efêmero das Actions não pertence a nenhum time autorizado nas whitelists de proteção. Como mantenedor do template, quero que o release conclua de ponta a ponta e que o `release.yml` fique em paridade com o `lidercap-platform-legacy`, workflow de referência da organização, para que as libs voltem a ser versionadas e publicadas sem intervenção manual e sem credencial dedicada a manter.

## Contexto

- **Problema**: o run 4803 (job 6300, disparado pelo merge do PR #4 de `release/0.0.1` para `master`) falhou no step `Nx Release (skip publish)` com exit code 1. O `nx release` executou bump `0.0.1` para `0.0.2` nas duas libs, gerou changelogs, commitou e criou as tags — e foi recusado no push com `pre-receive hook declined` nas três refs. Nenhuma tag jamais existiu no remoto, e as libs seguem em `0.0.1`.
- **Impacto**: enquanto não for resolvido, nenhuma lib do template pode ser versionada nem publicada, e o fluxo de release descrito no ADR-004 permanece inoperante. O template deixa de cumprir uma das funções que documenta.
- **Causa raiz**: o release escrevia em três refs protegidas usando uma identidade que não estava em nenhuma whitelist. A branch protection de `master` tinha `enable_push_whitelist: true` com os times `Owners` e `tech-leads`; a tag protection #1012, com `name_pattern: "*"`, cobria toda e qualquer tag e admitia apenas `Owners`. Ambas com `whitelist_usernames` vazio. O ator do push é o token efêmero das Actions do Gitea, que não é membro de nenhum time — logo, não havia caminho pelo qual fosse aceito. O bloqueio era de identidade, não de conteúdo.
- **Identidade do ator, medida**: duas hipóteses foram testadas e descartadas. O ator não é quem dispara o workflow (o run 4803 partiu de um membro de `Owners` e `tech-leads` e foi recusado) e não é o `ci-runner-bot` (o run 4820 empurrou uma tag com o token efêmero, já com o bot nas três whitelists e com permissão `owner`, e recebeu `Tag ... is protected`). É uma identidade distinta, não nomeável em whitelist.
- **Causa contribuinte**: o `release.yml` foi escrito assumindo que o step `Nx Release (skip publish)` apenas versiona, e que o push seria feito pelo step dedicado `Push version commit and tags`. O que liga o push automático, porém, é a chave `changelog.projectChangelogs.createRelease` do `nx.json` — o `nx release` deriva `shouldPush` da presença dela, não de um default do comando. Com `createRelease: "github"` declarado, o push saiu de dentro do step de versionamento e o step dedicado nunca executou.
- **Obstáculo latente**: a mesma chave faz o Nx tentar se comunicar com `api.github.com` a respeito de um repositório que existe apenas no Gitea. O Nx Release oferece os providers `github`, `github-enterprise-server` e `gitlab` — não há provider para Gitea. Removê-la resolve as duas coisas de uma vez, e por isso as duas edições do `nx.json` ficam acopladas: declarar `release.git.push: false` mantendo `createRelease` faz o Nx abortar com `GIT_PUSH_FALSE_WITH_CREATE_RELEASE`.
- **Obstáculo adicional**: a mensagem de commit do release carrega `[skip ci]`. No Gitea, as skip strings são avaliadas contra o commit apontado pela tag, então o push das tags de versão não dispararia o `publish-libs.yml` — o pipeline concluiria verde e nada seria publicado no Verdaccio.
- **Links relevantes**:
  - `docs/adr/004-workflows-verdaccio-release.md` — separação entre versionamento e publicação no Verdaccio
  - `docs/adr/005-plataforma-gitea.md` — decisão de plataforma e suas implicações
  - `AGENTS.md` — seção "Git e release" (branches protegidas, fluxo git-flow)
  - `ANALISE-RELEASE-4803.md` — relatório técnico da investigação; rascunho local, fora do versionamento
- **Referências externas**:
  - Run que falhou: https://gitea.lidercap.com.br/lidercap-apps/nx-base-template/actions/runs/4803
  - Ticket de trabalho: https://lider-cap.atlassian.net/browse/ARQ-462

<constraints>
- [P0] NUNCA conceder ao token de CI permissão de `Owner` na organização.
- [P0] Segredos NUNCA são versionados nem impressos em log.
- [P0] NUNCA usar `--no-verify` nem desabilitar hooks para contornar validação.
- [P0] O `release.yml` DEVE seguir o `lidercap-platform-legacy` como referência; toda divergência DEVE ter causa declarada.
- [P1] O push ao remoto DEVE acontecer em exatamente um step do workflow, e esse step DEVE ser `Push version commit and tags`.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] Destravar o push das refs**: o repositório NÃO DEVE ter proteção de branch nem de tag que barre o ator do push do CI. As proteções foram removidas, e com isso o token efêmero das Actions passa a empurrar sem credencial adicional.
  - Verificado empiricamente: com as proteções ativas, o push de uma tag descartável com `secrets.GITHUB_TOKEN` foi recusado (run 4820, `Tag ... is protected`); sem elas, o mesmo teste passou (run 4831, `* [new tag]`).
  - O ator do token efêmero não é o `ci-runner-bot` nem quem dispara o workflow — ambas as hipóteses foram testadas e descartadas. Enquanto houvesse whitelist, o push exigiria a credencial de um usuário autorizado.
  - A ausência de proteção é estado do servidor, não código versionado: um repositório derivado deste template não a herda.
  - Trade-off assumido e registrado no ADR-008: `master` deixa de exigir os dois approvals e qualquer tag pode ser criada ou removida.
- [x] **[P0] Manter o workflow em paridade com a referência**: o `release.yml` DEVE usar `secrets.GITHUB_TOKEN` no checkout, o mesmo nome de job, a mesma identidade git e o mesmo comando de push do `lidercap-platform-legacy`.
  - Divergências permitidas, com causa: versões de Node e pnpm (ADR-003) e steps de release Docker (ADR-004).
- [x] **[P0] Separar versionamento de push**: a chave `changelog.projectChangelogs.createRelease` DEVE ser removida do `nx.json` e `"push": false` DEVE ser declarado no bloco `release.git`, na mesma edição. Remover `createRelease` é o que de fato desliga o push do `nx release` — e elimina o provider incompatível; `push: false` entra como guardrail contra a reintrodução do comportamento. O step `Push version commit and tags` volta a ser o único ponto de escrita no remoto, com o mesmo comando usado pelo `lidercap-platform-legacy`.
  - As duas chaves são inseparáveis: `push: false` mantendo `createRelease` faz o Nx abortar com `GIT_PUSH_FALSE_WITH_CREATE_RELEASE`.
  - Nenhum dos providers suportados pelo Nx (`github`, `github-enterprise-server`, `gitlab`) atende ao Gitea.
  - Os arquivos `CHANGELOG.md` continuam sendo gerados, versionados e commitados — comportamento já observado no run 4803.
- [x] **[P0] Restaurar o gatilho da publicação**: o `[skip ci]` DEVE sair de `release.git.commitMessage`, que passa a ser `chore(release): publish`. No Gitea, a skip string é avaliada contra o commit apontado pela tag e impediria o `publish-libs.yml` de disparar.
  - A remoção não cria recursão: nenhum workflow do repositório dispara em `push` para `master` — `release.yml` dispara em `pull_request`/`workflow_dispatch` e `ci.yml` em `pull_request`.
- [x] **[P1] Registrar a decisão**: um ADR em `docs/adr/` DEVE documentar a identidade de automação do release, por que a autorização é por nome e não por time, e a ausência de provider Gitea no Nx Release.

### Não-funcionais

- [x] Segurança: nenhuma permissão administrativa sobre a organização DEVE ser concedida ao CI, e nenhum PAT de usuário DEVE ser persistido no runner. Com o token efêmero das Actions, a credencial é emitida por execução e expira com ela — superfície menor que a de um PAT em secret. Em contrapartida, a remoção das proteções elimina o controle de escrita do lado do servidor: `master` aceita push direto e qualquer tag pode ser criada ou removida. Trade-off assumido, registrado no ADR-008.
- [x] Compatibilidade: as mudanças DEVEM funcionar com Nx `23.1.0`, pnpm `11.14.0` e Node `^24`, conforme as versões fixadas no workspace.
- [x] Reversibilidade: as três frentes (plataforma, `nx.json`, `release.yml`) DEVEM ser revertíveis de forma independente entre si. Dentro do `nx.json`, porém, `createRelease` e `push` são acopladas e revertem juntas — reverter apenas uma reintroduz o push indevido ou o erro `GIT_PUSH_FALSE_WITH_CREATE_RELEASE`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Configuração do workspace (`nx.json`) | [x] | `changelog.projectChangelogs.createRelease` é removida; `release.git.push` passa a `false`; `[skip ci]` sai do `commitMessage` |
| CI/CD (`.github/workflows/`) | [x] | `release.yml` alinha nome do job, identidade git e comando de push ao `lidercap-platform-legacy`, e mantém `secrets.GITHUB_TOKEN` no checkout |
| Plataforma (Gitea) | [x] | Proteções de branch e de tag removidas do repositório — fora do repositório, não versionável |
| Documentação (`docs/adr/`) | [x] | ADR registrando a identidade de automação e a ausência de provider Gitea |
| Libs (`libs/shared/*`) | [ ] | Nenhuma mudança de código; apenas voltam a ser versionadas pelo pipeline |
| Apps | [ ] | Não há apps no template |

## Localização de código

```text
nx-base-template/
  nx.json                          — configuração do Nx Release
  .github/workflows/release.yml    — workflow de versionamento
  docs/adr/                        — decisões arquiteturais
  docs/specs/                      — esta spec
```

**Arquivos a modificar**:

- `nx.json` — remover `changelog.projectChangelogs.createRelease`; adicionar `"push": false` em `release.git`; remover `[skip ci]` do `commitMessage`
- `.github/workflows/release.yml` — remover as envs `GITHUB_TOKEN`/`GH_TOKEN` do step de versionamento e o `env` de nível de job; alinhar nome do job, identidade git e comando de push ao `lidercap-platform-legacy`. O `token:` do checkout permanece `secrets.GITHUB_TOKEN`
- `docs/adr/008-identidade-automacao-release.md` — ADR novo (numeração seguinte ao ADR-007)
- `docs/adr/README.md` — linha do ADR-008 no índice

**Configuração externa** (não versionável, executada no Gitea):

- Branch protection de `master` — removida
- Tag protections #1012 (pattern `*`) e #1013 (pattern `v*`) — removidas

Nenhum secret é criado: o workflow usa o `secrets.GITHUB_TOKEN` que o Gitea injeta em cada execução.

## Design

### Arquitetura

```text
Fluxo atual (quebrado)                Fluxo corrigido
──────────────────────                ───────────────
checkout (GITHUB_TOKEN)               checkout (GITHUB_TOKEN)
        │                                     │
nx release --skip-publish             nx release --skip-publish
  ├─ version                            ├─ version
  ├─ changelog                          ├─ changelog
  ├─ commit                             ├─ commit
  ├─ tag                                ├─ tag
  └─ push  ◄── RECUSADO                 └─ (push desabilitado)
        │                                     │
  [step nunca executa]                git push --follow-tags  ◄── ACEITO
                                              │
                                        publish-libs.yml
```

### Fluxo principal

1. Merge de PR `release/*` ou `hotfix/*` para `master` dispara o workflow `Release`.
2. `Checkout master` autentica com o `secrets.GITHUB_TOKEN` emitido para a execução, gravando o header de credencial usado nas operações git subsequentes.
3. `Nx Release (skip publish)` resolve o bump pelos Conventional Commits, escreve os manifestos, gera changelogs, commita e cria as tags — sem empurrar.
4. `Push version commit and tags` executa `git push --follow-tags origin master`. Sem proteção de branch ou de tag, o pre-receive aceita as três refs.
5. As tags criadas disparam `publish-libs.yml`, que publica no Verdaccio conforme o ADR-004.

### Ordem de aplicação

A remoção das proteções é pré-requisito: com elas ativas, o workflow continua falhando no push, agora no step dedicado. A ordem aplicada foi remover as proteções, validar com uma tag descartável e só então alinhar `release.yml` e `nx.json`.

## Decisões técnicas

- **Remover as proteções em vez de autorizar uma identidade**: escolhido por alinhar o repositório ao `lidercap-platform-legacy`, que roda o mesmo fluxo sem proteção alguma, e por dispensar credencial dedicada — nada a criar, rotacionar ou manter em whitelist. O custo é perder o controle de escrita do lado do servidor, registrado nas Consequências do ADR-008.
  Alternativa descartada: manter as proteções e autorizar `ci-runner-bot` por nome, com PAT no secret `RELEASE_TOKEN`. Chegou a ser implementada e revertida — funciona, mas cria dependência viva fora do código e um segredo de rotação manual.
  Alternativa descartada: adotar o padrão `/actions/GITEA_DEPLOY_TOKEN` via SSM, usado por sete templates do `actions-templates`, porque exigiria OIDC, AWS CLI e assume-role no `release.yml`, e o dono do token não tinha acesso a este repositório. Continua indicado caso as proteções voltem.
- **Desligar o push do Nx em vez de remover o step dedicado**: escolhido porque alinha o código à intenção já declarada nos comentários do `release.yml` e concentra a escrita no remoto em um ponto único, mais simples de diagnosticar e de tornar idempotente. O desligamento vem da remoção do `createRelease`; `push: false` fica como guardrail.
  Alternativa descartada: manter o push dentro do `nx release` e remover o step, porque a falha voltaria a ocorrer no meio do comando, com commit e tags já criados e sem controle sobre a retentativa.
- **Remover `createRelease` em vez de trocar de provider**: escolhido porque o Nx Release não oferece provider para Gitea e nenhum dos existentes (`github`, `github-enterprise-server`, `gitlab`) aponta para o servidor correto.
  Alternativa descartada: criar releases via API do Gitea no mesmo escopo, porque mistura a correção do push com uma capacidade nova; fica registrado em Escopo fora.

## Regras relacionadas

- `.claude/rules/git-safety.md` — branches protegidas, revisão antes do commit, proibição de segredos versionados
- `.claude/rules/process-enforcement.md` — plano aprovado para mudanças que tocam 2+ arquivos
- `docs/adr/004-workflows-verdaccio-release.md` — separação entre versionamento e publicação
- `docs/adr/005-plataforma-gitea.md` — implicações de operar no Gitea em vez do GitHub

## Verificação e testes

### Critérios de aceite

- [x] O token efêmero das Actions empurra uma tag descartável para o repositório, e a tag é removida em seguida (run 4831).
- [x] O workflow `Release` conclui com `conclusion: success`, com o commit de versão presente em `master` e as tags `{projectName}@{version}` visíveis em `GET /api/v1/repos/lidercap-apps/nx-base-template/tags`.
- [x] O push ao remoto ocorre exclusivamente no step `Push version commit and tags`; o step `Nx Release (skip publish)` não emite a linha `NX Pushing to git remote "origin"`.
- [x] O `publish-libs.yml` é disparado pelas tags geradas e conclui com sucesso.
- [x] O `release.yml` diverge do `lidercap-platform-legacy` apenas nos pontos com causa declarada: versões de Node e pnpm (ADR-003) e steps de release Docker (ADR-004).
- [x] Validação do projeto passando: `pnpm biome ci .` e `pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source`.

### Cenários de teste

```text
DADO que o repositório não tem proteção de branch nem de tag
  E que nx.json não declara createRelease e define release.git.push como false
QUANDO um PR de release/* é mergeado para master
ENTÃO o workflow Release conclui com sucesso
  E o commit de versão e as tags {projectName}@{version} existem no remoto

DADO que nenhuma tag {projectName}@{version} existe no repositório
QUANDO o step Nx Release (skip publish) executa
ENTÃO a flag --first-release é aplicada
  E o changelog de cada lib é gerado a partir do histórico completo

DADO que uma proteção com whitelist é reintroduzida no repositório
QUANDO o step Push version commit and tags executa
ENTÃO o push é recusado com pre-receive hook declined
  E o release exige uma credencial de usuário autorizado, conforme o ADR-008
```

<critical_constraints>
- [P0] NUNCA conceder ao token de CI permissão de `Owner` na organização.
- [P0] Segredos NUNCA são versionados nem impressos em log.
- [P0] NUNCA usar `--no-verify` nem desabilitar hooks para contornar validação.
- [P0] O `release.yml` DEVE seguir o `lidercap-platform-legacy` como referência; toda divergência DEVE ter causa declarada.
- [P1] O push ao remoto DEVE acontecer em exatamente um step do workflow, e esse step DEVE ser `Push version commit and tags`.
</critical_constraints>

## Escopo fora

- **Criação de releases no Gitea via API**: substituir o `createRelease` removido por um step que chame `POST /api/v1/repos/{owner}/{repo}/releases` é capacidade nova, não correção do push. Pertence a uma spec própria, depois que o pipeline estiver verde.
- **Publicação no Verdaccio**: o `publish-libs.yml` já existe e não muda; esta spec apenas restaura o gatilho (as tags) que o alimenta.
- **Release de imagens Docker**: os steps `Detect docker release candidates` e `Release — docker apps` permanecem como estão. O template não tem apps com `tag:type:app`, então eles seguem sendo pulados.
- **Modelo de proteção do repositório daqui em diante**: se e como reintroduzir branch e tag protection — por exemplo com whitelist por time, como o `lidercap-infra` mantém — é discussão de plataforma. Esta spec apenas registra o estado atual e o caminho de volta (ADR-008, alternativas descartadas).
- **Automação da configuração do Gitea**: versionar as regras de proteção como código (Terraform ou script idempotente) é melhoria de plataforma, não requisito para destravar o release.

## Checklist de qualidade da spec

- [x] Todas as seções obrigatórias presentes e preenchidas
- [x] Sem `[TODO]`, `[TBD]` ou seções vazias
- [x] Constraints em XML tags
- [x] Sandwich: constraints repetidas no final
- [x] Linguagem imperativa
- [x] Escopo fora com justificativa
- [x] Paths exatos na Localização de código
- [x] Camadas afetadas identificadas
- [x] Cenários de teste: 3 (happy path, edge case do primeiro release, erro esperado)
- [x] Critérios de aceite mensuráveis
