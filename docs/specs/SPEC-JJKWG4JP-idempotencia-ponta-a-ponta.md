---
id: SPEC-JJKWG4JP
slug: idempotencia-ponta-a-ponta
title: Idempotência de ponta a ponta — chave de comando nas bordas REST e gRPC e endurecimento do consumo de mensagens
stage: done
priority: P1
depends_on: []
ticket_url: null
subtask_urls: []
created: 2026-09-30
---

# SPEC-JJKWG4JP: Idempotência de ponta a ponta — chave de comando nas bordas REST e gRPC e endurecimento do consumo de mensagens

## Resumo

Todo comando do sistema passa a ter efeito único por chave: repetir uma requisição com a mesma
`Idempotency-Key`, pela borda REST do BFF ou direto pela borda gRPC de um contexto, devolve o
resultado da primeira execução sem novo efeito nem novo evento. Quem garante isso é a inbox, a
guarda de entrada do DMPF, que passa a receber os comandos além das mensagens e registra cada um na
mesma transação do efeito, dentro da UoW do contexto. O consumo de mensagens fecha as lacunas de
classificação, isolamento, quarentena e retenção.

Como cliente do BFF, quero repetir um comando cuja resposta se perdeu (timeout, 504, queda de rede)
com a mesma chave e receber o resultado original, para reconciliar sem duplicar efeito.

## Contexto

- **Problema**: a chave é exigida e descartada. O BFF valida o formato
  (`apps/backend/bff/app/api/middleware.go:182`) e a propaga como metadata gRPC
  (`apps/backend/bff/app/rpc/metadata.go:14`), mas o contexto só a escreve no log
  (`libs/backend/go/grpc/interceptor_context.go:196`). Não existe porta, tabela nem replay de
  resposta. Consequências medidas no código:

  | Comando | Segunda execução com a mesma chave hoje | Efeito duplicado? |
  |---------|------------------------------------------|-------------------|
  | `AddItem` | 201 de novo, outro item e outro `ItemAdded` (`orders/domain/add_item.go:25`) | Sim |
  | `PlaceOrder` | 422 `orders/order-not-open` no lugar do 200 original | Não, resposta diverge |
  | `Reserve`, `Cancel` | 422 `already-reserved` / `already-canceled` | Não, resposta diverge |
  | `ReserveBooking` | 409 "version conflict; replay the call", um retry que nunca converge | Não, resposta diverge |
  | `CancelBooking` | 422 `booking/not-reserved` | Não, resposta diverge |
  | `RegisterResource` | 201, no-op de domínio, `version` incrementada | Não |

  Depois de um 504 o cliente não sabe se o comando foi aplicado, e nenhum comando é retentável no
  BFF (`apps/backend/bff/app/rpc/clients.go:111`).
- **Consumo de mensagens**: a inbox deduplica por `(consumer_name, message_id)` na mesma transação
  do efeito, e o relay republica com `message_id` estável. Restam lacunas reais:
  - prazo estourado e erro transitório do Postgres viram D4 (quarentena e ack) em vez de D3
    (`libs/backend/go/application/disposition.go:51-70`);
  - o isolamento READ COMMITTED que o `INB-18` exige é herdado do servidor, sem verificação
    (`libs/backend/go/postgres/uow.go:39`);
  - a quarentena não tem chave única, e a mesma mensagem gera N linhas (`inbox.sql:16-26`);
  - `PurgeInbox` e `PurgePublished` não têm chamador fora de testes (`postgres/purge.go:25-47`),
    então inbox e outbox crescem sem limite e o `INB-14` não é verificável.
- **Impacto**: retry seguro de ponta a ponta, com a mesma chave, em qualquer ponto da cadeia
  cliente → BFF → contexto, e resposta estável para a mesma chave.
- **Inspiração**:
  - Draft IETF `draft-ietf-httpapi-idempotency-key-header-07`: 400 sem chave, 409 em andamento,
    422 para chave reusada com payload diferente. Expirado em 18/04/2026, sem RFC.
  - Stripe `docs.stripe.com/api/idempotent_requests`: replay do resultado, retenção de 24h,
    validação e rate limit fora do cache, header `Idempotent-Replayed`.
  - `brandur.org/idempotency-keys`: registro na transação local, `UNIQUE (user_id, key)`.
  - AWS Builders' Library (*making retries safe*): chave explícita em vez de hash, parâmetros
    guardados para validação.
  - Google AIP-155: chave de idempotência em gRPC.
  - `microservices.io` (*idempotent consumer*): marca de processado na mesma transação do trabalho.
- **Links relevantes**:
  - SPEC-7PJ5WVCS (UoW, inbox e outbox), SPEC-ANZX2WPG (consumo pela inbox), SPEC-CGPX20NP
    (relay da outbox) e SPEC-3R80KNMS (outbox Postgres): base da mensageria revisada aqui.
  - SPEC-ACYKBF9V e SPEC-6QT9SBAS declararam "replay persistido de resposta por
    `Idempotency-Key`" fora de escopo; esta spec o entrega.
  - SPEC-AHPRBZCT afirma que os comandos de `bookings` são "idempotentes por identidade do
    agregado". O código contradiz isso em `ReserveBooking`.
  - `docs/dmpf/uow-inbox-outbox.md`: UOW-09/10, INB-01..18, GAR-01/03/04/10.
  - `docs/dmpf/politicas-transporte.md`: RST-02, GRP-08/09/14/17, TRP-06/23/42.
  - ADR-024, ADR-026, ADR-034, ADR-036, ADR-044, ADR-050, ADR-051, ADR-053.

