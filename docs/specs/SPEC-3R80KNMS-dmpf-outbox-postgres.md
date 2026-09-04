---
id: SPEC-3R80KNMS
slug: dmpf-outbox-postgres
title: DMPF KRN-06 — Outbox Postgres: provider, mapeamento e serialização na escrita
stage: building
priority: P0
depends_on: [SPEC-MQA5HAXF, SPEC-WTAXFV8B, SPEC-ZHE7DN1H, SPEC-WYX5GW87]
ticket_url: https://lider-cap.atlassian.net/browse/ARQ-525
subtask_urls: []
created: 2026-09-04
---

# SPEC-3R80KNMS: DMPF KRN-06 — Outbox Postgres: provider, mapeamento e serialização na escrita

## Resumo

Criar o módulo Go `dmpf-provider-postgres-go` (`libs/backend/go/dmpf-provider-postgres`,
bloco `provider`), que realiza sobre PostgreSQL as três portas transacionais que o
`KRN-04` fixou em tipos de domínio — `UnitOfWork[R]`, `Repository[ID, S]` e
`Outbox` — e que, ao receber `(domain event, intenção de publicação)`, **mapeia**
o evento para o integration event do contrato Protobuf do `KRN-05`, **serializa**
os bytes de forma determinística e os **grava na mesma transação** do estado de
negócio, com o schema mínimo de FND-04 §4.1, os valores iniciais de `OBX-05` e a
unicidade de `message_id` de `OBX-01` impostos pelo banco. Como o agregado de
exemplo produz dois eventos e o `KRN-05` publicou um só contrato, esta história
também acrescenta `ItemAdded` ao pacote `company.orders.event.v1` e um campo
aditivo a `OrderPlaced`, sob os gates Buf. Como time de plataforma, queremos que a
atomicidade entre estado de negócio e intenção de publicar (`UOW-07`), o ramo
`Rejected` sem efeitos (`UOW-06`), a serialização na escrita (`BLK-03`, ADR-021)
e a proibição de publicar dentro da transação (`UOW-08`, §9.5) deixem de ser
provadas contra um mapa em memória e passem a ser provadas contra um banco real,
com isolamento real — para que o `KRN-08` tenha o que drenar e o `KRN-07` tenha
uma UoW de verdade sobre a qual gravar a inbox.

Esta é a primeira história do Incremento 3 ("Garantias") da spec guarda-chuva
[SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md): "Mensagem publicada
atomicamente e consumida com deduplicação". Ela substitui a realização em
memória do `KRN-04` no papel de provider de referência sem removê-la — a
realização em memória continua sendo o duplo de teste do `application`.

## Contexto

- **Problema**: a sequência canônica de FND-04 §3.2 está realizada em Go
  (`libs/backend/go/dmpf-application/example/orders/add_item.go:45-50`), mas o
  único provider que a executa é `example/memory`, cuja "transação" é um par de
  mutexes e cuja "outbox" é uma fatia `[]dmpfports.OutboxEntry` que guarda o
  evento de domínio em memória, sem bytes de wire, sem schema, sem unicidade e
  sem estado de drenagem (`libs/backend/go/dmpf-application/example/memory/tx.go:108-113`).
  O `README.md` do módulo registra o que falta: "Provider Postgres, schema de
  outbox e isolamento real (`KRN-06`)". Sem isso, `UOW-07` é provado por um
  `append` sob lock, `OBX-01` não tem quem o imponha, o mapeamento de ADR-021
  não tem sujeito e a contraprova de FND-04 §9.4 (outbox escrita pela tabela)
  não tem tabela contra a qual falhar.
- **Impacto**: o `KRN-08` recebe uma tabela `dmpf_outbox` com os campos de
  §4.1, registros nascendo `pending` e um índice sobre `published` para a
  purga; o `KRN-07` recebe uma `UnitOfWork[R]` Postgres para gravar a inbox e
  os efeitos locais na mesma transação; o verificador do `KRN-02` passa a ter,
  pela primeira vez no `develop`, uma unidade `provider` que importa `contract`
  (célula 30) e coexiste com `dmpf-contracts-go`, o que dá vetor de módulo à
  célula 12 — dívida declarada na
  [SPEC-ZHE7DN1H](./SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md) (linha 98).
- **Inspiração**: o exemplo conforme de FND-04 §9.2 e a contraprova de §9.5
  (publicar dentro da transação); a decisão real de `shared-titulos-services`
  (RFC §7.5), que grava `backoffice_outbox` na transação do caso de uso; a
  cadeia `application → port ← provider → contract` de ADR-021, que mantém as
  células 11, 12 e 24 intocadas; o `Pack` determinístico do `KRN-05`
  (`envelope.go:64`) como a única forma de produzir bytes contemporâneos do fato.
- **Links relevantes**:
  - [SPEC-YRJRADY9](./SPEC-YRJRADY9-dmpf-kernel-sdk-go.md) — guarda-chuva;
    requisito `KRN-06` (linha 164), tabela de tarefas (391), dependência do
    `KRN-08` (393), incremento 3 (405)
  - [SPEC-ZHE7DN1H](./SPEC-ZHE7DN1H-dmpf-kernel-aplicacao-go.md) — `KRN-04`;
    fixou `UnitOfWork[R]`, `Repository[ID, S]`, `Outbox`, `OutboxEntry`,
    `PublishIntent`, `Instant` e `MessageID`; "Escopo fora" delega a esta spec
    o provider Postgres, o schema e o isolamento real
  - [SPEC-WYX5GW87](./SPEC-WYX5GW87-dmpf-contratos-wire.md) — `KRN-05`; fixou
    `envelope.Pack`, o `payload_hash` versão 1, a fixture golden por contrato
    (`INT-02`) e a máquina de estados do baseline (`BUF-08`); esta spec é o
    primeiro consumidor de `dmpf-contracts-go` fora do próprio módulo
  - [SPEC-XF9TF9A0](./SPEC-XF9TF9A0-dmpf-kernel-dominio-go.md) — `KRN-03`; os
    eventos `OrderPlaced` e `ItemAdded` e o `Snapshot` do agregado são o que
    este provider mapeia e persiste
  - [SPEC-WTAXFV8B](./SPEC-WTAXFV8B-dmpf-verificador-conformidade-go.md) —
    `KRN-02`; o verificador é o instrumento dos critérios de dependência
  - [SPEC-MQA5HAXF](./SPEC-MQA5HAXF-dmpf-fundacao-nx-go.md) — `KRN-01`;
    convenção de módulo, tags 3D, cadeia de sete passos, `bounded_context:
    dmpf-kernel`
  - `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2.1–§2.3, §3.2, §3.3, §4.1–§4.3,
    §9.5
  - `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) — `PTB-03`, `PTB-06`,
    `PTB-09`, `INT-02`, `ENV-16`, `ENV-18`, `BUF-08`, `BUF-11`, `BUF-12`
  - `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §4.1, §6.2, §6.3, §7.3, §7.4
    (células 10, 11, 12, 24, 25, 26, 28, 30), §7.5, §10.2, P0-2, P0-3
  - `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `CTX-20`, `CTX-21`,
    `ERR-22`
  - ADRs: `docs/adr/010-*` (seis blocos e conjunção C1∧C2), `017-*` (condição
    de contexto C2), `020-*` (relay por polling e leasing — fronteira com o
    `KRN-08`), `021-*` (mapeamento no provider, serialização na escrita),
    `022-*` (codec `proto_data` e `payload_hash`), `028-*` (autorização da
    classificação), `031-*` (verificador), `032-*` (desfecho da UPR em Go),
    `033-*` (adaptações monorepo dos contratos), `034-*` (fronteira de UoW em
    Go). O ADR desta história é o **`035`**.

### Divergências entre o ticket e o repositório

O ticket ARQ-525 foi escrito em 30/08, antes de `KRN-04` e `KRN-05` serem
entregues. Sete pontos do texto não batem com o estado do repositório e ficam
fixados aqui, sem reabrir decisão alguma:

| Ticket diz | Repositório | Esta spec fixa |
| --- | --- | --- |
| Entregável 1: "**Porta de outbox** — contrato no bloco `port`, expondo tipo de domínio" | A porta já existe: `Outbox`, `OutboxEntry` e `PublishIntent` em `libs/backend/go/dmpf-ports/outbox.go`, entregues pelo `KRN-04` com os sete campos de autoria do `application service` (§2.3) e sem campo de wire nem de drenagem (`BLK-05`) | A porta **não é alterada**. O entregável 1 está cumprido; esta spec entrega o provider que a realiza. O critério "assinatura da porta não menciona tipo Protobuf" vira re-verificação sobre o módulo existente |
| "Porta em `libs/backend/dmpf-ports`" e "Provider em `libs/backend/dmpf-provider-postgres`" | Módulos Go vivem em `libs/backend/go/<módulo>` e o projeto Nx leva o sufixo `-go` (`AGENTS.md`, "Caminho por scope e stack") | Módulo em `libs/backend/go/dmpf-provider-postgres`, projeto Nx `dmpf-provider-postgres-go` |
| "Schema mínimo de §4.1 com … valores iniciais declarados (`OBX-05`): `pending`, `available_at = occurred_at`" e, em Riscos, "`available_at` nulo … mitigado pelo default no schema" | PostgreSQL não admite `DEFAULT` que referencie outra coluna da mesma linha; `available_at = occurred_at` não é expressável como default de coluna | `available_at` é `NOT NULL` com `CHECK (available_at >= occurred_at)`, e o provider o grava explicitamente igual a `occurred_at` no `INSERT`. O teste do cenário 5 prova o valor inicial. `status` e `attempt_count` têm `DEFAULT` de coluna |
| "Autoria conforme §2.3: … `metadata` sem dado sensível" (`OBX-02`) | `OutboxEntry` não carrega `metadata`; o `application service` de referência não autoria nenhum metadado, e o conteúdo de `metadata` é `encaminhado` a FND-07 (§4.1) | Coluna `metadata jsonb NOT NULL DEFAULT '{}'`; o provider grava o objeto vazio. Estender `OutboxEntry` com metadados é mudança de porta e fica encaminhada ao primeiro consumidor que os autorie (`KRN-07`/`KRN-09`) |
| "Formato do integration event, codec e fórmula do hash (KRN-05)" está **fora do escopo** | O `example/orders` produz `OrderPlaced` e `ItemAdded` (`libs/backend/go/dmpf-domain/example/orders/messages.go:44-62`); o `KRN-05` publicou apenas `order_placed.proto`. Sem contrato para `ItemAdded`, o caso de uso `AddItem` não tem o que serializar | **Decisão do time (04/09)**: esta história acrescenta `item_added.proto` e o campo aditivo `item_count` a `OrderPlaced`, com fixture golden própria (`INT-02`), sob os gates Buf. O codec e a fórmula do hash continuam intocados |
| DoD: "Critérios de aceite verificados no CI" | O job `main` de `.github/workflows/ci.yml` roda em `gitea-runner` sem bloco `services:`; não há Postgres alcançável no CI | O CI ganha um serviço `postgres:16-alpine` e a variável `DMPF_PG_DSN`; os testes de integração **reprovam** quando `CI` está definido e a variável está ausente (fail-closed), e são pulados só fora do CI |
| `README.md` de `dmpf-application`: "Provider Postgres, schema de outbox, `SKIP LOCKED` e isolamento real (`KRN-06`)" | `FOR UPDATE SKIP LOCKED` é admitido "somente durante o claim" (`OBX-07`), e o claim é do relay (ADR-020, `KRN-08`) | `SKIP LOCKED` **não** entra nesta história; a menção (uma só, em `README.md:144`) é corrigida para apontar o `KRN-08` |

