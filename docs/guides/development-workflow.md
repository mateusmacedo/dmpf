# Workflow de Desenvolvimento

Guia detalhado do dia a dia: fluxo de branches, commits, pull requests e
validação local. É a referência prática que complementa `AGENTS.md`
(seções `Convenções obrigatórias` e `Git e release`) e `CONTRIBUTING.md`.
Quando algum detalhe divergir, `AGENTS.md` e os arquivos de configuração do
projeto prevalecem.

## Da especificação à implementação DMPF

Depois de consolidar requisitos na spec e as decisões arquiteturais aplicáveis,
use o [guia prático de implementação DMPF](./dmpf-implementation.md) para
estruturar o bounded context e avançar para a implementação. O guia é derivado
e não normativo; as obrigações pertencem às fontes em
[`docs/dmpf/`](../dmpf/).

## Regra principal

A base padrão de todo pull request de trabalho é `develop`. Abra o PR contra
`develop`, descrevendo o que muda e por quê. As bases `release/**` e `master`
são exceções declaradas: `master` recebe o trabalho apenas pela branch
`release/X.Y.Z`, nunca por PR direto de uma feature.

Trabalhe sempre fora de `master` e `develop` — ambas são branches protegidas.
Editar direto nelas é bloqueado porque compromete o histórico revisado; a
alternativa é sempre uma branch de trabalho com PR.

## Fluxo gitflow adaptado

O ciclo tem três promoções encadeadas, cada uma com um portão de qualidade:

```text
feature ──PR──▶ develop ──merge da mesma árvore──▶ release/X.Y.Z ──PR──▶ master
   │              │                                    │                    │
   │       code review, testes             fim da sprint: consolida   janela de deploy
   │       exploratórios, regressão         as features já validadas
   │       e validação E2E
```

1. **feature → `develop`.** A branch de trabalho abre PR para `develop`. A
   promoção ocorre após code review, testes exploratórios, regressão e
   validação E2E. `develop` é o ambiente onde o conjunto do trabalho é
   validado de forma integrada antes de virar release.
2. **A mesma feature → `release/X.Y.Z`.** Depois de validada em `develop`, a
   **mesma** branch (mesmo tip, mesma árvore) é mergeada na branch de release
   corrente. A release acumula o que já passou pela validação.
3. **`release/X.Y.Z` → `master`.** Ao fim da sprint, a release é promovida para
   `master` na janela de deploy.

Por que este encadeamento existe:

- **Evitar trabalho não concluído em produção.** `master` reflete o que roda em
  produção. Só chega lá o que já foi validado em `develop` e consolidado numa
  release fechada — nunca uma feature solta ainda em revisão.
- **Evitar cherry-picks na janela de deploy.** Como a feature já está mergeada
  em `develop` e na `release/X.Y.Z` com a mesma árvore, promover a release para
  `master` é um merge previsível. Não é preciso reescrever a árvore, cherry-pick
  de commits prontos ou remontar features na janela de deploy.

## Modelo anti-drift

O anti-drift garante que `develop` e `release` carreguem a **mesma** árvore para
o mesmo trabalho:

1. A branch de trabalho abre PR para `develop` (validação).
2. Depois de validada, a **mesma** branch — mesmo tip, mesmo conteúdo — é
   mergeada em `release/X.Y.Z`.
3. Antes de abrir o PR para `develop`, a branch de trabalho faz merge de
   `release/X.Y.Z` nela. A release pode já ter updates aprovados de outras
   features, e trazê-los para a branch de trabalho mantém o diff do PR limitado
   ao delta real da feature.

```bash
git fetch origin
git merge origin/release/X.Y.Z   # traz updates já aprovados na release
# resolver eventuais conflitos aqui, uma única vez
git push
```

**Regra dura anti-drift:** promova para `release` a mesma árvore que entrou em
`develop`. A alternativa correta é reutilizar o tip (ou o merge commit) já
validado. Abrir um segundo PR paralelo com re-resolução de conflitos,
cherry-picks reescritos ou tip mais novo de conteúdo divergente faz o merge
seguinte de `release` nas branches de trabalho **reabrir o mesmo conflito** —
por isso essa divergência é proibida.

## Nomenclatura de branches

Sem issue tracker, use `tipo/descricao-curta`. Com issue tracker, inclua o
identificador do ticket logo após a barra: `tipo/TICKET-ID-descricao-curta`. A
descrição vai em kebab-case minúsculo.

```text
feat/notificacoes-webhook        # sem ticket
fix/lint-minha-lib               # sem ticket
feat/ENG-234-login-social        # com ticket
```

### Prefixos de tipo

| Prefixo     | Quando usar                          |
| ----------- | ------------------------------------ |
| `feat/`     | Nova funcionalidade                  |
| `fix/`      | Correção de bug                      |
| `refactor/` | Refatoração sem mudar comportamento  |
| `perf/`     | Otimização de performance            |
| `docs/`     | Alteração apenas em documentação     |
| `test/`     | Adição ou correção de testes         |
| `chore/`    | Manutenção, configs, dependências    |