<constraints>
- [P0] Uma chave repetida NUNCA produz segundo efeito de negócio nem segundo evento na outbox enquanto a entrada do comando estiver retida na inbox.
- [P0] A entrada do comando na inbox é gravada na MESMA transação do efeito; é proibido gravá-la em transação separada, antes ou depois.
- [P0] O efeito é único por tenant, subject e chave: o BFF deriva do subject a chave enviada ao contexto, e o contexto escopa por `(tenant, chave)`. A resposta de um tenant ou principal NUNCA é devolvida a outro, e o subject NUNCA atravessa para o contexto (CTX-12).
- [P0] Registro ilegível NUNCA leva à reexecução do comando: devolve erro técnico `Internal`.
- [P0] Nenhum texto, nome ou teste promete exactly-once fim a fim (GAR-01, vetor V31); o termo é "efeito único por chave dentro da retenção".
- [P1] Os contratos Protobuf não mudam mensagem, campo nem opção de método; `buf breaking` (categoria `FILE`) continua verde.
- [P1] Falha técnica, recusa de autenticação ou autorização, admissão (429) e formato inválido NÃO deixam entrada na inbox.
- [P1] Toda espera por chave concorrente tem teto menor que o prazo restante do método.
</constraints>

## Requisitos

### Funcionais

#### Chave e escopo

- [ ] **[P0] Chave obrigatória nas duas bordas**: os 7 comandos (`AddItem`, `PlaceOrder`,
  `Reserve`, `Cancel`, `ReserveBooking`, `CancelBooking`, `RegisterResource`) exigem a chave no
  header REST `Idempotency-Key` e na metadata gRPC `idempotency-key`, com o formato
  `^[A-Za-z0-9._-]{1,128}$` nas duas bordas.
  - Na borda gRPC, chave ausente devolve `InvalidArgument` com `ErrorInfo.reason =
    MISSING_IDEMPOTENCY_KEY`, e formato inválido devolve `INVALID_IDEMPOTENCY_KEY`.
  - As leituras não exigem nem usam a chave.
  - O serviço de aplicação recusa comando sem chave no portador, como defesa em profundidade além
    do interceptor.
- [ ] **[P0] Escopo e fingerprint**: a entrada do comando na inbox do contexto é identificada pelo
  tenant e pela chave. O fingerprint é SHA-256 sobre uma codificação canônica, com prefixo de tamanho,
  do nome da operação do caso de uso (por exemplo `orders.AddItem`) mais os campos que cada comando
  declara.
  - A codificação não depende da serialização Protobuf, que não é canônica.
  - A mesma chave com outra operação ou outro payload é divergência.
- [ ] **[P0] Chave derivada pelo BFF**: o BFF envia ao contexto `hex(SHA-256(subject ‖ 0x00 ‖ chave
  do cliente))`, com 64 caracteres, no lugar da chave do cliente.
  - O subject não atravessa para o contexto (CTX-12, IDN-02); só a chave derivada viaja.
  - A mesma chave do cliente enviada por subjects diferentes vira chaves diferentes no contexto.
  - O log do BFF registra a chave do cliente e a derivada, para correlação.
- [ ] **[P0] Chave num portador próprio**: a chave viaja no portador da requisição, num valor de
  `libs/backend/go/ports` fora do `ExecutionContext`, preenchido pelo interceptor `requestContext`.
  - O `ExecutionContext` mantém os nove campos de CTX-01.
  - A chave NÃO é propagada para chamadas de saída, para o `MessageContext` nem para a outbox.
  - Continua no log da chamada, como hoje.

#### Inbox de comandos no kernel

- [ ] **[P0] A inbox recebe comandos**: o comando se registra pela porta `ports.Inbox`, na mesma
  tabela `inbox` das mensagens, por uma binding própria (`Tx.CommandInbox`) com realização
  `postgres` e `memory`.
  - `consumer_name` fixo por contexto (`<ctx>.commands`), `message_type` = operação e
    `payload_hash` = hex do fingerprint.
  - `message_id` = `<tenant>/<chave>`: o provider embute o tenant resolvido pelo `tenantOf`, o
    mesmo choke point das escritas de agregado. A chave `(consumer_name, message_id)` do INB-01 não
    muda, e a inbox continua sem coluna de tenant (ADR-050).
  - A suíte `providerkit.CommandInbox` roda contra as duas realizações.
- [ ] **[P0] Colunas novas na inbox**: `outcome` (bytea) e `expires_at` (bigint), ambas anuláveis,
  criadas por migração idempotente (`ADD COLUMN IF NOT EXISTS`), com o índice parcial
  `inbox_consumer_name_expires_at_idx`. Numa mensagem as duas ficam nulas.
