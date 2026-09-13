# ADR-008: Destravar o release — proteções do repositório e ajuste do Nx Release ao Gitea

## Status

Aceito — 2026-08-11. Implementa SPEC-BG2K2W30. Supersede parcialmente o
[ADR-004](./004-workflows-verdaccio-release.md): apenas os dois itens de
configuração do Nx Release listados na seção "Decisão". Os demais itens do
ADR-004 (split version/publish, alvo Verdaccio, runner `gitea-runner`) seguem
válidos, como já registrado no [ADR-005](./005-plataforma-gitea.md).

## Contexto

O workflow `Release` versionava as libs corretamente e falhava no push: o Gitea
recusava as três refs (`master` e as duas tags de versão) com `pre-receive hook
declined`. A branch protection de `master` autorizava os times `Owners` e
`tech-leads`; a tag protection de pattern `*` autorizava apenas `Owners`. Ambas
tinham `whitelist_usernames` vazio, e o ator do push era o token efêmero das
Actions, que não pertence a time nenhum.

A investigação revelou dois problemas adicionais no caminho, ambos capazes de
manter o pipeline quebrado mesmo depois de resolver o push:

1. **O Nx Release não tem provider para Gitea.** O `nx.json` declarava
   `changelog.projectChangelogs.createRelease: "github"`, apontando para
   `api.github.com` a respeito de um repositório que existe apenas no servidor
   da organização. Os providers suportados são `github`,
   `github-enterprise-server` e `gitlab` — nenhum atende ao Gitea, conforme já
   registrado no ADR-005.
2. **`createRelease` é o que ligava o push automático.** O `nx release` deriva
   `shouldPush` da presença dessa chave, não de um default do comando. Por isso
   o push saía de dentro do step `Nx Release (skip publish)`, e o step dedicado
   `Push version commit and tags` nunca chegava a executar. As duas chaves são
   acopladas: declarar `release.git.push: false` mantendo `createRelease` faz o
   Nx abortar com `GIT_PUSH_FALSE_WITH_CREATE_RELEASE`.

Um terceiro obstáculo apareceu na revisão: a mensagem de commit do release
carregava `[skip ci]`. No Gitea, as skip strings são avaliadas contra o commit
apontado pela tag, então o push das tags de versão não dispararia o
`publish-libs.yml` — o pipeline concluiria verde e mesmo assim nada seria
publicado no Verdaccio.

### O que foi medido sobre a identidade do push

Antes de decidir, duas hipóteses foram testadas e descartadas empiricamente:

- **O ator do push não é quem dispara o workflow.** O run que originou esta
  decisão foi disparado por um membro de `Owners` e `tech-leads` — presente,
  portanto, nas duas whitelists — e o push foi recusado mesmo assim.
- **O ator do push também não é o `ci-runner-bot`.** Um workflow descartável
  (run 4820) tentou empurrar uma tag anotada com `secrets.GITHUB_TOKEN`, com o
  bot já inscrito nas três whitelists e com permissão `owner` no repositório. O
  servidor recusou com `Tag ... is protected`.

A conclusão é que o token efêmero das Actions carrega uma identidade distinta,
que não consta em nenhuma whitelist e não é nomeável nelas. Enquanto houvesse
proteção com whitelist, o push do CI exigiria a credencial de um usuário
autorizado — foi o que motivou a tentativa, depois revertida, de usar um PAT do
`ci-runner-bot` num secret `RELEASE_TOKEN`.

## Decisão

**As proteções de branch e de tag do repositório foram removidas.** Sem
whitelist a satisfazer, o push do release passa com o token efêmero das Actions
e nenhuma credencial adicional é necessária. Isso alinha o `dmpf` ao
`dmpf-legacy`, que roda o mesmo fluxo sem proteção alguma e sem
secret dedicado. A verificação usou o mesmo teste que havia falhado: no run
4831, já sem as proteções, a tag descartável subiu (`* [new tag]`) e foi
removida em seguida.

**O `nx.json` deixa de configurar release remota.** A chave
`changelog.projectChangelogs.createRelease` é removida — o que elimina a
tentativa de falar com `api.github.com` e, junto, o push automático do comando
agregado. `release.git.push: false` entra como guardrail explícito contra a
reintrodução do comportamento. As duas mudanças são inseparáveis, pelo
acoplamento descrito acima.

**O `[skip ci]` sai da mensagem de commit do release**, que passa a ser
`chore(release): publish`. Sem a string, o push das tags dispara o
`publish-libs.yml` como o ADR-004 previa. A remoção não cria recursão: nenhum
workflow do repositório dispara em `push` para `master` — o `release.yml`
dispara em `pull_request` e `workflow_dispatch`, e o `ci.yml` em
`pull_request`.

