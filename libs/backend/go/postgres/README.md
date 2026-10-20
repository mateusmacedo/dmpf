# postgres

Realização em PostgreSQL das três portas transacionais que o `KRN-04` fixou em
tipos de domínio: `UnitOfWork[R]`, `Repository[ID, S]` e `Outbox`. É o bloco
`provider` do kernel DMPF, e a primeira realização que prova a sequência
canônica de FND-04 §3.2 contra um banco de verdade — a de `memory` roda
sobre mutexes e declara não provar isolamento.

Uma unidade no manifesto, `provider`, em `kernel`:

| Unidade | Diretório | O que é |
| --- | --- | --- |
| `kernel/provider-postgres` | raiz | UoW sobre `pgx.Tx`, `Table[ID,S]` escopado por tenant, outbox, inbox, quarantine, migração e purga |

Os repositórios dos agregados de referência, que este módulo carregava em
`example/orders` e `example/reservations` como SQL escrito à mão, são hoje
instâncias de `Table[ID,S]` no bloco `provider` de cada contexto —
`apps/backend/orders/provider` e `apps/backend/reservations/provider`
(ADR-046, ADR-051).

## O que o módulo contém

- **`table.go`** — `Table[ID,S]`, o choke point de todo SQL de agregado
  (ADR-051): o contexto declara `Name`, `IDColumn`, `Columns`, `Encode`/`Decode`
  e, quando o estado carrega a própria identidade, `WithID`; o kernel compila
  os quatro statements (`select`, `owner`, `insert`, `update`) com `tenant_id`
  no predicado e na chave primária. `Reader(ReadPool)` serve o lado de leitura
  fora de transação; `Repository(*Tx)` liga o lado de escrita à transação
  aberta. `Relation(filterColumn)` serve a consulta por relação que os
  genéricos de `ports` não expressam — muitas linhas de uma tabela filtradas
  por uma coluna que não é a chave —, com a mesma disciplina de tenant.
  `Unique []Unique` declara a unicidade de negócio que o `schema.sql` impõe
  além da identidade (ADR-066); o `validate` entra em pânico com nome ou coluna
  fora de minúsculas, nome diferente de `<tabela>_<col1>_<colN>_key` ou acima
  de 63 bytes, menos de duas colunas, `tenant_id` fora da frente, rejeição nula,
  código inválido ou nome repetido. Só nas tabelas com `Unique`, o `Save` envia
  `SAVEPOINT dmpf_unique`, o `INSERT` ou o `UPDATE` e `RELEASE SAVEPOINT
  dmpf_unique` num único `pgx.Batch`. O nome é fixo: savepoints de mesmo nome
  empilham, `RELEASE` e `ROLLBACK TO` atingem o mais recente, que é o do
  próprio `Save`, e o `sp_N` do `Begin` aninhado do pgx fica abaixo dele; o
  `ROLLBACK TO` mantém o savepoint, e o seguinte empilha por cima. Com o nome
  fixo, cada statement é preparado uma vez por conexão. Continua um savepoint
  (subtransação) por `Save`. O pgx prepara ou descreve os statements de um
  batch antes de enviá-lo (`conn.go:1170`), e um prepare recusado antes do
  `SAVEPOINT` abortaria a transação; por isso, no modo padrão
  (`CacheStatement`), o primeiro `Save` de cada escrita numa conexão envia o
  `SAVEPOINT` sozinho e prepara por nome a escrita, o `SAVEPOINT` e o
  `RELEASE`, fora do LRU do cache de statements, e dali em diante o batch é um
  round trip. O plano fica no `CustomData` da conexão. `Exec` e
  `SimpleProtocol` mandam sempre um batch; `CacheDescribe` e `DescribeExec`
  mandam sempre o `SAVEPOINT` separado. Quando a escrita falha, inclusive no
  prepare (coluna divergente, `42703`), a falha vem depois do `SAVEPOINT`: o
  servidor descarta o `RELEASE` e a transação fica abortada até o `ROLLBACK
  TO SAVEPOINT`, que roda num contexto destacado do cancelamento, com prazo de
  5 s, e a deixa utilizável. O erro da escrita sai sem rótulo; `savepoint:`
  aparece só quando o servidor recusa o próprio `SAVEPOINT`
  (`*pgconn.PgError`, como `25P02`), no batch ou enviado sozinho. Cancelamento,
  timeout, falha de rede ou conexão fechada antes do resultado do `SAVEPOINT`
  saem sem rótulo e sem `ROLLBACK TO`, nos dois caminhos: nada garante
  que o savepoint exista, um `ROLLBACK TO` poderia atingir um savepoint mais
  antigo de mesmo nome, e o pgx fecha a conexão. Um `23505` da própria tabela
  volta então como `ports.UniqueViolation`, com a rejeição declarada se a
  restrição está em `Unique` e a transação ainda não escreveu, e `Rejection`
  nulo se não. Na criação, uma sonda confere antes se a identidade já existe
  no tenant: se existe, é corrida de identidade e volta `ErrVersionConflict`.
  O `Detail` do `PgError`, que cita os valores da linha, nunca chega ao erro
  (ERR-07). Em escritas no socket, no modo padrão: o primeiro `Save` de cada
  escrita numa conexão paga 5, uma vez; depois, 1, contra 3 com cada
  statement enviado sozinho; a primeira criação recusada numa conexão paga 4
  (batch, `ROLLBACK TO SAVEPOINT`, prepare e execução da sonda de identidade),
  as seguintes, 3, e o `Save` seguinte à recusa, 1.
  `CacheDescribe` paga 2 por `Save`, e `DescribeExec`, 3. O `BenchmarkSave`
  (`unique_bench_test.go`) grava na mesma tabela declarada com e sem `Unique`.
  O `-count` não intercala os sub-benchmarks, então o binário de teste rodou
  12 vezes com `-test.count=1`: medianas de 171 µs sem e 195 µs com `Unique`
  (razão 1,14), contra 177 e 356 µs (2,0) na versão anterior ao batch. Os
  tempos absolutos variam com a carga da máquina, e só a razão compara. Um
  `DEALLOCATE ALL` ou `DISCARD ALL` externo, como o de um `ResetSession`
  configurado pelo consumidor, invalida os statements preparados por nome sem
  o pgx saber, como invalidaria os do próprio pgx. Tabela sem `Unique` não
  ganha statement nem round trip. `Uniques()` devolve a declaração como
  `UniqueSet`, sem parâmetros de tipo. `Guard` e `QueryWithin` ficam neste
  arquivo e estão descritos no item `guard.go`, abaixo.