- [ ] **[P0] Semântica da recepção de um comando**: o `Register` é o primeiro statement dentro do
  `Within`.
  - R1 (chave livre): o comando executa, e o `Complete` grava `processed` (aceite) ou `rejected`
    (recusa de domínio) com o outcome, na mesma transação.
  - R2 e R3 (mesmo fingerprint): devolvem o outcome gravado sem chamar o domínio, sem `Save`, sem
    `Enqueue` e sem nova auditoria; o span registra o replay.
  - R4 (outro fingerprint): devolve `ErrIdempotencyMismatch`, que o servidor mapeia para
    `FailedPrecondition` com reason `REUSED_IDEMPOTENCY_KEY`.
  - Chave de transação ainda aberta: o `Register` espera. O teto é o menor entre
    `IdempotencyWait` (padrão 1s) e o prazo restante menos 100ms. No estouro, o
    `ErrRegisterTimeout` (INB-17) vira `ErrIdempotencyInFlight`, mapeado para `Aborted` com reason
    `IN_FLIGHT_IDEMPOTENCY_KEY`.
  - Entrada de comando com `expires_at` vencido e ainda não purgada conta como ausente e é
    substituída na mesma transação. Uma entrada de mensagem nunca expira.
- [ ] **[P0] Desfechos gravados**: a entrada guarda os dois ramos de `Outcome[R]`, aceite e recusa
  de domínio (`libs/backend/go/application/outcome.go:12`).
  - A codificação é versionada, com codec declarado pelo contexto para cada operação.
  - O codec lê entradas gravadas pela release anterior.
  - Outcome ilegível devolve `Internal`, sem reexecutar.
- [ ] **[P1] Nada sem commit**: callback que falha faz rollback e não deixa entrada. Falha técnica,
  autorização, admissão e formato não chegam a registrar o comando.
- [ ] **[P1] Isolamento explícito**: a UoW abre a transação com
  `pgx.TxOptions{IsoLevel: pgx.ReadCommitted}`, qualquer que seja o padrão do servidor, como o
  `INB-18` exige.

#### Borda gRPC dos contextos

- [ ] **[P0] Comandos pela inbox**: os serviços de aplicação de `orders`, `reservations` e
  `bookings` executam os 7 comandos pela inbox de comandos do contexto.
- [ ] **[P0] Declaração de comando no servidor**: cada servidor declara quais métodos são comandos.
  Um teste confronta a declaração com o descriptor do serviço: todo método que não é leitura é
  comando.
- [ ] **[P1] Sinal de replay**: a resposta reproduzida leva o header gRPC `idempotent-replayed:
  true`.
- [ ] **[P1] Identidade já existente**: `ReserveBooking` carrega o agregado antes de criar. Quando o
  `bookingId` já existe, devolve o sentinela novo `ports.ErrAlreadyExists`, mapeado para
  `AlreadyExists` com mensagem sem "replay".
  - O `Save` do kernel continua devolvendo `ErrVersionConflict` para criação sobre linha existente,
    como decidido em `libs/backend/go/postgres/table.go:181-183`.
  - Uma corrida entre chaves diferentes converge: o perdedor recebe `version-conflict`, retenta e
    recebe `already-exists`.
- [ ] **[P1] Retry de comando no BFF**: os 7 comandos passam a `Idempotent: true` e
  `RetryableCodes: [Unavailable]` em `apps/backend/bff/app/rpc/clients.go`.
  - Toda tentativa leva a mesma chave e cabe no `Budget` da rota (GRP-08/09/17).

#### Borda REST do BFF

- [ ] **[P0] Mapeamento dos novos desfechos**: o BFF traduz os desfechos do contexto assim:

  | Desfecho no contexto | Resposta do BFF |
  |----------------------|-----------------|
  | `REUSED_IDEMPOTENCY_KEY` | 422 `reused-idempotency-key` |
  | `IN_FLIGHT_IDEMPOTENCY_KEY` | 409 `in-flight-idempotency-key` |
  | `AlreadyExists` | 409 `already-exists` |
  | `MISSING` e `INVALID` vindos da borda gRPC | 400 com os códigos atuais `missing-idempotency-key` e `invalid-idempotency-key` |
- [ ] **[P0] Replay fiel**: o replay devolve o mesmo status e o mesmo corpo da primeira resposta,
  acrescidos do header `Idempotent-Replayed: true`. A primeira resposta não leva esse header.
- [ ] **[P1] Contrato OpenAPI alinhado**: nos 3 `openapi.yaml`, o header `Idempotency-Key` ganha
  `pattern` e `maxLength: 128` idênticos ao regex do BFF.
  - As respostas 400, 409 e 422 documentam os códigos da tabela acima.
  - O header de resposta `Idempotent-Replayed` e a retenção de 24h ficam documentados.
  - `apps/backend/bff/app/api/openapi_test.go` compara o `pattern` com o regex do BFF.
- [ ] **[P1] CORS**: a resposta expõe `Idempotent-Replayed` em `Access-Control-Expose-Headers`.

#### Consumo e publicação de mensagens

- [ ] **[P1] Classificação transitória**: `application.Classify` passa a mandar para D3
  (retentável), e não mais para D4, os casos abaixo:
  - `context.DeadlineExceeded`, porque sob a inbox o efeito interrompido fica ausente ou idempotente,
    que é a condição que ERR-10 exige;
  - erros do Postgres da classe SQLSTATE `08`, e os códigos `40001`, `40P01` e `57P01`.
  - `context.Canceled` continua D4: a tabela de retryability de FND-07 §5 declara `Cancelled`
    como não retentável.