### Pré-requisito externo: marca de baseline dos contratos

Como esta história toca `contracts/proto/`, o projeto `dmpf-contracts-go` entra
no `affected` e o gate `buf-breaking` roda. Em `tools/buf-gate.sh:106-107`, um
módulo que já existe em `NX_BASE` **sem** a marca `contracts-baseline/proto`
reprova por construção ("estado invalido, nao 'sem baseline'"). Hoje a marca não
existe nem localmente nem em `origin` (verificado em 04/09), e ela precisa ser
criada por alguém que **não** seja o autor do módulo (`contracts/README.md`,
"Gates e baseline"). Enquanto a marca não existir, **nenhum PR que toque
`contracts/` fica verde**. Esta spec registra o fato como dependência externa da
Etapa de execução, não como algo que ela resolve: a criação da marca é
autorização organizacional (equipes `tech-leads` e `Owners`), fora do alcance de
código.

### Fontes normativas

Esta spec **não reabre** nenhuma decisão da fundação nem da guarda-chuva. Toda
exigência abaixo tem origem:

| Exigência | Fonte |
| --- | --- |
| Três responsabilidades da outbox em três blocos; o caso de uso grava por porta, nunca pela tabela | FND-04 §2.1, `BLK-01`, `BLK-02`; RFC §7.5 |
| Mapeamento no `provider`, serialização na escrita, porta sem tipo de wire | FND-04 §2.2, `BLK-03`; ADR-021 |
| Autoria dos campos: identidade, tempo, roteamento e origem pelo `application service`; `message_type`, `schema_version`, `payload` pelo `provider`; estado de drenagem pelo schema e pelo relay | FND-04 §2.3, `BLK-04`, `BLK-05` |
| Passos 6 e 7 na mesma transação, commit no passo 8, nenhum passo publica | FND-04 §3.2, `UOW-05`..`UOW-08` |
| Optimistic locking; sem retry automático do callback | FND-04 §3.3, `UOW-09`, `UOW-10`; `dmpf-ports/repository.go` (`ErrVersionConflict`) |
| Schema mínimo e unicidade de `message_id` | FND-04 §4.1, `OBX-01`, `OBX-02` |
| Estados e valores iniciais do registro | FND-04 §4.2, `OBX-04`, `OBX-05` |
| Retenção: `published` purgável com evidência; os demais não | FND-04 §4.3, `OBX-17` |
| Publicar dentro da transação é não conforme | FND-04 §9.5 |
| Bytes determinísticos e `payload_hash` sobre os bytes transportados | FND-05 `ENV-18`; ADR-022; `payloadhash.go:14` |
| Uma fixture golden por contrato `event`; nome de tipo `PTB-03`; enum com `_UNSPECIFIED`; campo removido reservado | FND-05 `INT-02`, `PTB-03`, `PTB-06`, `PTB-09` |
| Gates Buf fail-closed e máquina de estados do baseline | FND-05 `BUF-08`, `BUF-11`, `BUF-12`; `contracts/README.md` |
| Blocos `provider` e `app` são permissivos em capability; `port` e `application` só `pure` | RFC §6.2; `internal/rule/capability.go` (`permissiveBlocks`) |
| Dependência externa entra por `external[]` com quatro elementos | RFC §6.3; `docs/guides/dmpf-manifesto.md` |
| Manifesto e baseline em commit próprio | RFC §10.2; ADR-028; precedente `SPEC-ZHE7DN1H` (linha 417) |
| Cancelamento por `ctx` antes de abrir; panic propaga após rollback | FND-07 `CTX-21`, `ERR-22`; ADR-034 |
| Entrega at-least-once; exactly-once fim a fim vedado | RFC §2.3, P0-3; ADR-020 |

<constraints>
- [P0] Matriz de blocos (RFC §7.3, §7.4; ADR-010): o módulo novo é bloco
  `provider`. Suas unidades importam SOMENTE `domain` (célula 25), `port`
  (célula 28) e `contract` (célula 30). NUNCA importar `application` (célula
  26) nem `app` (célula 27) em código de produção. O teste ponta a ponta que
  importa `dmpf-application/example/orders` vive em arquivo `_test.go`, que o
  toolchain exclui do universo (`internal/golist/golist.go:196`).
- [P0] Célula 11 intocada (`BLK-01`; RFC §7.5): NENHUM arquivo de
  `dmpf-application` importa o módulo novo, `pgx`, `database/sql` nem o tipo da
  linha. O caso de uso continua gravando por `dmpfports.Outbox` e
  `dmpfports.Repository`. O verificador reporta zero arestas
  `application → provider`.
- [P0] Porta intocada (`BLK-03`, ADR-021): `dmpf-ports/outbox.go` NÃO muda.
  Nenhum tipo gerado de Protobuf, nenhum `pgx`, nenhum `[]byte` de wire
  aparece na assinatura de `Outbox`, `OutboxEntry` ou `PublishIntent`. Quem
  conhece o wire é quem realiza a porta.
- [P0] Mesma transação (`UOW-07`): o `INSERT` em `dmpf_outbox` e o
  `UPDATE`/`INSERT` do agregado executam sobre a MESMA `pgx.Tx`, aberta por
  `Within`, e só ficam visíveis no `COMMIT`. NUNCA abrir conexão ou transação
  paralela dentro de `Enqueue` ou `Save`.
- [P0] Nenhuma publicação na transação (`UOW-08`; §9.5): o módulo novo NÃO
  importa SDK de broker, cliente HTTP nem `net`. Sua superfície não declara
  porta de publicação.
- [P0] Serialização na escrita (`BLK-03`; ADR-021; `ENV-18`): os bytes de
  `payload` são produzidos por `envelope.Pack` (proto determinístico) DENTRO de
  `Enqueue`, antes do `INSERT`, e `payload_hash` é `payloadhash.Sum` sobre esses
  mesmos bytes. Ler o registro NUNCA reserializa.
- [P0] Unicidade pelo schema (`OBX-01`): `UNIQUE (message_id)` é constraint da
  tabela. A recusa da duplicata vem do banco (SQLSTATE 23505), não de
  verificação prévia em código.
- [P0] Valores iniciais (`OBX-05`, `BLK-05`): todo registro nasce com
  `status = 'pending'`, `available_at = occurred_at`, `attempt_count = 0` e
  `locked_by`, `locked_until`, `published_at`, `last_error` NULOS. O provider
  NUNCA escreve outro valor nesses campos; transições são do relay (`KRN-08`).
- [P0] Destino lógico (`BLK-04`): `Enqueue` reprova com `ErrInvalidDestination`
  qualquer `Destination` fora da forma `^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`.
  ARN, URL, nome de tópico ou fila com `:`, `/` ou maiúsculas NUNCA são gravados.
- [P0] Sem retry automático (`UOW-09`, `UOW-10`): `Within` invoca `fn`
  EXATAMENTE uma vez. `ErrVersionConflict`, erro de commit e conflito de
  serialização são devolvidos ao caso de uso, sem repetição.
- [P0] Fail-closed no mapeamento: evento sem contrato registrado no mapeador
  devolve `ErrUnmappedEvent` e a transação faz rollback. NUNCA gravar registro
  sem `payload`, com `payload` vazio ou com `message_type` derivado do nome do
  evento de domínio.
- [P0] At-least-once (P0-3): NENHUM artefato desta entrega — código, godoc,
  README, ADR, `.proto`, fixture — declara ou sugere exactly-once fim a fim. O
  gate `buf-lint` varre o texto de `contracts/`.
- [P0] Governança (RFC §10.2; ADR-028): manifesto `dmpf-units.json` do módulo
  novo, entrada `external[]` para `pgx` e `protobuf`, e o baseline
  `tools/dmpf-baseline/units-baseline.json` regravado vêm em COMMIT PRÓPRIO,
  sem código Go. Toda dependência externa declara os quatro elementos
  (`package`, `versions`, `entrypoints`, `capability`).
- [P0] Gates Buf (`BUF-08`, `BUF-11`, `BUF-12`): `item_added.proto` e o campo
  novo de `order_placed.proto` passam por `buf-lint`, `buf-pins`,
  `buf-generate-check` e `buf-breaking`. `gen/go` é regenerado e versionado;
  NUNCA editado à mão. A marca `contracts-baseline/proto` é pré-requisito
  externo para o gate `breaking` ficar verde.
