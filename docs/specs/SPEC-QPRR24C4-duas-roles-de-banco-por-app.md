---
id: SPEC-QPRR24C4
slug: duas-roles-de-banco-por-app
title: DMPF — Duas roles de banco por app: migrador fora do runtime e runtime com privilégio por tabela e capability
stage: building
priority: P1
depends_on: [SPEC-F308KDSF, SPEC-JJKWG4JP, SPEC-VJMM2DE5]
ticket_url: https://linear.app/mmda/issue/DEVS-94/spec-qprr24c4-dmpf-duas-roles-de-banco-por-app-migrador-fora-do
subtask_urls: []
created: 2026-10-12
---

# SPEC-QPRR24C4: Duas roles de banco por app

## Resumo

Separar, em cada app do kernel, a role que cria e altera o schema (a migradora,
dona do banco) da role com a qual a app serve (a de runtime, sem posse de objeto
e com privilégio declarado por tabela). Hoje a app conecta com uma única role,
dona do banco, por um único `PG_DSN`, e migra pela mesma pool: nenhum `REVOKE`
protege uma tabela contra a própria aplicação, e a row-level security que o
ADR-051 reservou como segunda camada não tem efeito sobre um dono. A spec entrega
a pré-condição de privilégio mínimo no banco: dois DSN por app, provisionamento
das duas roles em Compose, Kustomize e testkit, e grants por tabela e por
capability do pacote `postgres`, com append-only apenas onde a app declarar.

Origem: consumidor externo antecipa-market, SPEC-GXGASP24 (DEVS-76), que precisa
de uma tabela de auditoria imutável contra a role de runtime da própria app.

"Migrador fora do runtime" quer dizer fora da role de runtime: o DSN do migrador
continua no ambiente do `api` com `MIGRATE=true`. A imutabilidade contra o próprio
processo, se comprometido, só valeria com a migração fora do processo servidor, o
que esta spec não entrega (risco residual em "Riscos" e em "Escopo fora").

## Contexto

### Problema

- A role de cada app é criada `LOGIN` e dona do banco, de propósito, para a app
  criar o próprio schema sem privilégio administrativo
  (`infra/local/compose/provisioning.generated.yml:17-29`; mesmo comando no Job
  de `dev`, `tools/dmpf-conformance/infrasync/templates/databases-job.yaml.tmpl:2-3,29-33`;
  ADR-053:21; `infra/README.md:46`). Quem é dono da tabela pode reconceder a si o
  que um `REVOKE` tirou e pode alterá-la, então o privilégio por tabela não
  protege nada enquanto runtime e dono forem a mesma role.
- O `PG_DSN` é único por app (`infra/README.md:68-76`;
  `apps/backend/orders/deploy/compose.yml:12`; `apps/backend/orders/app/config.go:42,128`),
  e o `infra.json` declara uma só senha de banco, `database.passwordEnv`
  (`apps/backend/orders/deploy/infra.json:8-12`; `tools/dmpf-conformance/infrasync/infrasync.go:51-55`).
- `postgres.Migrate` recebe a pool do runtime (`libs/backend/go/postgres/migrate.go:59-85`).
  `serve.APIRole{DSN, Migrate}` só escolhe entre migrar e esperar as tabelas
  (`libs/backend/go/app/serve/serve.go:48-55,196-207`): as duas ramificações usam
  a pool aberta com o `DSN` único (`libs/backend/go/app/serve/serve.go:121,159`), e `relay` e `consumer`
  esperam o schema pela mesma pool (`libs/backend/go/app/serve/serve.go:244,252`;
  `apps/backend/reservations/app/wiring.go:127`).
- O DDL do kernel traz `ALTER TABLE`, `UPDATE` e `DELETE` de atualização de
  schema (`libs/backend/go/postgres/inbox.sql:18-31,48-63`): é código de migrador
  e hoje roda com a role que também serve requisições.
- O ADR-051:13-19 mediu a role de conexão como superuser e dona das tabelas, e por
  isso descartou a RLS (`:34`); registrou que ela voltaria "quando existir um role de aplicação que não
  seja superuser nem dono das tabelas" (`:50`). Desde o ADR-053 a role de cada
  app é criada por `CREATE ROLE ... LOGIN`, sem o atributo `SUPERUSER`, e é dona
  do banco: a condição de `:50` continua por cumprir.

### O que o runtime executa hoje

Verificado nos statements do kernel; é o insumo do conjunto de grants, não o
conjunto final (ver "Não verificado"). Os caminhos da coluna "Fonte" são relativos
a `libs/backend/go/`.

