# ADR-051: Impor o escopo de tenant por choke point em Go, e não por RLS

## Status

Aceito — 2026-09-23. Implementa [SPEC-9B6SHEH8](../specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md). **Caminhos atualizados pelo [ADR-054](./054-apps-autocontidos-e-infras-separadas.md) (2026-09-28)**: o Kubernetes de cada app está em `apps/backend/<app>/deploy/k8s/`, e o Postgres usa o banco e as credenciais padrão, com banco e role por app. **Emendado pelo [ADR-063](./063-duas-roles-de-banco-por-app.md) (2026-10-15)**: em app com `database.migrator`, a role de conexão não é superuser nem dona das tabelas, e a condição do §Consequências para somar RLS ao choke point fica cumprida; a RLS continua fora. **Emendado pelo [ADR-066](./066-unicidade-declarada-como-rejeicao.md) (2026-10-17)**: além do filtro por coluna, o `Table` declara a unicidade de negócio do contexto, sempre com `tenant_id` à frente, conferida no catálogo na partida. **Emendado pelo [ADR-067](./067-leitura-relacional-na-uow-com-guarda.md) (2026-10-18)**: o pool segue chegando aos contextos só como `ReadPool`, mas o `Relation` também lê na `*Tx` do comando, por `QueryWithin` sob a guarda declarada no `Table`. **Emendado pelo [ADR-068](./068-acesso-entre-tenants-declarado.md) (2026-10-18)**, nos itens 1, 3 e 4: o `Table` ganha os dois caminhos declarados entre tenants, a leitura de plataforma e a projeção, o gate os restringe por arquivo e o acesso declarado registra o desfecho com falha fechada; a leitura comum não muda.

## Contexto

`IDN-14` (FND-07 §4) recusa isolamento de tenant cuja correção dependa de cada autor lembrar da condição, mesmo que nenhum caminho esteja errado hoje. A norma não prescreve mecanismo — constraint composta, row-level security, chave particionada ou cliente de persistência que injeta o escopo servem — e deixa a escolha ao `provider`.

Antes desta spec, cada contexto escrevia o próprio SQL literal sobre um `pgx.Tx` cru, obtido por `postgres.Tx.Conn()`, e os readers tomavam o `*pgxpool.Pool` direto. Nenhuma tabela de agregado tinha coluna de tenant.

Row-Level Security do Postgres era o candidato natural, e foi medido nos três ambientes versionados. Em todos, o role que abre conexão é o superuser de bootstrap do cluster e também o dono das tabelas:

- local: `POSTGRES_USER` do serviço `postgres` em `infra/local/compose/postgres.yml`;
- CI: `-e POSTGRES_USER=dmpf` no `docker run` do job em `.github/workflows/ci.yml`;
- Kubernetes: `POSTGRES_USER=app` no `secretGenerator` de `infra/k8s/base/postgres/kustomization.yaml`.

O Postgres ignora RLS incondicionalmente para superuser, e `FORCE ROW LEVEL SECURITY` só afasta a isenção do dono quando esse dono não é superuser. Ligar RLS aqui não protegeria nada.

## Decisão

**O escopo de tenant é imposto por um choke point em Go, fechado por gate mecânico.**