- [P0] Tempo como inteiro (ADR-034): `occurred_at`, `available_at`,
  `locked_until` e `published_at` são `bigint` de nanossegundos desde a época
  Unix, espelhando `dmpfports.Instant`. NUNCA `timestamptz` — perde três casas.
- [P1] Testes de integração fail-closed no CI: sem `DMPF_PG_DSN`, os testes que
  exigem banco chamam `t.Skip` fora do CI e `t.Fatal` quando a variável `CI`
  está definida. NUNCA passar em silêncio no CI sem exercitar o banco.
- [P1] `go.mod` do módulo novo declara `require` só das dependências externas
  (`pgx/v5`, `protobuf`); os irmãos do workspace resolvem pelo `go.work`, sem
  `require` e sem `replace`.
</constraints>

## Requisitos

### Funcionais

#### Módulo `dmpf-provider-postgres-go` — bloco `provider`

- [ ] **[P0] Projeto Nx e módulo Go**: `libs/backend/go/dmpf-provider-postgres`
  com `project.json` (nome `dmpf-provider-postgres-go`, tags `type:lib`,
  `scope:backend`, `stack:go`, os cinco targets `fmt-check`, `vet`, `build`,
  `test-race`, `govulncheck` copiados de `dmpf-application`), `package.json`
  (`@lidercap-apps/dmpf-provider-postgres-go`, `private: true`), `go.mod`
  (`go 1.26.4`, `require github.com/jackc/pgx/v5` e
  `google.golang.org/protobuf v1.36.12`), `go.sum`, e entrada `use` no
  `go.work`. Package raiz `dmpfpostgres`.
- [ ] **[P0] `UnitOfWork[R]` sobre `pgxpool.Pool`**:
  `NewUnitOfWork[R any](pool *pgxpool.Pool, bind func(tx *Tx) R) dmpfports.UnitOfWork[R]`.
  `Within` honra as seis cláusulas do contrato de `dmpf-ports/uow.go`:
  - `ctx.Err() != nil` antes de `BeginTx` devolve o erro sem abrir transação e
    sem invocar `fn` (`CTX-21`);
  - `fn` é invocada exatamente uma vez (`UOW-09`);
  - `fn` devolvendo `nil` → `Commit(ctx)`; erro de commit devolvido como o
    driver o produziu, e nada persiste;
  - `fn` devolvendo erro → `Rollback(ctx)` e o mesmo erro devolvido; se o
    rollback também falhar, os dois são combinados por `errors.Join`, o que
    preserva `errors.Is` sobre o erro de `fn`;
  - panic em `fn` → `Rollback` e re-panic, nunca convertido em erro (`ERR-22`);
  - `Tx` é o único caminho até as portas: `bind` recebe `*Tx` e monta `R`.
  - Nível de isolamento: o default do Postgres (`READ COMMITTED`); a exclusão
    entre escritores concorrentes vem do optimistic locking, não do isolamento.
- [ ] **[P0] `Tx`**: expõe `Outbox(mapper EventMapper) dmpfports.Outbox` e
  `Conn() pgx.Tx` (para repositórios do mesmo módulo construírem suas queries
  sobre a transação corrente). Nenhum método de commit ou rollback é exportado:
  encerrar a transação é de `Within`.
- [ ] **[P0] `EventMapper` e `Mapped`**:
  `type EventMapper interface { Map(event dmpfdomain.DomainEvent) (Mapped, error) }`
  e `type Mapped struct { Message proto.Message; Type string }`, onde `Type` é o
  nome de tipo CloudEvents no formato `PTB-03` (ex.:
  `com.company.orders.order-placed.v1`). Evento desconhecido devolve erro que
  embrulha `ErrUnmappedEvent`.
- [ ] **[P0] `Enqueue`** (realização de `dmpfports.Outbox`), nesta ordem:
  1. `entry.MessageID == ""` → `ErrEmptyMessageID`;
  2. `entry.Intent.Destination` fora da forma de `BLK-04` → `ErrInvalidDestination`;
  3. `mapper.Map(entry.Event)` → em erro, devolve-o (embrulhando `ErrUnmappedEvent`);
  4. confere que o sufixo `.v<N>` de `Mapped.Type` bate com o segmento `v<N>`
     do pacote do descriptor da mensagem (a mesma dupla conferência de `ENV-16`);
     divergência → `ErrMajorMismatch` (o do `envelope`, reexportado por
     `errors.Is`);
  5. `payload, typeURL, err := envelope.Pack(mapped.Message)`;
  6. `hash := payloadhash.Sum(payload)`;
  7. `INSERT INTO dmpf_outbox` com `message_type = mapped.Type`,
     `schema_version = typeURL`, `payload`, `payload_hash = hash`,
     `metadata = '{}'`, `available_at = occurred_at`, e os sete campos de
     autoria do `application service` copiados de `entry`;
  8. violação de `UNIQUE (message_id)` (SQLSTATE `23505`) → erro que embrulha
     `ErrDuplicateMessage` e o `*pgconn.PgError` original.
  `Enqueue` usa o `ctx` recebido só para cancelamento e prazo (`CTX-20`).