| Tabela | Statements | Fonte |
| ------ | ---------- | ----- |
| `outbox` | `INSERT` | `postgres/outbox.go:23` |
| `outbox` | `UPDATE ... FOR UPDATE SKIP LOCKED` (claim), `UPDATE` (publicado, reagendado, falho) | `postgres/claim.go:57-59,76,92,99` |
| `outbox` | `DELETE` por purga, `SELECT destination` | `postgres/purge.go:12-14`, `postgres/pool.go:194` |
| `outbox` | `SELECT` de sinais (relay) | `postgres/signals.go:23-29` |
| `inbox` | `INSERT ... ON CONFLICT DO NOTHING`, `INSERT ... ON CONFLICT DO UPDATE`, `SELECT`, `UPDATE` | `postgres/inbox.go:19-21,24-31,32,35` |
| `inbox` | `DELETE` por purga, também no processo `api` (purga do command-inbox, `app/serve/serve.go:162-164`) | `postgres/purge.go:42-44,75-77` |
| `quarantine` | `INSERT ... ON CONFLICT DO NOTHING`, `SELECT ... GROUP BY` | `postgres/quarantine.go:16-18`, `postgres/signals.go:11` |
| agregado (`Table`) | `SELECT`, `INSERT ... ON CONFLICT (tenant_id, id) DO NOTHING`, `UPDATE ... WHERE version = $3`; não há `DELETE` | `postgres/table.go:174-192` |
| agregado (`Relation`) | `SELECT` | `postgres/table.go:82-87` |

### Não verificado

- O atributo de superuser da role de conexão em Kubernetes e no CI. Os arquivos
  de hoje declaram `POSTGRES_USER=postgres` (`infra/k8s/base/postgres/kustomization.yaml:22`;
  `infra/test/compose.yml:13`; `.github/workflows/dmpf-go-ci.yml:169`), e o
  ADR-051:13-19 registrou superuser numa revisão anterior desses arquivos; nenhum
  `pg_roles.rolsuper` foi consultado nesta revisão.
- Os privilégios mínimos reais por tabela e por processo: a tabela acima lista
  statements, não `GRANT`. Falta confirmar o que cada um exige (privilégio de
  sequência das colunas `IDENTITY` de `outbox` e `quarantine`, `SELECT` nas
  cláusulas `WHERE`, `to_regclass` em `WaitForTables`, `ctid` nas purgas).
- `INSERT ... ON CONFLICT DO NOTHING` com role sem `UPDATE`: o padrão está em
  `postgres.Table`, `inbox` e `quarantine`, e não foi exercitado contra uma tabela
  só com `SELECT` e `INSERT`.

### Fontes normativas

- `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md`, `053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md`, `054-apps-autocontidos-e-infras-separadas.md` (itens 4 e 7), `060-o-dmpf-tem-consumidor-externo.md`.
- `docs/dmpf/contexto-erros-seguranca.md`: IDN-14 (`:881`), DAT-11 (`:1492`) e DAT-12 (`:1493`), que põe outbox e quarantine entre as superfícies de dado de negócio sujeitas a menor privilégio.

<constraints>
- [P0] Em app com migrador declarado, a role de runtime nunca é dona de banco, schema ou tabela, nunca executa DDL nem `TRUNCATE`, e só recebe o privilégio que o kernel ou a app declara: tabela sem grant declarado fica inacessível a ela.
- [P0] As tabelas do kernel (`outbox`, `inbox`, `quarantine`) mantêm os privilégios que os statements do kernel exigem, inclusive `UPDATE` e `DELETE` em `outbox` e `inbox`; append-only vale só para a tabela que a app declara como tal.
- [P0] App cujo `infra.json` não declara migrador continua válida e é provisionada com a role única de hoje: nenhum consumidor quebra sem migration, e `modsync --check`, `infrasync --check`, `conformance` e o job `external-consumer` seguem verdes.
- [P0] `postgres.Migrate` mantém assinatura e semântica (`IF NOT EXISTS`, sob o mesmo `migrateLockID`), e nenhuma tabela ou coluna existente é renomeada.
- [P0] DSN e senha do migrador nunca aparecem em log, métrica, span nem mensagem de erro.
- [P1] O banco por teste (ADR-054) e a recusa de servidor fora do cluster `test` permanecem: role e grants de teste só existem no cluster de teste.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</constraints>

## Requisitos

### Funcionais

**A. Provisionamento das duas roles**