- [ ] **[P1] Quarentena idempotente**: `quarantine` ganha `envelope_digest` (SHA-256 do envelope)
  e `UNIQUE (consumer_name, envelope_digest)`, e a contenção grava com `ON CONFLICT DO NOTHING`.
  - A migração é idempotente e consolida as linhas duplicadas preexistentes antes do índice.
- [ ] **[P2] Produtor Kafka travado por teste**: um teste de unidade prova que o publisher usa
  `AllISRAcks` e não desliga a escrita idempotente do franz-go.
  - `libs/backend/go/kafka/README.md` documenta que a republicação pelo relay não é coberta pelo
    produtor idempotente e é absorvida pela inbox (TRP-06/42).

#### Retenção e purga

- [ ] **[P1] Purga no processo dono**: cada processo purga o que é dele.
  - `serve-api` purga as entradas de comando vencidas da inbox, via `PurgeExpiredInbox`.
  - `serve-relay` purga a `outbox` publicada, via `PurgePublished`.
  - `serve-consumer` purga as entradas de mensagem da `inbox`, via `PurgeInbox`.
  - Os padrões ficam em `Defaults()` do config de cada contexto: idempotência 24h, outbox 168h,
    inbox 192h, intervalo 15min, lote 1000.
- [ ] **[P1] Guarda do INB-14**: `serve-consumer` recusa iniciar se a retenção da inbox for menor
  que a janela de redelivery do canal (7 dias em `transport/channel/kafka_channel.go:15`).
- [ ] **[P1] Purga concorrente segura**: réplicas purgam em lotes com `SKIP LOCKED`, sem bloquear
  umas às outras, e nunca removem entrada vigente.

#### Forma canônica, generator e gates

- [ ] **[P1] Generator**: o template em `tools/dmpf-plugin/src/generators/bounded-context/files/`
  migra `Inbox`, traz os campos de retenção e espera no config, hospeda a purga, declara os
  métodos de comando e mapeia os novos erros em `statusOf`. O generator não gera casos de uso nem
  harness, então passar os comandos pela inbox cabe a quem escreve o contexto.
  `generator.spec.ts` e `tools/dmpf-generator-check.sh` verificam isso.
- [ ] **[P1] Harness**: `apps/backend/<ctx>/appkit/harness.go` dos 3 contextos ganha o caso
  "comando repetido com a mesma chave".

#### Normas e documentação

- [ ] **[P1] Família IDM**: nova seção "Idempotência de comando" em `docs/dmpf/uow-inbox-outbox.md`,
  com as regras IDM-01 em diante.
  - Cobre, no mínimo: obrigatoriedade, formato, escopo, fingerprint, mesma transação, desfechos
    gravados, espera com teto, replay, retenção e não propagação.
  - `docs/dmpf/politicas-transporte.md` referencia a família a partir de RST-02 e GRP-08/09.
- [ ] **[P1] ADR-056**: registra as decisões D1 a D13 desta spec.
  - Registra a extensão do ADR-050: a inbox, que continua sem coluna de tenant, passa a guardar o
    outcome dos comandos, escopado pelo tenant embutido na chave.
  - Revisa a afirmação do ADR-044 de que só `FindOrder` e `FindReservation` são retentáveis.
- [ ] **[P2] Documentação de uso**: os `README.md` de `bff`, `orders`, `reservations`, `bookings`,
  `postgres` e `kafka` documentam a chave, os códigos, a retenção e o header de replay. O comentário
  de cada RPC de comando nos `.proto` cita a metadata `idempotency-key`.

### Não-funcionais

- [ ] **Custo**: a inbox acrescenta no máximo 5 statements por comando (até 4 para registrar e 1
  para concluir), todos dentro da transação do comando. Não há nenhuma ida ao banco fora da UoW, e
  o replay não carrega o agregado.
- [ ] **Segurança**:
  - A chave derivada no BFF e o escopo por tenant no contexto impedem replay entre tenants e entre
    principais.
  - A autorização da rota no BFF e a do contexto rodam antes do registro na inbox, então um
    principal sem permissão recebe 403 mesmo em replay.
  - O outcome armazenado não contém credencial.
  - Só o kernel acessa a tabela (ADR-051, gate `context-provider`).
- [ ] **Compatibilidade**:
  - `buf breaking` `FILE` continua verde.
  - O cliente que já manda uma chave nova por requisição não percebe mudança.
  - `apps/backend/bff/app/e2e_test.go` continua verde.
- [ ] **Observabilidade**: o span da operação recebe o atributo `dmpf.idempotency_outcome` com um destes
  valores: `new`, `replayed`, `mismatch` ou `in_flight`.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Domínio (`<ctx>/domain`) | [ ] | Sem mudança: a inbox envolve o caso de uso |
