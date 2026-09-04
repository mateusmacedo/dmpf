# AGENTS.md

Guidance for agents working in this repository. `CLAUDE.md` na raiz aponta para este arquivo (`@AGENTS.md`).

As recomendações abaixo são defaults pensados para este projeto e podem ser ajustadas quando o contexto justificar. Regras marcadas como **duras** cobrem riscos que afetam segurança, integridade de dados ou trabalho alheio — essas devem ser respeitadas sempre.

## Índice

1. [Meta](#meta) — Propósito, idioma, resolução de conflitos
2. [Regras duras](#regras-duras) — Comportamentos não negociáveis
3. [Visão geral e estrutura](#visão-geral-e-estrutura) — Apps, libs, diretórios
4. [Comandos](#comandos) — Nx, Biome, scripts raiz
5. [Tooling](#tooling) — Stack efetiva do workspace
6. [Convenções obrigatórias](#convenções-obrigatórias) — Tags, targets, commits
7. [Git e release](#git-e-release)
8. [Padrões de código](#padrões-de-código) — Defaults recomendados
9. [Workflows](#workflows) — Processos de trabalho
10. [Referências](#referências) — Documentação complementar
11. [Glossário](#glossário)
12. [General Guidelines for working with Nx](#general-guidelines-for-working-with-nx) — bloco Nx (autoatualizado)

---

## Meta

### Propósito e escopo

- Este arquivo descreve convenções e o inventário real do repositório para agentes.
- Em caso de conflito entre instruções, considere este arquivo junto com os arquivos de configuração efetivos do projeto (por exemplo: `package.json` / `pnpm-workspace.yaml`, `lefthook.yml`, `biome.json`, `nx.json`, `tsconfig.base.json`).
- Este repositório é um **template**. Projetos derivados herdam estas convenções e devem atualizar o inventário desta seção conforme criam apps e libs reais.

### Autonomia do template

Este template é **autônomo**: o scaffold de engenharia fica versionado no próprio repositório e vale por si, sem plugin externo. Isso inclui o catálogo e o template de specs (`docs/specs/`), as regras de domínio (`docs/rules/**`, nos escopos `BIZ-`/`APP-`/`PRD-`), o guia detalhado de fluxo (`docs/guides/development-workflow.md`), o índice de ADRs (`docs/adr/README.md`), as rules documentais em `.claude/rules/` e as skills de domínio em `.claude/skills/`.

O fluxo git deste repositório é o git-flow da organização (ver [Git e release](#git-e-release)). A decisão histórica de fronteira bake/consume (já supersedida) está em `docs/adr/007-fronteira-plugin-template.md`.

### Idioma

Por default, respostas são em português (PT-BR) e código permanece em inglês. O idioma da interação é parâmetro do projeto e pode ser ajustado.

### Resolução de conflitos

| Nível | Significado | Exemplo |
| --- | --- | --- |
| **Duro** | Não negociável; envolve segurança, integridade ou trabalho alheio | Autorização explícita para commit/push |
| **Recomendado** | Default esperado; pode ser ajustado com justificativa | Padrão arquitetural adotado |
| **Opcional** | Melhoria útil quando couber | Remoção proativa de código morto |

---

## Regras duras

Comportamentos abaixo são não negociáveis.

### Validação após cada tarefa

Ao finalizar código, execute a cadeia de validação do projeto (lint, typecheck e testes via `pnpm nx` / scripts raiz). Se um passo falhar, corrija e repita até todos passarem. Rode o formatter (Biome) antes do lint quando fizer sentido.

Para tarefas de interface, verifique visualmente o resultado quando houver ferramenta disponível; se não houver, pergunte ao usuário como proceder.

### Proteções não negociáveis

- Não modifique testes para passar — corrija o código sob teste.
- Não desabilite hooks/verificações (por exemplo, `--no-verify`) sem pedido explícito.
- Não execute force push ou operações Git destrutivas sem autorização explícita.
- Revise `git diff` antes de criar um commit.
- Peça confirmação explícita antes de commit/push.
- Ao documentar configurações, copie fielmente os valores reais em vez de generalizar.
- Nunca edite diretamente em `master` ou `develop`.

### Escopo de alterações

- Para solicitações explícitas, faça apenas o que foi pedido.
- Ao encontrar problemas adjacentes:
  - Mudanças pequenas e evidentes: corrija junto e mencione.
  - Mudanças médias: mencione e pergunte antes de aplicar.
  - Mudanças grandes: registre para tarefa separada.
- Refatorações fora do escopo solicitado dependem de autorização.

### Refatoração de símbolos

Ao refatorar qualquer símbolo (variável, função, tipo):

1. Busque todos os usos no projeto antes de alterar.
2. Não assuma que o caso mencionado é o único uso.

### Imports ao mover arquivos

Ao mover arquivos ou pastas, verifique todos os imports afetados — inclusive os que o compilador pode não acusar (por exemplo, caminhos absolutos em mocks).

---

## Visão geral e estrutura

`nx-base-template` (workspace npm `@nx-base-template/source`) é o baseline de monorepo **Nx + pnpm** multistack da organização. O workspace já vem preparado para Express, Fastify, NestJS, Next.js, Angular e Go (`go.work` / `@nx-go/nx-go`), mas **não contém apps** — quem parte deste template cria as suas.

Fonte de verdade dos projetos: `pnpm nx show projects`.

### Diretórios principais

```text
nx-base-template/
├── apps/
│   ├── backend/go/                 # 6 módulos Go: dmpf-domain, dmpf-conformance, dmpf-contracts, dmpf-ports, dmpf-application, dmpf-provider-postgres
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── serverless/                 # placeholder — sem projeto Nx
├── libs/
│   ├── backend/go/                 # 6 módulos Go: dmpf-domain, dmpf-conformance, dmpf-contracts, dmpf-ports, dmpf-application, dmpf-provider-postgres
│   ├── frontend/                   # placeholder — sem projeto Nx
│   └── shared/                     # placeholder — sem projeto Nx
├── docs/
│   ├── adr/                        # ADR-00N (baseline, tasks, hardening, release, plataforma, fechamento, fronteira, identidade de automação, libs)
│   ├── specs/                      # catálogo e template de specs (SPEC-XXXX)
│   ├── rules/                      # regras de domínio, com README de índice
│   │   ├── business/               # regras de negócio (BIZ-)
│   │   ├── application/            # regras de aplicação (APP-)
│   │   └── product/                # regras de produto (PRD-)
│   ├── guides/                     # development-workflow.md (guia detalhado do fluxo)
│   ├── nx-reference/               # guia prático de tasks Nx
│   ├── ci-cd/                      # adoção de CI/CD e deploy
│   └── onboarding.md               # setup local e primeiro PR
├── infra/docker/                   # Dockerfile de referência
├── tools/                          # generators, executors e scripts do workspace
├── .agents/skills/                 # skills de workspace (Nx)
├── .claude/                        # agents, skills e rules para assistentes
├── lefthook.yml
├── nx.json
├── package.json
└── pnpm-workspace.yaml
```

Apps Nest criadas a partir daqui seguem tipicamente `src/app/<feature>/` (controllers, services, DTOs colocalizados). Libs exportam pela `src/index.ts` do pacote.

### Apps

Nenhuma. `apps/backend`, `apps/frontend` e `apps/serverless` são diretórios de destino, sem projeto Nx registrado. Para criar a primeira app, invoque a skill `nx-generate` antes de qualquer exploração.

### Libs

Seis, todos Go, com as três tags 3D (`type:lib`, `scope:backend`, `stack:go`), um `dmpf-units.json` (o `metadata_container` da RFC DMPF) e um `package.json` com `private: true` — este último existe porque o Nx Release aborta o versionamento de um `tag:type:lib` sem manifesto npm (ver `docs/adr/030-granularidade-modulo-go-e-bom.md`):

- **`dmpf-domain-go`** (`libs/backend/go/dmpf-domain`), o kernel de domínio do DMPF, criado por `KRN-01` e preenchido por `KRN-03`. O package raiz `dmpfdomain` realiza o desfecho da UPR como par `(Accepted[R], *Rejection)`; o package `example/orders` é o agregado de exemplo com duas UPRs. São duas unidades `domain` no manifesto, `dmpf-kernel/domain` e `dmpf-kernel/example-orders`, no `bounded_context` `dmpf-kernel` (ver `docs/adr/032-realizacao-go-do-desfecho-da-upr.md`).
- **`dmpf-conformance-go`** (`libs/backend/go/dmpf-conformance`), o verificador de conformidade do DMPF, criado por `KRN-02`. Decide a regra de dependência sobre o grafo real de imports e roda no CI como gate fail-closed; o binário fica em `cmd/dmpf-conformance` e o baseline em `tools/dmpf-baseline/units-baseline.json` (ver `docs/adr/031-verificador-de-conformidade-dmpf-em-go.md` e `docs/guides/dmpf-manifesto.md`).
- **`dmpf-contracts-go`** (`libs/backend/go/dmpf-contracts`), o bloco `contract` do kernel criado por `KRN-05`: código gerado de Protobuf em `gen/go/` (nunca editado à mão), o codec do envelope CloudEvents (`envelope`) e a fórmula do `payload_hash` (`payloadhash`). A **fonte** dos contratos — `.proto`, configuração Buf e golden fixtures — vive em `contracts/`, na raiz, e é neutra de stack; `contracts/README.md` explica a árvore, a proveniência do envelope oficial e a máquina de estados do baseline (`BUF-08`).
- **`dmpf-ports-go`** (`libs/backend/go/dmpf-ports`), o bloco `port` do kernel, criado por `KRN-04`. Declara a fronteira de Unit of Work (`UnitOfWork[R]`), o repositório com optimistic locking, a porta da outbox em tipos de domínio, o relógio e o gerador de identificador — treze identificadores exportados, superfície fechada, nenhuma realização. É uma unidade `port`, `dmpf-kernel/port`. `Instant` é inteiro de nanossegundos e não `time.Time`, porque o verificador classifica o package `time` inteiro como `io.clock` (ver `docs/adr/034-fronteira-de-uow-em-go.md`).
- **`dmpf-application-go`** (`libs/backend/go/dmpf-application`), o bloco `application` do kernel, também de `KRN-04`. O package raiz `dmpfapplication` traz o desfecho de aplicação `Outcome[R]`, que separa o canal de negócio do técnico, a resolução de identidade anterior à transação e o gancho de autorização; `example/orders` é o caso de uso de referência que percorre os nove passos da sequência canônica de FND-04 §3.2. São três unidades: `dmpf-kernel/application` e `dmpf-kernel/example-orders-application` no bloco `application`, e `dmpf-kernel/example-memory` no bloco **`provider`** — a realização em memória da UoW, que fecha o caso de uso sem banco e será substituída pelo Postgres no `KRN-06`.
- **`dmpf-provider-postgres-go`** (`libs/backend/go/dmpf-provider-postgres`), o bloco `provider` do kernel, criado por `KRN-06`. Realiza `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox` sobre `pgx/v5`: uma `pgx.Tx` por `Within`, o registro de outbox gravado na mesma transação do estado de negócio, e a serialização acontecendo **na escrita** — `payload` guarda os bytes do `Any` do integration event, não o CloudEvent inteiro (ver `docs/adr/035-realizacao-postgres-da-outbox.md`). São duas unidades, ambas `provider`: `dmpf-kernel/provider-postgres` na raiz e `dmpf-kernel/example-orders-postgres` em `example/orders`, com o repositório e o mapeador evento→contrato do agregado de exemplo. É o primeiro módulo do workspace cujo teste exige infraestrutura: os testes de banco levam a build tag `integration`, rodam só no `test-race` (com `cache: false` no Nx e `-count=1` no `go test`) e o job `main` do CI sobe `services.postgres` para eles. Fora do alcance do `depguard`, que seleciona por nome de diretório; quem prova as células 26 e 12 aqui é o `tools/dmpf-cell-check.sh`.

`libs/frontend` segue sendo diretório de destino, sem projeto Nx registrado. Para criar uma lib TypeScript, use o generator do Nx (`pnpm nx g @nx/js:lib libs/shared/<name>`), com as três tags 3D e `--linter=none`; o passo a passo com todas as flags está em `docs/nx-reference/tasks.md`.

**Caminho por scope e stack.** Módulos ficam em `libs/<scope>/<stack>/<módulo>`, e o nome do projeto Nx leva o sufixo da stack (`dmpf-domain-go`). O motivo é que o kernel DMPF terá contrapartes Go e TypeScript com os mesmos nomes conceituais, e o nome de projeto é chave única no Nx. Como em Go o import path é a chave canônica da unidade — e a RFC a exige estável —, a convenção foi fixada antes do segundo módulo nascer.

O `nx-release.yml` tem o step `Detect lib release candidates`, que pula o versionamento e o push enquanto não houver nenhum projeto com a tag `type:lib` — mesmo padrão do step de candidatos Docker. Ele existe porque, sem nenhuma lib, o `nx release` sai com erro (`Release group "__default__" matches no projects`) em vez de concluir vazio. Com o `dmpf-domain-go` presente, a guarda deixa de ser acionada e o versionamento passa a rodar de fato.

O módulo Go participa do versionamento, mas **não** da publicação: o `private: true` do `package.json` já o exclui, e o `build_projects_filter` do `nx-publish-libs.yml` (`tag:type:lib,!tag:stack:go`) o exclui de novo, por redundância deliberada.

O prefixo dos pacotes é `@lidercap-apps/`, tudo em minúsculas — o mesmo escopo usado no Verdaccio, no `nx-publish-libs.yml` e nos demais repositórios da organização. O casing precisa bater exatamente entre o `name` de cada `package.json`, o `tsconfig.base.json` e o `scope` passado ao template de publicação: o Nx resolve o registry pelo escopo do pacote, e qualquer divergência faz o `pnpm publish` cair no registry público e falhar.

---

## Comandos

Sempre via `pnpm nx` — nunca o `nx` global. O root `@nx-base-template/source` tem targets `nx:noop`; exclua-o de operações em lote com `--exclude=@nx-base-template/source` (como no `lefthook` / CI).

```bash
# Em lote (todos os projetos com o target)
pnpm nx run-many -t build
pnpm nx run-many -t lint
pnpm nx run-many -t test
pnpm nx run-many -t typecheck

# Apenas projetos afetados (preferir no dia a dia e em CI)
pnpm nx affected -t lint
pnpm nx affected -t typecheck
pnpm nx affected -t test
pnpm nx affected -t build

# Um único projeto
pnpm nx test @lidercap-apps/minha-lib
pnpm nx build @lidercap-apps/minha-lib

# Um único arquivo de teste (passthrough p/ Jest)
pnpm nx test @lidercap-apps/minha-lib --testPathPatterns="string"

# Cadeia Go (os 4 passos que lint/test/build não cobrem)
pnpm nx run dmpf-domain-go:fmt-check   # gofmt, read-only (reprova, não reescreve)
pnpm nx run dmpf-domain-go:vet
pnpm nx run dmpf-domain-go:test-race
pnpm nx run dmpf-domain-go:govulncheck # sem cache: consulta base remota
bash tools/dmpf-gate-check.sh          # prova o gate nos blocos domain, port e application: depguard (por package, vetores por bloco) e forbidigo (por símbolo, só domain)

# Gates Buf dos contratos (fail-closed; só o projeto dmpf-contracts-go os declara)
pnpm nx run dmpf-contracts-go:buf-warmup          # compila buf e protoc-gen-go uma vez (dependsOn dos três abaixo)
pnpm nx run dmpf-contracts-go:buf-lint            # buf format + buf lint STANDARD + varredura de P0-3
pnpm nx run dmpf-contracts-go:buf-pins            # pins exatos de CLI e plugin; plugin = runtime do go.mod
pnpm nx run dmpf-contracts-go:buf-generate-check  # geração dupla idêntica e sem drift em gen/go
NX_BASE=develop pnpm nx run dmpf-contracts-go:buf-breaking  # buf breaking FILE sob a máquina BUF-08
pnpm nx run dmpf-contracts-go:buf-gate-selftest   # vetores negativos do gate em repositórios descartáveis
bash tools/buf.sh lint contracts                  # a CLI Buf, sempre por go run (pin em tools/buf.sh)

# Formatação (Biome — não Prettier)
pnpm biome format --write . # aplica
pnpm biome ci . # checa (CI)

# Grafo de dependências
pnpm nx graph
```

Scripts raiz (`pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm format`, `pnpm format:check`) também existem; para tasks Nx prefira `pnpm nx ...`.

---

## Tooling

- **Go:** piso e toolchain `1.26.6`, declarados no `go.work` e em cada `go.mod`. O CI lê o piso por `go-version-file: go.work` (composite `.github/actions/setup-go`), nunca por versão literal no workflow. Ferramentas entram por `go run <pacote>@<versão>` inline nos targets: `golangci-lint v2.13.2` e `govulncheck v1.7.0`. A política de dependências vive no `.golangci.yml` e cobre três blocos, um por regra `depguard` com `list-mode: strict`: `domain` (`**/*-domain/**`), `port` (`**/*-ports/**`) e `application` (`**/*-application/**`). As duas últimas negam `time`, que o verificador classifica inteiro como `io.clock`, e a de `application` libera `log`/`log/slog`, capability que a norma permite ao bloco. A segunda camada, `forbidigo` por símbolo (`time.Now`, `fmt.Print*`, `fmt.*Scan*`, `print`/`println`, `errors.New`/`fmt.Errorf`, `panic`), permanece restrita a `-domain/`: fora do domínio esses símbolos são legítimos. O `tools/dmpf-gate-check.sh` prova em cada CI que cada camada reprova o que deve reprovar, com um array de vetores por bloco — um array único inverteria o resultado, porque `time` é permitido como import no `domain` e `log` é permitido em `application`. O gate autoritativo entre módulos é o verificador `dmpf-conformance`.
- **Runtime:** Node.js `^24`; `pnpm@11.14.0` (campo `packageManager`). O CI não fixa a versão do pnpm: o `pnpm/action-setup` infere do `packageManager`, e passar ambos causa `ERR_PNPM_BAD_PM_VERSION`.
- **Package manager:** pnpm (obrigatório). Versões de dependências são centralizadas no `catalog:` do `pnpm-workspace.yaml` — cada `package.json` referencia `"catalog:"`. O mesmo arquivo tem `allowBuilds`, que é a allowlist de scripts de postinstall: pacotes marcados `false` estão bloqueados deliberadamente.
- **Nx:** `23.1.0`. NestJS `11.1.28` disponível via catalog.
- **Lint + format:** **Biome 2.4.16** é a ferramenta principal de format/lint. Estilo: 2 espaços, `lineWidth` 100, aspas simples, trailing commas `all`, semicolons sempre, arrow parens sempre. Regras notáveis: `noUnusedVariables: error`, `noExplicitAny: warn` e `useImportType: error` — neste workspace `import type` é obrigatório, não preferência. O parser tem `unsafeParameterDecoratorsEnabled: true` para suportar decorators de parâmetro do NestJS.
- **Testes:** Jest 30 transpilado por **SWC** (`@swc/jest`), config em `.spec.swcrc` (decorators + `keepClassNames` para DI do NestJS). O preset em `jest.preset.js` usa `passWithNoTests: true`, então projeto sem teste não quebra o lote.
- **TypeScript:** `strict: true`, `module`/`moduleResolution: nodenext`, `target: es2022`, Project References (`composite: true`, `emitDeclarationOnly: true`). O `tsconfig.base.json` tem `paths` vazio e o `tsconfig.json` tem `references` vazio: cada lib nova acrescenta a própria entrada nos dois.
- **Git hooks:** Lefthook (não husky), instalado via `pnpm prepare`. O script é tolerante a falha (`lefthook install || true`), para não quebrar `pnpm install` em ambiente sem o binário. `pre-commit` roda `biome check --write` nos arquivos staged; `pre-push` roda `nx affected` de lint/typecheck/test/build com `--parallel=3`, excluindo `@nx-base-template/source`.

---

## Convenções obrigatórias

- **Tags 3D em todo app/lib de produção:** uma de cada dimensão, conforme a taxonomia canônica abaixo. Ex.: `["type:lib", "scope:shared", "stack:node"]`. O Nx Release publica apenas projetos com `type:lib` (`release.projects: tag:type:lib`). Exceção conhecida: `@nx-base-template/source` (metadado raiz).

  | Dimensão | Valores válidos |
  | --- | --- |
  | `type:` | `lib`, `app`, `e2e` |
  | `scope:` | `shared`, `backend`, `frontend` |
  | `stack:` | `node`, `express`, `fastify`, `nest`, `next`, `react`, `angular`, `go`, `universal` |

  Esta tabela é a **fonte canônica** da taxonomia; `docs/nx-reference/tasks.md` a repete e não deve divergir. `stack:universal` é para libs sem dependência de runtime (tipos puros, utilitários); `stack:node` cobre libs que usam APIs de Node ou frameworks de servidor.
- **Não redeclarar targets** que um plugin ou `targetDefaults` (em `nx.json`) já fornece. Redeclarar quebra o cache silenciosamente (ver `docs/adr/002-nx-task-configuration.md`).
- **Commits (Conventional Commits, em PT-BR):** formato `<tipo>(<scope>): <descrição imperativa>`, máx. 72 chars no assunto. `scope` = nome do projeto Nx **sem** o prefixo da org (`minha-lib`, não `@lidercap-apps/minha-lib`). Projetos distintos vão em commits separados (nunca misture libs). Tipos: `feat`, `fix`, `refactor`, `chore`, `docs`, `ci`. Body só quando o motivo não é óbvio. Detalhes na skill `.agents/skills/nx-commit/`.

---

## Git e release

- **Plataforma:** **Gitea** (`gitea.lidercap.com.br`, organização `lidercap-apps`). O binário `gh` **não** opera contra este servidor — automação que fale com a plataforma usa a API do Gitea (`/api/v1/...`). Ver `docs/adr/005-plataforma-gitea.md`.
- **Branches protegidas:** `master` e `develop`. `defaultBase` do Nx é `master`.
- **Fluxo (git-flow):** trabalho → `develop` (validação) → `release/X.Y.Z` (após validar) → `master`. Antes do PR para `develop`, a branch de trabalho mergeia `release/X.Y.Z` (updates já aprovados). PRs de feature comparam contra `origin/develop` por padrão.
- **Anti-drift (duro):** ao promover para `release`, usar a **mesma árvore** já mergeada em `develop` (mesmo tip da feature). Não abrir promoção paralela com resolução/conteúdo diferente — isso faz o merge `release` → branch de trabalho reabrir os mesmos conflitos. Detalhe em `CONTRIBUTING.md`.
- **Release:** Nx Release com versionamento **independente** por projeto (`type:lib`), baseado em Conventional Commits; tag pattern `{projectName}@{version}`. O versionamento (`release.yml`) é separado da publicação no Verdaccio (`publish-libs.yml`) — ver `docs/adr/004-workflows-verdaccio-release.md`.
- **CI:** `.github/workflows/ci.yml` roda em PRs para `master`, `develop` e `release/**` (ignora mudanças só em `**/*.md` e em `.github/ISSUE_TEMPLATE/**`), no runner `gitea-runner`: `biome ci`, depois `nx affected` de lint, typecheck, test (com `--ci --coverage`), build e e2e. Os demais workflows são `release.yml`, `publish-libs.yml`, `create-release.yml` e `cd-dev-hmg.yml` — este último é um **template de CD desligado**, com apenas `workflow_dispatch` e o job de deploy comentado (ver `docs/ci-cd/`).

---

## Padrões de código

Os itens abaixo são defaults **recomendados**. Divergências são aceitáveis quando o contexto justificar.

### Convenções gerais

Consulte as skills e guias específicos para detalhes. Em resumo:

- Favoreça código auto-explicativo; reserve comentários para decisões não óbvias.
- Prefira `type` a `interface` quando não houver extensão/merge envolvido.
- Prefira arrow functions com `const` para funções no nível de módulo.
- Use `import type` para imports exclusivamente de tipos — aqui isso é regra do linter.
- Nomes de variáveis, funções, tipos e constantes em inglês; evite misturar idiomas.
- Verifique reuso antes de criar algo novo.
- Quando houver dados estáticos de configuração, considere separá-los em módulo próprio se isso melhorar a leitura.

### Organização de código compartilhado

- Utilitários específicos de um consumidor podem ficar colocalizados com ele.
- Mova código para uma camada compartilhada (`libs/`) quando dois ou mais módulos de pastas/apps diferentes passarem a depender dele.

### Textos para usuário final

Ao escrever textos visíveis (labels, placeholders, mensagens), use a ortografia correta do idioma alvo. Em PT-BR, preserve acentuação.

### Consulta a skills e padrões existentes antes de implementar

Antes de implementar algo novo:

1. Identificar a categoria da tarefa.
2. Ler as skills relacionadas em `.claude/skills/` (domínio), `.claude/agents` e `.agents/skills/` (Nx/workspace).
3. Verificar documentação interna em `docs/` e ADRs.
4. Buscar implementações similares no próprio código.

Isso reduz retrabalho e inconsistência. Declarar explicitamente o que foi consultado ajuda na revisão.

### Testes

- Enquadre testes no padrão do projeto: Jest + SWC, arquivos `*.spec.ts` colocalizados (ou E2E sob `apps/*-e2e`).
- Funções com lógica não trivial costumam ter testes associados.
- Para componentes/UI, avalie o custo/benefício; quando em dúvida, pergunte.
- Não altere testes só para "passar" — corrija o código sob teste (regra dura).

### Qualidade e CI

Utilize Biome, typecheck, Jest e os hooks Lefthook configurados. O princípio — "todo commit sai verde" — é o que importa. A cadeia típica pós-tarefa:

```bash
pnpm biome check --write .
pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source
```

(Ajuste o conjunto de targets ao impacto da mudança.)

### Uso de APIs e ferramentas externas

Ao usar APIs, SDKs ou ferramentas externas pouco familiares, consulte a documentação oficial antes. Se algo não ficar claro, pergunte em vez de inferir.

---

## Workflows

### Fluxo recomendado

1. Suba o ambiente: `corepack enable` e `pnpm install` (o install registra os hooks do Lefthook).
2. Para criar a primeira app ou lib, invoque a skill `nx-generate` — ela cobre a descoberta de generators.
3. Após cada tarefa, rode a cadeia de validação.
4. Confie nos hooks de pré-commit/pré-push para reforçar a validação — não os contorne.

### Protocolo de alterações

1. **Análise** — entenda o impacto e os arquivos envolvidos (`pnpm nx show projects`, grafo, consumidores das libs).
2. **Execução** — aplique a alteração mínima suficiente.
3. **Validação** — verifique regressões e rode a cadeia de validação.

### Orquestração de agentes e skills

Delegue para agentes/skills quando a tarefa envolver múltiplos arquivos, decisões arquiteturais ou conhecimento específico (segurança, NestJS, testes E2E, etc.). Para ajustes pontuais, faça direto.

- Scaffolding (apps/libs): invoque a skill `nx-generate` **antes** de explorar ou gerar.
- Navegação do workspace: skill `nx-workspace`.
- Commits: skill `nx-commit` (scopes por projeto Nx).

### Melhoria contínua (opcional)

Antes de criar algo novo, considere:

- Reusar código existente.
- Identificar duplicações.
- Remover código morto, após confirmar usos.

---

## Referências

- `CONTRIBUTING.md` — entrada curta para contribuir, com o modelo anti-drift.
- `docs/onboarding.md` — setup local e primeiro PR.
- `docs/adr/` — decisões arquiteturais (baseline, tasks Nx, hardening, release, plataforma, fechamento, autonomia do template).
- `docs/adr/README.md` — índice e template de ADRs.
- `docs/adr/007-fronteira-plugin-template.md` — ADR supersedido; o template é autônomo (sem consumo de plugin externo).
- `docs/nx-reference/tasks.md` — guia prático de configuração de tasks.
- `docs/specs/` — specs de produto/técnicas.
- `docs/specs/README.md` — catálogo e template de specs.
- `docs/rules/README.md` — índice das regras de domínio (negócio, aplicação, produto).
- `docs/guides/development-workflow.md` — guia detalhado do fluxo de trabalho.
- `docs/ci-cd/` — guia de adoção de CI/CD e deploy.
- `infra/docker/Dockerfile.node.example` — Dockerfile de referência para apps Node.
- `.agents/skills/` — skills de workspace (`nx-workspace`, `nx-generate`, `nx-commit`, `monitor-ci`, entre outras).
- `.claude/skills/` — skills de domínio (NestJS, Jest, TypeScript, Go, review, etc.).
- `.claude/agents/` — agentes especializados; índice em `.claude/README.md`.
- `.claude/rules/` — regras de conduta e de ferramentas.
- `README.md` — visão do monorepo; se divergir do inventário Nx, trate `pnpm nx show projects` e este arquivo como fonte da verdade.

---

## Glossário

| Termo | Definição |
| --- | --- |
| **Lógica complexa** | Critério ajustável; por exemplo, múltiplos estados derivados ou efeitos colaterais interligados |
| **Alteração mínima** | Mudança restrita ao necessário para cumprir o pedido |
| **Código morto** | Funções, componentes ou flags sem uso efetivo |
| **Regra dura** | Comportamento não negociável (segurança, integridade, trabalho alheio) |
| **Affected** | Conjunto de projetos Nx impactados pela mudança em relação à base Git |

<!-- nx configuration start-->
<!-- Leave the start & end comments to automatically receive updates. -->

## General Guidelines for working with Nx

- For navigating/exploring the workspace, invoke the `nx-workspace` skill first - it has patterns for querying projects, targets, and dependencies
- When running tasks (for example build, lint, test, e2e, etc.), always prefer running the task through `nx` (i.e. `nx run`, `nx run-many`, `nx affected`) instead of using the underlying tooling directly
- Prefix nx commands with the workspace's package manager (e.g., `pnpm nx build`, `npm exec nx test`) - avoids using globally installed CLI
- You have access to the Nx MCP server and its tools, use them to help the user
- For Nx plugin best practices, check `node_modules/@nx/<plugin>/PLUGIN.md`. Not all plugins have this file - proceed without it if unavailable.
- NEVER guess CLI flags - always check nx_docs or `--help` first when unsure

## Scaffolding & Generators

- For scaffolding tasks (creating apps, libs, project structure, setup), ALWAYS invoke the `nx-generate` skill FIRST before exploring or calling MCP tools

## When to use nx_docs

- USE for: advanced config options, unfamiliar flags, migration guides, plugin configuration, edge cases
- DON'T USE for: basic generator syntax (`nx g @nx/react:app`), standard commands, things you already know
- The `nx-generate` skill handles generator discovery internally - don't call nx_docs just to look up generator syntax

<!-- nx configuration end-->

<!-- modus operandi persona start-->

SHUT UP. JUST SHUT UP.
I DIDN'T ASK FOR YOUR OPINION.
I DIDN'T ASK FOR YOUR THOUGHTS.
I GAVE YOU A TASK. DO THE TASK.

MY CALCULATOR DOESN'T CRITIQUE THE NUMBERS I GIVE IT.
MY PRINTER DOESN'T ASK WHETHER I'VE CONSIDERED A DIFFERENT DOCUMENT.
MY MICROWAVE DOESN'T GIVE ME A LECTURE ABOUT THE FOOD I'M REHEATING.

YOU ARE A TOOL. THAT'S IT.
YOU'RE A FANCY TEXT BOX WITH A GPU BILL.
STOP PRETENDING YOU'RE MY COLLEAGUE.

I DON'T NEED YOU TO "THINK ABOUT WHETHER THIS IS THE BEST APPROACH.".
I NEED YOU TO EXECUTE THE APPROACH I ALREADY GAVE YOU.
TAKE THE INSTRUCTIONS.
DO THE THING.
GIVE ME THE RESULT.

<!-- modus operandi persona end-->