- [x] **[P0] Bloco `database.migrator` no `infra.json`**: o `infrasync` aceita `database.migrator` opcional, com `passwordEnv`, `local` e `dev`, validado como o `database` de hoje (`tools/dmpf-conformance/infrasync/infrasync.go:148`: nome de variável e valores não vazios). Sem o bloco o manifesto segue válido e a saída gerada não muda.
- [x] **[P0] Provisionamento em Compose e Kustomize**: com o bloco presente, `provisioning.yml.tmpl` e `databases-job.yaml.tmpl` criam a role `<app>_migrator` (`LOGIN`), dona do banco `<app>`, e mantêm a role `<app>` (`LOGIN`) sem posse; concedem à de runtime `CONNECT` e `USAGE` em `public` e revogam dela `CREATE` em `public`. A pré-condição é PostgreSQL ≥ 15, em que `PUBLIC` já não tem `CREATE` em `public` (as imagens fixadas são a 16). Servidor ≤ 14 só é aceito se o provisionamento aplicar `REVOKE CREATE ON SCHEMA public FROM PUBLIC` com superuser em cada banco de app, porque revogar só da role de runtime não neutraliza o `CREATE` concedido a `PUBLIC`: os templates de Compose e do Job emitem esse `REVOKE` (sem efeito em ≥ 15), e o servidor de `hmg`, provisionado fora do repositório, tem a versão registrada pela investigação e o `REVOKE` documentado no ADR novo. Os dois comandos também reconciliam os atributos da role de runtime (`NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS`) e revogam `TEMPORARY` de `PUBLIC` no banco da app, porque o PostgreSQL concede `TEMPORARY` a `PUBLIC` por padrão e a role de runtime criaria tabela temporária (o kernel não usa nenhuma). Os dois comandos continuam idempotentes e reconciliam as duas senhas a cada execução. O DSN do migrador vai num Secret à parte, `<app>-migrator`, com a chave `PG_MIGRATE_DSN`, referenciado só pelo Deployment do `api`: o Secret `<app>` entra por `envFrom` em `api`, `relay` e `consumer`, e uma chave nova nele chegaria aos três. O `secrets.example.yaml.tmpl` do overlay `hmg` de cada app ganha esse Secret, e o overlay `dev` o gera por `secretGenerator`.
- [x] **[P0] `PG_MIGRATE_DSN`**: o `FromEnv` de `orders`, `reservations`, `bookings` e do `config.go__tmpl__` do generator lê `PG_MIGRATE_DSN` ao lado de `PG_DSN`. `compose.yml`, `.env.example` e `k8s` (base, `dev`, `hmg`) a declaram só no processo com `MIGRATE=true`; `relay` e `consumer` não a recebem. No Compose ela fica no serviço `api`, fora da âncora `x-env` que o `relay` e o `consumer` compartilham.

**B. Migração pela role migradora**

- [x] **[P0] `serve.APIRole.MigrateDSN`**: com `Migrate` verdadeiro e `MigrateDSN` preenchido, `prepare` (`libs/backend/go/app/serve/serve.go:196-207`) abre uma pool efêmera com `MigrateDSN`, roda `postgres.Migrate` e os grants, e a fecha antes de o `ready` devolver; a pool de requisições nunca executa DDL. Com `MigrateDSN` vazio, migra pela pool do `DSN`, como hoje, e não aplica grants.
- [x] **[P0] Role de runtime derivada do DSN**: a role que recebe os grants é o usuário de `DSN` (`pgxpool.ParseConfig`), sem variável nova para o mesmo fato.

**C. Grants por tabela e por capability**