- [ ] **[P0] Migração embutida**: `Migrate(ctx context.Context, pool *pgxpool.Pool) error`
  aplica `schema.sql` (arquivo `.sql` embutido via `embed`), idempotente
  (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`), criando
  `dmpf_outbox` e `dmpf_example_orders` conforme a seção Design. Sem ferramenta
  de migração externa e sem tabela de versões de migração.
- [ ] **[P0] Purga com evidência** (`OBX-17`):
  `PurgePublished(ctx context.Context, pool *pgxpool.Pool, before dmpfports.Instant) (Purge, error)`
  executa `DELETE FROM dmpf_outbox WHERE status = 'published' AND published_at < $1`
  e devolve `Purge{Count int64, Before dmpfports.Instant}`. Registros
  `pending`, `publishing` e `failed` NUNCA são alcançados pelo `WHERE`. O
  registro da evidência (log, métrica) é do chamador, que é bloco `app`.
- [ ] **[P0] Erros exportados**: `ErrEmptyMessageID`, `ErrInvalidDestination`,
  `ErrUnmappedEvent`, `ErrDuplicateMessage`, todos `errors.New` com prefixo
  `dmpfpostgres:`; `ErrVersionConflict` e `ErrNotFound` são os de `dmpfports`,
  nunca redeclarados.
- [ ] **[P0] Repositório de exemplo** (`example/orders`, package `orderspg`):
  `NewRepository(tx *dmpfpostgres.Tx) dmpfports.Repository[orders.OrderID, orders.Snapshot]`
  sobre `dmpf_example_orders`:
  - `Load`: `SELECT version, snapshot WHERE order_id = $1`; sem linha →
    `ErrNotFound`; `snapshot` (jsonb) decodificado para `orders.Snapshot`;
  - `Save` com `expected == 0`: `INSERT ... VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`;
    zero linhas afetadas → `ErrVersionConflict`;
  - `Save` com `expected > 0`: `UPDATE ... SET version = $3 + 1, snapshot = $2 WHERE order_id = $1 AND version = $3`;
    zero linhas afetadas → `ErrVersionConflict`.
- [ ] **[P0] Mapeador de exemplo** (`example/orders`): `Mapper` realiza
  `EventMapper` para `orders.OrderPlaced` → `eventv1.OrderPlaced`
  (`order_id`, `item_count`; `customer_id`, `total_cents` e `channel` ficam no
  zero-value do proto3 porque o domínio de exemplo não os possui) com `Type`
  `com.company.orders.order-placed.v1`, e para `orders.ItemAdded` →
  `eventv1.ItemAdded` (`order_id`, `sku`, `quantity`) com `Type`
  `com.company.orders.item-added.v1`. Qualquer outro evento → `ErrUnmappedEvent`.
- [ ] **[P0] Manifesto**: `dmpf-units.json` com duas unidades no
  `bounded_context: dmpf-kernel`, ambas `block: provider`,
  `public_integration_surface: false`: `dmpf-kernel/provider-postgres`
  (include: raiz do módulo) e `dmpf-kernel/example-orders-postgres` (include:
  `.../dmpf-provider-postgres/example/orders`). `external[]` com duas entradas:
  `github.com/jackc/pgx/v5` (entrypoints raiz, `pgxpool`, `pgconn`;
  `capability: io.storage`) e `google.golang.org/protobuf` (entrypoint `proto`;
  `capability: wire.codec`), ambas com faixa de `versions` fechada por major.
  `exceptions: []`.

#### Contratos — `item_added.proto` e campo aditivo em `order_placed.proto`

- [ ] **[P0] `contracts/proto/company/orders/event/v1/item_added.proto`**:
  `message ItemAdded { string order_id = 1; string sku = 2; int32 quantity = 3; }`
  com comentário de cabeçalho declarando o tipo de envelope
  `com.company.orders.item-added.v1` (`PTB-03`). Sem enum novo. Passa
  `buf format` e `buf lint STANDARD`.
- [ ] **[P0] Campo aditivo em `OrderPlaced`**: `int32 item_count = 6;` (o
  número 5 e o nome `legacy_promo_code` permanecem reservados, `PTB-06`).
  Aditivo em `FILE`: `buf breaking` não reprova.
- [ ] **[P0] Regeneração**: `gen/go/company/orders/event/v1/item_added.pb.go`
  criado e `order_placed.pb.go` regenerado por `buf generate`;
  `buf-generate-check` prova ausência de drift (`BUF-11`). O manifesto de
  `dmpf-contracts` não muda: a unidade `dmpf-contracts/gen` já inclui o pacote
  `company/orders/event/v1`.
- [ ] **[P0] Fixture golden `contracts/fixtures/orders/event/v1/item-added.golden`**
  (`INT-02`): `identity` (`fixture`, `contract`, `type`, `dataschema`),
  `covers` (`profile_major: "1"`, `contract_major: "v1"`) e casos análogos aos
  de `order-placed.golden` que se apliquem — `all-conditionals-present`,
  `all-conditionals-absent`, `time-with-nanos`, `unknown-field`,
  `non-canonical-field-order` — cada um com `payload_bytes_hex` e
  `payload_hash`.
- [ ] **[P0] Suíte golden parametrizada**: `golden_test.go` deixa de ler uma
  constante `fixturePath` e passa a iterar uma tabela de fixtures (uma por
  contrato `event`), com `TestUpdateGolden` regenerando as duas.
  `order-placed.golden` ganha o caso `item-count-present`, que cobre o campo
  novo.

#### Gate local e vetores

- [ ] **[P0] Verificador `KRN-02` sobre o módulo novo**: `dmpf-conformance`
  aprova o `develop` com as duas unidades novas; as arestas `provider → domain`
  (25), `provider → port` (28) e `provider → contract` (30) são permitidas sob
  C2 (`bounded_context` idêntico ou `public_integration_surface: true` no
  destino, que é o caso do `contract`).
- [ ] **[P0] Vetor negativo de módulo para a célula 26** (`provider →
  application`): em `tools/dmpf-cell-check.sh` — script próprio, **não** o
  `tools/dmpf-gate-check.sh` —, um `git worktree` descartável em que uma unidade
  `provider` importa `dmpf-application` reprova com `DMPF-D001`. A célula 12
  (`application → contract`) ganha seu vetor de módulo, pago como dívida da
  `SPEC-ZHE7DN1H` (linha 98): uma unidade `application` importando
  `dmpf-contracts/envelope` reprova com `DMPF-D001`. **Ajustado na review do
  plano**: o `dmpf-gate-check.sh` prova o `depguard`, que decide por nome de
  diretório e não conhece aresta entre unidades; misturar as duas provas no
  mesmo script confundiria gates com alcances diferentes.
- [ ] **[P0] `.golangci.yml` inalterado**: os três blocos `depguard` casam por
  `**/*-domain/**`, `**/*-ports/**` e `**/*-application/**`; o módulo novo não
  casa nenhum e não precisa de exclusão. `forbidigo` continua restrito a
  `-domain/`.

#### CI e ambiente local

- [ ] **[P0] Serviço Postgres no CI**: o job `main` de
  `.github/workflows/ci.yml` ganha `services.postgres` (`postgres:16-alpine`,
  `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` = `dmpf`, healthcheck
  `pg_isready`) e `env.DMPF_PG_DSN` apontando para ele. Nenhum outro step muda:
  `test-race` e `test` já rodam os testes do módulo por `affected`.
- [ ] **[P1] Ambiente local documentado**: o README do módulo instrui
  `docker compose -f infra/local/docker-compose.yml --profile postgres up -d` e
  o `DMPF_PG_DSN` correspondente (`postgres://app:app@localhost:5432/app?sslmode=disable`,
  os defaults do compose).

#### Governança e documentação

- [ ] **[P0] Baseline**: `tools/dmpf-baseline/units-baseline.json` regravado
  com as duas entradas novas e o `digest`, em **um commit próprio** junto do
  `dmpf-units.json`, sem código Go (RFC §10.2; ADR-028).
- [ ] **[P0] ADR-035**: `docs/adr/035-realizacao-postgres-da-outbox.md` com as
  decisões da seção "Decisões técnicas" que adaptam a fundação ao terreno
  (`payload` como bytes do `Any` e não CloudEvent completo; tempo como `bigint`
  de nanossegundos; `available_at` gravado pelo provider por impossibilidade de
  default relacional; `metadata` vazio até haver autor). Entrada no
  `docs/adr/README.md`.
- [ ] **[P1] Inventário**: `AGENTS.md` (seção "Libs") ganha o sexto módulo, com
  as duas unidades e o bloco; `README.md` do módulo novo no molde do de
  `dmpf-application`; a **única** menção literal a `SKIP LOCKED` em
  `dmpf-application` (`README.md:144`) passa a apontar o `KRN-08`. **Ajustado
  na review do plano**: a spec falava em duas menções, mas `example/memory/doc.go`
  não cita `SKIP LOCKED` — ele referencia o `KRN-06`, e essa referência foi
  atualizada para nomear o módulo entregue.
- [ ] **[P1] Jira**: comentário em ARQ-525 com o link da spec e a lista de
  divergências fixadas.

### Não-funcionais

- [ ] **Cadeia Go verde** nos seis módulos: `gofmt`, `go vet`,
  `golangci-lint` (via `tools/dmpf-gate-check.sh`), `go build`, `go test`,
  `go test -race`, `govulncheck`.
- [ ] **Workspace verde**: `pnpm biome ci .` e
  `pnpm nx affected -t lint,typecheck,test,build --exclude=@nx-base-template/source`.
- [ ] **Gates fail-closed**: `dmpf-conformance` e os cinco targets Buf sem
  `continue-on-error`, sem `|| true`.
- [ ] **Determinismo**: duas execuções de `Enqueue` sobre entradas iguais
  produzem `payload` e `payload_hash` byte a byte idênticos
  (`proto.MarshalOptions{Deterministic: true}` já é o que `Pack` usa).
- [ ] **Sem dado sensível na outbox** (`OBX-02`): `metadata` é `{}`; `payload`
  contém só o que o contrato declara; nenhum erro devolvido por `Enqueue`
  reproduz o `payload`.
- [ ] **Segurança de dependências**: `govulncheck` sem achado sobre `pgx/v5`
  na versão pinada.

## Camadas afetadas

| Camada (bloco DMPF) | Afetada? | Descrição |
| --- | --- | --- |
| `domain` (`dmpf-domain-go`) | [ ] | Intocado. `OrderPlaced`, `ItemAdded` e `Snapshot` são consumidos como estão |
| `port` (`dmpf-ports-go`) | [ ] | Intocado. As três portas e os valores já existem; esta spec as realiza |
| `application` (`dmpf-application-go`) | [x] | Só documentação: duas menções a `SKIP LOCKED` redirecionadas ao `KRN-08`. Nenhum import novo, nenhuma aresta nova |
| `contract` (`dmpf-contracts-go`, `contracts/`) | [x] | `item_added.proto` novo, `item_count` em `OrderPlaced`, `gen/go` regenerado, fixture `item-added.golden`, suíte golden parametrizada |
| `provider` (novo `dmpf-provider-postgres-go`) | [x] | Módulo inteiro: UoW, outbox, migração, purga, repositório e mapeador de exemplo, manifesto |
| `app` | [ ] | Não existe composition root de produção; os testes fazem o papel de `bind` |
| Infra / CI | [x] | `services.postgres` e `DMPF_PG_DSN` no job `main`; `go.work` com o sexto módulo; baseline do verificador |
| Documentação | [x] | ADR-035, índice de ADRs, README do módulo, `AGENTS.md` |

## Localização de código

```text
lidercap-platform/
├── go.work                                                  # MODIFICAR — use ./libs/backend/go/dmpf-provider-postgres
├── .github/workflows/ci.yml                                 # MODIFICAR — services.postgres + env.DMPF_PG_DSN no job main
├── contracts/
│   ├── proto/company/orders/event/v1/
│   │   ├── order_placed.proto                               # MODIFICAR — int32 item_count = 6 (aditivo; 5 segue reservado)
│   │   └── item_added.proto                                 # CRIAR — ItemAdded {order_id, sku, quantity}; type com.company.orders.item-added.v1
│   └── fixtures/orders/event/v1/
│       ├── order-placed.golden                              # MODIFICAR — regenerado; caso item-count-present
│       └── item-added.golden                                # CRIAR — fixture do contrato novo (INT-02)
├── libs/backend/go/dmpf-contracts/
│   ├── gen/go/company/orders/event/v1/
│   │   ├── order_placed.pb.go                               # MODIFICAR — regenerado por buf generate, nunca à mão
│   │   └── item_added.pb.go                                 # CRIAR — gerado
│   └── golden/
│       ├── golden_test.go                                   # MODIFICAR — tabela de fixtures em vez de fixturePath
│       └── fixture_test.go                                  # MODIFICAR — gerador cobre ItemAdded e item_count
├── libs/backend/go/dmpf-provider-postgres/                  # CRIAR — projeto Nx dmpf-provider-postgres-go, bloco provider
│   ├── go.mod                                               # module .../libs/backend/go/dmpf-provider-postgres, go 1.26.4, require pgx/v5 + protobuf
│   ├── go.sum                                               # presente: há dependência externa real
│   ├── project.json                                         # 5 targets copiados de dmpf-application; tags 3D
│   ├── package.json                                         # @lidercap-apps/dmpf-provider-postgres-go, private
│   ├── dmpf-units.json                                      # 2 unidades provider + 2 external (commit próprio)
│   ├── README.md                                            # o que o módulo é, como subir o Postgres local, o que fica com KRN-07/08
│   ├── doc.go                                               # godoc de package: bloco provider, células 25/28/30, o que não contém
│   ├── schema.sql                                           # dmpf_outbox + dmpf_example_orders; embutido por embed
│   ├── migrate.go                                           # Migrate(ctx, pool)
│   ├── uow.go                                               # NewUnitOfWork[R], Tx, Within (seis cláusulas)
│   ├── outbox.go                                            # Enqueue: validar → mapear → Pack → Sum → INSERT
│   ├── mapper.go                                            # EventMapper, Mapped, conferência de major
│   ├── destination.go                                       # forma de BLK-04 (regex) e ErrInvalidDestination
│   ├── purge.go                                             # PurgePublished(ctx, pool, before) (Purge, error)
│   ├── errors.go                                            # ErrEmptyMessageID, ErrInvalidDestination, ErrUnmappedEvent, ErrDuplicateMessage
│   ├── testing_test.go                                      # openPool(t): DMPF_PG_DSN; Skip fora do CI, Fatal no CI; Migrate; truncate
│   ├── uow_test.go                                          # seis cláusulas de Within sobre Postgres real (duplica RunUnitOfWorkContract)
│   ├── outbox_test.go                                       # valores iniciais, unicidade, destino, unmapped, bytes congelados, hash
│   ├── purge_test.go                                        # só published sai; evidência devolvida
│   └── example/orders/                                      # unidade dmpf-kernel/example-orders-postgres
│       ├── doc.go                                           # repositório e mapeador de exemplo
│       ├── repository.go                                    # NewRepository(tx): Load/Save com optimistic locking em SQL
│       ├── mapper.go                                        # Mapper: OrderPlaced → eventv1.OrderPlaced; ItemAdded → eventv1.ItemAdded
│       ├── repository_test.go                               # create, update, conflito, not found
│       ├── mapper_test.go                                   # dois eventos mapeados; evento estranho → ErrUnmappedEvent; Type PTB-03
│       └── e2e_test.go                                      # PlaceOrder e AddItem de dmpf-application sobre a UoW Postgres (importa application: só em _test.go)
├── libs/backend/go/dmpf-application/
│   ├── README.md                                            # MODIFICAR — "SKIP LOCKED" → KRN-08
│   └── example/memory/doc.go                                # MODIFICAR — idem
├── tools/
│   ├── dmpf-baseline/units-baseline.json                    # MODIFICAR — +2 entradas, digest (commit próprio)
│   └── dmpf-cell-check.sh                                   # CRIAR — vetores das células 26 e 12 em git worktree descartável
├── docs/adr/
│   ├── 035-realizacao-postgres-da-outbox.md                 # CRIAR
│   └── README.md                                            # MODIFICAR — entrada 035
└── AGENTS.md                                                # MODIFICAR — sexto módulo no inventário de Libs
```

Import paths canônicos (as `canonical_key` das duas unidades novas):

- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres`
- `gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/orders`

**Arquivos a modificar**, em prosa: `go.work` (sexto `use`); `ci.yml`
(serviço e variável); `order_placed.proto` (campo 6); `order-placed.golden`,
`golden_test.go`, `fixture_test.go` (segunda fixture e caso novo); os dois
`.pb.go` (regeneração); `units-baseline.json` (duas entradas, digest);
`dmpf-gate-check.sh` (dois vetores); `README.md` e `doc.go` de
`dmpf-application` (redirecionar `SKIP LOCKED`); `docs/adr/README.md` e
`AGENTS.md` (inventário).

## Design

### Arquitetura

```text
                    bloco application (dmpf-application-go)          — INTOCADO
                    ┌──────────────────────────────────────┐
                    │ ordersapp.Service.PlaceOrder / AddItem │
                    │   UoW.Within(ctx, func(res Resources)  │
                    │     res.Orders.Save(...)      passo 6  │
                    │     res.Outbox.Enqueue(...)   passo 7  │
                    │     return nil                passo 8  │
                    └───────────────┬──────────────────────┘
                                    │ célula 10 (application → port)
                                    ▼
                    bloco port (dmpf-ports-go)                        — INTOCADO
                    UnitOfWork[R] · Repository[ID,S] · Outbox · OutboxEntry · Instant
                                    ▲                    ▲
          célula 28 (provider → port)│                    │ célula 25 (provider → domain)
                                    │                    │
   ┌────────────────────────────────┴────────────────────┴──────────────────────┐
   │ bloco provider (dmpf-provider-postgres-go)                          — NOVO │
   │                                                                             │
   │  dmpfpostgres                                 example/orders (orderspg)    │
   │  ├── NewUnitOfWork[R](pool, bind)             ├── NewRepository(tx)         │
   │  ├── Tx{ Conn() pgx.Tx; Outbox(mapper) }      │     Load / Save (version)   │
   │  ├── Enqueue: validar → Map → Pack → Sum      └── Mapper                    │
   │  │            → INSERT dmpf_outbox                  OrderPlaced → eventv1.OrderPlaced
   │  ├── Migrate(ctx, pool)  · schema.sql              ItemAdded   → eventv1.ItemAdded
   │  └── PurgePublished(ctx, pool, before)                                     │
   └───────────────────────────────────┬─────────────────────────────────────────┘
                                       │ célula 30 (provider → contract), C2 via public_integration_surface
                                       ▼
                    bloco contract (dmpf-contracts-go)
                    envelope.Pack · payloadhash.Sum · gen/go/.../event/v1 (OrderPlaced, ItemAdded)

   NÃO atravessa: 11 (application → provider), 12 (application → contract),
                  24 (port → contract), 26 (provider → application).
   NÃO existe:    porta de publicação, SDK de broker, claim, lease (KRN-08).
```

O `bind` que monta `R` a partir de `*Tx` é do composition root (bloco `app`);
nesta história, dos arquivos de teste. Ele é o único lugar que conhece os dois
lados — o tipo `Resources` do caso de uso e o `Tx` do provider —, exatamente
como o ADR-034 decidiu para a realização em memória.

### Fluxo principal — os passos 6, 7 e 8 em Postgres

1. O caso de uso já autorizou (1), resolveu `OccurredAt` e `MessageIDs` fora
   da transação (2) e chamou `uow.Within(ctx, fn)` (3).
2. `Within` confere `ctx.Err()`; se nulo, `pool.BeginTx(ctx, pgx.TxOptions{})`
   abre **uma** transação e `bind(&Tx{tx})` monta `Resources{Orders:
   orderspg.NewRepository(tx), Outbox: tx.Outbox(orderspg.Mapper{})}`.
3. Dentro de `fn`, `Load` lê `dmpf_example_orders` pela transação (4), a UPR
   decide (5).
4. **Passo 6** — `Save(ctx, id, snapshot, expected)`: um `UPDATE ... WHERE
   version = $expected` (ou `INSERT ... ON CONFLICT DO NOTHING` quando
   `expected == 0`); zero linhas → `ErrVersionConflict`, `fn` devolve o erro,
   `Within` faz `Rollback` e devolve o mesmo erro — sem repetir (`UOW-09`).
5. **Passo 7** — `Enqueue(ctx, entry)`, uma vez por evento: valida
   `MessageID` e `Destination`; `Mapper.Map` devolve `Mapped{Message, Type}`;
   `envelope.Pack` produz `payload` e `typeURL`; `payloadhash.Sum(payload)`;
   `INSERT INTO dmpf_outbox` na **mesma** `pgx.Tx`, com `status='pending'`,
   `available_at = occurred_at`, `attempt_count = 0`, campos de lease nulos.
   Duplicata de `message_id` é recusada pelo `UNIQUE` do banco.
6. **Passo 8** — `fn` devolve `nil`; `Within` chama `Commit(ctx)`. Só aqui o
   estado de negócio e a linha da outbox se tornam visíveis, juntos
   (`UOW-07`). Erro de commit é devolvido como o driver o produziu e nada
   persiste.
7. Sob `Rejected`, `fn` não chama `Save` nem `Enqueue` e devolve `nil`: o
   commit ocorre sobre uma transação vazia de efeitos (`UOW-06`) — nenhuma
   linha em `dmpf_example_orders`, nenhuma em `dmpf_outbox`.
8. Nada publica (`UOW-08`). O registro fica `pending` e elegível ao claim do
   `KRN-08` a partir de `available_at`, que já passou no instante do commit.

### Schema mínimo — `schema.sql`

```sql
CREATE TABLE IF NOT EXISTS dmpf_outbox (
  id                bigint  GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  message_id        text    NOT NULL,
  message_type      text    NOT NULL,
  schema_version    text    NOT NULL,
  aggregate_type    text    NOT NULL,
  aggregate_id      text    NOT NULL,
  aggregate_version bigint  NOT NULL,
  partition_key     text    NOT NULL DEFAULT '',
  destination       text    NOT NULL,
  payload           bytea   NOT NULL,
  payload_hash      text    NOT NULL,
  metadata          jsonb   NOT NULL DEFAULT '{}'::jsonb,
  occurred_at       bigint  NOT NULL,
  available_at      bigint  NOT NULL,
  attempt_count     integer NOT NULL DEFAULT 0,
  status            text    NOT NULL DEFAULT 'pending',
  locked_by         text,
  locked_until      bigint,
  published_at      bigint,
  last_error        text,
  CONSTRAINT dmpf_outbox_message_id_unique UNIQUE (message_id),
  CONSTRAINT dmpf_outbox_status_check
    CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
  CONSTRAINT dmpf_outbox_available_at_check CHECK (available_at >= occurred_at),
  CONSTRAINT dmpf_outbox_payload_not_empty CHECK (octet_length(payload) > 0)
);

CREATE INDEX IF NOT EXISTS dmpf_outbox_published_at_idx
  ON dmpf_outbox (published_at) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS dmpf_example_orders (
  order_id text   PRIMARY KEY,
  version  bigint NOT NULL,
  snapshot jsonb  NOT NULL
);
```

Correspondência com FND-04 §4.1: os dezoito campos da tabela normativa estão
presentes com os nomes exatos. `payload_hash` é a única coluna acrescida —
"um provider pode acrescentar colunas ao seu schema concreto" — e existe para
que o critério "os bytes não mudam" seja verificável sem reserializar
(`ENV-18`). O índice sobre `published` serve à purga (`OBX-17`); o índice de
elegibilidade ao claim (`OBX-09`) é do `KRN-08`, que conhece a query.

### Pseudocódigo — `Within` e `Enqueue`

```text
Within(ctx, fn):
  if ctx.Err() != nil: return ctx.Err()              // CTX-21: nem abre
  tx, err := pool.BeginTx(ctx, {})                    // UOW-01: uma transação
  if err != nil: return err
  resources := bind(&Tx{conn: tx})                    // UOW-03/04: R tipado, só o vinculado
  defer:
    if panicking:
      tx.Rollback(ctx); re-panic                      // ERR-22: nunca engolido
  err = fn(ctx, resources)                            // UOW-09: exatamente uma vez
  if err != nil:
    if rbErr := tx.Rollback(ctx); rbErr != nil:
      return errors.Join(err, rbErr)                  // errors.Is(err) preservado
    return err                                        // mesmo erro, sem embrulho
  return tx.Commit(ctx)                               // UOW-07: único ponto de visibilidade

Enqueue(ctx, entry):
  if entry.MessageID == "": return ErrEmptyMessageID
  if !destinationForm.MatchString(entry.Intent.Destination): return ErrInvalidDestination   // BLK-04
  mapped, err := mapper.Map(entry.Event)              // BLK-03: mapeamento no provider
  if err != nil: return err                           // embrulha ErrUnmappedEvent
  if major(mapped.Type) != major(package(mapped.Message)): return ErrMajorMismatch   // ENV-16
  payload, typeURL, err := envelope.Pack(mapped.Message)   // serialização NA ESCRITA (ADR-021)
  if err != nil: return err
  hash := payloadhash.Sum(payload)                    // ENV-18: sobre os bytes transportados
  _, err = tx.Exec(ctx, insertOutbox,
    entry.MessageID, mapped.Type, typeURL,
    entry.AggregateType, entry.AggregateID, entry.AggregateVersion,
    entry.Intent.PartitionKey, entry.Intent.Destination,
    payload, hash, '{}',
    entry.OccurredAt, entry.OccurredAt /* available_at */)   // OBX-05; status/attempt_count por DEFAULT
  if isUniqueViolation(err): return fmt.Errorf("%w: %w", ErrDuplicateMessage, err)   // OBX-01: do schema
  return err
