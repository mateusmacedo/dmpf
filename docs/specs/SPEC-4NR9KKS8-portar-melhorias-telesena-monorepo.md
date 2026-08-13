---
id: SPEC-4NR9KKS8
slug: portar-melhorias-telesena-monorepo
title: Portar melhorias do telesena-monorepo para o template base
stage: done
priority: P1
depends_on: [SPEC-W4RWD02M]
ticket_url: null
subtask_urls: []
created: 2026-07-30
---

# SPEC-4NR9KKS8: Portar melhorias do telesena-monorepo para o template base

## Resumo

O `telesena-monorepo` foi materializado a partir deste template em 2026-07-16 e,
desde então, acumulou correções de tooling, endurecimentos de segurança, ajustes
de CI para o runner Gitea e um corpo maduro de convenções de IA que este
repositório não possui. Esta spec porta esse ganho de volta ao template.

Como mantenedor do template, quero incorporar as melhorias validadas em produção
no `telesena-monorepo` para que todo projeto novo derivado deste repositório
nasça com o tooling corrigido, os segredos protegidos e as convenções de IA já
estabelecidas.

## Contexto

- **Problema**: os dois repositórios divergiram em paralelo a partir do commit
  raiz comum `7a42ab6`. O `telesena-monorepo` (211 commits) resolveu problemas
  concretos que continuam abertos aqui — entre eles um bug latente: o
  `create-release.yml` deste repositório já roda em `runs-on: gitea-runner`
  (commit `b7b513a`) mas ainda abre PR via `gh pr create`, comando indisponível
  nesse runner. Além disso, 91 arquivos de convenções de IA (`.claude/rules`,
  `.claude/skills`, `.claude/agents`) existem lá e não existem aqui.
- **Impacto**: projetos derivados deste template deixam de herdar correções já
  pagas, repetindo os mesmos erros — lint quebrado em código NestJS, `nx affected
  -t test` falhando em projeto sem spec, `.env` copiado para dentro da imagem
  Docker e release branch que não abre PR.
- **Inspiração**: `telesena-monorepo` é a única fonte considerada. A relação é de
  ancestral/descendente com evolução paralela comprovada por histórico git, não
  de superset.
- **Links relevantes**:
  - `SPEC-W4RWD02M` — evoluiu os workflows deste template a partir do
    `lidercap-platform`; esta spec continua o mesmo trilho de CI/CD
  - `SPEC-YWAWQHPX` — melhorias de tooling, docs, CI/CD e infra do template
  - `docs/adr/001-baseline-monorepo.md` — baseline que recebe o addendum
  - `docs/adr/004-*` — decisão de uso do `gitea-runner`
- **Referências externas**:
  - Repositório de origem: `/home/mmanjos/work/repositories/modernização/telesena-monorepo`
    (branch `develop`, HEAD `309ab4e`)
  - Commit de bifurcação: `chore: rename project references from nx-base-template
    to telesena-monorepo` (2026-07-16)
  - Commit de correção do setup pnpm: `ci: infere a versão do pnpm pelo
    packageManager no setup` (2026-07-16)
  - Templates de CD consumidos pelo telesena:
    `lidercap-apps/actions-templates` (`deploy-eks`, `detect-apps`)

<constraints>
- [P0] O `telesena-monorepo` prevalece em qualquer discrepância, EXCETO nos 12
  itens listados em "Exceções preservadas", onde este template está
  comprovadamente mais novo.
- [P0] NUNCA regredir versão de dependência: `nx` permanece em 23.1.0 e o
  `catalog:` do pnpm permanece completo.
- [P0] NUNCA importar código de produto do telesena (`apps/telesena-*`,
  `libs/backend/*`, `libs/shared/money`) — este repositório é um template.
- [P0] Todo conteúdo portado DEVE ser generalizado: nenhuma string
  `telesena`, `@telesena-monorepo`, `TSM-` ou path `apps/telesena-*` pode
  permanecer nos arquivos aplicados.