| Portas (`libs/backend/go/ports`) | [x] | Extensões da `Inbox`, `Fingerprint`, sentinelas, `ErrAlreadyExists`, portador da chave |
| Aplicação do kernel (`libs/backend/go/application`) | [x] | Comando pela inbox, fingerprint, envelope do outcome, `Classify` |
| Providers (`libs/backend/go/{postgres,memory}`) | [x] | `CommandInbox`, colunas da inbox, isolamento, quarentena, purga |
| Transporte (`libs/backend/go/{grpc,kafka}`) | [x] | Interceptor exige a chave em comando; `ErrorInfo`; header de replay; teste do produtor |
| Aplicação dos contextos (`apps/backend/{orders,reservations,bookings}`) | [x] | 7 comandos pela inbox, codecs, `AlreadyExists` no `ReserveBooking`, wiring, purga, harness |
| BFF (`apps/backend/bff`) | [x] | Chave derivada, mapeamento dos desfechos, header de replay, retry de comando, CORS |
| Contratos | [x] | OpenAPI alinhado; `.proto` só com comentários |
| Tooling (`tools/`) | [x] | Generator |
| Documentação (`docs/`) | [x] | Família IDM, ADR-056, READMEs |

## Localização de código

```text
libs/backend/go/
  ports/            — extensões da Inbox, Fingerprint, sentinelas, ErrAlreadyExists, portador da chave
  application/      — comando pela inbox, fingerprint canônico, envelope do outcome, Classify
  postgres/         — inbox.sql (outcome, expires_at), Tx.CommandInbox, isolamento, purga, quarentena
  memory/           — Tx.CommandInbox em memória
  grpc/             — requestContext com chave obrigatória por método de comando; ErrorInfo; replay
  kafka/            — teste de opções do publisher; README
  testkit/providerkit/ — suíte CommandInbox (memory e postgres)
apps/backend/
  orders/ reservations/ bookings/
    application/    — comandos pela inbox; AlreadyExists no ReserveBooking (bookings)
    app/            — codec por operação, wiring (capability), purga, declaração de comandos, config
    app/rpc/        — mapeamento de erros com ErrorInfo
    appkit/         — caso de harness "comando repetido"
    contract/openapi/v1/openapi.yaml — header, códigos, Idempotent-Replayed
  bff/app/api/      — mapeamento REST, Idempotent-Replayed, CORS, openapi_test
  bff/app/rpc/      — chave derivada, Idempotent e RetryableCodes dos comandos, Classify
tools/dmpf-plugin/src/generators/bounded-context/ — template e generator.spec.ts
docs/dmpf/uow-inbox-outbox.md, docs/dmpf/politicas-transporte.md, docs/adr/055-*.md
```

**Arquivos a modificar** (principais):
- `libs/backend/go/postgres/uow.go:39`: isolamento READ COMMITTED explícito.
- `libs/backend/go/postgres/inbox.go`: `CommandInbox`, com tenant na chave, expiração e outcome.
- `apps/backend/bookings/application/reserve_booking.go:37`: `Load` antes de criar e
  `ErrAlreadyExists` quando a identidade existe.
- `apps/backend/bff/app/rpc/metadata.go:51`: chave derivada do subject e da chave do cliente.
- `libs/backend/go/postgres/inbox.sql` e `quarantine.go`: colunas `outcome` e `expires_at` da
  inbox; `envelope_digest`, índice único e `ON CONFLICT` da quarentena.
- `libs/backend/go/application/disposition.go:51-70`: D3 para prazo e erros transitórios.
- `libs/backend/go/grpc/interceptor_context.go:158-199`: chave obrigatória em comando, gravada no
  portador da requisição.
- `apps/backend/<ctx>/app/rpc/errors.go`: `statusOf` com `AlreadyExists` e os reasons de
  idempotência.
- `apps/backend/bff/app/rpc/clients.go:102-117` e `errors.go:49`: retry de comando e
  classificação dos reasons.

## Design

### Arquitetura

```text
cliente ──HTTP POST + Idempotency-Key──▶ BFF (sem estado)
                                          │ valida formato, autentica, autoriza, admite
                                          │ deriva a chave: sha256(subject ‖ 0x00 ‖ chave)
                                          │ repassa a chave derivada em metadata idempotency-key
                                          ▼
                              contexto gRPC (orders | reservations | bookings)
                                          │ interceptor: comando sem chave → InvalidArgument
                                          │ chave no portador da requisição (fora do ExecutionContext)
                                          ▼
                              serviço de aplicação: Authorize → ResolveIdentity
                                          ▼
                    UoW.Within (READ COMMITTED) ─────────────────────────────┐
                      1. Inbox.Register(<ctx>.commands, tenant/chave, fp)     │ mesma
                      2. R1: domínio decide → Save → Enqueue                  │ transação
                      3. R1: Pending.Complete(status, outcome codificado)     │
                    ◀─────────────────────────── COMMIT ──────────────────────┘
```

### Fluxo principal

1. **Primeira execução (R1)**: o `Register` insere a entrada, o comando executa, e o `Complete`
   grava o status e o outcome; o commit torna tudo visível de uma vez. A resposta sai sem
   `Idempotent-Replayed`.
2. **Duplicata após a conclusão (R2 ou R3)**: o `Register` encontra a entrada com o mesmo
   fingerprint. O outcome gravado volta decodificado, e a transação commita sem escrita de negócio.
   O BFF devolve o mesmo status e corpo com `Idempotent-Replayed: true`.
3. **Duplicata concorrente**: o insert da segunda requisição bloqueia no índice único até a
   primeira commitar, e aí vira replay. Se a primeira fizer rollback, a segunda executa como nova.
   Estourado o teto, a resposta é 409 `in-flight-idempotency-key`, e o cliente repete com a mesma
   chave.