1. **Choke point.** `postgres.Table[ID,S]` é o único lugar onde se escreve SQL de agregado: o contexto declara o nome, a coluna do identificador, as colunas de estado e o codec, e o kernel compila os statements com `tenant_id` no predicado e na chave. O tenant vem do portador (`ports.ExecutionContext`), e a ausência recusa (`ErrTenantUnresolved`, `IDN-15`) em vez de alargar a consulta. A consulta por relação (`Table.Relation`) segue a mesma regra. O `memory.Table` escopa pelo mesmo contrato, e a suíte `providerkit.Repository` roda contra as duas realizações. Os únicos caminhos que alcançam outro tenant são os declarados do [ADR-068](./068-acesso-entre-tenants-declarado.md): `Table.PlatformReader`, que lê todos os tenants só quando o carrier `ports.WithPlatformReach` traz `ReachAllTenants` (sem ele, recusa com `ErrPlatformReachUndeclared`), em página de limite obrigatório e com teto; e `Table.Projector`, que nasce do `*Tx` e lê e grava no tenant de destino explícito sob a `ProjectionPolicy` da app. `Reader`, `Relation` e `Repository` não mudam.
2. **Portas fechadas.** `Tx.Conn()` foi removido; o pool chega ao provider de contexto só como `postgres.ReadPool`, handle opaco sobre o qual rodam apenas statements do kernel.
3. **Gate.** A regra `depguard` `context-provider` (`.golangci.yml`) nega `github.com/jackc/pgx`, `database/sql` e `gorm.io/gorm` em `apps/**/provider/**`, e `tools/dmpf-gate-check.sh` prova no CI que ela reprova os três drivers em cada contexto. A exceção nominal é a DDL de contexto, aplicada pela composition root por `postgres.Migrate(ctx, pool, provider.Schema)`. Desde o [ADR-068](./068-acesso-entre-tenants-declarado.md), o `forbidigo` por símbolo, com exclusão por caminho, restringe `Table.PlatformReader` e `Table.Projector` (de `postgres` e de `memory`) a `apps/**/provider/platform_*.go`, ao kernel e aos `_test.go`, a construção de `ports.WithPlatformReach` ao kernel de borda (`http`, `grpc`), aos kits de teste e aos `_test.go`, e a chamada de `http.WithVerifiedPlatformReach` à borda REST (`apps/**/app/api/`), além do kernel de borda, dos kits e dos testes; nenhum `apps/**/app` de contexto monta alcance. A checagem estrutural do `dmpf-context-check.sh` reprova `PlatformReader` e `Projector` fora de `provider/platform_*.go` e os dois alcances em qualquer arquivo de produção do contexto, o que fecha o contorno por interface local, e o `dmpf-gate-check.sh` prova as reprovações.
4. **Acesso cruzado registrado.** Quando o statement escopado não encontra a linha, o kernel sonda qual outro tenant detém o identificador, sem trazer o estado, e sinaliza `ports.CrossTenantAccess` — que responde como `ErrNotFound` ao chamador (`IDN-13`) e vira evento de segurança na instrumentação (`IDN-12`). Esse é o acesso cruzado **por engano**, que segue negado. O acesso **declarado** do [ADR-068](./068-acesso-entre-tenants-declarado.md) registra um evento por tenant alcançado, com desfecho `accepted` ou `denied`, antes de devolver ou gravar o dado, e falha fechado com `ErrAuditUnavailable` quando o `Sink` recusa o registro.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Row-Level Security | Medido como inerte: o role é superuser e dono das tabelas nos três ambientes. Só protegeria depois de criar um role de aplicação e propagá-lo por compose, CI, manifestos e harnesses |
| Construtor genérico sem gate | Fechar `Tx.Conn()` não impediria o contexto de tomar pool próprio e escrever SQL em autocommit; o isolamento voltaria a depender de disciplina |
| Liberar `pgxpool` como tipo e negar só `pgx` | `pool.Query` continuaria ao alcance do contexto; o gate passaria a ser nominal |

## Consequências

**Positivas:**

- A consulta nova escrita sem a condição de tenant é impossível no caminho da aplicação: o contexto não tem com que escrevê-la.
- A mesma suíte de conformidade prova o escopo em memória e em Postgres, então um caso de uso provado sobre o dublê não passa com um isolamento que a produção não tem.

**Negativas:**

- O `Table` cobre um filtro por coluna de uma tabela. Junção, agregação ou SQL mais amplo exige estender o kernel, nunca reabrir o driver no contexto.
- **Custo aceito:** A sonda do acesso cruzado custa uma consulta extra em toda falta de linha, servida por índice pelo identificador em cada tabela de agregado.

**O híbrido continua disponível.** Quando existir um role de aplicação que não seja superuser nem dono das tabelas, RLS pode ser somada ao choke point como segunda camada, sem substituí-lo: o choke point segue sendo o que torna o isolamento independente de convenção no código.