```

### Onde cada regra é provada

| Regra | Prova executável |
| --- | --- |
| `UOW-01`, `UOW-02` | `uow_test.go`: dois `Enqueue` na mesma `Within` → duas linhas após o commit; `pg_stat_activity` não é consultado — a prova é o efeito, não a contagem de conexões |
| `UOW-06` | `e2e_test.go`: `AddItem` sobre pedido `Placed` → `Outcome` rejeitado, `error == nil`, zero linhas novas nas duas tabelas |
| `UOW-07` | `uow_test.go`: `fn` grava `Save` + `Enqueue` e cancela o `ctx` pai antes de devolver `nil` → `Commit(ctx)` falha → zero linhas nas duas tabelas |
| `UOW-08` | Estrutural: `go list -deps` do módulo não contém SDK de broker, `net/http` nem `net`; conferido pelo verificador (capability) e por teste que inspeciona `go list` |
| `UOW-09`, `UOW-10` | `uow_test.go`: contador de invocações de `fn` = 1 sob sucesso, erro e `ErrVersionConflict` |
| `ERR-22` | `uow_test.go`: panic em `fn` propaga (`recover` no teste) e zero linhas persistem |
| `CTX-21` | `uow_test.go`: `ctx` já cancelado → `Within` devolve `context.Canceled` e `fn` não roda |
| `BLK-01` | Verificador: zero arestas `application → provider`; `grep` negativo de `pgx`/`database/sql` em `dmpf-application` no `dmpf-gate-check.sh` |
| `BLK-03`, ADR-021 | `outbox_test.go`: `payload` lido do banco == `envelope.Pack` do mesmo evento; após trocar o mapeador por um que produz bytes diferentes, o registro antigo permanece byte a byte igual e `payload_hash` bate com `payloadhash.Sum(payload)` |
| `BLK-04` | `outbox_test.go`: `arn:aws:sns:...`, `orders/events`, `kafka://orders`, `Orders.Events` → `ErrInvalidDestination`, zero linhas; `orders.events` → aceito |
| `BLK-05`, `OBX-05` | `outbox_test.go`: `SELECT status, available_at, attempt_count, locked_by, locked_until, published_at, last_error` do registro recém-gravado → `pending`, `= occurred_at`, `0`, `NULL ×4` |
| `OBX-01` | `outbox_test.go`: dois `Enqueue` com o mesmo `message_id` na mesma transação → segundo devolve `ErrDuplicateMessage`; após rollback, zero linhas |
| `OBX-02` | `outbox_test.go`: `metadata` lido == `{}` |
| `OBX-17` | `purge_test.go`: fixtures inseridas por SQL em `pending`, `publishing`, `published` (dois `published_at`) e `failed`; `PurgePublished(before)` remove só o `published` anterior a `before` e devolve `Purge{Count: 1, Before}` |
| Optimistic locking (§3.3) | `repository_test.go`: `Save(expected=0)` cria com `version=1`; `Save(expected=1)` → `version=2`; `Save(expected=1)` de novo → `ErrVersionConflict`; `Load` de id inexistente → `ErrNotFound` |
| `ErrUnmappedEvent` | `mapper_test.go` e `outbox_test.go`: evento anônimo que satisfaz `DomainEvent` → `Enqueue` erra, rollback, zero linhas |
| `ENV-16` (major) | `outbox_test.go`: mapeador de teste devolve `Type` `...order-placed.v2` para mensagem do pacote `v1` → `ErrMajorMismatch` |
| `INT-02`, golden | `golden_test.go` parametrizado: `item-added.golden` e `order-placed.golden` em sincronia com o gerador; `payload_hash` de cada caso confere |
| Célula 26, célula 12 | `dmpf-cell-check.sh`: dois `git worktree` descartáveis reprovam com `DMPF-D001`, mais um vetor positivo |
| P0-3 | `buf-lint`: varredura textual de `contracts/`; `grep` negativo de "exactly-once"/"exatamente uma vez" no módulo novo e no ADR-035 |