4. **Payload divergente (R4)**: o fingerprint difere, e a resposta é 422 `reused-idempotency-key`,
   sem efeito.
5. **Resposta perdida**: o BFF devolve 504, mas o contexto commitou. O retry com a mesma chave cai
   no passo 2. Se o contexto não commitou, o retry executa como novo.
6. **Chave expirada**: passadas 24h, a entrada vencida conta como ausente e o comando executa de
   novo. A retenção está publicada no OpenAPI.

### Pseudocódigo

```text
executar_comando(ctx, op, cmd, codec):
    autorizar(ctx, cmd)                        # 403 antes de qualquer replay
    chave = portador.idempotency_key(ctx)      # no BFF: sha256(subject ‖ 0x00 ‖ chave do cliente)
    se chave vazia: erro InvalidArgument
    fp = sha256(canonico(op.nome, cmd.campos_declarados()))
    identidade = resolver_identidade()
    uow.within(ctx, read_committed):
        recepcao = res.inbox.register(<ctx>.commands, chave, tipo = op, hash = hex(fp),
                                      espera = min(IdempotencyWait, prazo_restante - 100ms),
                                      expira = agora + 24h)      # o provider embute o tenant
        caso recepcao:
            timeout      -> erro ErrIdempotencyInFlight
            R4           -> erro ErrIdempotencyMismatch
            R2 ou R3     -> outcome = codec.decode(recepcao.stored) ou erro Internal
                            marcar_replay(ctx); retornar sem escrita e sem auditoria
            R1           -> outcome = op.executar(res, cmd, identidade)   # aceite ou recusa
                            pending.complete(processed|rejected, codec.encode(outcome))
    retornar outcome
```

## Decisões técnicas

- **D1 — A inbox do contexto garante a idempotência da entrada; BFF sem estado**: no DMPF a inbox
  guarda a entrada e a outbox guarda a saída, e a entrada síncrona (comando) passa pela mesma
  guarda da assíncrona (mensagem). A classificação R1-R4 e a espera do INB-17 já são o que o
  comando precisa, e só a transação do contexto torna a entrada e o efeito atômicos.
  - Alternativa descartada: tabela e porta próprias de idempotência, que duplicariam a inbox com
    outro nome e outro contrato.
  - Alternativa descartada: store no BFF. O BFF não tem banco, e gravar fora da transação do efeito
    deixa uma janela entre o commit e o registro.
- **D2 — Chave na metadata gRPC, não em campo do request**: a metadata `idempotency-key` já é
  propagada (`metadata.go:51`), e um interceptor genérico a trata.
  - Alternativa descartada: campo `request_id` (AIP-155). Mudaria as 7 mensagens de request e o
    código gerado para transportar a mesma informação.
- **D3 — Sem `idempotency_level` no `.proto`**: o `buf breaking` usa a categoria `FILE`
  (`apps/backend/*/contract/buf.yaml:9-11`), que inclui `RPC_SAME_IDEMPOTENCY_LEVEL`, e a
  declaração nos métodos atuais reprovaria o gate contra a base. A classificação vive no servidor,
  com teste contra o descriptor, e no ADR-056.
- **D4 — Espera bloqueante no índice único, com teto**: é o padrão da inbox (INB-06/17). A duplicata
  de retry rápido vira replay em vez de 409.
  - Alternativas descartadas: `NOWAIT`, que devolve 409 desnecessário, e advisory lock, que é outra
    primitiva para o mesmo fim.
- **D5 — Gravar aceite e recusa**: os dois ramos já commitam com `error == nil` (DEC-04, ADR-018).
  O draft IETF manda devolver o resultado anterior, de sucesso ou de erro.
  - Não grava falha técnica, autenticação, admissão nem formato; o Stripe exclui validação e rate
    limit.
  - Alternativa descartada: gravar só o aceite. O retry de uma recusa poderia virar aceite com o
    estado mudado, e a mesma chave teria respostas diferentes.
- **D6 — Escopo por tenant e subject, com o subject dobrado na chave pelo BFF**: o BFF deriva do
  subject e da chave do cliente a chave enviada ao contexto, e o contexto escopa por `(tenant,
  chave)`. É a chave composta com atributo conhecido só pelo servidor, do draft (§5), no lugar do
  `UNIQUE (user_id, key)` do Brandur, porque o subject não atravessa para o contexto (CTX-12). A
  operação entra no fingerprint, e a chave reusada em outra operação vira 422.
  - Alternativa descartada: propagar o subject até o contexto. CTX-12 veda o subject como valor do
    contexto a jusante.
  - Alternativa descartada: escopo só por tenant. A Adyen documenta que credenciais da mesma conta
    veem as respostas umas das outras.
- **D7 — 422 para divergência e 409 para em andamento**: é o que manda o draft -07 §2.7. O Brandur
  usa 409 para os dois, e seguimos o draft.
- **D8 — Formato de token atual, não sf-string**: o draft que exige Structured Field String expirou
  sem virar RFC, e o Stripe usa token. Mantém compatibilidade com os clientes e com o regex do BFF.