### Branches de workflow

`hotfix/` e `release/` são branches de fluxo, não prefixos de tipo de commit:

| Prefixo    | Quando usar                                        |
| ---------- | -------------------------------------------------- |
| `hotfix/`  | Correção urgente com destino a `master`            |
| `release/` | Branch de release da sprint (`release/X.Y.Z`)      |

## Conventional Commits

As mensagens seguem [Conventional Commits](https://www.conventionalcommits.org)
em PT-BR:

```text
<tipo>(<escopo>): <descrição no imperativo>
```

- A descrição é imperativa e o assunto tem no máximo **72 caracteres**.
- O **escopo** é o nome do projeto Nx afetado, **sem** o prefixo da org: use
  `minha-lib`, não `@lidercap-apps/minha-lib`. Use `workspace` para
  mudanças na raiz do repositório.
- Projetos distintos vão em **commits separados** — mantenha um commit por
  projeto Nx em vez de misturar libs no mesmo commit, para que o versionamento
  por projeto do Nx Release fique correto.
- Inclua corpo apenas quando o motivo não for óbvio pelo assunto.

Tipos usados no repositório: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`.

```text
feat(minha-lib): adicionar helper de agrupamento
fix(outra-lib): corrigir tipo do payload de evento
docs(workspace): documentar fluxo de release
ci(workspace): adicionar passo de cache ao release
```

O versionamento e o changelog são derivados desses commits pelo Nx Release, com
versionamento independente por projeto marcado com `type:lib`.

## Pull Requests

- **Base padrão: `develop`.** As bases `release/**` e `master` são exceções
  declaradas explicitamente por quem abre o PR.
- **Plataforma: Gitea** (`gitea.lidercap.com.br`, organização `lidercap-apps`).
  Automação que fala com a plataforma usa a API do Gitea em `/api/v1/...`. O
  binário `gh` **não** opera contra este servidor — para operar a plataforma,
  chame a API do Gitea diretamente.
- Prefira PRs pequenos e focados: uma feature ou um fix por PR.
- Descreva o que muda e por quê, com contexto suficiente para o revisor.
- PRs que tocam apenas arquivos Markdown não disparam o pipeline, por causa do
  `paths-ignore` do `ci.yml`.

## Validação local antes do PR

Rode a cadeia de validação antes de abrir o PR e corrija o que falhar. Comece
pelo formatter e depois valide apenas o que foi afetado:

```bash
pnpm biome check --write .
pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source
```

O `--exclude=@nx-base-template/source` retira o projeto raiz (targets `nx:noop`)
das operações em lote. Os hooks reforçam a mesma validação: `pre-commit` roda
`biome check --write` nos arquivos em stage e `pre-push` roda lint, typecheck,
test e build nos projetos afetados. Confie nos hooks — mantenha-os ativos em vez
de contorná-los, pois eles são o que garante que todo push sai verde.

## Provisionamento das branches remotas

`develop` e `release/X.Y.Z` precisam existir no remoto para o fluxo funcionar.
Provisione `develop` no remoto ao adotar o template; a branch `release/X.Y.Z` é
criada pelo workflow `create-release.yml`.

O `create-release.yml` calcula o próximo número a partir das releases já
mergeadas em `master`, deriva o incremento de versão dos commits em
`origin/master..origin/develop` e abre o PR de release pela API do Gitea.
Como esse cálculo lê `origin/develop`, o workflow depende de `develop` existir
no remoto — provisione-a primeiro para que a automação de release funcione.

## Antipadrões

Evite os padrões abaixo; a alternativa correta está indicada em cada um:

- **Abrir PR de feature direto para `master`** → abra para `develop`; `master`
  recebe trabalho apenas via `release/X.Y.Z`, para preservar o histórico
  revisado e o portão de validação.
- **Promover para `release` uma árvore diferente da que entrou em `develop`**
  (cherry-picks reescritos, re-resolução paralela de conflitos) → reutilize o
  tip já validado; divergência reabre conflitos nos merges seguintes.
- **Editar direto em `master` ou `develop`** → use uma branch de trabalho com
  PR; ambas são protegidas.
- **Misturar múltiplos projetos Nx no mesmo commit** → um commit por projeto,
  para manter o versionamento por projeto correto.
- **Contar com `gh` para operar a plataforma** → use a API do Gitea
  (`/api/v1/...`); o `gh` não fala com este servidor.
- **PRs grandes misturando várias features** → prefira PRs pequenos e focados.

## Referências

- `AGENTS.md` — seções `Convenções obrigatórias` e `Git e release`.
- `CONTRIBUTING.md` — entrada curta com o modelo anti-drift.
- `docs/adr/005-plataforma-gitea.md` — decisão sobre a plataforma Gitea.
- `docs/ci-cd/` — guia de adoção de CI/CD e deploy.