- [P1] Cada exceção preservada DEVE ter justificativa objetiva registrada nesta
  spec e no ADR de fechamento.
</constraints>

## Requisitos

### Funcionais

- [x] **[P0] RF1 — Qualidade de código**: aplicar as correções de lint e teste.
  - `biome.json`: adicionar `javascript.parser.unsafeParameterDecoratorsEnabled: true`
  - `biome.json`: adicionar às exclusões `!**/out-tsc`, `!**/test-output`,
    `!**/plans`, `!**/tmp`, `!**/.tokensave`, `!**/.remember`
  - `jest.preset.js`: adicionar `passWithNoTests: true`
  - Edge case: manter `style.useImportType: "error"` do template — o telesena
    desligou (`"off"`) por conveniência de port, não por decisão técnica

- [x] **[P0] RF2 — Higiene de artefatos e segredos**: impedir vazamento de `.env`.
  - `.gitignore`: adicionar bloco de secrets com `.env`, `.env.local`, `**/.env`
    e `!**/.env.example`; adicionar `test-output`, `debug.log`
  - `.dockerignore`: reescrever usando globs `**/` (a forma atual, sem `**/`,
    não alcança subdiretórios de `apps/`), com `**/.env` e `!**/.env.example`
  - Edge case: preservar as entradas exclusivas do template (Next.js, Go,
    Terraform, `.pnpm-store`) que o telesena não possui

- [x] **[P0] RF3 — Robustez de instalação e supply chain**.
  - `package.json`: alterar `"prepare"` de `lefthook install` para
    `lefthook install || true`
  - `pnpm-workspace.yaml`: adicionar a `allowBuilds` as entradas com valor
    `false` — `@datadog/pprof`, `@newrelic/fn-inspect`, `@newrelic/native-metrics`,
    `@scarf/scarf`, `protobufjs`
  - Edge case: NÃO importar o bloco `overrides: tarn` — é workaround de
    resolução específico do Verdaccio do telesena

- [x] **[P0] RF4 — Correção do CI para o runner Gitea**.
  - `.github/actions/setup-node-pnpm/action.yml`: remover o input
    `pnpm-version` e o `with.version` do `pnpm/action-setup@v4`, deixando a
    versão ser inferida de `packageManager`; manter o comentário explicando que
    passar ambos gera `ERR_PNPM_BAD_PM_VERSION`
  - `.github/workflows/*.yml`: remover o parâmetro `pnpm-version` de todas as
    chamadas ao composite action
  - `.github/workflows/create-release.yml`: substituir o step `gh pr create` por
    chamada à API do Gitea (`jq` para montar o payload + `curl -fsS -X POST` em
    `${GITHUB_SERVER_URL}/api/v1/repos/${GITHUB_REPOSITORY}/pulls`), preservando
    o corpo do PR com o checklist de release

- [x] **[P1] RF5 — Ferramenta de build Docker**: portar
  `tools/normalize-pruned-manifest.mjs` (110 linhas), que normaliza o output do
  `nx prune` para `pnpm install --prod --frozen-lockfile` reconstruindo as deps
  do importer raiz e removendo o bloco `overrides:` do lockfile.
  - Edge case: o script é agnóstico de projeto; validar que roda sem referência
    a nomes do telesena

- [x] **[P1] RF6 — DX no editor**: criar `.vscode/` com `extensions.json`
  (recomendações Nx Console) e `settings.json`.
  - Edge case: NÃO portar `launch.json` — as configurações de debug apontam para
    as apps do telesena