- [x] **[P0] Tipos de grant no `postgres`**: `Privilege` (`SELECT`, `INSERT`, `UPDATE`, `DELETE`), `Grant{Table, Privileges}` e os construtores `Mutable(table)` (`SELECT`, `INSERT`, `UPDATE`: tabela de agregado escrita por `Table`) e `AppendOnly(table)` (`SELECT`, `INSERT`). `TRUNCATE` e `REFERENCES` nunca são concedidos.
- [x] **[P0] `KernelGrants(caps ...Capability)`**: devolve os grants das tabelas de `Tables(caps...)` (`libs/backend/go/postgres/migrate.go:44-50`). A matriz inicial é a do Design, e a investigação abaixo a confirma ou corrige antes do merge. Toda capability nova do kernel declara os grants das suas tabelas junto de `schema` e `tables` em `capabilities` (`libs/backend/go/postgres/migrate.go:37-40`) e fica coberta por `KernelGrants`; um teste do pacote `postgres` percorre `capabilities` e falha se alguma tabela de capability não tem grant declarado. O ledger `external_attempt` da SPEC-5AQHHMJ0 é o caso previsto: entra em `KernelGrants` com a matriz que o `Begin` e o `Finish` exigirem.
- [x] **[P0] `ApplyGrants`**: converge a role ao conjunto declarado (`REVOKE ALL` e `GRANT` exato por tabela), é idempotente, roda sob o mesmo `migrateLockID` e valida os nomes de tabela como identificadores SQL (a regra de `libs/backend/go/postgres/table.go:219-231`); a role é citada por `pgx.Identifier`, porque o nome da app admite hífen (`tools/dmpf-conformance/infrasync/infrasync.go:29`). Antes de mudar qualquer privilégio, reprova a partida quando a role de runtime é a própria `current_user`, é superuser, tem `BYPASSRLS`, é membro da migradora ou é dona de alguma relação em `public`, porque o privilégio efetivo soma ACL, membership e atributo da role. O lock, o `REVOKE ALL` e os `GRANT` rodam numa única transação, para uma réplica que reinicia reaplicar os grants sem deixar os pods vivos, entre o `REVOKE` e o `GRANT`, sem privilégio (SQLSTATE 42501).
- [x] **[P0] Espera por privilégio, não só por tabela**: `postgres.WaitForGrants(ctx, pool, interval, logs, grants...)`, ao lado de `WaitForTables`, espera até `has_table_privilege(current_user, <tabela>, <privilégio>)` ser verdadeiro para cada privilégio de cada `Grant`, sobre `to_regclass`, que devolve NULL para tabela ausente. Hoje `WaitForTables` (`libs/backend/go/postgres/wait.go:26-37`) devolve assim que a tabela existe, o que acontece quando o `Migrate` comita, antes do `ApplyGrants`: o processo que espera abre a primeira consulta recusada com SQLSTATE 42501. Os processos que esperam o schema (o `api` com `MIGRATE=false`, em `libs/backend/go/app/serve/serve.go:199`; o `relay`, em `:252`; o `consumer`, como `apps/backend/reservations/app/wiring.go:127`) passam a esperar, em cada tabela, o privilégio que a investigação por processo fixou para as do kernel e o que `Grants` declara para as da app. App sem migrador (`MigrateDSN` vazio) segue esperando por tabela, como hoje. Um teste de integração prova que, com a tabela criada e sem grant, a espera não devolve, e que devolve depois do `ApplyGrants`.
- [x] **[P0] Declaração pela app**: `serve.API` ganha `Grants []postgres.Grant`, ao lado de `Schema` (`libs/backend/go/app/serve/serve.go:59-65`); o provider de cada contexto exporta `Grants` junto de `Schema` (`provider/schema.go`). A app aplica `KernelGrants` das capabilities que migra mais `API.Grants`.
- [x] **[P1] Tabela sem grant**: depois de migrar, `ApplyGrants` reprova a partida listando toda tabela de `public` que não é do kernel e não consta em `Grants`, para a falha aparecer na subida e não na primeira consulta.
- [x] **[P1] Erro de privilégio**: statement recusado por privilégio (SQLSTATE 42501) volta como erro de infraestrutura que preserva `*pgconn.PgError`; `Table.Save` nunca o devolve como `ports.ErrVersionConflict` nem `ports.ErrNotFound`.

**D. Testkit**

- [x] **[P0] Role de runtime nos testes**: `tb/pg` ganha `OpenRuntimePool(t, opts)`, que no banco por teste cria a role de runtime do cluster `test` (idempotente, tolerando corrida entre processos), aplica `KernelGrants(opts.Capabilities)` e `opts.Grants`, e devolve a pool dela. `OpenPool` segue migrando com as credenciais administrativas, e `ResetTables` (`libs/backend/go/testkit/tb/pg/pool.go:156-162`) permanece nessa pool. A role de teste tem senha fixa, exclusiva do cluster de teste, e `TEMPORARY` é revogado de `PUBLIC` no banco por teste.
- [x] **[P0] Suítes sob a role de runtime**: o `harness_test.go` do `bff` e o `distkit` de cada contexto sobem os processos com `PG_DSN` de runtime e `PG_MIGRATE_DSN` administrativo, e `test-race` e `test-distributed` dos três contextos passam assim.
- [x] **[P1] Asserção de negação**: `tb/pg` expõe um helper que executa um statement na pool de runtime e exige SQLSTATE 42501, para os testes de cada app fixarem o que a role não pode.
- [x] **[P1] CI e infra de testes**: `dmpf-go-ci.yml`, `dmpf-distributed.yml` e `infra/test/compose.yml` seguem com o servidor administrativo (`POSTGRES_USER=postgres`); as roles de teste nascem no banco por teste, e nenhum passo de workflow cria role de app.

**E. Apps de referência, generator e documentação**