**O `release.yml` adota o `dmpf-legacy` como workflow de
referência da organização**: mesmo nome de job, mesma identidade git
(`gitea-actions[bot]@noreply.github.com`, no lugar do domínio do GitHub
que estava configurado), mesmo comando de push e o mesmo `secrets.GITHUB_TOKEN`
no checkout. As únicas divergências mantidas têm causa declarada: as versões de
Node e pnpm (fixadas pelo ADR-003) e os steps de release Docker (previstos pelo
ADR-004).

**Itens do ADR-004 que esta decisão substitui**:

| Item do ADR-004 | Situação |
|-----------------|----------|
| "Manter no template: [...] `changelog.createRelease: github`" | Substituído: a chave é removida — não há provider Gitea, e é ela que ligava o push automático |
| "Manter no template: [...] `[skip ci]` no commit de release" | Substituído: a string sai do `commitMessage`, senão as tags não disparam o `publish-libs.yml` |
| "`createRelease: false` no changelog / Template mantém releases GitHub (`github`)", na matriz do que não foi portado | Substituído: o template deixa de configurar release remota |

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter as proteções e autorizar `ci-runner-bot` por nome, com PAT no secret `RELEASE_TOKEN` | Chegou a ser implementada e depois revertida. Funciona, mas custa um secret por repositório, de rotação manual, e cria uma dependência viva fora do código: se o bot sair de qualquer whitelist, o release volta a falhar com a mesma mensagem. |
| Promover `ci-runner-bot` ao time `Owners` | Resolveria em um passo, mas daria poder administrativo sobre toda a organização a um token de CI. |
| Adotar o padrão `/actions/GITEA_DEPLOY_TOKEN` via SSM, usado por sete templates do `actions-templates` | É o padrão da organização para push autenticado no CI e dispensaria secret por repositório. Rejeitada porque exigiria acrescentar OIDC, AWS CLI e assume-role ao `release.yml`, e porque o dono desse token não tinha acesso a este repositório. Continua sendo o caminho indicado caso as proteções sejam reintroduzidas. |
| Manter o push dentro do `nx release` | Não é possível sem release remota: o comando agregado deriva `shouldPush` de `createRelease`, então preservar o push exigiria manter a chave — e com ela o provider incompatível e a chamada a `api.github.com`. |
| Trocar o provider de `createRelease` em vez de removê-lo | Nenhum dos providers suportados aponta para o servidor correto; não há troca possível. |
| Criar releases via API do Gitea (`POST /api/v1/repos/{owner}/{repo}/releases`) | É capacidade nova, não correção do push. Fica para uma spec própria, depois que o pipeline estiver verde. |
| Usar `--atomic` no push, ou neutralizar o `pre-push` do Lefthook com `LEFTHOOK: "0"` | Ambas divergiriam do workflow de referência sem problema comprovado: o `dmpf-legacy` tem o mesmo `pre-push` e o mesmo `prepare: lefthook install`, roda o release sem nenhuma das duas e conclui. |

## Consequências

**Positivas:**

- O release conclui de ponta a ponta sem credencial dedicada: nenhum secret a
  criar, nenhuma whitelist a manter, nada que expire.
- O `release.yml` fica em paridade com o workflow de referência da organização,
  o que reduz o custo de manter os dois repositórios alinhados.
- O push tem um ponto único de escrita no remoto, mais simples de diagnosticar
  e de tornar idempotente.
- Sem `createRelease`, o Nx para de tentar operações autenticadas contra um
  serviço que não hospeda este repositório: com o cliente de release remota
  desligado, ele nem chega a resolver `GITHUB_TOKEN`/`GH_TOKEN`.
- Com o `[skip ci]` removido, o `publish-libs.yml` volta a ser disparado pelas
  tags, restaurando o fluxo do ADR-004.

**Negativas:**

- **As proteções que valiam para pessoas saíram junto.** `master` deixa de
  exigir os dois approvals e passa a aceitar push direto de qualquer
  colaborador com acesso de escrita; qualquer tag pode ser criada ou removida
  sem restrição. O fluxo git-flow do `AGENTS.md` e o gate de revisão passam a
  depender de disciplina do time, e não de imposição do servidor. O
  `infra-reference`, por contraste, mantém whitelist por time em `master`.
- **A condição não é reproduzível a partir de um clone.** A ausência de
  proteção é estado do servidor, não código versionado. Um repositório criado a
  partir deste template herda os workflows, mas não herda essa condição — e
  falhará no push se tiver proteção com whitelist, pelo mesmo motivo registrado
  aqui. Nesse caso, a alternativa do PAT ou a do SSM voltam à mesa.
- O repositório deixa de ter releases na interface da plataforma enquanto a
  criação via API do Gitea não for implementada; os `CHANGELOG.md` versionados
  seguem sendo a fonte de histórico.
- O teste do run 4831 exercitou a criação e a remoção de tag, não a atualização
  de `master`. Essa parte só fica comprovada no primeiro release real.
