---
id: SPEC-R8645FVR
slug: abstracoes-apps-go
title: DMPF — Abstrações dos apps Go, com esqueletos no kernel, kits de teste e correção de deriva
stage: building
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-63/spec-r8645fvr-dmpf-abstracoes-dos-apps-go-com-esqueletos-no-kernel
subtask_urls: [https://linear.app/mmda/issue/DEVS-64/spec-r8645fvr-onda-0-corrigir-a-deriva-entre-contextos-d1-d2-t5, https://linear.app/mmda/issue/DEVS-65/spec-r8645fvr-onda-1-funcoes-novas-no-kernel-e-mudancas-locais, https://linear.app/mmda/issue/DEVS-66/spec-r8645fvr-onda-2-template-method-estrutural, https://linear.app/mmda/issue/DEVS-67/spec-r8645fvr-onda-3-adrs-unidades-novas-e-promocoes-ao-kernel]
created: 2026-10-04
---

# SPEC-R8645FVR: Abstrações dos apps Go

## Resumo

Implementar o catálogo do relatório `docs/guides/dmpf-oportunidades-abstracao-go.md`.
A spec corrige as derivas D1 a D3, já observadas entre as cópias dos contextos,
e elimina a repetição estrutural dos apps Go (`bff`, `orders`, `reservations`,
`bookings`) nas 35 sugestões A1–A7, P1–P8, B1–B9 e T1–T11, com Template
Method, generics e extração. A spec também trata como oportunidade de
abstração a duplicação estrutural que o detector `dupl` mede em `apps/backend`
e `libs/backend/go`, além do catálogo.

Como mantenedor do kernel DMPF, quero que o que é igual por natureza viva uma
única vez, no kernel ou num helper local, para que a próxima correção não
precise ser repetida em três lugares nem deixe um contexto para trás.

A entrega segue as quatro ondas da seção 5 do relatório, e cada onda é um PR.

## Contexto

- **Problema**: os três contextos e o `bff` repetem arquivos e funções
  inteiros: `app/rpc/errors.go` é idêntico byte a byte, e o mesmo vale para
  `serveAPI`, `classify`, `subject`, o `distkit` inteiro de `orders` e
  `bookings` e as 648 linhas de teste de telemetria de cada contexto. A
  repetição já produziu deriva:
  - o `bookings` não audita comandos (D1);
  - o `distkit` de `orders` e `bookings` lê a saída do processo filho sem
    sincronização (D2);
  - os testes de idempotência do `bookings` correm sobre um store feito à mão,
    mais fraco que o `memory` do kernel (D3).
- **Impacto**: a estimativa do relatório, sem descontar a sobreposição entre
  achados, é de cerca de 1,5 mil linhas de produção e 3,5 mil de teste a menos.
  FND-04 §3.2, IDM-05, ERR-20 e RST-02 passam a valer por construção. O
  generator passa a gerar contextos que importam o esqueleto em vez de
  copiá-lo.
- **Inspiração**: o `reservations` já aplica localmente o esqueleto do comando
  (`decision[R]` e `write[R]` em `reservations/application/write.go`). O kernel
  já tem precedentes do mesmo desenho: `kafka.ReadClientAuth`,
  `boot.SignalsFromEnv`, `kafka.NewConfig(ctx, rt, catalog, …)` e
  `providerkit.Repository[ID, S]`.
- **Links relevantes**:
  - Relatório de origem, com evidência `arquivo:linha` no commit `8f544bc0` e
    assinaturas de referência por item: `docs/guides/dmpf-oportunidades-abstracao-go.md`.
  - ADRs: 034 (UoW, retry como gancho do caso de uso), 044 (`bff` como única
    borda REST), 045 (nomes, alias pelo papel), 046 (`libs` só para kernel de
    reuso), 048 (o que fica no contexto), 053 (forma canônica), 056
    (idempotência pela inbox) e 058 (projeto solo, mantendo o baseline com
    `DMPF-T001`).
  - SPEC-F308KDSF (`done`): previa a auditoria do `bookings`, e D1 fecha esse
    item.
  - SPEC-JJKWG4JP (`done`): cita o `errors.go` de cada contexto, que P1 retira.
  - SPEC-VZ16X0MS (`deferred`): planejava `enqueueAll` como template do
    generator, e A2 e A3 revogam isso.
  - Normas em `docs/dmpf/`, `.claude/rules/dmpf-bounded-context.md`,
    `docs/guides/dmpf-manifesto.md` e `docs/guides/dmpf-composicao.md`.

Convenção de caminho: `orders/…`, `reservations/…`, `bookings/…` e `bff/…`
ficam sob `apps/backend/`, e `application/…`, `grpc/…`, `testkit/…` e os demais
módulos do kernel ficam sob `libs/backend/go/`.

<constraints>
- [P0] NUNCA criar dependência de `provider` para `application` (`DMPF-D001`) nem entre contextos (`DMPF-D002`); o destino de cada item é o que o relatório indica em §4.
- [P0] NUNCA subir ao kernel o que é igual por coincidência ou diverge por papel (critério do ADR-048); os candidatos da §6 do relatório ficam fora.
- [P0] O comportamento observável é preservado, com exatamente três exceções declaradas: o `bookings` passa a auditar (D1, A1), a categoria do span cliente do kernel para `deadline.ErrNoDeadline` e `ErrDeadlineExhausted` deixa de ser `_OTHER` (B7), e 13 textos de falha de `orders` e `reservations` passam à forma única de A3, só em log e span.
- [P0] Nenhum item da Onda 3 entra em código antes de o ADR que o autoriza estar commitado.
- [P0] Unidade nova exige `include` no manifesto (`DMPF-U001`); o agente prepara a unidade, e o `--write-baseline` (`DMPF-T001`) fica com a pessoa responsável.
- [P0] Toda promoção ao kernel retira a cópia dos templates de `tools/dmpf-plugin/src/generators/bounded-context/files/` e da skill `.agents/skills/dmpf-bounded-context/` na mesma onda.
- [P1] Teste de comportamento só sai com cobertura equivalente no destino; alterar teste apenas para fazê-lo passar é proibido.
- [P1] Nomes de variável de ambiente, chaves de atributo de telemetria, layout de payload persistido e valores do fio não mudam (ADR-053, ADR-056).
- [P1] Esqueleto genérico é função livre (método não aceita parâmetro de tipo); alias genérico é permitido (`go 1.26.6`).
- [P1] Projetos Nx distintos vão em commits separados.
</constraints>

## Requisitos

### Funcionais

#### Onda 0 — corrigir deriva

- [x] **[P0] D1 — Auditoria no `bookings`**: `reserve_booking.go`, `cancel_booking.go` e `register_resource.go` de `bookings/application` usam o `replayed` de `idempotent` e chamam `Audit` só quando não há replay, como `orders/application/place_order.go`.
  - Teste por caso de uso: o comando novo audita uma vez, e o replay da mesma chave não audita.
- [x] **[P0] D2 — Saída do filho sincronizada**: em `orders/distkit/harness.go` e `bookings/distkit/harness.go`, `Process.Output()` adota a semântica de `reservations/distkit/harness.go`. Se `finished` já fechou, devolve o buffer; caso contrário, devolve `"(process still running)"` sem bloquear e sem ler o buffer.
  - A semântica final, com snapshot sob mutex, chega com T1 na Onda 3.
- [x] **[P0] T5 — Dublês do `bookings` sobre o `memory` (fecha D3)**: `memStore`, `memTx`, `txCommands` e `memUoW` de `bookings/application/doubles_test.go` dão lugar a `memory.Table`, `memory.NewUnitOfWork`, `tx.CommandInbox` e `Table.Reader`; `testOccurred` passa a nanossegundos.
- [x] **[P2] Manifestos com o franz-go**: os `dmpf-units.json` de `orders` e `bookings` declaram o franz-go que o `distkit` importa, como o do `reservations`.
- [x] **[P2] N1 (parte do `bookings`) — Arranjo repetido nos testes do `bookings`**: extração local, sem mudar o nome dos testes de topo.
  - `bookings/application/idempotency_test.go` ganha um helper de arranjo (primeiro comando aceito, segundo recusado, outbox com uma entrada), depois de T5 (G27).
  - `bookings/provider/mapper_test.go` ganha um helper local para os testes por evento (G25).

#### Onda 1 — funções novas em packages existentes ou mudança local

- [x] **[P1] P1 — `kernelgrpc.StatusOf`**: `grpc/status.go` ganha `StatusOf(err error) error`, que reproduz o `statusOf` atual caso a caso, na mesma precedência. A primeira linha que casar decide; "categoria" é o valor de `interface{ ErrorCategory() string }`, obtido sem importar `application`:

  | # | Condição (sentinela por `errors.Is` **ou** categoria) | Código gRPC | Mensagem |
  |---|-------------------------------------------------------|-------------|----------|
  | 1 | `kernelgrpc.IdempotencyStatus(err)` reconhece | o devolvido por ela | a devolvida por ela |
  | 2 | `ports.ErrNotFound` ou categoria `NotFound` | `NotFound` | `not found` |
  | 3 | `ports.ErrVersionConflict` ou categoria `Conflict` | `Aborted` | `version conflict; replay the call` |
  | 4 | `context.DeadlineExceeded` | `DeadlineExceeded` | `deadline exceeded` |
  | 5 | `context.Canceled` | `Canceled` | `canceled` |
  | 6 | `ports.ErrDenied` ou categoria `Forbidden` | `PermissionDenied` | `permission denied` |
  | 7 | `ports.ErrCredentialAbsent`, `ports.ErrCredentialRejected`, `ports.ErrSubjectUnresolved` ou categoria `Unauthenticated` | `Unauthenticated` | `unauthenticated` |
  | 8 | qualquer outro | `Internal` | `internal failure` |

  - Saem os três `app/rpc/errors.go`, seus `errors_internal_test.go` e o template `files/app/app/rpc/errors.go__tmpl__`.
  - Um teste de tabela no kernel cobre cada linha, além de `context.DeadlineExceeded` e `context.Canceled` puros e de erros híbridos (um `application.Failure` com categoria `X` que envolve a sentinela de outra linha). Nos híbridos, vale a linha de menor número.
  - Um teste de contrato no kernel `app` fixa a correspondência por string entre as categorias de `application` e as do `StatusOf`. Para isso, o `go.mod` do `app` passa a requerer `grpc` já nesta onda.
- [x] **[P1] A7 — Categoria do outcome e resultado de autorização**: `func (o Outcome[R]) Category() ports.OutcomeCategory` vai para o kernel `application`, e `func AuthorizationResult(err error) Result` vai para `ports`, ao lado de `ErrDenied`. Saem `outcomeCategory` e `authorizationResult` dos três contextos.
- [x] **[P1] A2 — `usecase.Enqueue`**: `Origin{Destination, AggregateType, AggregateID}` e `Enqueue(ctx, outbox, id, origin, written, events)` vão para o kernel `application` e preenchem os sete campos de FND-04 §2.3. Saem os três `enqueueAll`.
- [x] **[P1] A4 — Esqueleto de consulta**: `Query[Op, S]` no kernel `application`, usado por `find_order.go`, `find_reservation.go` e `find_booking.go`. `FindBookingByResource` (ramo IDN-13) fica local, mas usa `ports.AuthorizationResult`.
  - Assinatura: `Query[Op, S any](ctx, inst, authz Authorize[Op], operation string, input Op, load func(context.Context) (S, error)) (S, error)`, com `inst` nulo tratado como `ports.NoInstrumentation()`.
  - O chamador instancia o parâmetro de tipo explicitamente (`usecase.Query[Operation](...)`), porque o Go não infere `Op` quando a entrada é o comando concreto.
- [x] **[P1] P4 — Helpers de wiring**: `Catalog.AddressOf` e `NewCatalog` vão para `transport/channel`; `relay.Instrument(c, rt, address)` vai para `app/relay`; `PurgeOutbox`, `PurgeCommandInbox` e `PurgeMessageInbox` vão para `app`.
  - Saem `startPurge`, `RelayConfig`, `topicOf`, os closures de purge e o corpo de `NewCatalog` dos três `app/`. Cada contexto chama `kernelapp.StartPurge` inline com o `Purge*` do kernel.
  - `channel.NewCatalog(channels ...Channel) (Catalog, error)` recusa nome de canal repetido e catálogo sem canais, com sentinelas próprias. `Catalog.AddressOf` devolve vazio para destino fora do catálogo.
  - `PurgeCommandInbox(pool, consumer)` e `PurgeMessageInbox(pool, consumer)` recebem o nome do consumidor, que vive no `application` do contexto e o kernel não pode importar.
  - `Instrument(c Config, rt Telemetry, address func(string) string) Config` recebe uma interface local `relay.Telemetry` (tracer, meter provider e logger provider), que `*otelboot.Runtime` satisfaz, para o fechamento de produção do `app` não importar o SDK OTel. O `go.mod` do `app` não ganha `require` direto de `transport`; o indireto vem do `grpc`, que o teste de contrato do P1 requer.
  - `Instrument` não fixa `System`: cada contexto declara o sistema de mensageria nos `Defaults` do próprio relay, e o relay do kernel continua agnóstico de transporte.
  - O teste de `RelayConfig` dos três `app/wiring_test.go` migra para `app/relay`, junto de `Instrument` (G62).
- [x] **[P1] P7 — `postgres.SnapshotTable`**: `SnapshotTable[ID ~string, S, J]` no kernel `postgres`, usado por `order_repository.go`, `reservation_repository.go` e `resource_repository.go`, com o struct `J` privado do provider. A `bookingTable` fica manual, e o template do provider ganha o molde.
- [x] **[P1] B3 — Protocolo de hop do lado cliente**: o `bff` usa as constantes de metadata já exportadas e `kernelgrpc.DefaultLocale`. O kernel `grpc` passa a exportar `MetadataCarrier`, `NewID(component string) string`, `ValidCorrelation(string) bool` e `ValidLocale(string) bool`. O `component` prefixa o `panic` da fonte de entropia (`grpc` no interceptor e `api` no `bff`), para cada lado manter a mensagem que já tinha. Saem do `bff` o `carrier`, `correlationFormat`, `localeFormat`, `newIdentifier` e a cópia do locale.
- [x] **[P1] B6 — `redact.WithoutValues`**: `WithoutValues(message string, statement *regexp.Regexp) string` vai para `observability/redact`. `bff/app/wiring.go` e `observability/boot/libraries.go` passam a usá-la, cada um com o próprio regex.
- [x] **[P1] B4 — Chave de idempotência pela rota**: `requireIdempotencyKey(route kernelhttp.Route, next http.Handler)` decide por `route.IdempotencyKey`. Um GET com chave inválida continua recebendo 400, fixado por teste.
- [x] **[P2] B5 — Access log e resposta 500**: `logRequest`, `newRecorder` e `writeInternal` ficam locais ao `bff`, usados por `finishRequest`, `withAccessLog` e `healthAccess.ServeHTTP`. A gravação de atributos de span continua em `finishRequest`.
- [x] **[P2] B8 — Tabela de contextos no wiring do `bff`**: um laço faz dial, close e readiness sobre `[]backend{name, target, config}`, e outro faz o `serveContract` em `NewHandler`. `Config` e os nomes de variável não mudam.
- [x] **[P1] T7 — Asserções de domínio**: `tb.RequireRejected[R comparable]`, `tb.RequireAccepted` (sem parâmetro de tipo), `tb.RequireSameEvents` e `tb.RunProjection` vão para `testkit/tb`, e `Verdict.Merge` vai para `domainkit`. Saem dos `*/domain/helpers_test.go` as cópias de `requireRejected`, `requireAccepted` e `sameSequence`, além dos laços repetidos de fixture de projeção. Os arquivos ficam, com as constantes e os construtores do contexto.
  - `tb.RunProjection` devolve o veredito, e o chamador grava a evidência, porque o `evidence` já importa o `tb`.
  - `testkit/domainkit/fixture_test.go` também adota `tb.RunProjection` (G42).
- [x] **[P1] T8 — Leitores das tabelas do kernel**: `pg.Outbox`, `pg.Settled` e `pg.Counts` vão para `testkit/tb/pg`. `orders` e `bookings` exportam `app.NewService(pool, clock, ids, waits)`, como o `reservations`, e o `appkit` deixa de reimplementar o `bind`.
  - O template `files/app/appkit/pool.go__tmpl__` passa a refletir o `OpenPool` dentro de `harness.go`.
  - O comentário de doc do `Harness` em `reservations/appkit/harness.go` volta para cima do tipo.
- [x] **[P2] T9 — `provider/e2e_test.go` sobre o `appkit`**: `orders` e `bookings` trocam `fixedClock`, `sequenceIDs`, `outboxRow` e `newService` por `appkit.New*`, `h.Outbox`, `ids.Sequence` e `clock.New`. `Enqueued` ganha `SchemaVersion`.
- [x] **[P2] N1 (restante) — Arranjo repetido nos testes do kernel e do `reservations`**: helper de arranjo no próprio arquivo, com tabela e `t.Run` só onde o nome do teste de topo não muda.
  - `postgres/inbox_test.go`: dois testes que só mudam o consumidor e o erro esperado (G3) e três que só mudam o status, o hash e o ramo esperado (G35).
  - `postgres/inbox_concurrency_test.go`: a goroutine B, idêntica nos dois testes (G26).
  - `observability/logging/sampling_test.go`: o arranjo de trace e sampler (G39).
  - `reservations/application/consume_test.go`: o arranjo de reentrega de R2, R3 e R4 (G30, G56).
- [x] **[P2] N2 — Leitura das tabelas do schema**: `pg.Tables(t, pool) []string` vai para `testkit/tb/pg`, ao lado dos leitores de T8. Os `appkit/schema_test.go` dos contextos passam a usá-la e mantêm o `want` literal local como oráculo (G4).
- [x] **[P2] N3 — Prova do `Reader` no kernel**: um teste em `postgres` fixa que `Table.Reader` sobre `NewReadPool` envia só o `SELECT` (UOW-11). Saem o teste equivalente, o `tracedPool` e o `sqlRecorder` de `orders/provider/order_reader_test.go` e `reservations/provider/reservation_reader_test.go` (G45).
- [x] **[P2] N5 — Subjects do `memory` no `providerkit`**: `testkit/providerkit` exporta os construtores de subject sobre o `memory`, usados pelo self-test do kit e por `memory/conformance_test.go` (G12, G60). A superfície de `kernel/testkit-provider` cresce, sem unidade nova.
- [x] **[P2] N7 — Processo filho de execução única**: um helper de reexecução do binário de teste (ex.: `tb.Reexec(t, run, env...)`, que devolve stdout, stderr e o erro) vai para `testkit/tb` e é usado pelos `panic_test.go` de `kafka` e `sqs` (G58). Os módulos `kafka` e `sqs` passam a requerer `testkit`.

#### Onda 2 — Template Method estrutural

- [x] **[P0] A1 — Esqueleto do comando**: `Executor[Res, Op]`, `Command[Res, Op, R]` e `Execute[Res, Op, R]` vão para um arquivo novo do kernel `application`. O esqueleto fixa a sequência `BeginOperation` → `Authorize` → `ResolveIdentity` → `Within{RunIdempotent}` → `end` → `Audit`, este último só sem replay. Todos os casos de uso de comando dos três contextos e os dois wrappers `idempotent[R]` passam a usá-lo.
  - O retry continua sendo gancho do caso de uso, não decorator da UoW (ADR-034, KRN-09).
  - O `Command` recebe o `*Fingerprint` pronto do contexto, porque derivá-lo no esqueleto mudaria o `PayloadHash`. O contexto monta o `Executor` por um método `executor()` do próprio `Service`, sem mudar os composition roots.
  - `application/README.md` reescreve o mapa de passos, que hoje aponta para os arquivos do `orders`.
  - Os `sequence_test.go` dos três contextos passam. O passo `Audit` que o `bookings` ganha é provado por um teste de ordem `begin` → … → `end` → `audit` em cada contexto, porte do teste do `orders`, e por um teste no kernel.
- [x] **[P1] A3 — Passos 4 a 7 com `Loader`**: `Loader[ID, S, A]`, `OrNew`, `Existing`, `Absent` e `Decide` vão para o kernel `application`, sobre A2. `Absent` devolve `ports.ErrAlreadyExists`. A escolha do `Loader` fica no contexto, e o consumo do `reservations` reaproveita só o `OrNew`.
  - A forma de erro do `bookings` vale para os três contextos. `Decide` só envolve a falha de `Enqueue` com `enqueue: %w` e devolve a carga e o `Save` sem prefixo. O caso de uso envolve tudo com `application: <operação> <id>: %w` (ex.: `application: reserve %s: %w`, `application: add item to %s: %w`).
  - No `bookings`, os textos ficam idênticos aos de `8f544bc0`. Em `orders` e `reservations` mudam 13 textos, só em log e span, porque o `kernelgrpc.StatusOf` usa mensagem fixa:
    - no `orders`, o `Save` e o `Enqueue` de `AddItem` e de `PlaceOrder` (4);
    - no `reservations`, a carga, o `Save` e o `Enqueue` de `Reserve` e de `Cancel` (6), com o prefixo `load reservation` trocado por `reserve` e `cancel`;
    - no consumo do `reservations`, o `Save`, o `Save` com conflito e o `Enqueue` (3). O `Save` e o `Enqueue` ficam inline em `consume.go`, na mesma forma (`application: consume <id>:` e `enqueue:`), com o `Failure(Conflict)` por dentro do prefixo.
  - Os testes verificam ao mesmo tempo o `errors.Is` e a mensagem completa de cada caminho de falha (carga, `Absent`, `Save` e `Enqueue`), numa tabela por contexto: 7 caminhos no `orders`, 10 no `reservations` e 11 no `bookings`.
- [x] **[P2] A5 — UPR por cópia**: `DecideOver[A, R]` e `Refuse[R]` vão para o kernel `domain`. As sete UPRs e os doze pontos `kernel.Accepted[X]{}, kernel.Reject(...)` passam a usá-los, e o `clone` continua privado.
- [x] **[P1] P2 — Registro de métodos no `kernelgrpc`**: o kernel `grpc` ganha as funções abaixo. As assinaturas mantêm o descritor protobuf, que hoje garante que todo método registrado existe no `.proto`:
  - `Method(desc protoreflect.ServiceDescriptor, name protoreflect.Name) protoreflect.MethodDescriptor` entra em pânico quando o serviço não declara o método, como o `method()` atual;
  - `Unary[S, Req any, PReq interface{ *Req; proto.Message }, Resp proto.Message](service string, md protoreflect.MethodDescriptor, call func(S, context.Context, PReq) (Resp, error)) grpc.MethodDesc`;
  - `MethodNames(desc protoreflect.ServiceDescriptor) []string`;
  - `FullMethod(service, method string) string`.

  Também vale o seguinte:
  - Saem `unary`, `method`, `Methods` e o `FullMethod` inline dos três `app/rpc/service.go`, além da quarta cópia de `unary` em `bff/app/api/testing_test.go`.
  - O entrypoint `google.golang.org/protobuf/reflect/protoreflect` é declarado no `external` de `kernel/provider-grpc`.
  - Um teste fixa o pânico de `Method` para nome inexistente.
  - `executionOf` continua no handler.
- [x] **[P1] P5 — Seções comuns de `Config`**: `APIEnv`, `ReadAPIEnv` e `APIEnv.Missing` vão para o kernel `grpc`; `ClientAuth.Missing(insecure)` vai para o `kafka`; `Policies` e `Policies.Validate(consumes, invalid)` vão para o `app`. `requirements()` continua local e compõe os `Missing()`, e o `config.go__tmpl__` acompanha a mudança.
  - A `Config` recebe os dois como campos nomeados, `API kernelgrpc.APIEnv` e `Policies kernelapp.Policies`. Embutidos, eles promoveriam `Missing` e `Validate` para a `Config`, e o literal de `Defaults()` com campo promovido exige go1.27.
  - O contexto mantém o próprio `ErrInvalidPolicy`, com o prefixo dele, e o passa a `Policies.Validate`, que o embrulha. Um alias do kernel mudaria o texto da recusa e faria os três contextos dividirem o mesmo sentinela, o que P0 não admite.
  - Os testes de política e do par TLS (IDN-03) dos `app/config_test.go` migram para o kernel, junto de `Policies.Validate` e `APIEnv.Missing` (G7, G10). Cada `app/config_test.go` guarda um caso que prova a delegação do `Validate` (no `reservations`, com `consumes`). O teste de variável ausente do relay continua local (G57, fora de escopo).
- [x] **[P1] B1 — Esqueleto de handler REST**: `endpoint[Req, Resp]`, `decoder[Req]`, `fromPath`, `fromQuery`, `fromBody`, `fromPathAndBody`, `outcome` e `view` ficam locais em `bff/app/api` e são usados pelos onze handlers. `present` mantém o type switch do oneof.
  - O construtor da requisição e o apresentador do oneof de cada handler são funções nomeadas. Com closures inline, os três comandos só de path (`placeOrder`, `cancel` e `cancelBooking`) continuam clones, e o G43 fica de pé.
  - As respostas `malformed-body` e `invalid-request` ficam iguais byte a byte.
- [x] **[P1] B2 — Fábrica de rotas**: `surface.command`, `surface.query` e `binding{route, build}` vão para `bff/app/api/routes.go`.
  - Todo `POST` leva `IdempotencyKey`, a permissão é `<ctx>:write` ou `<ctx>:read`, e o `ContractRef` é derivado do método e do path.
  - O mapa `serve` indexado por string sai.
  - `Routes(budget)` continua pública e funcional sem clientes: o `build` de cada `binding` é uma method expression sobre os handlers, e só o `NewHandler` o chama.
- [x] **[P1] T2 — Suíte golden de contrato**: `Spec[M]` e `GoldenSuite[M]` vão para `testkit/tb`. `orders/contract/golden` e `reservations/contract/golden` passam a usá-los, e o `bookings/contract` ganha golden para os seus três protos de evento.
  - O teste golden de `libs/backend/go/contracts` também adota a `GoldenSuite`, porque a cópia do oráculo ENV-18 viraria par de clone dela (G37). O módulo `contracts` já requer `testkit`, e a fixture do kernel não muda.
- [x] **[P1] T4 — `serviceskit` adotável**: `serviceskit.UnitOfWork[R](f, bind)`, `Tx.CommandInbox(consumer)` e `serviceskit.Authorize[C]` passam a existir. `recordingClock`, `recordingIDs`, `recordingOutbox`, `recordingUnitOfWork[R]`, `recordingAuthorize` e `foldDigest` saem dos `application/*doubles_test.go` dos três contextos, assim como as cópias `sync*` do `reservations`.
  - Essas três peças não bastam para retirar os dublês. O kit ganha também `Steps`, `Fakes.Clock` e `Fakes.IDs`, `FoldDigest`, `MarkSeeded`, os contadores `WithinCalls`, `Commits` e `Binds`, `Ledger.Count` e `FailRegister`.
  - Os dublês do kit gravam em `Steps` os passos `within`, `commit`, `outbox.Enqueue`, `commands.Register` e `commands.Complete`. Os `sequence_test.go` de `orders` e `reservations` ganham o passo `commands.Complete`, que hoje só o `bookings` registra.
  - A injeção de falha vai para o kit como decoradores genéricos de falha de `Load`, `Save` e `Enqueue`. `option`, `setup` e os `with*` ficam no contexto só como montagem sobre o kit, e o `withAuthorize` também fica, porque montá-lo no kit faria o `serviceskit` importar `application` (`DMPF-D001`).
  - Na ordem nativa, os dublês não formam grupo no `-t 100`. A conclusão se prova pela busca das cópias nos critérios de aceite.
- [x] **[P2] T10 — `providerkit.Repository` por agregado**: os testes de repositório dos três contextos chamam `providerkit.Repository` por `postgres.Table` e mantêm só o teste de ida e volta do codec. O `bookings` tem uma chamada por agregado (booking e resource).
  - `RepositorySubject` ganha o campo `Concurrent`, e `providerkit.Repository` ganha duas cláusulas:
    - recusa criar sobre agregado existente, a cobertura equivalente dos testes de conflito na criação de `orders` e `reservations`;
    - deixa passar exatamente um de dois escritores concorrentes (KRN-06), com barreira e prazo de 5 s. Ela substitui os `provider/concurrency_test.go` de `orders` e `bookings` e o `TestConcurrentSaveReservation` (G9).
  - A `memory` serializa as transações por desenho e declara `Concurrent: false`, então o `providerkit/memory_test.go` passa a esperar um pulo. O `postgres/conformance_test.go` declara `Concurrent: true`.
  - `withRepo`, `seed` e `load`, que `e2e_test.go` e `order_reader_test.go` também usam, viram helpers genéricos em `testkit/tb/pg`. Sem isso, o G5 fica de pé.
- [x] **[P2] P8 — Handler gRPC de comando**: `command[R, Resp]` fica local a cada `app/rpc`, e os sete handlers de comando passam a usá-lo (dois no `orders`, dois no `reservations` e três no `bookings`); os de consulta ficam de fora.
  - O template não recebe o `command`: o `service.go__tmpl__` não tem handlers, e o `unused` do `golangci-lint`, que o `tools/dmpf-generator-check.sh` roda, reprovaria a função. O molde fica na skill `dmpf-bounded-context`.
- [x] **[P2] B9 — Clientes gRPC tipados**: `Unary[Req, Resp]` e `unaryOf` vão para `bff/app/rpc/clients.go`. O nome `Call` colidiria com o `rpc.Call` que já existe, a metadata do salto. `OrdersClient` e as demais interfaces viram structs de campos func, e a política de retry continua declarada por método.
- [x] **[P2] T11a — Dublês de teste do `bff`**: `fakeContexts` e o restante de `bff/app/api/testing_test.go` e `bff/app/rpc/testing_test.go` passam para um helper por package de teste, sobre `kernelgrpc.Unary` e `kernelgrpc.Method`. O dublê de `bff/app/degradation_test.go` entra no helper do package `app`.
  - Um package compartilhado de dublês seria código de produção para o verificador e exigiria unidade e `--write-baseline`, por isso fica para a Onda 3.
- [x] **[P2] N4 — Cobertura do descritor gRPC**: `kernelgrpc.Uncovered(sd *grpc.ServiceDesc, desc protoreflect.ServiceDescriptor) []string` vai para o kernel `grpc`, e os `app/rpc/service_test.go` passam a chamá-la (G53). O teste prova que todo método declarado está registrado exatamente uma vez, o complemento do que P2 garante, e usa o entrypoint `protoreflect` que P2 declara no manifesto.

#### Onda 3 — ADR e classificação

- [ ] **[P0] ADR de produção (próximo número livre, hoje ADR-059)**: registra P3, P6, A6 e B7. Supersede em parte o ADR-048: a frase "fica no contexto: `classify`, `subject`" e a leitura de "diverge por papel" aplicada ao corpo de cada papel. Também atualiza o status do ADR-048 e o índice em `docs/adr/README.md`.
  - Atualiza ainda o ADR-053 nas §7 e §14, que descrevem um `errors.go` por contexto e emitido pelo generator. O P1 retira esse arquivo já na Onda 1, e o PR dessa onda registra a deriva até o ADR.
- [ ] **[P0] ADR de testes (próximo número livre, hoje ADR-060)**: registra T1 e T3, supersede em parte o ADR-046 (o "sem segundo consumidor" do `distkit`) e atualiza o status dele e o índice.
- [ ] **[P1] P3 — `CategoryOf` e `SubjectOf`**: `application.CategoryOf(err) string`, que devolve vazio quando não classifica, e `ports.SubjectOf(ctx) string`. Saem `classify` e `subject` dos três `app/telemetry.go`.
- [ ] **[P1] P6 — Package `app/serve`**: package novo `libs/backend/go/app/serve` com `API`, `ServeAPI` e `RunRelay`, usado pelos três `serveAPI` e `runRelay`.
  - `RunWith` continua local.
  - O `go.mod` do `app` passa a requerer `transport`; o `grpc` já entrou com P1.
  - Unidade nova em `libs/backend/go/app/dmpf-units.json`, no mesmo molde de `kernel/app-relay`, e `kernel/app-serve` entra em `shared_kernel_units` do baseline:
    `{"id": "kernel/app-serve", "block": "app", "bounded_context": "kernel", "include": ["github.com/mateusmacedo/dmpf/libs/backend/go/app/serve"], "public_integration_surface": false}`.
- [ ] **[P1] A6 — `Instant` no kernel `domain`**: `type Instant int64` vai para `domain`, e `ports.Instant` vira alias. Saem as três declarações dos contextos e as sete conversões `domain.Instant(identity.OccurredAt)`.
- [ ] **[P2] B7 — `kernelgrpc.Category`**: `Category(err error) string` passa a ser exportada, com os casos `deadline.ErrNoDeadline` e `ErrDeadlineExhausted`, e `bff/app/rpc/errors.go` passa a usá-la.
- [ ] **[P1] T1 — Núcleo do `distkit`**: package novo `testkit/tb/dist`, em duas camadas.
  - **Supervisor genérico:** `Supervise(t testing.TB, name string, cmd *exec.Cmd) *Process`, com `Process.Wait(t, timeout)` e `Process.Stop(t, timeout)` (SIGTERM, espera e kill ao fim do prazo). Também tem `Process.Output()`, que devolve um snapshot do que o filho escreveu até ali, sob mutex, sem bloquear, no molde de `bff/app/harness_test.go`. Inclui ainda o `t.Cleanup` que encerra o filho.
  - **Harness por papel:** `Spec`, `New`, `Harness.Start(t, role)`, que monta o `*exec.Cmd` de reexecução do binário de teste e chama `Supervise`, além de `RunRole` e `RelayVector`.
  - Os três `<ctx>/distkit` ficam finos: `Spec`, papéis e, no `reservations`, `Plan` e `DMPF-R004`.
  - O `reservations` passa a provar `DMPF-P001` a `P003` do próprio relay.
  - O `go.mod` do `testkit` passa a requerer o franz-go, com entradas `external` para `github.com/twmb/franz-go/pkg/kgo` e `github.com/twmb/franz-go/pkg/kadm` em `libs/backend/go/testkit/dmpf-units.json`.
  - Unidade nova no mesmo manifesto, no molde de `kernel/testkit-tb`:
    `{"id": "kernel/testkit-dist", "block": "app", "bounded_context": "kernel", "include": ["github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"], "public_integration_surface": true}`.
  - `testkit/README.md` e `.agents/skills/dmpf-testkit/references/package-routing.md` deixam de dizer que `appkit` e `distkit` só existem no `reservations`.
  - `dist.CreateTopics` é exportada e usada também pelos três `app/wiring_e2e_test.go` (G11).
- [ ] **[P2] T11b — Supervisor do `bff`**: `process` e `start` de `bff/app/harness_test.go` passam a usar `dist.Supervise`. O `bff` continua montando o próprio `*exec.Cmd` (binário, argumentos e ambiente) e mantém o despejo de logs quando o teste falha.
- [ ] **[P1] T3 — Package `testkit/obstest`**: package novo com `Subject[C, R]`, `Manifest[C, R]` e `StartCollector`. Os `telemetry_manifest_test.go`, `telemetry_tenant_test.go`, `telemetry_internal_test.go` e `subject_test.go` de cada contexto viram uma chamada, e o `bff` passa a usar `StartCollector`.
  - O `go.mod` do `testkit` promove `google.golang.org/grpc` e `go.opentelemetry.io/proto/otlp` a requisitos diretos, com entradas `external`. O `otelboot` não é dependência nova, porque vive no módulo `observability`, que o `testkit` já requer.
  - Unidade nova em `libs/backend/go/testkit/dmpf-units.json`:
    `{"id": "kernel/testkit-obs", "block": "app", "bounded_context": "kernel", "include": ["github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"], "public_integration_surface": true}`.
  - Entram também os `telemetry_fields_test.go` dos contextos (`configWithSecrets`, `processConfigured` e a asserção da allowlist por papel, com o mapa de chaves local), e o `bff` passa a usar a verificação de manifesto e o `processConfigured` do `obstest` (G21, G50, G55, G67).
  - Entra ainda o teste da trilha de auditoria do wiring (`TestTheAuditTrailOfTheServiceReachesTheLoggerProviderOfTheRuntime`, nos três `app/wiring_test.go`). Ele vira grupo de clone quando o P4 tira dali o teste de `RelayConfig` (G62).
- [ ] **[P1] T6 — Fixture de `ExecutionContext`**: `WithExecution(t, ctx, opts...)`, `WithKey(t, ctx, key)`, a opção `WithSlot()` e uma opção `AutoKey()` vão para `testkit/tb/execctx`, package irmão do `tb`. Os 25 `_test.go` que montam o literal `ExecutionContextSpec` (24 nos contextos e 1 no `bff`) passam a usá-los.
  - O `tb` não recebe a fixture: ela importa `ports`, e o guard `TestDomainTestsNeedNoInfrastructureDouble` (V29/V30) reprova unidade `domain` cujo fechamento de teste alcance `port`. É o motivo pelo qual o pool Postgres vive em `tb/pg` (ADR-040).
  - O package novo entra no `include` de `kernel/testkit-tb` e exige `--write-baseline` (`DMPF-T001`), por isso o item fica na Onda 3. O nome `execctx` evita a sombra das variáveis locais `execution` dos testes.
  - `AutoKey()` gera a chave `k-<n>` por processo, e os wrappers locais ficam com uma linha. Sem ela, os `execution_test.go` continuam com o contador de chaves e viram dois grupos de clone novos.
  - Os `execution_test.go` do kernel (`memory`, `observability/resilience`, `observability/usecase` e `testkit/serviceskit`) também passam a usar o `execctx`, para não virarem par de clone dele (G15, G36). O módulo `observability` passa a requerer `testkit`, só por package `_test` externo.
  - Os `app/authorization_test.go` usam a fixture sem opção (G54), porque o padrão já não concede permissões. Uma opção sem permissões seria redundante.
- [ ] **[P2] N6 — Caso de uso de referência do kit**: o `counterService` vai para um arquivo não-teste de um package de bloco `app` do `testkit` (package novo em `kernel/testkit-tb` ou unidade nova). `observability/usecase/counter_service_test.go` e `testkit/serviceskit/counter_service_test.go` passam a usá-lo (G44).
  - O `serviceskit` não recebe o fixture: ele é bloco `provider`, e o `counterService` usa `application.Authorize`, `application.Outcome` e `application.ResolveIdentity`, o que viola o `DMPF-D001`.
  - O destino exige `include` novo ou unidade nova e `--write-baseline`, por isso o item fica na Onda 3.
  - A cópia do `usecase` é um superconjunto da do `serviceskit` (instrumentação, `Audit` e `Find`); o fixture compartilhado trata `Instrumentation` nula como `ports.NoInstrumentation()`.
  - O módulo `observability` passa a requerer `testkit`, como já fazem `memory`, `postgres`, `contracts` e `app`. O uso fica restrito a package `_test` externo.
- [ ] **[P2] N8 — Coletores OTLP de falha**: `UnreachableCollector(t)` e `StallingCollector(t)` vão para `testkit/obstest` e são usados pelos testes de degradação de `bff`, `grpc`, `kafka` e `sqs` (G47, G59).
  - Depende de T3 e do ADR de testes: o SDK OTel (`sdk/metric`, `sdk/log` e `sdk/trace`) vira requisito direto do `testkit`, e `kafka`, `sqs`, `grpc` e `bff` passam a requerer `testkit`.
- [ ] **[P2] N9 — Leitores de telemetria de teste**: `testkit/obstest` ganha `MetricLabels(t, reader)`, o exportador de log em memória, a espera de span por prefixo, o runtime com leitor de métrica e a contagem de conexões do pool (G23, G46, G51, G66).
  - O N9 tem as mesmas dependências do N8. Para G51 e G66, a alternativa é P6 levar os e2e de papel para `app/serve`.
- [ ] **[P2] N10 — Handler gRPC de comando no kernel**: o `command[R, Resp]` que o P8 deixou local nos três `app/rpc` forma grupo de clone no fim da Onda 2 (14 linhas em cada contexto). Promovê-lo exige rever a decisão técnica de manter o `executionOf` no handler (P2) e tratar o tipo de rejeição de cada contrato; a decisão e o destino ficam para esta onda.

#### Transversais (toda onda)

- [ ] **[P0] Generator e skill**: em cada onda que promove algo ao kernel:
  - retirar a cópia de `tools/dmpf-plugin/src/generators/bounded-context/files/`;
  - ajustar `tools/dmpf-generator-check.sh`;
  - atualizar `.agents/skills/dmpf-bounded-context/` e `.claude/commands/dmpf-new-context.md`.
- [ ] **[P0] Evidência e baseline**: medir os digests de evidência por subject antes e depois de cada onda e registrar, no PR (e no ADR, na Onda 3), quais divergem e por quê, como fez o ADR-048. Quando houver unidade nova, o `--write-baseline` e o `biome format` de `tools/dmpf-baseline/units-baseline.json` ficam com a pessoa responsável.
- [ ] **[P1] SPEC-VZ16X0MS revista**: registrar na spec deferida, junto da Onda 1, que `enqueueAll` deixa de ser template mecânico e passa a ser importado de `usecase.Enqueue`. Junto da Onda 2, registrar o mesmo para os passos 4 a 7, que passam a vir de `Decide` com `Loader`.
- [ ] **[P1] READMEs dos módulos**: todo módulo do kernel que ganha API documenta essa API no próprio `README.md`.
- [ ] **[P0] Duplicação como oportunidade de abstração**: o catálogo do relatório não esgota o escopo. A duplicação estrutural medida também entra.
  - **Medição:** `go run github.com/mibk/dupl@v1.1.0 -t 100 -files`, sobre os `.go` de `apps/backend` e `libs/backend/go`, fora `contract/gen` e `contracts/gen`. A lista de arquivos vem de `fd -e go . apps/backend libs/backend/go -E gen`. A linha de base é de **67 grupos de clone**, medida em `689fbfce`, com código idêntico ao de `8f544bc0`. Desses, 45 cruzam áreas (15 em produção ou misto e 30 só em teste). Os outros 22 ficam dentro de uma área: 14 no kernel, todos em teste, e 8 nos apps.
  - **Ordem da lista:** a contagem oficial usa a ordem nativa do `fd`, e a meta de 34 grupos se mede nela. O `dupl` é sensível à ordem da lista, e na ordem nativa clones de arquivo inteiro ficam ocultos. Por isso cada registro traz também, como diagnóstico, uma medição com a mesma lista embaralhada por semente fixa (`random.seed(1)` do Python 3).
  - **Quando:** no início do plano de cada onda e ao fim dela, com o número de grupos registrado no PR.
  - **Classificação:** cada grupo que nenhum item do catálogo já cobre é classificado pelo critério do ADR-048, com as restrições desta spec.
    - **Igual por natureza:** vira requisito novo `N<n>` nesta spec, com padrão (Template Method, generics ou extração), destino e onda. Entra na primeira onda cuja natureza o comporta: a Onda 0 só recebe o que cai nos arquivos que ela corrige; a Onda 1, a extração local ou a função nova em package existente; um `N<n>` que dependa de um item entra na onda desse item; e o que exige ADR ou unidade nova vai para a Onda 3.
    - **Coincidência, divergência por papel ou vocabulário do contexto:** vai para "Escopo fora", com o motivo.
  - Os grupos que um item do catálogo já cobre são marcados com o ID do item no registro da onda e não geram requisito novo. Quando o item só elimina o grupo com escopo maior, a ampliação fica registrada no próprio item.
  - Um item do catálogo que, ao ser implementado, deixe o próprio grupo de clone de pé não conta como concluído.
  - **Registro do início da Onda 0:** 67 grupos em `ca804ba6`, numerados G1 a G67 na ordem da saída do `dupl`. São 36 cobertos por item do catálogo (13 deles com ampliação registrada no item), 21 nos requisitos N1 a N9 e 10 fora de escopo.
  - **Registro do início da Onda 1:** 67 grupos em `e946bf7f`. A ordem da saída mudou, então a numeração G desta spec continua a de `ca804ba6`, e o registro da onda cita cada grupo por arquivo e linhas.
    - T6 e N6 passam para a Onda 3, com os grupos deles.
    - O grupo que surge no teste de auditoria do wiring quando o G62 sai fica coberto pelo T3, com ampliação registrada no item.
    - A7, P7, B3, B4, B5, B6 e B8 não têm grupo no `-t 100`; a conclusão deles se prova por teste e pela busca das cópias nos critérios de aceite.
  - **Registro do fim da Onda 1:** 50 grupos. Saíram os 18 que os itens da onda cobrem (G3, G4, G12, G13, G26, G29, G30, G35, G39, G40, G42, G45, G48, G56, G58, G60, G61 e G62). Entrou um, o teste de auditoria do wiring nos três `app/wiring_test.go`, coberto pelo T3. Os demais seguem com os mesmos arquivos, só com outras faixas de linha.
  - **Registro do início da Onda 2:** 50 grupos em `b16188c6` na ordem nativa, e 67 com a lista embaralhada. A ordem da saída mudou de novo, então o registro cita cada grupo por arquivo e linhas e usa a numeração G de `ca804ba6` quando o grupo já existia.
    - Doze são cobertos por itens da onda: G2 (P8), G5 e G9 (T10), G7 e G10 (P5), G14 (P2), G18 e G22 (A1), G37 (T2), G43 (B1), G53 (N4) e G64 (B9). Vinte e oito ficam com itens da Onda 3 e dez estão fora de escopo. Nenhum gera requisito `N<n>`.
    - O G22 ganhou `orders/application/place_order.go` quando o D1 alinhou o `bookings`, e segue coberto pelo A1, com a ampliação.
    - Dois grupos nasceram do D2 na Onda 0 e não têm número em `ca804ba6`. Ambos ficam cobertos pelo T1, com ampliação: o `launch` dos três `distkit/harness.go`, que `dist.Supervise` substitui, e os três `distkit/output_test.go`, cujo teste de `Output()` vira um só em `tb/dist`.
    - Os 17 grupos que só a lista embaralhada mostra são clones de arquivo inteiro:
      - os de `contract/golden` ficam com o T2, que retira o `fixture_test.go` e o `golden_test.go`; as specs `order_placed_fixture` e `item_added_fixture` do `orders` continuam como espelho das do kernel, cuja fixture fica fora de escopo;
      - os de `distkit` (`verdict.go`, `verdict_test.go` e `roles.go`) ficam com o T1;
      - os de `app/telemetry_*_test.go` ficam com o T3, que precisa citar também o `telemetry_usecase_test.go`;
      - os de `app/wiring.go`, `wiring_test.go` e `wiring_e2e_test.go` ficam com o P6, o T3 e o N9;
      - `application/codecs.go` e `codecs_test.go` de `orders` e `reservations` ficam fora de escopo por vocabulário do contexto: os tipos de resposta são do domínio de cada um, e o teste fixa o layout persistido do outcome (ADR-056).
  - **Registro do fim da Onda 2:** 41 grupos na ordem nativa, e 57 com a lista embaralhada.
    - Saíram os 12 que os itens da onda cobrem: G2, G5, G7, G9, G10, G14, G18, G22, G37, G43, G53 e G64.
    - Entraram três. O `command[R, Resp]` dos três `app/rpc` é igual por natureza e vira o requisito N10 da Onda 3. O subject do `providerkit.Repository` por agregado, nos testes de repositório de `bookings` e `reservations`, fica fora de escopo por vocabulário do agregado (identificador, estado e marcador), no critério de G1 e G24. As specs golden de `BookingCancelled` e `ResourceRegistered`, eventos de mesma forma, ficam fora de escopo por vocabulário do contrato.
    - Na medição, três clones nascidos da própria onda saíram por mudança local: os handlers gRPC de comando passam a recusa e o aceite como funções nomeadas, os três specs golden do `bookings` compartilham o construtor da fixture, e o teste de defaults dos `config_test` compara a struct `Policies`.

### Não-funcionais

- [ ] Compatibilidade: Go `1.26.6`, a diretiva do `go.mod` dos apps.
- [ ] Dependências: o `go.mod` do `testkit` só ganha como requisitos diretos novos o franz-go (T1), `google.golang.org/grpc` e `go.opentelemetry.io/proto/otlp` (T3) e o SDK OTel (`sdk/metric`, `sdk/log` e `sdk/trace`, N8 e N9), todos autorizados pelo ADR de testes. Entre módulos do kernel, `app` passa a requerer `grpc` (P1) e `transport` (P6); `kafka` e `sqs` (N7, Onda 1) e `observability` (T6 e N6, Onda 3) passam a requerer `testkit`, assim como `grpc` (N8). O `bff` já requer `testkit`. O `dupl` roda por `go run` e não entra em nenhum `go.mod`.
- [ ] Reflexão: o caminho de requisição não ganha nenhum uso novo de reflexão; os esqueletos usam generics e funções.
- [ ] Redução: ao fim da Onda 3, **a soma dos quatro apps** tem ao menos 1.000 linhas de produção e 2.500 linhas de teste a menos que a árvore de `8f544bc0`. Produção são os `.go` que não terminam em `_test.go`, e teste são os `_test.go`. Os dois excluem `contract/gen`. A contagem é física, por quebra de linha (`wc -l`), e é registrada no PR da Onda 3.
- [ ] Duplicação: ao fim da Onda 3, a medição do `dupl` acusa no máximo 34 grupos de clone, metade da linha de base de 67. Todo grupo restante está classificado como fora de escopo, com motivo.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| Kernel `domain` e `ports` | [x] | `Instant`, `DecideOver`, `Refuse`, `AuthorizationResult` e `SubjectOf` |
| Kernel `application` | [x] | `Execute`, `Enqueue`, `Loader`, `Decide`, `Query`, `Outcome.Category` e `CategoryOf` |
| Kernel `provider` (`grpc`, `postgres`, `kafka`, `transport/channel`, `observability/redact`) | [x] | `StatusOf`, `Unary`, `APIEnv`, protocolo de hop, `Category`, `SnapshotTable`, `ClientAuth.Missing`, `NewCatalog` e `WithoutValues` |
| Kernel `app` (`app`, `app/relay`, `app/serve`) | [x] | `Purge*`, `Policies`, `Instrument`, `ServeAPI` e `RunRelay` |
| Kernel `testkit` (`tb`, `tb/pg`, `tb/dist`, `obstest`, `serviceskit`, `domainkit`) | [x] | Fixtures, asserções, suíte golden, leitores, núcleo do `distkit`, suíte de telemetria e dublês |
| Contextos (`orders`, `reservations`, `bookings`) | [x] | Consomem os esqueletos e removem as cópias, nos blocos `domain`, `application`, `provider`, `app`, `app/rpc`, `appkit`, `distkit` e `contract/golden` |
| `bff` | [x] | Handler REST, rotas, chave de idempotência, access log, wiring, clientes gRPC e helpers de teste |
| Tooling | [x] | Templates do generator, scripts `tools/dmpf-*-check.sh`, manifestos e baseline |
| Docs | [x] | Dois ADRs, status dos ADRs 046 e 048, READMEs, skills `dmpf-bounded-context` e `dmpf-testkit` e a SPEC-VZ16X0MS |

## Localização de código

```text
libs/backend/go/
  domain/                 — Instant (A6), DecideOver e Refuse (A5)
  ports/                  — alias Instant (A6), AuthorizationResult (A7), SubjectOf (P3)
  application/            — Execute (A1), Enqueue (A2), Loader e Decide (A3), Query (A4), Outcome.Category (A7), CategoryOf (P3)
  grpc/                   — StatusOf (P1), Unary, MethodNames e FullMethod (P2), APIEnv (P5), MetadataCarrier, NewID e Valid* (B3), Category (B7)
  kafka/                  — ClientAuth.Missing (P5)
  postgres/               — SnapshotTable (P7)
  transport/channel/      — Catalog.AddressOf, NewCatalog (P4)
  observability/redact/   — WithoutValues (B6)
  app/                    — Purge* (P4), Policies (P5), teste de contrato de StatusOf (P1)
  app/relay/              — Instrument (P4)
  app/serve/              — novo (P6)
  testkit/tb/             — Require* e RunProjection (T7), GoldenSuite (T2)
  testkit/tb/execctx/     — novo (T6): WithExecution, WithKey, WithSlot e AutoKey
  testkit/tb/pg/          — Outbox, Settled, Counts (T8)
  testkit/tb/dist/        — novo (T1): Supervise e Process, Spec, Harness, RunRole e RelayVector
  testkit/obstest/        — novo (T3)
  testkit/serviceskit/    — UnitOfWork[R], Tx.CommandInbox, Authorize (T4)
  testkit/domainkit/      — Verdict.Merge (T7)
apps/backend/
  orders/ reservations/ bookings/  — consumidores dos esqueletos
  bff/app/api/            — B1, B2, B4, B5, B8, T11a
  bff/app/rpc/            — B3, B7, B9, T11a
  bff/app/wiring.go       — B6, B8
tools/dmpf-plugin/src/generators/bounded-context/files/  — templates do contexto
.agents/skills/dmpf-bounded-context/, .agents/skills/dmpf-testkit/, .claude/commands/dmpf-new-context.md
docs/adr/                 — ADR de produção, ADR de testes, status dos ADRs 046 e 048, índice
docs/specs/SPEC-VZ16X0MS-dmpf-generator-templates-agregado.md
```

**Arquivos a remover**:

- `{orders,reservations,bookings}/app/rpc/errors.go` e `errors_internal_test.go`
  (P1), substituídos por `kernelgrpc.StatusOf`;
- `tools/dmpf-plugin/src/generators/bounded-context/files/app/app/rpc/errors.go__tmpl__`
  (P1), pelo mesmo motivo.

Os `*/domain/helpers_test.go` ficam: o T7 retira deles só as asserções que
`tb.Require*` substitui.

Os demais itens alteram arquivos existentes; os caminhos de cada um estão nos
requisitos acima e, com a linha, no relatório (§4).

## Design

### Arquitetura

O kernel recebe esqueletos: funções livres genéricas que fixam a ordem dos
passos normativos e recebem os pontos de variação como função ou valor. O
contexto declara só os ganchos, ou seja, a operação, o comando, o codec, o
corpo de `Run`, o `Loader` e a decisão de domínio.

```text
contexto (ganchos)                             kernel (esqueleto)
Command{Operation, Input, Codec, Run}    →    application.Execute
  Run → Decide(repo, outbox, origin, id, Loader, decide)
          Loader = OrNew | Existing | Absent  (o contexto escolhe)
          Enqueue(outbox, id, origin, written, events)
app/rpc: handler                         →    kernelgrpc.Unary → StatusOf(err)
app: RunWith (local)                     →    serve.ServeAPI | serve.RunRelay
<ctx>/distkit: Spec{Pool, AppEnv, Roles} →    testkit/tb/dist.RunRole
```

### Fluxo do comando (A1 com A3)

```text
Execute(ctx, executor, command):
  op := BeginOperation(ctx, command.Operation)
  if err := executor.Authorize(ctx, command.Input) falha:
      end(op, AuthorizationResult(err)); return err
  identity := ResolveIdentity(ctx, executor.Clock, executor.IDs)
  outcome, replayed, err := executor.UoW.Within(ctx, res →
      RunIdempotent(executor.Inbox(res), executor.Consumer, command.Fingerprint, command.Codec,
                    () → command.Run(ctx, res, identity)))
  end(op, outcome.Category(), err)
  if err == nil and not replayed: Audit(ctx, command.Object, outcome)
  return outcome, err

command.Run(ctx, res, identity):
  return Decide(res.Repo, res.Outbox, origin, id, identity, loader, aggregate → aggregate.Decide(...))
    Decide: load → rejeição vira Rejected → Save(stored) → Enqueue(stored+1) → Accepted
```

### Ordem de entrega

- As ondas seguem a ordem 0 → 1 → 2 → 3, e cada onda é uma branch e um PR para
  `develop`.
- O plano de cada onda começa pela medição do `dupl` e pela classificação dos
  grupos ainda não cobertos, que pode acrescentar requisitos `N<n>` à onda ou
  às seguintes, pela regra do requisito transversal. A onda termina com uma
  nova medição registrada no PR.
- Dentro de uma onda, a ordem é: dependências, kernel, contextos, `bff`,
  generator e skill e, por último, docs.
- Dependências entre itens:
  - A3 depende de A2;
  - A1 usa A7;
  - T9 depende de T8;
  - T11a vem depois de P2;
  - T11b depende de T1;
  - na Onda 3, os dois ADRs vêm antes de qualquer código.

## Decisões técnicas

- **Spec única, entregue em quatro ondas**: cada onda é uma sub-issue no
  Linear, com uma branch e um PR, o que respeita a regra de uma tarefa por
  branch e por plano sem fragmentar o catálogo.
  Alternativa descartada: uma spec por onda, por escolha do usuário.
- **Destino e padrão de cada item**: são os do relatório (§4); esta spec não
  reabre a análise.
- **Esqueletos como funções genéricas com ganchos por função**: Go não tem
  herança e método não aceita parâmetro de tipo.
  Alternativa descartada: interface com métodos abstratos, que exigiria um tipo
  por caso de uso.
- **`executionOf` continua no handler (P2)**: levá-lo ao esqueleto inverte a
  precedência entre `Internal` e `InvalidArgument` e deixa os `service_test`
  sem exercitar a checagem.
  Alternativa descartada: passo fixo do `Unary`, com a nova precedência fixada
  por teste.
- **Checagem por `protoreflect` no kernel (P2)**: o entrypoint é declarado no
  manifesto de `kernel/provider-grpc`.
  Alternativa descartada: `method()` local em cada contexto, que mantém três
  cópias.
- **T3 na variante `testkit/obstest`**: é a de maior ganho, cerca de 1.000
  linhas.
  Alternativa descartada: template do generator em `files/app/app/`, que evita
  o grafo novo, mas não remove as cópias existentes.
- **B7 incluído**: a categoria do span cliente do kernel para os dois erros de
  deadline passa de `_OTHER` à categoria FND-07. A decisão é registrada no ADR
  de produção.
- **Dois ADRs (produção e testes) em vez de um por item**: P3, P6, A6 e B7
  tocam a mesma frase do ADR-048 e a superfície pública do kernel; T1 e T3
  tocam o grafo do `testkit` e o ADR-046.
- **T11 dividido em T11a (Onda 2) e T11b (Onda 3)**: o relatório não posiciona
  T11 na seção 5. Os helpers dependem de P2, e o supervisor depende de T1.
- **`Output()` sem bloqueio (D2, T1)**: a Onda 0 copia o sentinela do
  `reservations`, e a T1 troca tudo por snapshot sob mutex, no molde do `bff`.
  Alternativa descartada: esperar `finished`, que trava os testes de papéis de
  longa duração e mudaria o comportamento do harness.
- **Supervisor genérico separado do harness por papel (T1, T11b)**: o
  `distkit` reexecuta o binário de teste por papel, e o `bff` lança binários
  arbitrários. Só o supervisor de `*exec.Cmd` serve aos dois.
  Alternativa descartada: retirar a T11b e manter o helper local do `bff`.
- **`StatusOf` transcrito do switch atual (P1)**: a precedência é observável
  quando um `Failure` categorizado envolve uma sentinela de outra linha, então
  a tabela da spec é o contrato e o teste de tabela, o oráculo.
- **Classificação das unidades novas**: `kernel/app-serve` segue
  `kernel/app-relay` (bloco `app`, superfície `false`, shared kernel), e
  `kernel/testkit-dist` e `kernel/testkit-obs` seguem `kernel/testkit-tb`
  (bloco `app`, superfície `true`, fora do shared kernel).
- **`dupl` como detector de duplicação**: compara a estrutura sintática,
  ignorando nomes de identificador, e por isso acha as cópias entre contextos
  que diferem só pelo nome do contexto. Roda por `go run` com versão fixa,
  sobre a árvore toda e sem fronteira de módulo.
  Alternativa descartada: o linter `dupl` do `golangci-lint`, que roda por
  módulo e não vê a cópia entre `orders` e `bookings`.
  O limiar de 100 tokens descarta trechos curtos, como laços de erro e
  literais de teste. A meta de 34 grupos é metade da linha de base, coerente
  com os 45 grupos que cruzam áreas e que o catálogo ataca.
- **Os `determinism_test.go` ficam (T7)**: eles duplicam o
  `domainkit.ReadTwice`, mas removê-los passa de 30 linhas, e o relatório pede
  confirmação à parte.

## Regras relacionadas

- `.claude/rules/dmpf-bounded-context.md` (forma do contexto, ADR-053)
- `docs/guides/dmpf-manifesto.md` (unidades, `include`, baseline)
- `docs/guides/dmpf-composicao.md` (composição e generator)
- `tools/dmpf-context-check.sh`, `tools/dmpf-generator-check.sh`,
  `tools/dmpf-harness-check.sh`, `tools/dmpf-shared-kernel-check.sh` e
  `tools/dmpf-cell-check.sh`
- SPEC-VZ16X0MS, SPEC-F308KDSF e SPEC-JJKWG4JP

## Verificação e testes

### Critérios de aceite

- [ ] Uma busca em `apps/backend` por `func statusOf`, `func outcomeCategory`, `func authorizationResult`, `func enqueueAll`, `func classify`, `func serveAPI` e `func runRelay` não retorna resultado.
- [x] `{orders,reservations,bookings}/app/rpc/errors.go` não existem.
- [x] Todo caso de uso de comando dos três contextos chama `Execute`, e o `bookings` audita comando novo sem auditar replay, o que é fixado por um teste por caso de uso.
- [ ] O `go test -race` do `distkit` de cada contexto passa com um teste que lê `Output()` enquanto o processo roda. Na Onda 0, a leitura devolve o sentinela; a partir da Onda 3, devolve o snapshot de `dist.Supervise`.
- [x] O teste de tabela de `kernelgrpc.StatusOf` cobre as oito linhas, `context.DeadlineExceeded` e `context.Canceled` puros e os erros híbridos.
- [x] Um teste fixa o pânico de `kernelgrpc.Method` para método inexistente.
- [x] As mensagens de erro dos casos de uso de comando seguem a forma única de A3: no `bookings` são idênticas às de `8f544bc0`, e em `orders` e `reservations` mudam só os 13 textos listados em A3. Cada caminho de falha é verificado por `errors.Is` e pela mensagem completa.
- [ ] `kernel/app-serve`, `kernel/testkit-dist` e `kernel/testkit-obs` estão nos manifestos com os campos desta spec, e `kernel/app-serve` consta de `shared_kernel_units`.
- [ ] O registro de cada onda lista todos os grupos de clone do `dupl` como cobertos por um item, como novos requisitos `N<n>` ou como fora de escopo com motivo, e o registro da Onda 3 acusa no máximo 34 grupos.
- [x] `bookings/application/doubles_test.go` não declara `memStore`, `memTx`, `txCommands` nem `memUoW`.
- [ ] O contexto gerado por `tools/dmpf-generator-check.sh` não contém nenhuma das funções promovidas, e `tools/dmpf-harness-check.sh` passa.
- [ ] Ao fim de cada onda, `go run ./tools/dmpf-conformance/cmd/conformance --root .` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check` passam.
- [ ] `pnpm biome check .` e `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source` passam, assim como `fmt-check`, `vet`, `test-race` e `govulncheck` de cada projeto Go tocado.
- [ ] `tools/dmpf-context-check.sh`, `tools/dmpf-shared-kernel-check.sh` e `tools/dmpf-cell-check.sh` passam.
- [ ] `node tools/adr-verify.mjs` passa com os dois ADRs novos e os status dos ADRs 046 e 048 atualizados.
- [ ] O `reservations` prova `DMPF-P001` a `P003` do relay por meio de `testkit/tb/dist`.
- [x] O `bookings/contract` tem golden para os três protos de evento.
- [ ] O PR de cada onda registra as divergências de digest por subject, e o da Onda 3 registra a redução de linhas do requisito não-funcional.

### Cenários de teste

```text
DADO o bookings com um comando ReserveBooking cuja chave de idempotência é nova
QUANDO Execute conclui sem replay
ENTÃO Audit é chamado uma vez, e o sequence_test registra o passo Audit depois de end

DADO o mesmo comando reenviado com a mesma chave
QUANDO Execute encontra o registro na inbox de comandos
ENTÃO o outcome gravado é devolvido, e Audit não é chamado

DADO um erro sem categoria e fora de ports.Err* e de context
QUANDO o handler gRPC o converte com kernelgrpc.StatusOf
ENTÃO o status é codes.Internal com a mensagem "internal failure"

DADO context.DeadlineExceeded puro, que não vem envolto em Failure
QUANDO passa por kernelgrpc.StatusOf
ENTÃO o status é codes.DeadlineExceeded com a mensagem "deadline exceeded"

DADO um application.Failure com categoria Conflict que envolve ports.ErrNotFound
QUANDO passa por kernelgrpc.StatusOf
ENTÃO o status é codes.NotFound, porque vale a linha de menor número, como no statusOf de 8f544bc0

DADO Absent como Loader e um booking que já existe no repositório
QUANDO o caso de uso ReserveBooking chama Decide
ENTÃO errors.Is(err, ports.ErrAlreadyExists) é verdadeiro, a mensagem é "application: reserve <id>: <texto da sentinela>" e não há Save nem Enqueue

DADO um Enqueue que falha depois de um Save bem-sucedido
QUANDO Decide devolve o erro ao caso de uso
ENTÃO a mensagem é "application: reserve <id>: enqueue: <causa>", idêntica à de 8f544bc0

DADO uma rota GET recebendo um cabeçalho Idempotency-Key inválido
QUANDO requireIdempotencyKey(route, next) avalia a requisição
ENTÃO a resposta é 400, como antes de B4

DADO um processo filho do distkit que ainda escreve no stdout
QUANDO o teste lê Process.Output()
ENTÃO a leitura retorna sem bloquear (sentinela na Onda 0, snapshot parcial a partir da Onda 3), e go test -race não acusa corrida

DADO a medição do dupl ao fim de uma onda
QUANDO um grupo de clone não é coberto por nenhum item do catálogo
ENTÃO o registro da onda o classifica como requisito N<n> nesta spec ou como fora de escopo com motivo, e nenhum grupo fica sem classificação
```

<critical_constraints>
- [P0] NUNCA criar dependência de `provider` para `application` (`DMPF-D001`) nem entre contextos (`DMPF-D002`); o destino de cada item é o que o relatório indica em §4.
- [P0] NUNCA subir ao kernel o que é igual por coincidência ou diverge por papel (critério do ADR-048); os candidatos da §6 do relatório ficam fora.
- [P0] O comportamento observável é preservado, com exatamente três exceções declaradas: o `bookings` passa a auditar (D1, A1), a categoria do span cliente do kernel para `deadline.ErrNoDeadline` e `ErrDeadlineExhausted` deixa de ser `_OTHER` (B7), e 13 textos de falha de `orders` e `reservations` passam à forma única de A3, só em log e span.
- [P0] Nenhum item da Onda 3 entra em código antes de o ADR que o autoriza estar commitado.
- [P0] Unidade nova exige `include` no manifesto (`DMPF-U001`); o agente prepara a unidade, e o `--write-baseline` (`DMPF-T001`) fica com a pessoa responsável.
- [P0] Toda promoção ao kernel retira a cópia dos templates de `tools/dmpf-plugin/src/generators/bounded-context/files/` e da skill `.agents/skills/dmpf-bounded-context/` na mesma onda.
- [P1] Teste de comportamento só sai com cobertura equivalente no destino; alterar teste apenas para fazê-lo passar é proibido.
- [P1] Nomes de variável de ambiente, chaves de atributo de telemetria, layout de payload persistido e valores do fio não mudam (ADR-053, ADR-056).
- [P1] Esqueleto genérico é função livre (método não aceita parâmetro de tipo); alias genérico é permitido (`go 1.26.6`).
- [P1] Projetos Nx distintos vão em commits separados.
</critical_constraints>

## Escopo fora

- **Candidatos descartados da §6 do relatório**: cada um já tem o motivo
  registrado lá, seja abstração sem cliente, vocabulário do contexto, contrato
  publicado ou divergência por papel.
- **Remoção dos `determinism_test.go`**: pede confirmação à parte (ver
  Decisões técnicas).
- **Fixture golden de `libs/backend/go/contracts`**: os dados da fixture do
  kernel seguem como estão; só o teste adota a `GoldenSuite` (T2, G37).
- **Retomada da SPEC-VZ16X0MS**: só a revisão do texto entra aqui; a retomada
  é decisão da própria spec.
- **Esqueleto de handler no `kernelhttp`**: o `bff` é o consumidor único
  (ADR-044); a reavaliação fica para quando surgir uma segunda borda REST.
- **Parecer de event sourcing (DEVS-61)**: é uma frente separada, com o próprio
  documento de design.
- **Contexto novo ou mudança de contrato**: nenhum `.proto`, OpenAPI ou tabela
  muda de forma.
- **`dupl` como gate de CI ou linter do `golangci-lint`**: a medição desta
  spec é registro por onda. Transformá-la em gate exige outra decisão, sobre
  limiar e exceções.
- **Grupos do `dupl` fora de escopo** (numeração da medição em `ca804ba6`):
  - **Vocabulário do contexto:** G1 e G24, construtores de `domainkit.Subject`
    por comando nos `domain/projection_test.go`; G6 e G33, cenários RES-16 e
    CTX-07/08 nos `app/rpc/chain_test.go` de `orders` e `reservations`, sobre
    o harness local que a §6 do relatório manteve; G31, chave reutilizada nos
    `app/rpc/idempotency_test.go` de `orders` e `bookings`, cujo teste genérico
    a §6 descartou.
  - **Divergência por papel:** G8 e G32, testes paralelos dos builders de
    rótulos de métrica (`observability/metrics/labels_test.go`) e de atributos
    de span (`observability/tracing/attributes_test.go`), cada um com regra
    própria; G16, `requirements()` dos `app/config.go`; G57, o teste do
    `requirements()` do relay nos `app/config_test.go`.
  - **§6 do relatório:** G19, os `cmd/main.go` dos três contextos, porque o
    `Main` genérico foi descartado.