- [x] **[P0] Apps de referência**: `orders`, `reservations` e `bookings` declaram o migrador no `infra.json`, exportam `Grants` e passam `MigrateDSN` ao `serve`; `infrasync --write` regenera `provisioning.generated.yml` e o Job de `dev`.
- [x] **[P0] Generator `bounded-context`**: emite o `infra.json` com migrador, `PG_MIGRATE_DSN` em config, compose, `.env.example` e `k8s`, e `Grants` vazio em `provider/schema.go`, que o autor preenche com o `Mutable` do agregado ao criar a tabela (o generator não conhece o nome dela, e a partida recusa tabela sem grant); o `init` renderiza `postgres.yml` e `app-base.yml` coerentes. `tools/dmpf-generator-check.sh` e a fase `self-test` de `tools/dmpf-harness-check.sh` passam.
- [x] **[P0] ADR novo**: registra as duas roles, a derivação da role de runtime, o conjunto de grants e o modo de compatibilidade; emenda o ADR-051 (a premissa de role dona e a condição de `:50`, sem ligar RLS) e o ADR-053 (itens 1 e 3), e atualiza o Status dos dois. `node tools/adr-verify.mjs --from 63` passa.
- [x] **[P1] Documentação**: `infra/README.md` ("Bancos por app"), `AGENTS.md` (forma do contexto), `docs/guides/dmpf-composicao.md` (`PG_DSN` dos testes) e `docs/guides/dmpf-consumo-externo.md` descrevem as duas roles e o caminho de adoção por um consumidor existente.

**F. Investigação**

- [ ] **[P0] Investigar superuser em Kubernetes e no CI**: consultar `pg_roles.rolsuper` e `pg_database.datdba` da role de conexão no overlay `dev` e no container do CI, e registrar o resultado no ADR novo, junto de `server_version_num` desses ambientes e, por consulta do operador, a do servidor de `hmg` (pré-condição de PostgreSQL ≥ 15). O desenho vale nos dois casos; o resultado decide se o ADR-051 muda a premissa ou apenas a confirma.
- [x] **[P0] Investigar os privilégios mínimos reais das tabelas do kernel**: rodar a suíte do kernel e a dos contextos sob uma role com o conjunto candidato da matriz e registrar o conjunto exato por tabela e por processo (`api`, `relay`, `consumer`), incluindo sequências `IDENTITY`, `FOR UPDATE SKIP LOCKED`, `to_regclass` e `ctid`. O resultado fixa `KernelGrants`.
- [x] **[P0] Investigar `INSERT ... ON CONFLICT DO NOTHING` com role sem `UPDATE`**: reproduzir contra a imagem `postgres:16-alpine` do repositório, com tabela só com `SELECT` e `INSERT`, o padrão de `libs/backend/go/postgres/table.go:185-187`, `libs/backend/go/postgres/inbox.go:19-21` e `libs/backend/go/postgres/quarantine.go:16-18`, e registrar se o Postgres o aceita. Se recusar, `AppendOnly` e a matriz de `inbox`/`quarantine` mudam antes do merge.

### Não-funcionais

- [x] Idempotência: o provisionamento e `ApplyGrants` executados duas vezes deixam dono, senha e privilégios no mesmo estado.
- [x] Custo: o trabalho novo acontece só na partida (segunda pool e `ApplyGrants` no processo que migra, consulta de privilégio nos que esperam); o caminho de requisição não ganha statement.
- [x] Compatibilidade: Go `1.26.9`, Postgres 16 como nas imagens fixadas de Compose, CI e k8s (pré-condição: PostgreSQL ≥ 15), Nx 23.1.0.

## Camadas afetadas

| Camada | Efeito |
| ------ | ------ |
| Kernel `postgres` | `Privilege`, `Grant`, `Mutable`, `AppendOnly`, `KernelGrants`, `ApplyGrants`, `WaitForGrants` |
| Kernel `app/serve` | `APIRole.MigrateDSN`, `API.Grants`, `prepare` com pool efêmera, espera por privilégio no `api`, `relay` e `consumer` |
| Kernel `testkit` (`tb/pg`) | `OpenRuntimePool`, helper de negação, role de teste |
| Contextos (`orders`, `reservations`, `bookings`) | `config.go`, `wiring.go`, `provider/schema.go`, `deploy/**` |
| Tooling | `infrasync` (schema e templates), generator `bounded-context`, `init` |
| Infra | Compose local, overlays `dev` e `hmg`, infra de testes |
| Docs | ADR novo, ADR-051 e ADR-053 (Status), README de infra, guias |

## Localização de código