- [x] **[P1] RF7 — Convenções de IA**: estabelecer o corpo de convenções.
  - `AGENTS.md`: substituir as 24 linhas atuais pela estrutura de 347 linhas do
    telesena (meta, regras duras, estrutura, comandos, tooling, convenções,
    git/release, padrões de código, workflows, glossário), generalizada
  - `CLAUDE.md`: reduzir para `@AGENTS.md`, adotando fonte única
  - `.claude/rules/`: portar as 9 regras (`ci-quality`, `code-patterns`,
    `error-handling`, `file-organization`, `file-size-limits`, `performance`,
    `process-enforcement`, `security`, `testing-conventions`)
  - `.claude/skills/`: portar as 32 skills, incluindo as `references/` de
    `skill-typescript` e `skill-go`
  - `.claude/agents/`: portar os 49 agentes
  - `.claude/README.md`: portar E corrigir a dessincronização — o índice atual
    documenta 7 agentes com nomes que não existem no diretório (cita
    `agents/architecture.md`; o arquivo real é `agents/architect-reviewer.md`)
  - Edge case: preservar `.claude/settings.json`, que existe neste template e
    não existe no telesena

- [x] **[P1] RF8 — Documentação de processo**.
  - `CONTRIBUTING.md`: incorporar a seção "Modelo develop → release (anti-drift)"
    sem descartar as seções exclusivas do template (Conventional Commits,
    validação local, padrões de código, reportar problemas)
  - `docs/onboarding.md`: criar a partir do telesena, generalizado
  - `docs/adr/001-baseline-monorepo.md`: incorporar o addendum (67 linhas) com
    estado atual, drift em relação ao texto histórico e débitos derivados
  - `docs/ci-cd/`: criar `README.md`, `DEPLOY_APPLICATIONS.md` e
    `EXPANSION_MODELS.md`, parametrizando app, ECR e path de GitOps
  - `docs/nx-reference/tasks.md`: incorporar os exemplos de criação de lib
    backend NestJS via generator

- [x] **[P2] RF9 — Template de CD**: criar `.github/workflows/cd-dev-hmg.yml`
  como modelo comentado e desabilitado por padrão, derivado do telesena.
  - Manter o comentário que documenta por que os jobs de deploy são explícitos:
    o `act_runner` do Gitea não injeta matriz vinda de expressão em job
    reutilizável (`uses:`)
  - Substituir `telesena-soft-bff` e `telesena-auth-api` por placeholders
  - Edge case: o workflow NÃO pode disparar em `push` para `develop` no
    template; usar apenas `workflow_dispatch`

- [x] **[P0] RF10 — Exceções preservadas**: manter o estado atual deste
  repositório nos 12 itens abaixo e registrar a justificativa.

| # | Item | Estado a preservar | Justificativa |
|---|------|--------------------|---------------|
| 1 | `nx` e plugins `@nx/*` | 23.1.0 | Telesena está em 22.7.5 |
| 2 | `catalog:` do pnpm | 30 entradas | Telesena reduziu a 7 e pinou o resto no `package.json` |
| 3 | `tools/generators.json` | generator `shared-lib` | Telesena esvaziou (`generators: {}`) |
| 4 | `nx.json` → `nxCloudId` | presente | Telesena removeu |
| 5 | `nx.json` → `sharedGlobals` | inclui `biome.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml` | Telesena encolheu para só `biome.json`, degradando a invalidação de cache |
| 6 | `nx.json` → plugin `@nx/jest` | sem `exclude` | O `exclude` do telesena aponta para e2e de apps que não existem aqui |
| 7 | `release.yml` | release Docker (GHCR, `dockerVersionScheme`, hotfix) | Telesena removeu porque usa CD próprio |
| 8 | `publish-libs.yml` → `pnpm_version` | `"11"` | Telesena está em `"9"`, incoerente com `packageManager: pnpm@11.9.0` |
| 9 | `libs/shared/` | `types` e `utils` | `money` é domínio do telesena |
| 10 | `README.md` | orientado a template | O do telesena é orientado ao produto |
| 11 | `infra/`, `renovate.json`, `migrations.json`, `LICENSE`, `SECURITY.md`, `CODE_OF_CONDUCT.md` | presentes | Não existem no telesena |
| 12 | Aspas em YAML | `"` | O uso de `'` no telesena é cosmético, não é melhoria |

### Não-funcionais

- [x] Compatibilidade: manter Node `^24` e pnpm `^11` declarados em `engines`;
      manter `packageManager: pnpm@11.14.0`