- **`guard.go`** — a guarda de pré-condição relacional (ADR-067). `Guard`
  nomeia uma coluna de `Columns`; o `validate` entra em pânico com coluna fora
  delas ou que não é identificador. `Relation(Guard).QueryWithin(ctx, tx,
  valor)` lê como `Query`, mas na `*Tx` do comando: antes do `SELECT`, toma
  `pg_advisory_xact_lock_shared` sobre `hashtextextended(concat_ws(chr(31),
  tabela, coluna, length(tenant), tenant, valor), 0)` (o comprimento impede que
  um tenant com `chr(31)` chegue à chave de outro), em instrução própria do mesmo
  `pgx.Batch`, porque o snapshot em `READ COMMITTED` é da instrução e a leitura
  precisa enxergar o que commitou durante a espera. `QueryWithin` sobre outra
  coluna entra em pânico. Todo `Save` de tabela com `Guard` toma
  `pg_advisory_xact_lock` da mesma chave, com o valor codificado da coluna, no
  batch da escrita; com `Unique`, antes do `SAVEPOINT dmpf_unique`, porque o
  `ROLLBACK TO SAVEPOINT` libera o lock tomado depois dele. Assim, nenhuma
  escrita da série, `UPDATE` ou `INSERT` de supersessão, commita entre a
  leitura de um comando e o commit dele: a pré-condição vale no commit, e a
  escrita é serializada depois. Não é invariante permanente entre agregados. O
  valor da guarda é só `string` (inclusive `~string`); outro tipo volta
  `ErrGuardValue` sem ir ao servidor. A chave é o texto que o Go envia, não o
  valor tipado da coluna: a coluna de guarda é `text`, com o valor já
  canônico, porque em `uuid` ou `citext` valores iguais com grafias diferentes
  tomariam locks diferentes. Um erro de preparo de outra instrução do batch
  sai com o rótulo dela, não com o da guarda. O `UPDATE` inclui a guarda no `WHERE`,
  e mudar o valor de um agregado existente volta `ErrVersionConflict`. A `Tx`
  que tomou guarda converte qualquer `40P01` de `Load`, `Save` ou
  `QueryWithin` em `ErrVersionConflict` (`Conflict`, `Aborted` no gRPC),
  porque a vítima do ciclo pode ser o `UPDATE`, não o lock; sem guarda, o erro
  segue o do driver. O deadlock é esperado quando dois comandos leem e depois
  escrevem a mesma série, e a repetição é do chamador (UOW-10). Quem adota a
  guarda assume quatro custos, detalhados no ADR-067: a transação que leu
  bloqueia as escritas da série até o commit (configurar `lock_timeout` e
  `idle_in_transaction_session_timeout` no pool); escritas da mesma série
  correm em fila; cada valor lido ocupa uma entrada de
  `max_locks_per_transaction`; e, com o contexto cancelado, o backend fica na
  fila até perceber a queda (`client_connection_check_interval`). O advisory lock
  não exige privilégio de tabela. Em escritas no socket, no modo padrão
  (`CacheStatement`) e com conexão quente: a leitura com linhas paga 1, a
  vazia 2 (a sonda de dono) e o `Save` com guarda 1, com ou sem `Unique`. Com
  `Unique`, o primeiro `Save` de cada escrita numa conexão, e todo `Save` nos
  modos `CacheDescribe` e `DescribeExec`, enviam o lock e o `SAVEPOINT`
  sozinhos antes do batch; o lock é preparado por nome junto da escrita, e um
  batch recusado não o tira do cache. Tabela sem `Guard` não ganha statement nem round
  trip. Testes: `guard_test.go`, `guard_internal_test.go`, `guard_table_test.go`
  e `guard_concurrency_test.go`, que mede 50 rodadas por escrita e reproduz a
  corrida pelo `ReadPool` como controle.