## Decisões técnicas

- **`payload` guarda os bytes do `Any` (proto do integration event), não o
  CloudEvent completo**, porque `envelope.Validate()` exige `correlationid`,
  `causationid` e `traceparent` (`envelope.go:81-103`), atributos de contexto
  de FND-07 que nem `OutboxEntry` nem o `application service` de referência
  autoriam. Gravar o envelope inteiro obrigaria a inventá-los na escrita. O
  relay (`KRN-08`) monta o CloudEvent na drenagem a partir das colunas
  (`message_id`→`id`, `message_type`→`type`, `schema_version`→`dataschema`,
  `occurred_at`→`time`, `partition_key`, `aggregate_version`) e de `metadata`;
  os bytes do fato, que é o que ADR-021 protege, já estão congelados.
  Alternativa descartada: serializar `cloudeventsv1.CloudEvent` inteiro —
  exigiria atributos que não existem e faria o provider inventar contexto de
  execução (`CTX-05`).
- **`schema_version` recebe o type URL completo do `Any`**
  (`type.googleapis.com/company.orders.event.v1.OrderPlaced`), porque é a
  identidade exata do schema que `Encode` precisa em `dataschema` e a que a
  golden fixture registra; o major está embutido no segmento `v1`. Alternativa
  descartada: gravar só `v1` — perderia o nome da mensagem e obrigaria o relay
  a recompô-lo.
- **Tempo como `bigint` de nanossegundos**, espelhando `dmpfports.Instant`,
  porque `timestamptz` tem precisão de microssegundos e o ADR-034 fixou
  nanossegundos justamente para que `available_at` não colida sob carga.
  Alternativa descartada: `timestamptz` — perde três casas e quebra a
  ordenação que a drenagem exige.
- **`available_at` é gravado pelo provider e protegido por `NOT NULL` +
  `CHECK (available_at >= occurred_at)`**, porque PostgreSQL não aceita
  `DEFAULT` que referencie outra coluna. Alternativa descartada: trigger
  `BEFORE INSERT` — mais uma peça de schema para expressar uma atribuição de
  uma linha no provider, que já é quem autoria o `INSERT`.
- **Driver `github.com/jackc/pgx/v5` com `pgxpool`, sem `database/sql`**,
  porque é o driver que a própria política de dependências do repositório já
  nomeia (`.golangci.yml`, mensagens de `deny` dos três blocos) e o único com
  manutenção ativa e `pgconn.PgError` tipado para SQLSTATE. Alternativa
  descartada: `database/sql` + `lib/pq` — `lib/pq` está em modo de manutenção
  e a camada `database/sql` esconde o SQLSTATE atrás de string.
- **Migração por SQL embutido e idempotente, sem ferramenta externa**, porque
  o kernel entrega o schema mínimo, não um pipeline de migração; `CREATE ...
  IF NOT EXISTS` basta para o primeiro schema e para os testes. Alternativa
  descartada: `golang-migrate`/`goose` — dependência e CLI a mais para um
  arquivo; a decisão de ferramenta de migração é de quem tiver a segunda versão
  do schema.
- **Mapeador como interface do provider (`EventMapper`) realizada em
  `example/orders`**, porque o mapeamento é por bounded context e o kernel só
  fixa a fronteira; o `Pack`, o hash e o `INSERT` são genéricos e ficam no
  package raiz. Alternativa descartada: mapeador por reflexão sobre
  `EventName()` — tornaria `message_type` derivado de nome de domínio, que é
  exatamente o vocabulário que FND-03 `MSG-N02` mantém fora do wire.
- **Falha de commit provada por cancelamento do `ctx` dentro de `fn`**, porque
  `Commit(ctx)` com contexto cancelado falha e o pgx aborta a transação, sem
  precisar de gancho de injeção em código de produção. Alternativa descartada:
  hook `FailNextCommit` como o da realização em memória — superfície de teste
  vazando para o provider real.
- **Testes de integração sob build tag `integration`, fail-closed no CI**
  (`t.Skip` sem `DMPF_PG_DSN` fora do CI; `t.Fatal` quando `CI` está definido),
  porque um skip silencioso no CI faria os critérios de aceite "verificados no
  CI" passarem sem banco. **Ajustado na review do plano**: a spec previa
  dispensar a tag por receio de que o `test-race` não a passasse; o `test-race`
  do módulo declara `-tags=integration` no próprio `project.json`, então os
  testes rodam no pipeline e o `go test ./...` comum segue sem exigir banco,
  como `.claude/rules/testing-conventions.md` pede. O mesmo target leva
  `cache: false` (Nx) e `-count=1` (`go test`): os dois caches devolveriam
  resultado antigo para teste de banco.
- **`services.postgres` no job do CI**, porque é o mecanismo declarativo do
  runner e mantém o DSN fixo. Risco registrado: se o `gitea-runner` (act_runner)
  não estiver em modo Docker, `services:` não sobe; o fallback é um step
  `docker run -d postgres:16-alpine` antes dos testes, ou um Postgres do host.
  A escolha entre os dois é do plano, após verificar o runner.