- [x] Segurança: após a mudança, nenhum arquivo `.env` (em qualquer nível de
      diretório) pode ser rastreável pelo git nem entrar no contexto de build Docker
- [x] Regressão zero: `pnpm nx run-many -t lint typecheck test build` deve
      terminar com o mesmo conjunto de projetos com sucesso que hoje
- [x] Rastreabilidade: cada arquivo portado deve ter origem identificável no
      `telesena-monorepo` (path + commit)

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Tooling e lint | [x] | `biome.json`, `jest.preset.js`, `lefthook.yml` |
| Gestão de dependências | [x] | `package.json`, `pnpm-workspace.yaml` |
| CI/CD | [x] | `.github/actions/`, `.github/workflows/` |
| Build e empacotamento | [x] | `.dockerignore`, `tools/normalize-pruned-manifest.mjs` |
| Convenções de IA | [x] | `AGENTS.md`, `CLAUDE.md`, `.claude/` |
| Documentação | [x] | `docs/adr/`, `docs/ci-cd/`, `docs/onboarding.md`, `CONTRIBUTING.md` |
| DX / editor | [x] | `.vscode/` |
| Código de aplicação (`apps/`, `libs/`) | [ ] | Sem alteração — nenhum código de produto é portado |
| Infraestrutura (`infra/`) | [ ] | Sem alteração — exclusiva do template |

## Localização de código

```text
nx-base-template/
  .claude/          — rules, skills, agents e README (novos)
  .github/          — actions e workflows (correções + cd template)
  .vscode/          — extensions e settings (novos)
  docs/             — adr, ci-cd, onboarding, nx-reference
  tools/            — normalize-pruned-manifest.mjs (novo)
```

**Arquivos a modificar**:
- `biome.json` — parser de decorators NestJS e exclusões de artefatos
- `jest.preset.js` — `passWithNoTests: true`
- `.gitignore` — bloco de secrets, `test-output`, `debug.log`
- `.dockerignore` — reescrita com globs `**/`
- `package.json` — `prepare` tolerante a falha
- `pnpm-workspace.yaml` — `allowBuilds` com bloqueios explícitos
- `.github/actions/setup-node-pnpm/action.yml` — remover `pnpm-version`
- `.github/workflows/ci.yml` — remover `pnpm-version` da chamada
- `.github/workflows/release.yml` — remover `pnpm-version` da chamada
- `.github/workflows/create-release.yml` — `gh pr create` → API do Gitea
- `AGENTS.md` — substituição integral, generalizada
- `CLAUDE.md` — reduzir para `@AGENTS.md`
- `CONTRIBUTING.md` — incorporar modelo anti-drift
- `docs/adr/001-baseline-monorepo.md` — incorporar addendum
- `docs/nx-reference/tasks.md` — exemplos de lib backend

**Arquivos a criar**:
- `.claude/README.md`, `.claude/rules/*` (9), `.claude/skills/*` (32),
  `.claude/agents/*` (49)
- `.vscode/extensions.json`, `.vscode/settings.json`
- `docs/onboarding.md`
- `docs/ci-cd/README.md`, `docs/ci-cd/DEPLOY_APPLICATIONS.md`,
  `docs/ci-cd/EXPANSION_MODELS.md`
- `tools/normalize-pruned-manifest.mjs`
- `.github/workflows/cd-dev-hmg.yml`

## Design

### Arquitetura

O trabalho é uma transferência dirigida entre dois repositórios, organizada em
três trilhos independentes que podem avançar em paralelo:

```text
Trilho 1 — Tooling e segurança   (RF1, RF2, RF3)   → validável por lint/test
Trilho 2 — CI/CD e build         (RF4, RF5, RF9)   → validável por execução do workflow
Trilho 3 — Convenções e docs     (RF6, RF7, RF8)   → validável por revisão de conteúdo
```

O trilho 1 é pré-requisito dos demais porque `biome.json` e `.gitignore`
determinam se os arquivos criados nos trilhos 2 e 3 passam pelo lint e pelo
hook de pre-commit.