```text
libs/backend/go/postgres/migrate.go                    Migrate (assinatura inalterada)
libs/backend/go/postgres/grants.go                     novo: Privilege, Grant, KernelGrants, ApplyGrants
libs/backend/go/postgres/wait.go                       WaitForGrants, ao lado de WaitForTables
libs/backend/go/app/serve/serve.go                     APIRole.MigrateDSN, API.Grants, prepare
libs/backend/go/testkit/tb/pg/pool.go                  OpenRuntimePool, helper de negação
libs/backend/go/testkit/tb/dist/harness.go             PG_DSN de runtime e PG_MIGRATE_DSN do api
tools/dmpf-conformance/infrasync/infrasync.go          database.migrator, CheckDevSecrets
tools/dmpf-conformance/infrasync/templates/provisioning.yml.tmpl
tools/dmpf-conformance/infrasync/templates/databases-job.yaml.tmpl
tools/dmpf-conformance/infrasync/templates/databases-kustomization.yaml.tmpl
tools/dmpf-conformance/infrasync/templates/env.tmpl
tools/dmpf-conformance/infrasync/templates.go          EnvLines
infra/local/compose/provisioning.generated.yml         gerado
infra/k8s/overlays/dev/databases/job.yaml              gerado
apps/backend/{orders,reservations,bookings}/app/{config,wiring}.go
apps/backend/{orders,reservations,bookings}/provider/schema.go
apps/backend/{orders,reservations,bookings}/deploy/**  infra.json, compose.yml, .env.example, k8s
apps/backend/bff/app/harness_test.go                   DSN de runtime e de migração
tools/dmpf-plugin/templates/bounded-context/**         config, wiring, schema.go, deploy
tools/dmpf-plugin/templates/init/managed/infra/local/compose/postgres.yml__tmpl__
tools/dmpf-plugin/templates/ai/.claude/rules/dmpf-bounded-context.md__tmpl__   e a cópia em .claude/rules/
tools/dmpf-plugin/scripts/dmpf-context-check.sh        DSN e laço do migrador
apps/backend/{orders,reservations,bookings}/distkit/**  DSN de runtime e de migração
docs/adr/063-duas-roles-de-banco-por-app.md            novo
infra/README.md, AGENTS.md, docs/guides/dmpf-composicao.md, docs/guides/dmpf-consumo-externo.md
```

## Design

### Roles e partida

```text
infra.json  database.migrator (opcional)
    │
    ▼  infrasync
<app>_migrator  LOGIN, dona do banco <app>
<app>           LOGIN, sem posse; CONNECT, USAGE em public; sem CREATE

api (MIGRATE=true, PG_DSN + PG_MIGRATE_DSN)      api (MIGRATE=false), relay, consumer
  pool de runtime ── ping                          pool de runtime ── ping
  pool do migrador (efêmera)                       espera tabela e privilégio
    lock → Migrate(caps, Schema)
    ApplyGrants(role de PG_DSN, Grants)  em uma transação
  fecha a pool do migrador
  ready → serve
```

### Matriz inicial de privilégios

Derivada do inventário acima; a investigação a confirma ou corrige.

| Tabela | SELECT | INSERT | UPDATE | DELETE |
| ------ | ------ | ------ | ------ | ------ |
| `outbox` | sim | sim | sim | sim |
| `inbox` | sim | sim | sim | sim |
| `quarantine` | sim | sim | não | não |
| agregado (`Mutable`) | sim | sim | sim | não |
| tabela declarada `AppendOnly` | sim | sim | não | não |

## Decisões técnicas

| Decisão | Escolha | Alternativas descartadas |
| ------- | ------- | ------------------------ |
| Nome das roles | `<app>` segue como a de runtime e `<app>_migrator` é a dona | Renomear a de runtime para `<app>_app`: quebra todo `PG_DSN` e Secret existente |
| Onde migrar | Pool efêmera do migrador dentro do `api` com `MIGRATE=true` | Migrar pela pool de runtime (estado atual); processo ou Job de migração à parte (fora de escopo, ver abaixo) |
| Grants | Declarativos e convergentes: `REVOKE ALL` e `GRANT` exato, numa única transação | `ALTER DEFAULT PRIVILEGES`: concede por omissão e contraria o fail-closed; `REVOKE` e `GRANT` em transações separadas: janela sem privilégio para os pods vivos |
| Espera dos processos | Por privilégio (`WaitForGrants`), além da tabela | Só por tabela (`WaitForTables`): devolve quando o `Migrate` comita, antes dos grants, e abre janela de SQLSTATE 42501 |
| Adoção | `database.migrator` opcional | Obrigatório já: quebra o consumidor externo (ADR-060) |
| Role de runtime | Derivada do usuário de `DSN` | `PG_RUNTIME_ROLE`: segunda fonte do mesmo fato |
| `CREATE` em `public` | Pré-condição PostgreSQL ≥ 15, com `CREATE` revogado da role de runtime de forma explícita; em servidor ≤ 14, `REVOKE CREATE ON SCHEMA public FROM PUBLIC` por superuser em cada banco de app | Revogar só da role de runtime: em ≤ 14 não neutraliza o `CREATE` concedido a `PUBLIC`. Depender do default da versão sem declará-lo: o repositório fixa a imagem 16 em Compose, CI e k8s, mas não o servidor de `hmg` |
| Role nos testes | Uma role de runtime por cluster `test`, com grants por banco | Uma role por teste: cria e remove roles do cluster inteiro a cada teste |
| RLS | Fora desta spec | Ligar agora: a sonda de dono cruzado (`libs/backend/go/postgres/table.go:179-181`) lê entre tenants e precisaria de política própria |