- **`item_added.proto` sem campo de tempo e `OrderPlaced` com `item_count`,
  sem `placed_at`**, porque o instante do fato viaja em `occurred_at` →
  `time` do CloudEvent e duplicá-lo no payload criaria duas fontes para o mesmo
  dado. `customer_id`, `total_cents` e `channel` permanecem em zero-value: o
  contrato de exemplo do `KRN-05` foi desenhado antes do domínio e a spec não
  o encolhe (remover campo é breaking). Alternativa descartada: reservar os
  três campos — breaking em `FILE` e sem ganho.
- **Vetor da célula 12 pago aqui**, porque esta é a primeira história em que
  uma unidade `application` e uma `contract` reais coexistem no `develop`
  (dívida declarada na `SPEC-ZHE7DN1H`, linha 98).
- **Isolamento `READ COMMITTED`** (default), porque a exclusão entre
  escritores concorrentes vem da comparação de `version` no `UPDATE`, não do
  nível de isolamento; `SERIALIZABLE` acrescentaria erros de serialização que
  `UOW-10` proíbe repetir automaticamente, sem ganho para o padrão outbox.

## Regras relacionadas

- `docs/dmpf/uow-inbox-outbox.md` (FND-04) — §2, §3.2, §3.3, §4, §9.5
- `docs/dmpf/cloudevents-protobuf-buf.md` (FND-05) — `PTB-03`, `PTB-06`,
  `INT-02`, `ENV-16`, `ENV-18`, `BUF-08`, `BUF-11`, `BUF-12`
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `CTX-05`, `CTX-20`,
  `CTX-21`, `ERR-22`
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §6.2, §6.3, §7.3, §7.4, §7.5,
  §10.2, P0-2, P0-3
- `docs/guides/dmpf-manifesto.md` — `external[]` com quatro elementos
- `contracts/README.md` — "Gates e baseline"
- `.claude/rules/git-safety.md`, `.claude/rules/process-enforcement.md`
- SPEC-YRJRADY9 — guarda-chuva; esta spec fecha o requisito `KRN-06`
- SPEC-ZHE7DN1H — `KRN-04`; portas realizadas aqui; dívida da célula 12 paga aqui
- SPEC-WYX5GW87 — `KRN-05`; codec e hash consumidos; contrato estendido sob seus gates
- SPEC-XF9TF9A0 — `KRN-03`; eventos e `Snapshot` mapeados e persistidos aqui

## Verificação e testes

### Critérios de aceite

- [ ] A assinatura de `dmpfports.Outbox`, `OutboxEntry` e `PublishIntent` não
  menciona tipo gerado de Protobuf, `pgx` nem `[]byte`; `git diff` de
  `dmpf-ports/` é vazio e o verificador aprova a unidade `dmpf-kernel/port`
  como `pure`.
- [ ] `dmpf-application` não importa driver, cliente de banco nem o tipo da
  linha: `go list -deps ./...` em `dmpf-application` não contém `pgx` nem
  `database/sql`; o verificador reporta zero arestas `application → provider`.
- [ ] Existe teste que faz o commit falhar (cancelamento do `ctx` dentro de
  `fn`) e comprova, por `SELECT count(*)` nas duas tabelas, que nem o estado
  de negócio nem o registro de outbox persistiram.
- [ ] Existe teste em que a UPR devolve `Rejected` (`AddItem` em pedido
  `Placed`) e comprova que `Within` devolveu `nil`, que o `Outcome` carrega a
  rejeição e que nada foi persistido nem enfileirado.
- [ ] Um registro recém-gravado nasce `pending`, com `available_at =
  occurred_at`, `attempt_count = 0` e `locked_by`, `locked_until`,
  `published_at`, `last_error` nulos — lido por `SELECT` após o commit.
- [ ] Gravar duas linhas com o mesmo `message_id` é recusado pelo schema: o
  segundo `Enqueue` devolve erro que satisfaz `errors.Is(err, ErrDuplicateMessage)`
  e `errors.As(err, &pgconn.PgError{})` com `Code == "23505"`.
- [ ] `destination` é o nome do fluxo lógico; teste negativo reprova
  `arn:aws:sns:us-east-1:123:orders`, `orders/events`, `kafka://orders`,
  `Orders.Events` e `""` com `ErrInvalidDestination`; `orders.events` e
  `billing.invoice.issued` são aceitos.
- [ ] Os bytes gravados em `payload` não mudam após alteração posterior do
  mapeador: o teste grava com o `Mapper` real, troca o mapeador por um que
  produz payload diferente para o mesmo evento, grava um segundo registro e
  comprova que o primeiro permanece byte a byte igual e que `payload_hash`
  ainda é `payloadhash.Sum` dos bytes lidos.
- [ ] Nenhum artefato desta entrega declara ou sugere exactly-once fim a fim:
  `buf-lint` verde e `grep -ri` negativo por "exactly-once" e "exatamente uma
  vez" em `libs/backend/go/dmpf-provider-postgres/` e em `docs/adr/035-*.md`
  (salvo a frase que o veda).
- [ ] A purga de `published` devolve `Purge{Count, Before}` com o que foi
  purgado e até qual instante; registros `pending`, `publishing` e `failed`
  permanecem, comprovado por `SELECT` após a purga.
- [ ] Evento sem contrato registrado devolve `ErrUnmappedEvent` em `Enqueue`;
  após o rollback, zero linhas.
- [ ] `AddItem` e `PlaceOrder` de `dmpf-application/example/orders` rodam ponta
  a ponta sobre a UoW Postgres (`e2e_test.go`): item adicionado criando o
  pedido em `version = 1`, pedido colocado em `version = 2`, dois registros na
  outbox com `message_type` `com.company.orders.item-added.v1` e
  `com.company.orders.order-placed.v1`, `schema_version` com os type URLs
  correspondentes e `aggregate_version` `1` e `2`.
  **Ajustado na implementação**: a ordem é imposta pelo agregado, não é
  preferência. `Service.PlaceOrder` só faz `Load` (`place_order.go:26`) e
  `Order.Place` recusa pedido sem itens (`place.go:12`), então `PlaceOrder`
  primeiro devolveria `ErrNotFound`. Quem cria o pedido é o `AddItem`, pelo
  ramo `loadOrCreate` (`add_item.go:63`). Os dois eventos, as duas versões e os
  dois contratos continuam provados; o que troca é qual vem primeiro.
- [ ] `item_added.proto` e `item_count` passam `buf-lint`, `buf-pins`,
  `buf-generate-check` e `buf-breaking`; `gen/go` sem drift; fixture
  `item-added.golden` em sincronia com o gerador; `order-placed.golden` com o
  caso `item-count-present`.
- [ ] Vetores de módulo para as células 26 e 12 reprovam com `DMPF-D001` em
  `tools/dmpf-cell-check.sh`, executado no CI no step "DMPF cell gate".
- [ ] Manifesto e baseline em commit próprio: o gate com `--base` avalia a
  mudança normativa em commit sem código Go e aprova.
- [ ] CI: o job `main` sobe `services.postgres`, exporta `DMPF_PG_DSN`, e os
  testes de integração do módulo rodam (não são pulados) no `test-race`.
- [ ] Cadeia Go verde nos seis módulos; `pnpm biome ci .` e
  `pnpm nx affected -t lint,typecheck,test,build` verdes; `dmpf-conformance`
  aprovando; ADR-035 criado e indexado; `AGENTS.md` com o sexto módulo.

### Cenários de teste