### Fluxo principal

1. Aplicar o trilho 1 e rodar `pnpm nx run-many -t lint typecheck test build`
2. Aplicar o trilho 2, validando cada workflow por análise estática do YAML
3. Aplicar o trilho 3, generalizando cada arquivo antes de gravar
4. Rodar a varredura de generalização (busca por `telesena`, `@telesena-monorepo`,
   `TSM-`, `apps/telesena-`) — resultado deve ser zero ocorrências
5. Revalidar o conjunto completo e registrar o ADR de fechamento

### Pseudocódigo

```text
// Varredura de generalização — gate obrigatório antes do commit
padroes = ["telesena", "@telesena-monorepo", "TSM-", "apps/telesena-"]
ocorrencias = buscar(padroes, em=arquivos_alterados_ou_criados)

se ocorrencias.total > 0:
    falhar("generalização incompleta", listar=ocorrencias)
senao:
    prosseguir()
```

## Decisões técnicas

- **Regra de desempate assimétrica**: o telesena prevalece por padrão, mas cede
  nos 12 itens da tabela RF10, porque a análise de histórico git provou evolução
  paralela — e não linear — a partir do commit raiz comum `7a42ab6`.
  Alternativa descartada: prevalência literal e irrestrita do telesena, porque
  causaria downgrade de `nx` 23.1.0 → 22.7.5, desmonte do `catalog:` do pnpm e
  perda do generator `shared-lib`.
- **`create-release.yml` via API do Gitea**: usar `curl` contra
  `/api/v1/repos/.../pulls` porque `gh` não está disponível no `gitea-runner`,
  já adotado por este repositório no commit `b7b513a`.
  Alternativa descartada: instalar o `gh` no runner, porque adiciona uma
  dependência de setup a cada execução para resolver um problema que a API
  nativa já resolve.
- **`CLAUDE.md` como ponteiro `@AGENTS.md`**: fonte única evita divergência entre
  os dois arquivos.
  Alternativa descartada: manter conteúdo duplicado nos dois, porque o próprio
  telesena chegou ao ponteiro depois de sofrer com a divergência.
- **`cd-dev-hmg.yml` apenas com `workflow_dispatch`**: um template não deve
  disparar deploy ao receber push em `develop`.
  Alternativa descartada: não portar o workflow, porque o valor está no padrão
  documentado (Modelo A, jobs explícitos por app).
- **Spec única em vez de duas encadeadas**: decisão do usuário, priorizando
  rastreabilidade num só documento sobre velocidade de fechamento parcial.

## Regras relacionadas

- `docs/adr/001-baseline-monorepo.md` — baseline de plataforma do monorepo
- `docs/adr/004-*` — adoção do `gitea-runner`
- `SPEC-W4RWD02M` — precedente direto: evolução dos workflows a partir de outro
  repositório da organização
- `SPEC-YWAWQHPX` — melhorias anteriores de tooling, docs, CI/CD e infra

## Verificação e testes

### Critérios de aceite

- [x] Os 13 itens do balde A estão aplicados (RF1 a RF6, RF8)
- [x] Os 6 itens do balde B estão aplicados e generalizados (RF7, RF9)
- [x] Os 12 itens da tabela RF10 permanecem no estado atual do template
- [x] A busca por `telesena`, `@telesena-monorepo`, `TSM-` e `apps/telesena-`
      nos arquivos alterados ou criados retorna 0 ocorrências
- [x] `pnpm nx run-many -t lint typecheck test build` termina com sucesso
- [x] `pnpm biome ci .` termina com sucesso
- [x] `git check-ignore -q .env apps/x/.env` retorna 0 para ambos, e
      `git check-ignore -q .env.example` retorna 1
- [x] `nx` permanece em 23.1.0 no `pnpm-workspace.yaml`; o `catalog:` permanece
      íntegro com **37** entradas (a spec dizia 30 — o número estava errado, o
      arquivo estava correto; ver ADR-006)