- **`unique.go`** — `Unique{Name, Columns, Rejection}`, `UniqueSet{Table,
  Constraints}` e `VerifyUniques(ctx, pool, sets...)`, que confere em
  `pg_constraint`, pela tabela resolvida em `public`, que cada restrição
  declarada existe com `contype = 'u'`, nas colunas e na ordem declaradas, sem
  `DEFERRABLE` e sem `NULLS NOT DISTINCT`, e que as restrições de cada
  `UniqueSet` seguem a ordem dos índices no catálogo, que é a do DDL: sob colisão
  em mais de uma, o servidor reporta o primeiro índice que confere, e a
  declaração precisa apontar a mesma. Tabela particionada, com filhas por
  herança, ou ela própria partição ou filha diverge como `partitioned or
  inherited table`: na partição, o `23505` traz a tabela e a restrição da
  partição folha, e a tradução, que compara a tabela, devolveria o erro cru no
  lugar da rejeição. Devolve a primeira divergência como
  `ErrUniqueDivergent`. Antes do catálogo, valida os conjuntos com as regras do
  `validate` e devolve erro em vez de pânico. Sem nada declarado, não faz round
  trip. `CREATE UNIQUE INDEX` não tem linha em `pg_constraint` e não serve.
- **`platform.go`** — `Table.PlatformReader(ReadPool, ports.AuditTrail,
  ...PlatformOption)` e `Table.Projector(*Tx, ports.ProjectionPolicy,
  ports.AuditTrail)`, os dois acessos entre tenants declarados (ADR-068),
  descritos em [Acesso entre tenants declarado](#acesso-entre-tenants-declarado);
  `WithPageMax(n)` troca o teto da página.
- **`snapshot_table.go`** — `SnapshotTable[ID, S, J](name, idColumn, to, from,
  withID)` monta a `Table[ID,S]` do agregado cujo estado inteiro vive na coluna
  `snapshot` (ADR-053): `J` é o struct de estado privado do provider, cujas tags
  JSON fixam os bytes gravados, `to` e `from` convertem entre o snapshot do
  domínio e `J`, e a identidade fica só em `idColumn`, devolvida por `withID`.
  Agregado com coluna tipada além do snapshot continua com a `Table` escrita à
  mão.
- **`pool.go`** — `NewPool` monta o `*pgxpool.Pool` com o tracer de query do
  processo (`dbtrace.go`): um span CLIENT por query só dentro de operação já
  traçada — a query do claim do relay, que roda sem pai, não abre span; as
  transições sob o `outbox drain`, sim. O span se chama
  `{db.operation.name} {db.collection.name}`, recua para
  `{db.operation.name}` e depois para `postgresql`, e leva
  `db.system.name=postgresql`, `db.namespace`, `server.address`,
  `server.port`, `db.query.text` parametrizado e sem argumentos,
  `db.response.returned_rows` no `SELECT` e `dmpf.db.rows_affected` no DML;
  operação e coleção só saem de statement declarado (`declare`), porque a
  semconv veda derivá-las de SQL com mais de uma, e `BEGIN`, `COMMIT`,
  `ROLLBACK`, `SAVEPOINT`, `RELEASE SAVEPOINT` e `ROLLBACK TO SAVEPOINT` levam
  `db.operation.name` no lugar de `db.query.text`, sem o nome do savepoint
  (`sp_N` ou `dmpf_unique`). Num `pgx.Batch`, cada statement abre o seu
  span, filho do chamador e com os mesmos nome e atributos do statement
  enviado sozinho. Os statements de um batch estão em voo juntos: cada span
  começa no envio do batch e termina quando o seu resultado é lido, e a
  duração de cada um não separa o tempo de servidor de cada statement (sob
  espera num índice único, `SAVEPOINT`, escrita e `RELEASE` mostram a mesma
  espera). Um erro do servidor (`*pgconn.PgError`) fica no primeiro statement
  sem resultado, venha ele no resultado de um statement ou no fim do batch, e
  os statements que o servidor pulou, como o `RELEASE`, não abrem span. Um erro
  que não vem do servidor, como timeout, rede ou conexão fechada, marca com
  `error.type=_OTHER` todos os spans ainda abertos, venha ele antes de qualquer
  resultado, no resultado de um statement ou no fim do batch; os statements
  cujo resultado já foi lido ficam sem status. O
  `SAVEPOINT` enviado sozinho é um span de query próprio. O pool com
  `WithMeterProvider` também traça o batch. O erro é o SQLSTATE, em `db.response.status_code` e `error.type`, nunca `Message`,
  `Detail` nem `Hint` do `PgError` (RF-B6). Com `WithMeterProvider`, o pool grava as
  métricas da semconv por `dbconv`, todas por `db.client.connection.pool.name`
  (`{server.address}:{server.port}/{db.namespace}`):
  `db.client.connection.count` por `db.client.connection.state` (`idle` ou
  `used`), `db.client.connection.max`, `db.client.connection.pending_requests`
  e `db.client.connection.timeouts` por callback — `pending_requests` das
  aquisições em curso, os demais de `pgxpool.Stat()` —, e
  `db.client.connection.wait_time` pela aquisição, com as fronteiras advisory
  da semconv para segundos (RF-D6, RF-D4). `DescribeDSN` decompõe o DSN para
  o `process configured` — host, porta, banco e `sslmode` —, com o usuário só
  como `set` ou `unset` e nunca a senha (RF-A6, LOG-09). `AssertOwnOutbox`
  recusa a partida se a `outbox` tiver registro pendente de um destino que o
  contexto não publica — a proteção contra banco compartilhado entre
  contextos, promovida da composition root para o kernel.
- **`uow.go`** — `NewUnitOfWork[R]`, `Tx` e `Within`. Uma transação por
  chamada, em `READ COMMITTED` explícito qualquer que seja o padrão do
  servidor, o callback invocado uma única vez, e as seis cláusulas do
  contrato provadas em `uow_test.go`. O `Tx` conta as escritas de agregado com
  linha afetada e as de outbox, nunca as da inbox: é a guarda que deixa a
  violação declarada convertível só como primeira escrita (DEC-10).
- **`outbox.go`** — `Tx.Outbox(mapper)` e `Enqueue`. Grava na mesma `pgx.Tx` do
  estado de negócio: um commit torna os dois visíveis, ou nenhum. A `metadata`
  grava o contexto de mensagem de `ENV-08` que o adapter autorou —
  `correlationid`, `causationid`, `traceparent` e `tracestate`, o contexto de
  criação que o relay leva ao envelope sem alteração — mais o `tenantid`,
  resolvido do contexto de execução na escrita (`enqueueTenant`) — ausência é
  legítima aqui, porque uma cadeia de plataforma não tem tenant (ADR-050).
- **`mapper.go`** — a interface `EventMapper` e a conferência de major. O
  mapeamento concreto é de cada bounded context.
- **`destination.go`** — a forma de `BLK-04`: nome de fluxo lógico em segmentos
  minúsculos separados por ponto. ARN, URL, caminho e maiúscula são recusados.
- **`outbox.sql` / `inbox.sql` / `migrate.go`** — as capacidades `Outbox` e
  `Inbox` que `Migrate` aplica sob pedido, seguidas do schema do contexto
  (ADR-053): na `outbox`, os dezoito campos de FND-04 §4.1 mais
  `payload_hash`, com `UNIQUE (message_id)` e três `CHECK`; na `inbox` de
  §6.1, `UNIQUE (consumer_name, message_id)` e `CHECK` de dois valores
  terminais (`INB-01`, `INB-02`); na `quarantine`, o envelope em `bytea`.
  As três ficam **fora** do escopo de tenant (ADR-050): o relay reivindica por
  lease sem contexto de requisição, a inbox dedupe pela identidade da mensagem
  e a quarantine guarda até envelope cujo tenant não é confiável. As tabelas
  de agregado são de cada contexto, em `apps/backend/<ctx>/provider/schema.sql`,
  com `tenant_id NOT NULL` à frente da chave primária. `Migrate` é idempotente
  e não há tabela de versão: não existe ferramenta de migração aqui.
- **`grants.go`** — o privilégio da role de runtime por tabela (ADR-063):
  `Privilege`, `Grant`, `Mutable` (tabela que um `Table` escreve) e
  `AppendOnly` (tabela que só recebe linhas), e `KernelGrants`, que cada
  capacidade declara junto do schema (`outbox` e `inbox` com `SELECT`, `INSERT`,
  `UPDATE` e `DELETE`; `quarantine` com `SELECT` e `INSERT`). `ApplyGrants`
  converge a role ao conjunto declarado numa transação, sob o lock do
  `Migrate`, depois de recusar runtime superuser, com `BYPASSRLS`, membro da
  role que aplica ou dona de objeto fora dos schemas de sistema (relação,
  função, tipo) ou de large object, com o `REASSIGN OWNED` que a corrige,
  tabela de `public` sem grant e sequência de `public` que não é de coluna
  `IDENTITY`. `AssertGrantable` roda só essas recusas da role, sem mudar nada,
  para o migrador recusar antes do `Migrate`. Depois dos `GRANT`, na mesma
  transação, confere o privilégio efetivo (`has_table_privilege`,
  `has_any_column_privilege`, `has_sequence_privilege`, `has_schema_privilege`,
  `has_database_privilege`, `has_function_privilege`, `pg_has_role`) e só
  comita se a role tiver todo privilégio declarado e nada além dele. Falta
  recusada: privilégio declarado que o `GRANT` não concedeu, porque quem o
  emitiu não é dono da tabela e o PostgreSQL só avisa. Sobra recusada:
  privilégio de tabela herdado ou de `PUBLIC`, inclusive o concedido só a uma
  coluna e o `MAINTAIN` a partir do PostgreSQL 17; `USAGE`, `SELECT` ou
  `UPDATE` em sequência de `public`, que a coluna `IDENTITY` dispensa e o
  `pg_write_all_data` concede; `CREATE` em qualquer schema que não é de
  sistema; `CREATE` ou `TEMPORARY` no banco; membership no dono do banco ou de
  um schema; `REPLICATION`, `CREATEROLE` ou `CREATEDB`; membership, direta ou
  indireta e mesmo sem `INHERIT`, em qualquer outra role, com ou sem atributo
  (`pg_execute_server_program`, `pg_write_all_data`); e `EXECUTE` em função
  `SECURITY DEFINER` fora dos schemas de sistema. O erro nomeia, de uma vez, o
  que falta e o que sobrou. `Migrate` e `ApplyGrants` fixam
  `search_path = public` na transação.
- **`session.go`** — `SessionOf` lê a role (`current_user`) e o banco de uma
  pool, distinguindo o servidor pela hora de partida do postmaster;
  `SameDatabase` compara banco, oid e servidor, nunca endereço, para valer
  através de pooler.
- **`wait.go`** — `WaitForTables` espera a tabela existir e `WaitForGrants`, o
  privilégio de cada grant (`has_table_privilege`), para o processo que lê o
  schema que outro aplica, cada passada numa consulta só. As duas procuram a
  tabela em `public`, com o nome citado. Dono e superuser têm o privilégio
  assim que a tabela existe. `WaitForUniques` repete a consulta de
  `VerifyUniques` só enquanto ela divergir (`ErrUniqueDivergent`) e devolve na
  hora qualquer outro erro; o registro `waiting for unique constraint` leva
  `db.collection.name`, `dmpf.db.constraint.name` e
  `dmpf.db.constraint.divergence`. A tabela particionada ou com herança
  nunca converge: a espera só acaba no prazo, com o motivo no registro e no
  erro. No fim do contexto, as três esperas
  devolvem o erro dele junto com o que ainda faltava; a de restrições preserva
  o `ErrUniqueDivergent`.
- **`inbox.go`** — `Tx.Inbox(consumer, wait)`, `Register` e `Pending.Complete`
  (`KRN-07`). `Register` é `INSERT … ON CONFLICT DO NOTHING` seguido de leitura:
  devolve a classificação (R1, R2, R3 ou R4), nunca erro de constraint, e a
  transação segue viva (`INB-04`). Sob concorrência a inserção aguarda a
  transação concorrente e a classificação é sempre de commit (`INB-06`,
  `INB-18`). `wait > 0` vira `SET LOCAL lock_timeout`; o estouro chega como
  `ports.ErrRegisterTimeout` (`55P03`), que o service classifica R1×D3
  (`INB-17`). Entre `Register` e `Complete` a linha existe com `status`
  provisório dentro da transação — ninguém a observa (ver
  `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`).
- **`inbox.go`, comandos** — `Tx.CommandInbox(consumer, wait)` é a mesma inbox
  para comandos (FND-04 §7.6, ADR-056). O `message_id` é `<tenant>/<chave>`,
  com o tenant do contexto de execução, e o comando sem tenant ou sem
  `ExpiresAt` é recusado. A `inbox` ganha `outcome` e `expires_at`, anuláveis:
  a entrada de comando vencida é substituída na inserção
  (`ON CONFLICT … DO UPDATE WHERE expires_at <= received_at`), e R2 e R3
  devolvem o `outcome` gravado. A espera é `WaitUntil − ReceivedAt`, limitada
  por `wait`, com `lock_timeout` de pelo menos 1ms.
- **`quarantine.go`** — `NewQuarantine(pool, opts...)`, a realização de
  `ports.Containment` fora de qualquer UoW: grava o envelope byte a byte
  como foi publicado (`GAR-07`) e o erro sanitizado (`ERR-20`, `ERR-21`), uma
  vez por `(consumer_name, envelope_digest)`: a mesma mensagem contida de novo
  não gera outra linha. Gravada a contenção, sai `message contained` em
  `warn`, com `dmpf.containment.reason` e `messaging.message.id`, no
  `LoggerProvider` de `WithQuarantineLoggerProvider` (RF-A6).
- **`signals.go`** — `InboxSignals(pool, consumer)`: profundidade da quarantine
  e contagens por motivo (`terminal-failure`, `collision`,
  `attempts-exhausted`, `invalid-envelope`, `untrusted-boundary`), lidas da
  própria tabela (`GAR-12`).
- **`purge.go`** — `PurgePublished`, `PurgeInbox` e `PurgeExpiredInbox`, que
  devolvem o que purgaram, de qual consumidor e até quando (`OBX-17`, `INB-16`,
  `IDM-09`). Cada chamada remove no máximo um lote, com `FOR UPDATE SKIP LOCKED`,
  para que réplicas purguem ao mesmo tempo sem se bloquear; `PurgeExpiredInbox`
  remove só as entradas de comando vencidas. O laço que as chama é o
  `app.RunPurge`, e a invariante `retenção_inbox ≥ janela_redelivery`
  (`INB-14`) é conferida pelo contexto que consome.

O que `memory` declara não provar — isolamento e conflito de serialização
entre transações concorrentes — é provado sobre este módulo pela cláusula de
escritores concorrentes do `providerkit.Repository` (`Concurrent: true` em
`conformance_test.go` e nos testes de repositório dos contextos): dois escritores
leem a mesma versão, e exatamente um passa; o outro recebe `ErrVersionConflict`
em vez de sobrescrever em silêncio. `inbox_concurrency_test.go` faz o mesmo para a inbox: duas
transações registram a mesma chave, a segunda bloqueia até o desfecho da
primeira e recebe R2, R3 ou R1 conforme ela commitou `processed`, commitou
`rejected` ou desfez; e o teto de espera é interrompido pelo servidor.
`unique_concurrency_test.go` prova a unicidade declarada sob concorrência: de
duas criações com o mesmo valor, uma grava e a outra recebe a rejeição, que a
inbox commita como `rejected` na transação que a violação deixou utilizável;
duas criações da mesma identidade terminam em `ErrVersionConflict`, nunca em
recusa.

`apps/backend/reservations/provider` é o lado do consumo do agregado de
referência: o repositório com chave natural (`order_id`, `ON CONFLICT DO
NOTHING` — a convergência de `GAR-10`) e o mapeador de `ReservationConfirmed`.
A composition root que liga tudo é o package raiz de `apps/backend/reservations`,
o bloco `app` do contexto.

## Escopo de tenant: choke point fechado por gate

Antes desta entrega, cada contexto escrevia o próprio SQL sobre um `pgx.Tx`
cru, obtido por `Tx.Conn()`, e os readers tomavam o `*pgxpool.Pool` direto.
`Tx.Conn()` foi removido; o pool chega ao provider de contexto só como
`ReadPool` (`NewReadPool`), um handle opaco sobre o qual só rodam os
statements que `Table` compila (ADR-051). A regra `depguard` `context-provider`
(`.golangci.yml`) nega `github.com/jackc/pgx`, `database/sql` e
`gorm.io/gorm` em `apps/**/provider/**`, e `tools/dmpf-plugin/scripts/dmpf-gate-check.sh` prova no
CI que ela reprova os três drivers em cada contexto; a única exceção nominal é
a DDL de contexto, aplicada pela composition root por `postgres.Migrate`.

Row-Level Security foi medida e descartada: nos três ambientes versionados
(local, CI, Kubernetes) o role que abre conexão é o superuser de bootstrap e o
dono das tabelas, e o Postgres ignora RLS incondicionalmente para superuser —
ligá-lo não protegeria nada (ver ADR-051).

Quando um statement escopado não encontra a linha, `miss` sonda qual outro
tenant a detém — sem trazer o estado — e devolve `ports.CrossTenantAccess`, que
se desembrulha para `ErrNotFound` e responde igual ao chamador (`IDN-13`)
enquanto o registro interno diz qual tenant é o dono (`IDN-12`). `Relation`
segue a mesma regra para a consulta por relação. Ausência de tenant no
contexto de execução recusa com `ErrTenantUnresolved`, nunca alarga a consulta
(`IDN-15`).

## Acesso entre tenants declarado

O `Table` tem dois caminhos que não escopam ao tenant do contexto, e só esses
dois (ADR-068, que guarda a decisão e as alternativas). Os dois recebem na
construção a trilha como `ports.AuditTrail`, que a composition root obtém do
`audit.Recorder` por `Recorder.Trail()`, e entram em pânico sem ela: montar
acesso entre tenants sem trilha é erro da composition root.

**`Table.PlatformReader(pool, trail, opts...)`** realiza
`ports.PlatformReader` sobre o `ReadPool` da role de runtime, sem predicado de
tenant, só com `SELECT` compilado e declarado pelo kernel. Os statements de
plataforma são compilados à parte (`compilePlatform`), então a tabela que
nunca lê entre tenants não declara statement sem tenant. A ordem das recusas é
fixa:

1. Sem `ports.ReachAllTenants` no carrier, emite o evento `denied` e devolve
   `ports.ErrPlatformReachUndeclared`.
2. Sem contexto de execução, devolve `ports.ErrContextAbsent`, sem evento.
3. No `List`, `limit` zero, negativo ou acima do teto devolve
   `ports.ErrPlatformLimitInvalid`. O teto é `ports.PlatformPageMax` (500), e
   `WithPageMax(n)` o troca por outro valor positivo; zero ou negativo entra em
   pânico.

O `LoadIn(ctx, tenant, id)` emite o evento `accepted` do tenant pedido antes de
ler, e a linha ausente é `ports.ErrNotFound`, sem sonda de dono; tenant vazio é
`ports.ErrNotFound` sem evento. O `List` pagina por
keyset `(tenant_id, id)`, em `ORDER BY tenant_id, <id>`, com `after == nil` na
primeira página e `WHERE (tenant_id, <id>) > ($1, $2)` nas seguintes, e emite
um evento `accepted` por tenant da página antes de devolvê-la; página vazia não
alcança tenant e não emite. Os eventos levam a ação
`ports.AuditPlatformRead`.

**`Table.Projector(tx, policy, trail)`** realiza `ports.Projector` sobre o
`*Tx`, e não sobre a pool, para que a projeção commite com a inbox e a outbox
sob o mesmo `Within` (`UOW-01`). `Load` e `Save` passam pela mesma autorização
e emitem a mesma ação, `ports.AuditProjectionWrite`:

1. Sem contexto de execução, `ports.ErrContextAbsent`, sem evento.
2. Destino vazio: evento `denied` e `ports.ErrProjectionDestinationAbsent`.
3. Destino igual ao tenant do contexto: evento `denied` e
   `ports.ErrProjectionDenied`, sem consultar a política, porque a projeção não
   é um segundo caminho de escrita no próprio tenant.
4. Política nula ou que nega: evento `denied` e `ports.ErrProjectionDenied`,
   embrulhando a causa da política quando há uma (default fechado, `IDN-17`).
   A política recebe a origem do contexto, `nil` quando ele não tem tenant
   (`IDN-20`).
5. Autorizado: evento `accepted` do destino antes de ler ou gravar.

O `Save` tem a semântica de `version` do `Repository` e grava `tenant_id` com o
destino. O `Load` lê no destino **sem** a sonda de dono do `miss`: a linha
ausente é `ports.ErrNotFound` sem `CrossTenantAccess`, porque a sonda
registraria um `IDN-12` falso e revelaria a existência em outro tenant.

**Trilha que falha fechada.** O `audit.Recorder` só deixa o acesso prosseguir depois
de o `Sink` confirmar o registro; a recusa do `Sink` vira
`ports.ErrAuditUnavailable`, sem devolver linha nem gravar. Na recusa
(`denied`), a falha do `Sink` não muda o desfecho, que já é recusa. O evento
leva `Object` da tabela, `DataTenant` do tenant alcançado (vazio quando a
recusa é anterior a ele), `Tenant` do contexto e `Subject` do portador.
**Custo aceito:** a falha fechada vale o que vale o `Sink` configurado pelo
consumidor. O `audit.NewLogSink` sempre confirma e não serve como trilha
durável; sem `Sink` durável, a falha fechada só é exercida no kit e nos testes
com `Sink` que recusa (ADR-068, Consequências).

**Gate (`IDN-14`).** O `forbidigo` do `.golangci.yml`, com `analyze-types`,
reprova `Table.PlatformReader` e `Table.Projector` de `postgres` e de `memory`
fora de `apps/**/provider/platform_*.go`, ao lado da regra `context-provider`
do `depguard`; o kernel, os kits de teste e os `_test.go` ficam isentos. Como o
`forbidigo` não vê a chamada por uma interface local que o `Table` satisfaz, o
`tools/dmpf-plugin/scripts/dmpf-context-check.sh` reprova, por nome,
`PlatformReader`, `Projector` e `WithPlatformReach` em `apps/*/provider/*.go`
fora de `platform_*.go` e dos `_test.go`; um identificador local homônimo
também reprova e precisa ser renomeado. O
`dmpf-gate-check.sh` prova as reprovações e as passagens.

## O que o módulo não contém

Claim, lease, `SKIP LOCKED`, transição de `status` e publicação são do `KRN-08`;
o codec do envelope e a fórmula do `payload_hash`, do `KRN-05` — este módulo os
chama, não os define; a tradução de destino lógico para alvo físico, o gesto de
ACK e a DLQ, do `KRN-10`; o conteúdo de `metadata` além do contexto de
`ENV-08` e do tenant, de FND-07; métricas além das do pool, o valor do teto de
espera e os prazos de retenção, de FND-08; um test kit exportado, do `KRN-11`.
O application service de consumo e as sete disposições são do bloco
`application` do contexto (`apps/backend/reservations/application`); o adapter
que aplica o efeito de
broker e que verifica se a mensagem chega de dentro da fronteira confiável é
do bloco `app` (`app`, ADR-052).

A garantia é **at-least-once**. A deduplicação por identidade de mensagem é da
inbox; a idempotência do efeito é do domínio, por chave natural (`GAR-03`).

## Como rodar os testes localmente

Os testes de banco levam a build tag `integration` e só rodam no target
`test-race`. Sem `PG_DSN` eles pulam com instrução fora do CI, e **falham**
dentro dele — um skip silencioso deixaria a outbox sem prova executável.

O target sobe a infra de testes (`testkit:test-infra-up`) e o
`tools/dmpf-plugin/scripts/test-env.sh` preenche o `PG_DSN` com o Postgres dela, na porta 15432, a
partir do `.env.example` da raiz:

```bash
pnpm nx run postgres:test-race
```

Sem a infra de testes no ar, `pnpm nx run-many -t test -p postgres`
segue verde: os testes unitários (destino, mapeador) não têm a tag.

O `test-race` deste módulo roda com `cache: false` no Nx **e** `-count=1` no
`go test`. São dois caches distintos, e os dois devolveriam resultado antigo
para teste de banco: nenhum deles enxerga o estado do Postgres.

Desde o `KRN-07`, `app` também tem testes de banco, sobre as mesmas
tabelas. Os dois `test-race` não podem correr em paralelo — cada harness faz
`TRUNCATE` — e por isso o do `app` declara `dependsOn` sobre o deste
módulo: o Nx os sequencia mesmo com `--parallel=3`, que é como o CI roda.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,build,lint,test-race -p postgres
go run ./tools/dmpf-conformance/cmd/conformance --root .
bash tools/dmpf-cell-check.sh
```

O `dmpf-gate-check.sh` **não** exercita este módulo: o `depguard` do
`.golangci.yml` seleciona por nome de diretório (`domain/`, `ports/`,
`application/`, `contracts/`) e não alcança `postgres`. O gate autoritativo
aqui é o verificador, e o `dmpf-cell-check.sh` prova as duas células que este
módulo torna alcançáveis: 26 (`provider → application`) e 12
(`application → contract`). A regra `context-provider` que fecha `Tx.Conn()`
para os contextos é provada pelo `dmpf-gate-check.sh`, não por este módulo — ela
mira `apps/**/provider/**`, não `postgres` em si.

## Referências

- `docs/adr/035-realizacao-postgres-da-outbox.md` — as decisões deste módulo.
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que ele realiza.
- `docs/adr/050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md` — por
  que `outbox`, `inbox` e `quarantine` ficam fora do escopo.
- `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md` — `Table`, `ReadPool`
  e o gate `context-provider`.
- `docs/adr/068-acesso-entre-tenants-declarado.md` — `PlatformReader`,
  `Projector`, a trilha que falha fechada e a regra de plataforma do gate.
- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.3, §3.1 a §3.3, §4.1, §4.2.
- `docs/specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md` — os `RF-*`
  citados aqui: os spans `db.*`, as métricas do pool e o `process configured`.
- `libs/backend/go/ports/README.md` — as portas em tipos de domínio, o
  contexto de execução e `CrossTenantAccess`.
- `libs/backend/go/application/README.md` — os nove passos e quem os anda.