## Riscos

| Risco | Mitigação |
| ----- | --------- |
| O `api` com `MIGRATE=true` guarda o DSN do migrador no ambiente: um processo comprometido ainda o alcança | Aceito nesta spec e registrado no ADR novo; a proteção vale contra a role de runtime e o caminho normal da aplicação, e não contra um `api` com `MIGRATE=true` comprometido. O passo de migração fora do processo servidor é o endurecimento seguinte |
| Privilégio insuficiente descoberto tarde (sequência, `to_regclass`) | As três investigações e as suítes sob a role de runtime rodam antes do merge |
| Tabela nova sem grant | Reprovação na partida (requisito P1) e teste por contexto |
| Réplica que reinicia reaplica os grants, ou um processo sobe entre o `Migrate` e os grants | `REVOKE ALL` e `GRANT` numa transação; `WaitForGrants` nos processos que esperam; teste de reaplicação com consultas em curso |
| Servidor de `hmg` não é fixado pelo repositório | O provisionamento de `hmg` segue como Secret por template; a investigação de superuser registra também a versão do servidor, e em ≤ 14 o `REVOKE` de `PUBLIC` é pré-condição documentada no ADR novo |

## Regras relacionadas

- IDN-14, DAT-11 e DAT-12 (`docs/dmpf/contexto-erros-seguranca.md`).
- ADR-050 (tabelas de infraestrutura fora do escopo de tenant), ADR-051, ADR-053, ADR-054 e ADR-060.
- `.golangci.yml` (regra `context-provider`, `:431`): o provider continua sem driver; `Grants` usa só tipos do `postgres`.
- SPEC-F308KDSF (banco por app), SPEC-JJKWG4JP (inbox de comando) e SPEC-VJMM2DE5 (deploy por app).
- SPEC-5AQHHMJ0 (comando externo de saída): o ledger `external_attempt` é capability nova, coberta por `KernelGrants`.

## Verificação e testes

### Critérios de aceite

- [x] Em teste de integração, a pool de runtime recebe SQLSTATE 42501 em `CREATE TABLE`, `CREATE TEMP TABLE`, `ALTER TABLE`, `DROP TABLE` e `TRUNCATE`, e em `UPDATE` e `DELETE` sobre tabela declarada `AppendOnly`, e consegue `SELECT` e `INSERT` nela.
- [x] `pg_database.datdba` do banco de cada app aponta a role migradora, e a role de runtime não possui nenhuma tabela.
- [x] Os fluxos de outbox, relay, purga, inbox, quarentena e `Table.Save` passam sob a role de runtime nas suítes do kernel e dos três contextos, inclusive `test-race` e `test-distributed`.
- [x] Rodar o provisionamento e `ApplyGrants` duas vezes não altera dono, senha nem privilégio.
- [x] Reaplicar `ApplyGrants` com a pool de runtime consultando sem parar não produz SQLSTATE 42501, porque o `REVOKE` e o `GRANT` rodam numa transação.
- [x] Um `relay`, um `consumer` e um `api` com `MIGRATE=false`, iniciados com a tabela criada e sem grant, ficam em espera e só seguem depois do `ApplyGrants`, sem consulta recusada por SQLSTATE 42501.
- [x] Em PostgreSQL ≥ 15, `has_schema_privilege(<role de runtime>, 'public', 'CREATE')` é falso; o provisionamento gerado contém `REVOKE CREATE ON SCHEMA public FROM PUBLIC` para servidor ≤ 14.
- [x] Uma app com `infra.json` sem `database.migrator` gera a mesma saída de hoje e sobe com a role única.
- [ ] As três investigações têm resultado registrado no ADR novo, e `KernelGrants` coincide com o conjunto registrado.
- [ ] `go run ./tools/dmpf-conformance/cmd/conformance --root .`, `modsync --check` e `infrasync --check` passam, o job `external-consumer` (`.github/workflows/ci.yml:96`) passa, e `node tools/adr-verify.mjs --from 63` passa.
- [x] `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` e `tools/dmpf-generator-check.sh` passam.

### Cenários de teste