- **D9 — Retenção de 24h**: é o prazo do Stripe, e fica publicado no OpenAPI.
  - Alternativas descartadas: 72h e 7 dias, porque nenhum cliente pede retry acima de 24h.
- **D10 — Purga no processo dono, sem deployable novo**: cada processo já é dono da sua tabela.
  - Alternativa descartada: CronJob Kubernetes, que acrescenta manifestos por contexto nos 3
    overlays.
- **D11 — Tenant embutido na chave, sem coluna de tenant na inbox**: o provider compõe o
  `message_id` do comando como `<tenant>/<chave>`, com o tenant resolvido pelo `tenantOf`, o mesmo
  choke point das escritas de agregado (ADR-051). A chave do INB-01 não muda, a inbox continua
  tabela de plataforma sem coluna de tenant (ADR-050), e o outcome de um comando, que é dado de
  negócio (IDN-11), só é alcançável pelo tenant que o gravou. A chave do cliente não contém "/"
  pelo pattern, então a composição é inequívoca.
  - Alternativa descartada: coluna `tenant_id` na inbox, com o único passando a
    `(consumer_name, tenant_id, message_id)`. Mudaria o INB-01 e exigiria, nas mensagens, um valor
    para "sem tenant", que o IDN-20 veda.
- **D12 — Chave fora do `ExecutionContext`**: CTX-01 fixa nove campos, e a matriz de travessia prova
  que eles atravessam as fronteiras; a chave não deve atravessar. Um valor próprio no portador da
  requisição atende o contexto sem tocar a norma.
  - Alternativa descartada: décimo campo, que exigiria emendar CTX-01 e a matriz só para, em
    seguida, excluí-lo da travessia.
- **D13 — `AlreadyExists` local ao `ReserveBooking`**: o `Save` do kernel devolve
  `ErrVersionConflict` para criação sobre linha existente por decisão registrada no código. Só o
  `ReserveBooking` cria com identidade vinda do cliente sem carregar antes, então só ele precisa
  distinguir a identidade duplicada.
  - Alternativa descartada: mudar o `Save` do kernel. Isso muda o desfecho de corridas de criação
    em `orders`, `reservations` e no consumer, que hoje convergem por retry de conflito.

## Regras relacionadas

- `docs/dmpf/uow-inbox-outbox.md`: UOW-09 e UOW-10 (sem repetição do callback; retry só com
  idempotência comprovada), INB-06/14/17/18, e GAR-01/03/04/10.
- `docs/dmpf/politicas-transporte.md`: RST-02, GRP-08, GRP-09, GRP-14, GRP-17, e TRP-06/23/42.
- ADR-044: cadeia do BFF; esta spec revisa a retentabilidade dos comandos.
- ADR-050: a inbox segue sem coluna de tenant; o ADR-056 registra que ela guarda o outcome dos
  comandos, escopado pelo tenant embutido na chave.
- ADR-051: choke point de tenant; a tabela é acessada só pelo kernel.
- ADR-053: forma canônica do contexto; generator e `dmpf-context-check.sh`.
- SPEC-XJWRJPVZ (suíte de carga k6): usa uma chave nova por requisição e segue compatível; a
  jornada de replay fica fora desta spec.
- SPEC-Z2HM6NAP (backlog): a drenagem do consumer na janela de graça é vizinha, sem dependência.

## Verificação e testes

### Critérios de aceite

- [x] O mesmo `AddItem` enviado 2 vezes com a mesma chave deixa `item_count` = 1 e exatamente 1
  `ItemAdded` na outbox. A segunda resposta é idêntica em status e corpo e traz
  `Idempotent-Replayed: true`.
- [x] Os 7 comandos, repetidos com a mesma chave, devolvem o status e o corpo da primeira resposta,
  inclusive quando a primeira foi uma recusa 422.
- [x] Chave reusada com payload diferente devolve 422 `reused-idempotency-key`, sem efeito.
- [x] 2 requisições concorrentes com a mesma chave produzem 1 efeito. A outra recebe replay, ou 409
  `in-flight-idempotency-key` quando a primeira passa do teto.
- [x] A mesma chave em tenants ou subjects diferentes executa 2 vezes, sem replay cruzado.
- [x] Um comando gRPC direto sem `idempotency-key` recebe `InvalidArgument` com reason
  `MISSING_IDEMPOTENCY_KEY`.
- [x] `ReserveBooking` com `bookingId` existente e chave nova devolve 409 `already-exists`, e não
  `version-conflict`.
- [x] Com o padrão do servidor em REPEATABLE READ, a transação da UoW roda em READ COMMITTED.
- [x] Handler de consumo com prazo estourado faz `Release` (D3), e não quarentena. A mesma mensagem
  contida 2 vezes gera 1 linha em `quarantine`.
- [x] A purga remove só as entradas de comando vencidas da inbox, a `outbox` publicada e as
  entradas de mensagem da inbox fora da retenção, e o `serve-consumer` recusa iniciar com retenção
  da inbox menor que 7 dias.
- [ ] `buf breaking` verde nos 3 contratos; `pnpm nx affected -t lint,typecheck,test,build` verde;
  `fmt-check`, `vet`, `test-race` e `govulncheck` verdes nos projetos Go afetados.
  **Pendente:** `buf breaking` reprova por BUF-08 (marcas `contracts-baseline/*` ausentes),
  condição anterior a esta entrega e sem `.proto` no diff; os demais gates estão verdes.