```text
DADO um pool para um Postgres 16 com o schema migrado e as tabelas vazias
  E a UoW Postgres vinculada a Resources{Orders: orderspg.NewRepository(tx), Outbox: tx.Outbox(orderspg.Mapper{})}
QUANDO ordersapp.Service.AddItem é executado para o pedido "o-1001" com SKU "sku-1" e quantidade 2
  # AddItem vem primeiro por imposição do agregado: PlaceOrder só carrega, e
  # Place recusa pedido sem itens. Quem cria o pedido é o ramo loadOrCreate.
ENTÃO Within devolve nil e o Outcome é aceito
  E dmpf_example_orders tem uma linha (order_id "o-1001", version 1)
  E dmpf_outbox tem uma linha com message_type "com.company.orders.item-added.v1",
    schema_version "type.googleapis.com/company.orders.event.v1.ItemAdded",
    status "pending", available_at = occurred_at, attempt_count 0,
    locked_by/locked_until/published_at/last_error NULL, metadata '{}'
  E payload == envelope.Pack(eventv1.ItemAdded{OrderId: "o-1001", Sku: "sku-1", Quantity: 2})
  E payload_hash == payloadhash.Sum(payload)

DADO o mesmo pool com o pedido "o-1001" persistido em version 1, com um item
QUANDO PlaceOrder é executado
ENTÃO o pedido passa a version 2
  E dmpf_outbox ganha uma linha com message_type "com.company.orders.order-placed.v1",
    aggregate_version 2 e payload == envelope.Pack(eventv1.OrderPlaced{OrderId: "o-1001", ItemCount: 1})

DADO um pedido "o-1001" com status Placed
QUANDO AddItem é executado
ENTÃO Within devolve nil, o Outcome carrega a Rejection e error == nil
  E count(*) das duas tabelas é o mesmo de antes da chamada (UOW-06)

DADO um callback fn que executa Save e Enqueue com sucesso e, antes de devolver nil, cancela o ctx pai
QUANDO Within tenta o Commit
ENTÃO Within devolve erro que satisfaz errors.Is(err, context.Canceled)
  E count(*) de dmpf_example_orders e de dmpf_outbox é zero (UOW-07)

DADO um ctx já cancelado
QUANDO Within é chamado
ENTÃO devolve context.Canceled, fn não é invocada e nenhuma transação foi aberta (CTX-21)

DADO um callback que entra em panic após Save e Enqueue
QUANDO Within é chamado
ENTÃO o panic propaga até o teste (recover) e as duas tabelas seguem vazias (ERR-22)

DADO uma transação aberta por Within
QUANDO Enqueue é chamado duas vezes com o mesmo MessageID "m-000001"
ENTÃO o segundo devolve erro que satisfaz errors.Is(err, ErrDuplicateMessage)
  E errors.As(err, &pgErr) com pgErr.Code == "23505" (OBX-01)
  E após fn devolver esse erro, count(*) de dmpf_outbox é zero

DADO uma OutboxEntry com Intent.Destination "arn:aws:sns:us-east-1:123456789012:orders"
QUANDO Enqueue é chamado
ENTÃO devolve ErrInvalidDestination sem executar INSERT (BLK-04)
  E o mesmo vale para "orders/events", "kafka://orders", "Orders.Events" e ""
  E "orders.events" e "billing.invoice.issued" são aceitos

DADO uma OutboxEntry cujo Event é um tipo anônimo que satisfaz dmpfdomain.DomainEvent
QUANDO Enqueue é chamado com orderspg.Mapper{}
ENTÃO devolve erro que satisfaz errors.Is(err, ErrUnmappedEvent) e nada é gravado

DADO um registro gravado com o Mapper real para OrderPlaced{Order: "o-1", Items: 3}
QUANDO o mapeador é trocado por um que produz eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 99}
  E um segundo registro é gravado para o mesmo evento
ENTÃO o payload do primeiro registro, relido do banco, é byte a byte igual ao gravado
  E payload_hash do primeiro == payloadhash.Sum(payload relido)
  E o payload do segundo registro difere do primeiro (serialização na escrita, ADR-021)

DADO um mapeador de teste que devolve Type "com.company.orders.order-placed.v2" para eventv1.OrderPlaced (pacote v1)
QUANDO Enqueue é chamado
ENTÃO devolve erro que satisfaz errors.Is(err, envelope.ErrMajorMismatch) (ENV-16)

DADO dmpf_outbox com quatro linhas inseridas por SQL: pending, publishing, published (published_at = 100), published (published_at = 300), failed
QUANDO PurgePublished(ctx, pool, 200) é chamado
ENTÃO devolve Purge{Count: 1, Before: 200}
  E restam quatro linhas: pending, publishing, published (300) e failed (OBX-17)

DADO dmpf_example_orders vazio
QUANDO Save("o-1", snapshot, expected 0) é chamado
ENTÃO a linha nasce com version 1
QUANDO Save("o-1", snapshot', expected 1) é chamado
ENTÃO a linha passa a version 2
QUANDO Save("o-1", snapshot'', expected 1) é chamado de novo
ENTÃO devolve dmpfports.ErrVersionConflict e a linha segue em version 2 (§3.3)
QUANDO Load("o-inexistente") é chamado
ENTÃO devolve dmpfports.ErrNotFound

DADO o repositório em NX_BASE = develop com a marca contracts-baseline/proto existente
QUANDO buf-lint, buf-pins, buf-generate-check e buf-breaking rodam sobre contracts/
ENTÃO os quatro passam: item_added.proto novo e item_count aditivo não são breaking em FILE
  E gen/go regenerado é byte a byte igual ao versionado (BUF-11)

DADO um repositório descartável com uma unidade provider que importa dmpf-application
QUANDO dmpf-conformance roda
ENTÃO reprova com DMPF-D001 na célula 26
DADO um repositório descartável com uma unidade application que importa dmpf-contracts/envelope
QUANDO dmpf-conformance roda
ENTÃO reprova com DMPF-D001 na célula 12

DADO a variável CI definida e DMPF_PG_DSN ausente
QUANDO go test roda no módulo
ENTÃO os testes de integração falham com mensagem que nomeia DMPF_PG_DSN (fail-closed)
DADO CI ausente e DMPF_PG_DSN ausente
QUANDO go test roda no módulo
ENTÃO os testes de integração são pulados com t.Skip e os unitários (destination, mapper) rodam
```

<critical_constraints>
- [P0] Matriz de blocos (RFC §7.3, §7.4; ADR-010): o módulo novo é bloco
  `provider`. Suas unidades importam SOMENTE `domain` (célula 25), `port`
  (célula 28) e `contract` (célula 30). NUNCA importar `application` (célula
  26) nem `app` (célula 27) em código de produção; o teste ponta a ponta que
  importa o `application` vive em `_test.go`, fora do universo.
- [P0] Célula 11 intocada (`BLK-01`): NENHUM arquivo de `dmpf-application`
  importa o módulo novo, `pgx`, `database/sql` nem o tipo da linha.
- [P0] Porta intocada (`BLK-03`, ADR-021): `dmpf-ports/outbox.go` NÃO muda;
  nenhum tipo de wire na assinatura.
- [P0] Mesma transação (`UOW-07`): `INSERT` da outbox e escrita do agregado
  sobre a MESMA `pgx.Tx`; visibilidade só no `COMMIT`. NUNCA transação
  paralela dentro de `Enqueue` ou `Save`.
- [P0] Nenhuma publicação na transação (`UOW-08`; §9.5): sem SDK de broker,
  sem `net`, sem porta de publicação.
- [P0] Serialização na escrita (`BLK-03`; `ENV-18`): `payload` por
  `envelope.Pack` DENTRO de `Enqueue`; `payload_hash` sobre esses bytes; ler
  NUNCA reserializa.
- [P0] Unicidade pelo schema (`OBX-01`): `UNIQUE (message_id)`; recusa vem do
  banco (SQLSTATE 23505).
- [P0] Valores iniciais (`OBX-05`, `BLK-05`): `pending`, `available_at =
  occurred_at`, `attempt_count = 0`, lease e `published_at`/`last_error`
  NULOS; o provider NUNCA escreve outro valor nesses campos.
- [P0] Destino lógico (`BLK-04`): forma `^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`;
  fora dela → `ErrInvalidDestination`, sem `INSERT`.
- [P0] Sem retry automático (`UOW-09`, `UOW-10`): `fn` EXATAMENTE uma vez.
- [P0] Fail-closed no mapeamento: evento sem contrato → `ErrUnmappedEvent` e
  rollback; NUNCA `message_type` derivado do nome do evento de domínio.
- [P0] At-least-once (P0-3): NENHUM artefato declara ou sugere exactly-once.
- [P0] Governança (RFC §10.2; ADR-028): manifesto, `external[]` e baseline em
  COMMIT PRÓPRIO, sem código Go; quatro elementos por dependência externa.
- [P0] Gates Buf (`BUF-08`, `BUF-11`, `BUF-12`): `.proto` novo e campo
  aditivo sob os quatro gates; `gen/go` regenerado, NUNCA à mão; marca
  `contracts-baseline/proto` é pré-requisito externo.
- [P0] Tempo como `bigint` de nanossegundos (ADR-034); NUNCA `timestamptz`.
- [P1] Testes de integração fail-closed no CI: sem `DMPF_PG_DSN`, `t.Skip`
  fora do CI e `t.Fatal` com `CI` definido.
- [P1] `go.mod` do módulo novo com `require` só das externas; irmãos pelo
  `go.work`, sem `require` e sem `replace`.
</critical_constraints>

## Escopo fora

- **Claim, lease, `FOR UPDATE SKIP LOCKED`, transições de `status` e
  publicação no broker**: são a drenagem (FND-04 §5, `OBX-04`, `OBX-07`..
  `OBX-16`, `OBX-18`; ADR-020) e pertencem ao `KRN-08`. Esta spec deixa o
  registro em `pending` e não escreve `locked_by`, `locked_until`,
  `published_at` nem `last_error`. O índice de elegibilidade ao claim
  (`OBX-09`) também é do `KRN-08`.
- **Inbox, deduplicação e disposições de consumo**: `KRN-07`, que reusa a
  `UnitOfWork[R]` Postgres desta spec para gravar a inbox e os efeitos locais
  na mesma transação.
- **Formato do integration event, codec `proto_data`, fórmula do
  `payload_hash` e `Encode`/`Decode` do envelope**: `KRN-05`, intocados. Esta
  spec **consome** `Pack` e `Sum` e **acrescenta** um contrato e um campo sob os
  gates existentes; não altera codec, fórmula, perfil nem `buf.yaml`.
- **Tradução do destino lógico para endereço concreto** (tópico, fila, ARN):
  FND-06, `KRN-10`. `destination` aqui é só validado em forma e gravado.
- **Conteúdo de `metadata`, atributos de contexto do CloudEvent
  (`correlationid`, `causationid`, `traceparent`, `tenantid`, `tracestate`)**:
  FND-07; chegam quando houver autor no `application service` e consumidor no
  relay. `metadata` nasce `{}`.
- **Nomes de métricas, limiares de `pending`/`lag`, prazo concreto de retenção
  de `published` e agendamento da purga**: FND-08 (`OBX-12`, `OBX-17`
  `encaminhado`); esta spec entrega a função de purga, não o cronograma.
- **Retry de conflito de versão**: política explícita do caso de uso
  (`UOW-10`), e mecanismo de retry por conjunção é do `KRN-09`. Nem `Within`
  nem `Save` repetem.
- **Test kit exportado da UoW**: `KRN-11`. Os testes das seis cláusulas
  duplicam `RunUnitOfWorkContract` no módulo, como a realização em memória fez.
- **Composition root de produção** (`app`): não existe app no repositório; o
  `bind` vive nos testes.
- **Ferramenta de migração e versionamento de schema**: decisão de quem tiver
  a segunda versão do schema; aqui `CREATE ... IF NOT EXISTS` embutido.
- **Postgres em produção, pooling, TLS, credenciais**: infraestrutura; a spec
  fixa um DSN de teste e o compose local existente.
- **Criação da marca `contracts-baseline/proto`**: autorização organizacional
  por segundo aprovador (`BUF-08`); pré-requisito, não entregável.
- **Extensão de `OutboxEntry` com metadados**: mudança de porta (`KRN-04`
  reaberto), encaminhada ao primeiro consumidor real de `metadata`.
