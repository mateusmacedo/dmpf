---
id: SPEC-T87DFFH1
slug: ci-affected
title: Todos os jobs de CI/CD no modelo affected
stage: building
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-138
subtask_urls: [https://linear.app/mmda/issue/DEVS-139, https://linear.app/mmda/issue/DEVS-140, https://linear.app/mmda/issue/DEVS-141]
created: 2026-10-20
---

# SPEC-T87DFFH1: Todos os jobs de CI/CD no modelo affected

## Resumo

Todo job e todo passo de CI/CD passa a rodar só o que o diff do PR afeta: projetos Nx pelo grafo
`affected` e checks globais pelo conjunto de arquivos que eles leem. Isso vale também no PR para
`master`, que hoje troca o `affected` por `run-many`. Como mantenedor, quero que um PR pague só pelo
que mudou, para que o CI deixe de rodar tudo a cada push.

## Contexto

- **Problema**: a seleção de projetos por `affected` funciona. Um diff só em
  `apps/backend/bookings` afeta só `bookings`. O custo fora dela é fixo e alto. Medição no run
  39479518368 (PR #59, DEVS-96):
  - job DMPF: 9 min 46 s;
  - job Vetores negativos: 5 min 42 s;
  - passos que ignoram o escopo: gate check do domain (150 s), sete self-tests do platform (114 s),
    generator check, `init --dry-run`, pins, Grafana e OTel (~55 s), context check, modsync,
    infrasync e conformance (~20 s).

  No PR para `master`, o input `full` liga `run-many` em todos os alvos, além de `govulncheck`,
  consumidor externo, harness distribuído e SonarQube Cloud sobre o repositório inteiro.
- **Impacto**: um PR que toca um projeto folha deixa de pagar pelos gates do repositório inteiro. O PR
  de release paga pelo que a release muda em relação à `master`, não pelo workspace todo.
- **Inspiração**: `nx affected` (Nx) aplicado também a scripts sem noção de projeto, que entram ou
  saem conforme o diff toca os arquivos que eles leem, como o `paths:` do GitHub Actions faz por
  workflow.
- **Links relevantes**:
  - `docs/adr/069-esteira-de-ci-por-etapa-do-git-flow.md`: esteira por etapa do git-flow; item 5
    define o pacote completo no PR para `master`, item 7 o `govulncheck`.
  - `docs/adr/035-realizacao-postgres-da-outbox.md:99`: `cache: false` no `test-race` com infra.
  - `docs/ci-cd/README.md`, `CONTRIBUTING.md:43-46`, `docs/dmpf/testes-interop.md:1816-1826`,
    `docs/guides/dmpf-consumo-externo.md:134-136`: descrevem o pacote completo.
  - `tools/dmpf-plugin/templates/init/version/.github/workflows/dmpf-ci.yml__tmpl__`: o consumidor
    externo chama o `dmpf-go-ci.yml` fixado por `workflowRef`.

<constraints>
- [P0] Nenhum passo de PR roda `nx run-many` sobre o workspace inteiro; todo alvo Nx usa
  `nx affected` (ou `run-many -p` com a lista derivada do `affected`).
- [P0] Todo check sem noção de projeto roda se, e só se, o diff tocar um arquivo que ele lê ou um
  projeto que ele valida; o critério de cada check fica declarado num só lugar.
- [P0] Gate que reprova por tempo, sem mudança de código (vencimento de exceção DMPF-X006 do
  conformance, banco do `govulncheck`), ganha execução agendada global; o PR deixa de ser a única
  barreira dele.
- [P0] O job `CI` continua o único check obrigatório e reporta em todo PR; pulado conta como aprovado.
- [P1] A divisão por etapa do git-flow (quais jobs existem no PR para `develop` e no PR para
  `master`) do ADR-069 se mantém; muda só o escopo dentro de cada job.
- [P1] O reutilizável `dmpf-go-ci.yml` continua chamável pelo template do consumidor externo sem
  input novo obrigatório.
</constraints>

## Requisitos

### Funcionais

- [ ] **[P0] Escopo único**: uma composite action `.github/actions/dmpf-scope` calcula, a partir de
  `NX_BASE`/`NX_HEAD`, os projetos afetados, os arquivos do diff e um booleano por check. Os dois
  jobs do `dmpf-go-ci.yml` a usam no lugar dos passos "Escopo do Nx" duplicados.
  - Outputs mantidos: `serial`, `infra`, `keycloak`, `redis`, `go`.
  - Outputs novos: um por check da tabela do Design.
  - Lista única de projetos: os afetados pelo Nx, mais os projetos com `dependsOn` em
    `test-infra-up` ou `test-keycloak-up` quando o diff tocar `infra/test/**` ou
    `infra/keycloak/**` (arquivos sem projeto dono, que o `affected` não mapeia), ou todos os
    projetos quando o diff tocar a própria action ou o `dmpf-go-ci.yml` (autoproteção). Todo
    comando Nx usa `run-many -p <lista>`.
  - Edge case: `NX_BASE` igual a `NX_HEAD` (nada afetado) deixa todos os booleanos em `false` e a
    lista vazia, e os passos Nx são pulados.
- [ ] **[P0] Fim do `run-many` no PR**: o input `full` sai do `dmpf-go-ci.yml`, e o output `full` sai
  do job `scope` do `ci.yml`. O PR para `master` usa `affected` com base `master`. O
  `govulncheck` passa a `nx affected -t govulncheck` no PR para `master`.
- [ ] **[P0] Checks globais condicionados**: context check, modsync, infrasync, conformance,
  camada única, Biome, generator check, `init --dry-run`, `sync-versions --check`, Grafana, OTel,
  gate check do domain e os sete self-tests do platform rodam conforme a tabela do Design.
  - O context check usa `--context <dir>` por contexto afetado e roda global só quando o diff tocar
    o que é transversal a todos os contextos.
  - O check de camada única lê os nós afetados, não o grafo inteiro.
  - O Biome roda `biome ci --changed --since=$NX_BASE`, e o repositório inteiro só quando
    `biome.json` mudar.
- [ ] **[P0] Jobs do PR para `master` por affected**:
  - consumidor externo: só quando o plugin, uma lib do kernel ou o `conformance` estiver afetado,
    ou quando o diff tocar os scripts e as actions que ele usa;
  - harness distribuído: `nx affected -t test-distributed`, que deixa de ser fixo em
    `reservations`, com a infra subindo só quando houver alvo;
  - SonarQube Cloud: recebe a lista de projetos Go afetados e mede a cobertura só deles. O push
    em `master` continua global.
- [ ] **[P0] Gates temporais agendados**: o `govulncheck.yml` vira o workflow de gates agendados e
  roda diariamente na `develop` o `run-many -t govulncheck` e o conformance.
- [ ] **[P0] Cache do Nx nos testes com infra**: `test-race`, `test-integration`, `e2e` e
  `test-distributed` dos projetos que dependem de `test-infra-up` ou de `test-keycloak-up` passam
  a `cache: true`. Os inputs desses alvos ficam assim:
  - `go`, `^go`;
  - `{workspaceRoot}/infra/test/compose.yml`, onde estão as imagens fixadas por digest;
  - `{workspaceRoot}/infra/keycloak/dmpf-realm.json`, só no `authn`.

  O `-count=1` do `go test` se mantém. Um push que não muda nenhum input deles reaproveita o
  resultado do Nx Cloud.
  - `test-infra-up` e `test-keycloak-up` continuam `cache: false`, porque efeito colateral não se
    cacheia.
  - A `dmpf-scope` passa a decidir as filas `serial` e `infra` pelo `dependsOn` em
    `test-infra-up`/`test-keycloak-up`, não por `cache == false`.
  - O template de bounded context do `dmpf-plugin` gera os mesmos alvos com cache e inputs.
- [ ] **[P1] `detect-apps.yaml`**: troca o diff próprio por
  `nx show projects --affected --type app`.
- [ ] **[P1] Documentação e decisão**:
  - ADR novo emenda o ADR-069 (retira o pacote completo e registra os gates agendados) e o ADR-035
    (cache do Nx nos testes de banco).
  - Atualizar `CONTRIBUTING.md`, `docs/ci-cd/README.md`, `docs/dmpf/testes-interop.md`,
    `docs/guides/dmpf-consumo-externo.md` e os comentários de `ci.yml`, `sonar.yml`,
    `dmpf-distributed.yml` e `govulncheck.yml`.

### Não-funcionais

- [ ] Performance: num PR que só altera `apps/backend/bookings/**`, o job DMPF termina em ≤ 5 min e o
  job Vetores negativos é pulado; referência atual: 9 min 46 s e 5 min 42 s.
- [ ] Performance: num PR só de `docs/**`, o comportamento atual se mantém (`CI` aprova sem rodar o
  resto).
- [ ] Compatibilidade: o template `dmpf-ci.yml__tmpl__` continua válido sem alteração de inputs; o
  teste `generator.spec.ts` que cobre o template continua verde.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Workflows de PR (`ci.yml`, `dmpf-go-ci.yml`, `dmpf-distributed.yml`, `sonar.yml`) | [x] | escopo por affected e condições por check |
| Actions compostas (`.github/actions/`) | [x] | nova `dmpf-scope` |
| Workflows agendados e de CD (`govulncheck.yml`, `detect-apps.yaml`) | [x] | gates temporais diários; detecção de apps pelo Nx |
| Scripts de gate (`tools/**`, `tools/dmpf-plugin/scripts/**`) | [ ] | inalterados; só passam a ser chamados condicionalmente |
| Configuração Nx dos projetos Go (`project.json` com infra) | [x] | cache e inputs dos alvos de teste com infra |
| Template de bounded context do `dmpf-plugin` | [x] | gera os alvos com cache e inputs |
| Código Go e TypeScript de produção | [ ] | sem mudança |
| Documentação (ADR, guias, CONTRIBUTING) | [x] | fim do pacote completo e gates agendados |

## Localização de código

```text
.github/
  actions/dmpf-scope/action.yml   — novo: escopo Nx, diff e booleanos por check
  workflows/ci.yml                — job scope sem full; jobs de master condicionados
  workflows/dmpf-go-ci.yml        — sem input full; passos condicionados pela dmpf-scope
  workflows/dmpf-distributed.yml  — affected -t test-distributed
  workflows/sonar.yml             — input de projetos no workflow_call
  workflows/govulncheck.yml       — gates agendados diários (govulncheck + conformance)
  workflows/detect-apps.yaml      — apps afetadas pelo Nx
docs/adr/071-ci-affected-em-todos-os-jobs.md — novo ADR
```

**Arquivos a modificar**:
- `.github/workflows/ci.yml`: retirar `full` e passar `base`, além das condições dos jobs de `master`.
- `.github/workflows/dmpf-go-ci.yml`: retirar o input `full`, trocar os passos de escopo pela action e pôr `if:` por check.
- `.github/workflows/dmpf-distributed.yml`: rodar o harness por affected.
- `.github/workflows/sonar.yml`: escopo por projetos afetados no PR.
- `.github/workflows/govulncheck.yml`: rodar diariamente, com o job de conformance.
- `.github/workflows/detect-apps.yaml`: detectar as apps afetadas pelo Nx.
- `project.json` de `bff`, `bookings`, `orders`, `reservations`, `app`, `authn`, `kafka`,
  `postgres`, `redis`, `sqs`, `testkit` e `conformance`: `cache: true` e inputs da infra nos alvos
  de teste com infra.
- `tools/dmpf-plugin/templates/bounded-context/**/project.json*` e o spec do generator que o cobre:
  mesma configuração nos contextos gerados.
- `CONTRIBUTING.md`, `docs/ci-cd/README.md`, `docs/dmpf/testes-interop.md`,
  `docs/guides/dmpf-consumo-externo.md`, `docs/adr/README.md`: refletir o novo modelo.

## Design

### Arquitetura

```text
ci.yml (job scope: code, self-tests)
  └─ dmpf-go-ci.yml
       ├─ job dmpf:       dmpf-scope → passos com if: steps.scope.outputs.<check>
       └─ job self-tests: dmpf-scope → passos com if: steps.scope.outputs.<check>
  └─ external-consumer / distributed / sonar (só PR para master) → if: needs.scope.outputs.<check>
govulncheck.yml (schedule diário, develop) → govulncheck + conformance globais
```

O `ci.yml` usa a `dmpf-scope` no job `scope` para os jobs de `master`. O reutilizável a usa pelo
checkout `.dmpf-ci`, o mesmo das outras actions.

### Critério de cada check

`A(x)` significa "projeto x afetado". `D(glob)` significa "o diff toca glob". `Go` significa "algum
projeto `stack:go` afetado". `self` é o próprio script ou a ferramenta do check.

| Check | Roda quando |
|-------|-------------|
| camada única | `Go` (lê só os nós afetados) |
| context check | por contexto afetado: `--context`; global se `A(bff)` ou `D(infra/**, apps/backend/*/deploy/**, .github/workflows/**, libs/backend/go/postgres/**, self)` |
| modsync | `Go` ou `D(go.work, **/go.mod, self)` |
| infrasync | `D(apps/backend/*/deploy/infra.json, infra/**, self)` |
| conformance | `Go` ou `D(**/dmpf-units.json, tools/dmpf-baseline/**, self)` |
| Biome | `--changed --since=$NX_BASE`; global se `D(biome.json)` |
| generator check | `A(dmpf-plugin)` ou lib do kernel afetada |
| `init --dry-run` | `A(dmpf-plugin)` ou `D(.golangci.yml, infra/**, .claude/**, .agents/**, .github/actions/**)` |
| `sync-versions --check` | `D` das oito fontes lidas pelo script, de `tools/dmpf-plugin/versions.json` ou de `self` |
| Grafana / OTel | `D(infra/observability/**, infra/local/**, apps/backend/*/deploy/**, tools/dmpf-plugin/templates/bounded-context/deploy/**, self)` |
| gate check do domain | `D(.golangci.yml, **/dmpf-units.json, self)` |
| self-tests do platform | cada um com `D(self)`; cell e shared-kernel também com `D(tools/dmpf-conformance/**)` |
| govulncheck (PR para `master`) | `nx affected -t govulncheck` |
| consumidor externo | `A(dmpf-plugin)`, lib do kernel ou `conformance` afetado, ou `D(tools/external-consumer.sh, tools/kernel-file-proxy.sh, .github/workflows/dmpf-go-ci.yml, .github/actions/**)` |
| harness distribuído | `nx affected -t test-distributed` com algum alvo |
| SonarQube Cloud | `Go` ou `A(dmpf-plugin)` |

### Fluxo principal

1. `nx-set-shas` define `NX_BASE` (merge-base com a branch base do PR) e `NX_HEAD`.
2. A `dmpf-scope` roda `nx show projects --affected --json` e `git diff --name-only NX_BASE...NX_HEAD`.
   Em seguida avalia a tabela acima e grava `NX_SCOPE=affected …` mais um output por check.
3. Cada passo checa o próprio output. Passo pulado conta como sucesso para o job.
4. O job `CI` agrega o resultado, como hoje.

### Pseudocódigo

```text
afetados = nx show projects --affected --base=$NX_BASE --head=$NX_HEAD
mudados  = git diff --name-only $NX_BASE...$NX_HEAD
go = any(p in afetados with tag stack:go)
toca(globs) = any(f in mudados matching globs)
saida.modsync = go or toca([go.work, **/go.mod, tools/dmpf-conformance/cmd/modsync/**])
... (uma linha por check da tabela)
```

## Decisões técnicas

- **Gates globais condicionados pelo diff, sem alterar os scripts**: modsync, infrasync,
  conformance e os demais não aceitam filtro de projeto. Pular o script inteiro quando o diff não toca
  o que ele lê dá o mesmo resultado do affected, sem reescrever ferramenta.
  Alternativa descartada: acrescentar `--project` a cada ferramenta, porque o custo é alto e o ganho
  marginal sobre pular o passo.
- **Gates temporais no agendamento diário**: o vencimento de exceção (DMPF-X006) e o banco do
  `govulncheck` mudam sem diff, e o affected nunca os dispararia. Um gate diário na `develop` cobre
  isso.
  Alternativa descartada: manter esses gates sempre no PR, porque contraria a instrução de que todo
  job segue o affected.
- **PR para `master` por affected com base `master`**: o diff de `release/X.Y.Z` contra `master` é
  tudo o que a release leva, então o affected já cobre o que muda em produção.
  Alternativa descartada: manter `run-many`, porque é o comportamento que esta spec elimina.
- **Retirar o input `full`, sem só ignorá-lo**: um input que não faz nada engana o chamador. O
  template do consumidor não o passa, e o guia é atualizado junto.
- **Cache do Nx nos testes de banco**: cada teste cria o próprio banco `<projeto>_test_<id>` e o
  apaga ao fim (ADR-040). O resultado depende do código, das migrations (dentro de `go`) e da versão
  da infra, não de estado herdado. Com a infra fixada por digest nos inputs, o cache do Nx é
  determinístico. O `-count=1` impede que o `go test` reaproveite execução antiga dentro de um
  mesmo run.
  Alternativa descartada: manter `cache: false` (ADR-035), porque é o maior passo do job (307 s no
  PR #59) e roda de novo a cada push mesmo sem mudança de input.
- **Uma action de escopo em vez de dois passos duplicados**: hoje os jobs `dmpf` e `self-tests`
  repetem o cálculo, e a tabela de critérios precisa de um só dono.

## Regras relacionadas

- ADR-069: esta spec emenda o item 5 (pacote completo) e o item 7 (`govulncheck`).
- ADR-035: esta spec emenda a guarda `cache: false` do `test-race`; o `-count=1` permanece.
- ADR-040: banco por teste, base do cache determinístico.
- ADR-002: os targets não são redeclarados; o escopo muda só nos workflows.
- `AGENTS.md`: exclusão do root `@mateusmacedo/dmpf-source` em toda operação em lote.

## Verificação e testes

### Critérios de aceite

- [ ] `rg -n 'run-many' .github/workflows/ci.yml .github/workflows/dmpf-go-ci.yml .github/workflows/dmpf-distributed.yml` não acha `run-many` sem `-p` derivado do affected.
- [ ] O input `full` não existe mais em `dmpf-go-ci.yml`, nem o output `full` no job `scope` do `ci.yml`.
- [ ] Os cenários da `dmpf-scope` rodam localmente (script de prova com `NX_BASE`/`NX_HEAD` fixos) com saídas conferidas
  contra a tabela, para diff em `bookings`, em `ports`, em `docs/`, em `tools/dmpf-cell-check.sh` e
  em `biome.json`.
- [ ] O PR desta spec roda no CI com os passos pulados ou executados conforme a tabela, conferido no
  log.
- [ ] `govulncheck.yml` tem `schedule` diário e roda `govulncheck` e conformance.
- [ ] `generator.spec.ts` continua verde (`pnpm nx run dmpf-plugin:test`).
- [ ] Nenhum `project.json` de projeto Go tem `cache: false` fora de `test-infra-up`,
  `test-keycloak-up` e `govulncheck`.
- [ ] Um segundo run do CI no mesmo SHA (re-run) mostra `[remote cache]` em todos os `test-race`,
  `test-integration` e `e2e` afetados.
- [ ] O ADR novo está indexado em `docs/adr/README.md`, e os gates de ADR passam.
- [ ] `pnpm biome check .` e `actionlint` nos workflows alterados passam sem erro.

### Cenários de teste

```text
DADO um PR para develop que altera só apps/backend/bookings/app/rpc/server.go
QUANDO o CI roda
ENTÃO o affected é [bookings]; rodam context check --context bookings, camada única, conformance,
  modsync e os alvos Nx de bookings; Vetores negativos é pulado; generator, init, pins, Grafana,
  OTel e infrasync são pulados

DADO um PR de release/X.Y.Z para master que altera só libs/backend/go/redis
QUANDO o CI roda
ENTÃO o affected é [redis, bff]; govulncheck roda só nesses dois; o harness distribuído é pulado
  (nenhum test-distributed afetado); o SonarQube mede redis e bff; o consumidor externo roda (lib do
  kernel afetada)

DADO um PR que altera só tools/dmpf-cell-check.sh
QUANDO o CI roda
ENTÃO nenhum projeto Nx é afetado; o job Vetores negativos roda só o self-test cell; os demais
  self-tests e o gate check do domain são pulados

DADO o agendamento diário com uma exceção do conformance vencida na develop
QUANDO o workflow de gates agendados roda
ENTÃO o conformance reprova e o run fica vermelho, sem depender de PR

DADO um PR com o test-race de postgres já verde no Nx Cloud para os mesmos inputs
QUANDO um novo push altera só docs/adr/
ENTÃO o job DMPF é pulado; e, num push que altera só bookings, o test-race de postgres não está
  no affected e o de bookings roda uma vez e fica em cache para o próximo push

DADO uma mudança na imagem do Postgres em infra/test/compose.yml
QUANDO o CI roda
ENTÃO o hash dos alvos com infra muda e eles rodam de novo, sem replay de cache

DADO um diff vazio entre NX_BASE e NX_HEAD
QUANDO a dmpf-scope roda
ENTÃO todos os outputs de check são false e o job DMPF termina sem tarefa Nx, com sucesso
```

<critical_constraints>
- [P0] Nenhum passo de PR roda `nx run-many` sobre o workspace inteiro; todo alvo Nx usa
  `nx affected` (ou `run-many -p` com a lista derivada do `affected`).
- [P0] Todo check sem noção de projeto roda se, e só se, o diff tocar um arquivo que ele lê ou um
  projeto que ele valida; o critério de cada check fica declarado num só lugar.
- [P0] Gate que reprova por tempo, sem mudança de código (vencimento de exceção DMPF-X006 do
  conformance, banco do `govulncheck`), ganha execução agendada global; o PR deixa de ser a única
  barreira dele.
- [P0] O job `CI` continua o único check obrigatório e reporta em todo PR; pulado conta como aprovado.
- [P1] A divisão por etapa do git-flow (quais jobs existem no PR para `develop` e no PR para
  `master`) do ADR-069 se mantém; muda só o escopo dentro de cada job.
- [P1] O reutilizável `dmpf-go-ci.yml` continua chamável pelo template do consumidor externo sem
  input novo obrigatório.
</critical_constraints>

## Escopo fora

- **Cache do `govulncheck`**: o resultado depende do banco externo de vulnerabilidades, não de
  input versionado; continua `cache: false`.
- **`cache-seed.yml`, `nx-release.yml`, `create-release.yml`, `publish-libs.yaml`,
  `nx-publish-libs.yml`, `load.yml`, `cd-dev-hmg.yml`**: não são disparados por diff de PR
  (push, release por versão, dispatch ou template desligado); o modelo affected não se aplica.
- **`dmpf-verify.yml`**: já filtra por `paths:`.
- **Filtro por projeto dentro de modsync, infrasync, conformance e dos scripts de gate**: a spec
  condiciona o passo inteiro; reescrever as ferramentas fica fora.