- [ ] `tools/dmpf-context-check.sh`, `go run ./tools/dmpf-conformance/cmd/conformance --root .
  --base develop` e o teste do generator verdes.
  **Pendente:** os três itens estão verdes; o critério fica aberto até o
  `dmpf-generator-check`, que só roda depois dos commits.

### Cenários de teste

```text
DADO um pedido aberto sem itens no tenant T1
QUANDO o cliente envia POST /orders/{id}/items duas vezes com Idempotency-Key "k1" e o mesmo corpo
ENTÃO as duas respostas são 201 com corpo idêntico, a segunda com Idempotent-Replayed: true,
      o pedido tem 1 item e a outbox tem 1 ItemAdded

DADO um PlaceOrder concluído com a chave "k2"
QUANDO o cliente repete o PlaceOrder com "k2" depois de receber 504
ENTÃO a resposta é o 200 original com Idempotent-Replayed: true, sem 422 order-not-open

DADO um Reserve em andamento com a chave "k3", segurando a transação por 2s
QUANDO uma segunda requisição com "k3" chega e o teto de espera é 1s
ENTÃO a segunda recebe 409 in-flight-idempotency-key, e a primeira conclui com 1 efeito

DADO um AddItem concluído com a chave "k4" e quantidade 1
QUANDO o cliente envia "k4" com quantidade 2
ENTÃO a resposta é 422 reused-idempotency-key, e o pedido continua com 1 item

DADO um AddItem concluído com a chave "k5" pelo subject S1 do tenant T1
QUANDO o subject S2 do tenant T1 envia "k5" com o mesmo corpo para outro pedido
ENTÃO o comando executa como novo, sem devolver a resposta de S1

DADO uma entrada de comando gravada na inbox há 25h e ainda não purgada
QUANDO o cliente repete o comando com a mesma chave
ENTÃO o comando executa como novo, e a entrada é substituída na mesma transação

DADO uma mensagem OrderPlaced cujo handler estoura o prazo antes do commit
QUANDO o consumer classifica a falha
ENTÃO a disposição é D3 (Release), a redelivery é processada como R1, e nada vai para a quarentena

DADO um banco com default_transaction_isolation = repeatable read
QUANDO a UoW abre uma transação
ENTÃO SHOW transaction_isolation devolve read committed dentro do Within
```

<critical_constraints>
- [P0] Uma chave repetida NUNCA produz segundo efeito de negócio nem segundo evento na outbox enquanto a entrada do comando estiver retida na inbox.
- [P0] A entrada do comando na inbox é gravada na MESMA transação do efeito; é proibido gravá-la em transação separada, antes ou depois.
- [P0] O efeito é único por tenant, subject e chave: o BFF deriva do subject a chave enviada ao contexto, e o contexto escopa por `(tenant, chave)`. A resposta de um tenant ou principal NUNCA é devolvida a outro, e o subject NUNCA atravessa para o contexto (CTX-12).
- [P0] Registro ilegível NUNCA leva à reexecução do comando: devolve erro técnico `Internal`.
- [P0] Nenhum texto, nome ou teste promete exactly-once fim a fim (GAR-01, vetor V31); o termo é "efeito único por chave dentro da retenção".
- [P1] Os contratos Protobuf não mudam mensagem, campo nem opção de método; `buf breaking` (categoria `FILE`) continua verde.
- [P1] Falha técnica, recusa de autenticação ou autorização, admissão (429) e formato inválido NÃO deixam entrada na inbox.
- [P1] Toda espera por chave concorrente tem teto menor que o prazo restante do método.
</critical_constraints>

## Escopo fora

- **Ordem por agregado na publicação do relay**: é ordenação, não idempotência; o efeito continua
  único pela inbox. Pertence a uma spec de ordenação.
- **`source` na chave da inbox (INB-01)**: é risco latente, porque hoje cada canal admite um único
  source (`Boundary.Sources`).
- **Binding persistido por mensagem (TRP-09/46)**: pendente na ARQ-549; só importa com 2
  transportes por processo.
- **Corpo de erro `application/problem+json`**: a forma do corpo de erro não tem dona normativa, e
  o contrato atual fixa `{code, message}`.
- **Idempotência natural sem chave** (por exemplo, identificador de linha no `AddItem`): exige mudar
  o contrato; a chave cobre o retry.
- **`RegisterResource` repetido incrementando `version`**: é no-op de domínio sem evento, e o
  replay passa a evitar a escrita.
- **Vetor de fitness V33**: a prova fica no e2e e no harness. Um vetor novo exige realização por
  stack (RAS-16).
- **Jornada de replay na suíte k6**: pertence à SPEC-XJWRJPVZ ou a uma evolução dela.
- **Testes de caos com kill real de processo**: a redelivery e o rollback já são provados por
  integração.
- **Chave compartilhada entre contextos**: cada contexto tem sua inbox de comandos, e a mesma chave usada em
  contextos diferentes não é detectada como reuso.
- **Reescrever specs concluídas** (SPEC-AHPRBZCT:161, SPEC-ACYKBF9V:373, SPEC-6QT9SBAS:396): o
  ADR-056 registra a correção.