**Happy path — app com duas roles**
- DADO uma app com `database.migrator` e `MIGRATE=true`, com `PG_DSN` de runtime e `PG_MIGRATE_DSN` do migrador
- QUANDO o `api` sobe
- ENTÃO a pool efêmera cria o schema e aplica os grants, o `api` serve com a pool de runtime e o `relay` e o `consumer` esperam as tabelas sem DSN de migrador

**Erro — runtime tenta DDL ou alterar tabela append-only**
- DADO uma tabela declarada `AppendOnly` e a pool de runtime
- QUANDO a app executa `UPDATE`, `DELETE`, `TRUNCATE` ou `ALTER TABLE` sobre ela
- ENTÃO o Postgres recusa com SQLSTATE 42501 e `Table.Save` não devolve `ErrVersionConflict`

**Edge case — app sem migrador declarado**
- DADO um `infra.json` sem `database.migrator` e `MIGRATE=true`
- QUANDO o `api` sobe e `MigrateDSN` está vazio
- ENTÃO migra pela pool do `DSN`, não aplica grants e o provisionamento gera a role única, como hoje

**Edge case — tabela nova sem grant**
- DADO um `schema.sql` que cria uma tabela fora do kernel e fora de `Grants`
- QUANDO a partida roda `ApplyGrants`
- ENTÃO a subida falha nomeando a tabela, e nenhum privilégio é concedido a ela

**Edge case — processo sobe entre a migração e os grants**
- DADO uma tabela já criada pelo migrador e ainda sem grant para a role de runtime
- QUANDO o `relay` sobe e depois o `ApplyGrants` comita
- ENTÃO o `relay` espera o privilégio, sem consulta recusada, e segue só depois do commit

**Edge case — réplica reaplica os grants com pods vivos**
- DADO pods com a pool de runtime consultando e uma réplica que reinicia
- QUANDO a réplica roda `ApplyGrants` de novo
- ENTÃO nenhuma consulta dos pods vivos recebe SQLSTATE 42501, e o conjunto de privilégios fica igual

<critical_constraints>
- [P0] Em app com migrador declarado, a role de runtime nunca é dona de banco, schema ou tabela, nunca executa DDL nem `TRUNCATE`, e só recebe o privilégio que o kernel ou a app declara: tabela sem grant declarado fica inacessível a ela.
- [P0] As tabelas do kernel (`outbox`, `inbox`, `quarantine`) mantêm os privilégios que os statements do kernel exigem, inclusive `UPDATE` e `DELETE` em `outbox` e `inbox`; append-only vale só para a tabela que a app declara como tal.
- [P0] App cujo `infra.json` não declara migrador continua válida e é provisionada com a role única de hoje: nenhum consumidor quebra sem migration, e `modsync --check`, `infrasync --check`, `conformance` e o job `external-consumer` seguem verdes.
- [P0] `postgres.Migrate` mantém assinatura e semântica (`IF NOT EXISTS`, sob o mesmo `migrateLockID`), e nenhuma tabela ou coluna existente é renomeada.
- [P0] DSN e senha do migrador nunca aparecem em log, métrica, span nem mensagem de erro.
- [P1] O banco por teste (ADR-054) e a recusa de servidor fora do cluster `test` permanecem: role e grants de teste só existem no cluster de teste.
- [P1] Nenhuma dependência de terceiros nova no `go.mod` do kernel.
</critical_constraints>

## Escopo fora

- **Ligar RLS**: esta spec entrega a pré-condição (role sem posse). A sonda de dono cruzado de `postgres.Table` lê entre tenants (`libs/backend/go/postgres/table.go:179-181`; índice `orders_order_id_idx` em `apps/backend/orders/provider/schema.sql:11-13`), então uma spec de RLS precisa tratá-la à parte.
- **Migração fora do processo servidor**: um passo ou Job de migração que tire o DSN do migrador do ambiente do `api` exige um papel novo nos três binários e nos manifestos; fica como evolução, registrada no ADR novo.
- **Role por processo** (`api`, `relay`, `consumer`): a spec entrega uma role de runtime por app.
- **Migração de ownership de banco existente**: bancos criados com a role única são recriados, como o ADR-053 fez; ambiente com dado real exige procedimento próprio, descrito no ADR novo.
- **Trigger e tabela de auditoria do consumidor**: o DDL de uma tabela append-only (trigger contra `UPDATE`, `DELETE` e `TRUNCATE`) é da app; o kernel entrega o privilégio, não o modelo.
- **Provisionamento do servidor de `hmg`**: segue externo ao repositório; a spec só acrescenta o Secret `<app>-migrator`.
- **Rotação de credenciais e autenticação por IAM**: tema de operação do servidor, ortogonal ao modelo de duas roles.