- [x] `.claude/README.md` lista apenas arquivos que existem em `.claude/agents/`,
      `.claude/rules/` e `.claude/skills/`
- [x] `create-release.yml` não contém a string `gh pr create`
- [x] Nenhum arquivo em `apps/` ou `libs/` foi alterado — **uma exceção
      autorizada**: `libs/shared/utils/src/index.ts` teve 2 linhas reordenadas
      (safe fix do Biome para `assist/source/organizeImports`), corrigindo defeito
      **pré-existente** que impedia `pnpm biome ci .` de passar. Reproduzido com o
      `biome.json` de `HEAD` antes da correção, o que confirma que não era
      regressão desta entrega. Entregue em commit isolado, antes dos commits do
      port. Sem efeito de runtime.
- [x] ADR de fechamento criado em `docs/adr/`, registrando a regra de desempate
      e as 12 exceções

### Cenários de teste

```text
DADO um projeto NestJS com decorators de parâmetro (@Inject, @Body)
QUANDO o desenvolvedor executa `pnpm biome ci .`
ENTÃO o comando termina com sucesso, sem erro de parsing dos decorators

DADO um projeto Nx recém-gerado sem nenhum arquivo .spec.ts
QUANDO o hook de pre-push executa `pnpm nx affected -t test`
ENTÃO o comando termina com sucesso em vez de falhar por ausência de testes

DADO um arquivo apps/exemplo/.env com credenciais
QUANDO `git status` e `docker build` são executados na raiz do monorepo
ENTÃO o arquivo não aparece como não rastreado e não entra no contexto de build,
      enquanto apps/exemplo/.env.example permanece rastreado e disponível

DADO o workflow create-release.yml executando no gitea-runner
QUANDO o job chega ao passo de abertura de pull request
ENTÃO a PR é criada via POST na API do Gitea, sem depender do binário `gh`

DADO um agente de IA lendo as convenções do repositório
QUANDO abre CLAUDE.md
ENTÃO encontra apenas o ponteiro @AGENTS.md e segue para a fonte única
```

<critical_constraints>
- [P0] O `telesena-monorepo` prevalece em qualquer discrepância, EXCETO nos 12
  itens da tabela RF10, onde este template está comprovadamente mais novo.
- [P0] NUNCA regredir versão de dependência: `nx` permanece em 23.1.0 e o
  `catalog:` do pnpm permanece completo.
- [P0] NUNCA importar código de produto do telesena (`apps/telesena-*`,
  `libs/backend/*`, `libs/shared/money`) — este repositório é um template.
- [P0] Todo conteúdo portado DEVE ser generalizado: nenhuma string
  `telesena`, `@telesena-monorepo`, `TSM-` ou path `apps/telesena-*` pode
  permanecer nos arquivos aplicados.
- [P1] Cada exceção preservada DEVE ter justificativa objetiva registrada nesta
  spec e no ADR de fechamento.
</critical_constraints>

## Escopo fora

- **Código de aplicação do telesena** (`apps/telesena-soft-bff`,
  `apps/telesena-auth-api` e respectivos e2e — 163 arquivos): é produto, não
  template. Pertence ao próprio `telesena-monorepo`.
- **Libs de backend do telesena** (`libs/backend/{http-client,auth,contracts,observability,health}`
  — 97 arquivos): os padrões podem virar skills em `.claude/skills/`, mas o
  código pertence ao produto ou a um pacote publicado separadamente.
- **`libs/shared/money`**: lógica de domínio financeiro, não utilitário genérico.
- **Downgrade de dependências**: coberto pela tabela RF10 como exceção explícita.
- **Migração deste repositório para Nx 22**: não há motivo técnico; o template
  está à frente.
- **Adoção do Verdaccio/`overrides: tarn`**: workaround de ambiente específico do
  telesena. Se necessário, vira spec própria.
- **Correção do prefixo do escopo em `tsconfig.base.json`**: débito
  pré-existente detectado durante a análise, sem relação com o port. Resolvido
  depois, fora desta spec.
