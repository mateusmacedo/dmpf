# Oportunidades de abstração nos apps Go

Relatório de análise, **não normativo**. Levanta onde os apps Go do workspace
repetem estrutura que Template Method, generics ou extração para uma abstração
eliminariam — e onde a repetição deve ficar. As obrigações continuam nos ADRs e
em `docs/dmpf/`; nada aqui altera gate, norma ou código.

- **Data:** 2026-10-04.
- **Base:** commit `8f544bc0`. As referências `arquivo:linha` valem para esse
  commit e não são mantidas depois dele.
- **Escopo:** `apps/backend/{bff,orders,reservations,bookings}` — cerca de
  13,4 mil linhas de Go de produção (incluindo o gerado em `contract/gen/go`) e
  os testes. O kernel em `libs/backend/go` entra só como destino de uma
  promoção ou como abstração que já existe e não é usada.
- **Caminhos:** salvo indicação, `orders/…`, `reservations/…`, `bookings/…` e
  `bff/…` ficam sob `apps/backend/`; `grpc/…`, `application/…`, `testkit/…` e
  demais módulos do kernel ficam sob `libs/backend/go/`.

## 1. Resumo

A repetição entre os três contextos é maior do que o ADR-048 previu e já
produz deriva. Arquivos e funções inteiros são iguais a menos do nome do
contexto: `app/rpc/errors.go` (idêntico byte a byte), `serveAPI`, `classify` e
`subject`, o `distkit` inteiro de `orders` e `bookings` e 648 linhas de teste
de telemetria por contexto.

- **A cópia já custou correção.** O `bookings` não audita comandos, o `distkit`
  de `orders` e `bookings` lê a saída do processo filho sem sincronização, e os
  testes de idempotência do `bookings` correm sobre um store feito à mão, mais
  fraco que o `memory` do kernel (seção 3).
- **Maior ganho estrutural:** o esqueleto do comando no kernel `application`
  (A1, com A2 e A3), o mapeamento de erro e o registro de métodos no
  `kernelgrpc` (P1, P2) e os kits de teste (T1 a T4).
- **Duas frases dos ADRs perderam a premissa.** O "sem segundo consumidor" do
  ADR-046 não vale mais para o `distkit`, e o "fica no contexto: `classify`,
  `subject`" do ADR-048 é desmentido pelo diff. Reabrir pede ADR, não
  refatoração direta (P3, P6, T1).
- **O generator precisa acompanhar.** Tudo o que subir ao kernel tem de sair
  também dos templates de `tools/dmpf-plugin` e da skill
  `dmpf-bounded-context`; senão o próximo contexto nasce copiado de novo.

Ordem de grandeza, pelas estimativas dos diffs e sem descontar a sobreposição
entre achados: cerca de 1,5 mil linhas de produção e 3,5 mil linhas de teste.

## 2. Método e critério

**Como foi medido.** Arquivos equivalentes dos contextos foram comparados por
`diff` depois de normalizar o nome do contexto (`orders`, `reservations` e
`bookings` viram um marcador). A análise correu em quatro frentes —
`domain`/`application`, `provider`/`app`/`app/rpc`, `bff` e kits/testes —, e
os achados de maior peso foram conferidos de novo no código; a coluna
"Conferido" da seção 4 marca quais.

**Critério de destino (ADR-048).** Sobe ao kernel o que é **igual por
natureza** — imposto por uma norma do kernel, como os nove passos de FND-04
§3.2, a taxonomia de ERR-20 ou a idempotência de RST-02. Fica no contexto o que
é **igual por coincidência** — mesma forma hoje, evolução independente, como o
regex de identificador de cada contrato — ou o que **diverge por papel**.

**Restrições que toda sugestão respeita:**

- `libs/` só guarda kernel de reuso, e abstração sem cliente não entra
  (ADR-046).
- A regra de dependência é por bloco, não por módulo: `provider` não importa
  `application` (`DMPF-D001`). Foi o que reverteu a primeira promoção de
  `classify` e `subject` (ADR-048).
- Contextos não se importam entre si (`DMPF-D002`).
- Package de produção novo precisa de `include` no manifesto (`DMPF-U001`).
  Unidade nova é ato de classificação, com `--write-baseline` executado por
  pessoa (`DMPF-T001`).
- Os `require` entre módulos irmãos são sincronizados pelo `modsync`; uma
  promoção que puxe um módulo novo para o `go.mod` de outro muda o grafo.

**Generics em Go, na prática.** Método não aceita parâmetro de tipo, então os
esqueletos propostos são funções livres (`Execute[Res, Op, R]`), não métodos de
um tipo genérico. O `go.mod` dos apps declara `go 1.26.6`, que já tem alias
genérico: `type command[R any] = usecase.Command[Resources, Operation, R]`
encurta os argumentos de tipo no contexto.

## 3. Deriva já observada

São desvios entre cópias que deveriam ser iguais. Corrigi-los não depende de
nenhuma abstração, mas cada um mostra o custo de mantê-las à mão.

| # | Desvio | Evidência | Efeito |
|---|--------|-----------|--------|
| D1 | O `bookings` não audita comandos | `bookings/application/reserve_booking.go:31`, `cancel_booking.go:29` e `register_resource.go:30` descartam o `replayed` de `idempotent` e não chamam `Audit`. `orders/application/add_item.go:46`, `place_order.go:44` e `reservations/application/write.go:64` auditam | Escritas do `bookings` sem trilha de auditoria. O item "[P0] Alinhar a instrumentação do `bookings`" da SPEC-F308KDSF, Fase 5, previa a auditoria; a spec está em `stage: done`, a parte de negação chegou ao código e a de auditoria não |
| D2 | Leitura da saída do filho sem sincronização | `orders/distkit/harness.go:155` (e o mesmo no `bookings`) lê o `bytes.Buffer` direto; `reservations/distkit/harness.go:191-198` só lê depois de `finished` | Corrida com a cópia do stdout do processo filho, visível sob `-race` quando um teste falha e imprime a saída |
| D3 | Store de teste feito à mão no `bookings` | `bookings/application/doubles_test.go:51-211` (`memStore`, `memTx`, `txCommands`, `memUoW`) contra `memory.Table`, `NewUnitOfWork` e `tx.CommandInbox`, usados por `orders` e `reservations` | O `txCommands` ignora o escopo por tenant e a expiração que `memory/inbox.go:24-28` aplica: a idempotência do `bookings` é testada sobre uma semântica mais fraca que a certificada. `testOccurred` está em segundos (`:22`), e o `orders` usa nanossegundos |

## 4. Catálogo de sugestões

Visão geral. "Exige" indica o maior rito envolvido: **—** é função nova em
package existente; **docs** é atualizar README ou template; **comport.** muda
comportamento observável; **ADR** pede decisão registrada; **unidade** é ato
de classificação com baseline.

| ID | Padrão | Alvo | Destino | Ganho est. (linhas) | Exige | Conferido |
|----|--------|------|---------|---------------------|-------|-----------|
| A1 | Template Method + Generics | Esqueleto do comando | kernel `application` | −180 / +60 | comport., docs | sim |
| A2 | Extração | `enqueueAll` | kernel `application` | −90 / +30 | docs | — |
| A3 | Generics + Template Method | Passos 4 a 7 com `Loader` | kernel `application` | −125 / +55 | — | — |
| A4 | Template Method | Esqueleto de consulta | kernel `application` | −27 / +15 | — | — |
| A5 | Template Method | UPR por cópia | kernel `domain` | −25 | — | — |
| A6 | Extração | `Instant` | kernel `domain` + alias em `ports` | −10 | tipo publicado | sim |
| A7 | Extração | `outcomeCategory`, `authorizationResult` | kernel `application`/`ports` | −50 / +12 | — | — |
| P1 | Extração | `statusOf` | `kernelgrpc` | −110 (+170 de teste) | docs | sim |
| P2 | Generics + Template Method | `unary`, `method`, `Methods` | `kernelgrpc` | −190 | manifesto | sim |
| P3 | Extração | `classify`, `subject` | kernel `application`/`ports` | −85 | ADR | sim |
| P4 | Extração | Helpers de wiring e catálogo | `transport/channel`, `app/relay`, `app` | −100 | — | — |
| P5 | Extração + Template Method | Seções comuns de `Config` | `grpc`, `kafka`, `app` | −200 | docs | — |
| P6 | Template Method | `serveAPI`, `runRelay` | `app/serve` (novo) | −350 | ADR, unidade | sim |
| P7 | Generics | Repositório de snapshot | kernel `postgres` | −30 | — | — |
| P8 | Template Method | Handler gRPC de comando | local + template | −20 | — | — |
| B1 | Template Method + Generics | Handler REST | local ao `bff` | −100 | — | — |
| B2 | Template Method + Extração | Declaração de rotas | local ao `bff` | ≈0 (correção por construção) | — | — |
| B3 | Extração | Lado cliente do protocolo de hop | `kernelgrpc` | −43 | API pública nova | sim |
| B4 | Extração | `requireIdempotencyKey` | local ao `bff` | ≈0 | — | — |
| B5 | Template Method | Access log e resposta 500 | local ao `bff` | −20 | — | — |
| B6 | Extração (Strategy) | `withoutValues` | `observability/redact` | −18 | — | sim |
| B7 | Extração | `rpc.Category` | `kernelgrpc` | −25 | comport. | — |
| B8 | Extração | Tabela de contextos no wiring | local ao `bff` | −15 | — | — |
| B9 | Generics | Clientes gRPC tipados | local ao `bff` | −50 a −70 | — | — |
| T1 | Template Method + Extração | Núcleo do `distkit` | `testkit/tb/dist` (novo) | −380 líquidas | ADR, unidade | sim |
| T2 | Generics + Extração | Suíte golden de contrato | `testkit/tb` | −500 líquidas | — | sim |
| T3 | Generics + Extração | Manifesto de telemetria e coletor OTLP | `testkit/obstest` (novo) ou template | −1.000 líquidas | unidade ou docs | sim |
| T4 | Generics | Dublês de gravação e `UnitOfWork[R]` | `testkit/serviceskit` | −400 | — | sim |
| T5 | Extração | Store à mão do `bookings` | local, sobre `memory` | −170 | — | sim |
| T6 | Extração | Fixture de `ExecutionContext` | `testkit/tb` | −500 | — | — |
| T7 | Generics | Asserções de domínio | `testkit/tb`, `domainkit` | −150 | — | — |
| T8 | Extração | Leitores das tabelas do kernel | `testkit/tb/pg` + local | −130 | — | — |
| T9 | Extração | `e2e_test` reimplementa o `appkit` | local | −100 | — | — |
| T10 | Generics (já existente) | `providerkit.Repository` por agregado | local | −250 | — | — |
| T11 | Extração | Helpers de teste do `bff` | local ao `bff` | −100 | — | — |

### 4.1 `domain` e `application`

**A1 — Esqueleto do comando.** *Template Method + Generics · igual por
natureza.*

- **Evidência:** `orders/application/add_item.go:16-52` e `place_order.go:15-52`;
  `reservations/application/write.go:19-72`; `bookings/application/reserve_booking.go:13-42`,
  `cancel_booking.go:12-41` e `register_resource.go:13-41`. O wrapper
  `idempotent[R]` de `orders/application/service.go:90-110` e
  `bookings/application/service.go:88-108` só difere no alias do import
  (`ports` contra `port`).
- **Igual:** `BeginOperation` → `Authorize` → `ResolveIdentity` →
  `Within{RunIdempotent}` → `end` → `Audit` só sem replay.
- **Varia:** a operação, o comando, o fingerprint, o codec, o objeto da
  auditoria e o corpo de `Run`.
- **Observação:** o `reservations` já aplica o padrão localmente, com
  `decision[R]` e `write[R]` (`reservations/application/write.go:14-72`); `orders`
  e `bookings` copiam o mesmo esqueleto à mão.

```go
type Executor[Res, Op any] struct {
	UoW ports.UnitOfWork[Res]; Inbox func(Res) ports.Inbox; Consumer string
	Clock ports.Clock; IDs ports.IDGenerator; MaxEvents int; Policy IdempotencyPolicy
	Authorize Authorize[Op]; Instrumentation ports.Instrumentation
}
type Command[Res, Op, R any] struct {
	Operation, Object string; Input Op; Fingerprint *Fingerprint; Codec OutcomeCodec[R]
	Run func(ctx context.Context, res Res, id Identity) (Outcome[R], error)
}
func Execute[Res, Op, R any](ctx context.Context, x Executor[Res, Op], c Command[Res, Op, R]) (Outcome[R], error)
```

- **Destino:** arquivo novo no package `application` do kernel; não cria
  unidade.
- **Por que é natureza:** a ordem dos nove passos (FND-04 §3.2), o IDM-05 e a
  regra de auditar só sem replay são normas do kernel. Codificá-las na
  estrutura fecha a deriva D1.
- **Riscos e gates:** o `bookings` passa a auditar (mudança de comportamento
  desejada, D1). O mapa de passos de `application/README.md:125-140` aponta os
  arquivos do `orders` e precisa ser reescrito. Os digests de evidência de
  `reference` e `services` mudam, como no ADR-048. O retry continua gancho do
  caso de uso, não decorator da UoW (ADR-034, KRN-09). `DMPF-D001` não muda:
  `application` já importa `ports` e `domain`. Os `sequence_test.go` de cada
  contexto provam que a ordem foi preservada.

**A2 — `enqueueAll` vira `usecase.Enqueue`.** *Extração · natureza.*

- **Evidência:** `orders/application/service.go:143-177` e
  `reservations/application/consume.go:104-134` são iguais sem os comentários;
  `bookings/application/service.go:138-168` só parametriza tipo e id.

```go
type Origin struct{ Destination, AggregateType, AggregateID string }
func Enqueue(ctx context.Context, outbox ports.Outbox, id Identity, o Origin,
	written ports.Version, events []domain.DomainEvent) error
```

- **Por que é natureza:** só usa tipos do kernel e preenche os sete campos de
  FND-04 §2.3 (BLK-04, BLK-05).
- **Risco:** a SPEC-VZ16X0MS (`stage: deferred`) planejava `enqueueAll` como
  template "mecânico" do generator. Gerar o código por template recria as três
  cópias; a decisão da spec precisa ser revista antes de retomá-la.

**A3 — Passos 4 a 7 genéricos, com `Loader` como estratégia.** *Generics +
Template Method · natureza.* Depende de A2.

- **Evidência:** `orders/application/add_item.go:56-97` e `place_order.go:54-75`;
  `reservations/application/write.go:74-108` (e `consume.go:62-71`, que repete o
  `loadOrCreate`); `bookings/application/reserve_booking.go:44-71`,
  `cancel_booking.go:42-64` e `register_resource.go:43-76`.
- **Igual:** `Load` → rejeição vira `Rejected` → `Save(stored)` →
  `Enqueue(stored+1)` → `Accepted`.
- **Varia:** o modo de carga — cria ou carrega, só existente, só ausente —, os
  mesmos três modos que a SPEC-VZ16X0MS descreve.

```go
type Loader[ID comparable, S, A any] func(context.Context, ports.Reader[ID, S], ID) (A, ports.Version, error)
func OrNew[ID comparable, S, A any](fresh func(ID) A, from func(S) A) Loader[ID, S, A]
func Existing[ID comparable, S, A any](from func(S) A) Loader[ID, S, A]
func Absent[ID comparable, S, A any](fresh func(ID) A) Loader[ID, S, A] // ErrAlreadyExists
func Decide[ID comparable, S any, A interface{ Snapshot() S }, R any](ctx context.Context,
	repo ports.Repository[ID, S], outbox ports.Outbox, o Origin, id ID, idn Identity,
	load Loader[ID, S, A], decide func(A) (domain.Accepted[R], *domain.Rejection)) (Outcome[R], error)
```

- **Por que é natureza:** é o contrato de `Repository` em
  `ports/repository.go:32-52`. A escolha do `Loader` fica no contexto, porque
  diverge por papel.
- **Riscos:** os prefixos de erro (`add item to`, `register`) passam ao
  chamador. O consumo só reaproveita o `Loader`, porque recusa com `LastError`
  e trata conflito como `Failure(Conflict)` (`reservations/application/consume.go:77-92`).

**A4 — Esqueleto de consulta.** *Template Method · natureza.*
`orders/application/find_order.go:14-31`, `reservations/application/find_reservation.go:14-31`
e `bookings/application/find_booking.go:11-28` são iguais depois de renomear os
identificadores. `FindBookingByResource`, com o ramo IDN-13, fica local.

```go
func Query[Op, S any](ctx context.Context, inst ports.Instrumentation, authz Authorize[Op],
	operation string, input Op, load func(context.Context) (S, error)) (S, error)
```

**A5 — UPR por cópia no domínio.** *Template Method · natureza.* Sete UPRs
repetem `next := x.clone()` … `*x = next` (`orders/domain/{add_item,place}.go`,
`reservations/domain/{reserve,cancel}.go`, `bookings/domain/{reserve,cancel,resource}.go`),
e doze pontos montam `kernel.Accepted[X]{}, kernel.Reject(...)`. O ganho de
linhas é pequeno; o valor é tornar DEC-10/11 estrutural. O `clone` continua
privado e entra como valor de método, sem reabrir o ADR-034.

```go
func DecideOver[A, R any](target *A, copyOf func(*A) A,
	decide func(next *A) (Accepted[R], *Rejection)) (Accepted[R], *Rejection)
func Refuse[R any](code Code, msg string, d ...Detail) (Accepted[R], *Rejection)
```

**A6 — `Instant` no kernel `domain`.** *Extração · natureza.* `type Instant int64`
está em `orders/domain/messages.go:29`, `reservations/domain/messages.go:22`,
`bookings/domain/messages.go:11` e `ports/values.go:8`, com sete conversões
`domain.Instant(identity.OccurredAt)`. A SPEC-F308KDSF precisou "unificar a
unidade do instante", e D3 mostra que a deriva de unidade continua nos testes.
Proposta: declarar o tipo em `domain` e tornar `ports.Instant` um alias
(`ports` já importa `domain`). **Risco médio:** muda a declaração de um tipo
publicado do kernel; como `ports.Instant` não tem método, o alias preserva a
API. O fio não muda.

**A7 — `outcomeCategory` e `authorizationResult`.** *Extração · natureza.*
Iguais byte a byte em `orders/application/service.go:124-138`,
`reservations/application/instrumentation.go:19-31` e
`bookings/application/service.go:122-136`. Proposta: `func (o Outcome[R])
Category() ports.OutcomeCategory` no `application` e `func
AuthorizationResult(err error) Result` em `ports`, ao lado de `ErrDenied` — que
já declara que a negação nunca é inferida (ERR-11). A1 absorve este item; sozinho,
é a vitória mais barata do grupo.

### 4.2 `provider`, `app` e `app/rpc`

**P1 — `statusOf` vira `kernelgrpc.StatusOf`.** *Extração · natureza.*

- **Evidência:** `{orders,reservations,bookings}/app/rpc/errors.go` são idênticos
  byte a byte (39 linhas), assim como o template
  `tools/dmpf-plugin/src/generators/bounded-context/files/app/app/rpc/errors.go__tmpl__`
  e os `errors_internal_test.go` (57 linhas cada).
- **Por que não viola `DMPF-D001`:** o `kernelgrpc` (bloco `provider`) já
  reconhece a categoria pela forma, via `interface{ ErrorCategory() string }`
  (`grpc/classifier.go:56-63`), sem importar `application`; o
  `*application.Failure` satisfaz `redact.Categorized`
  (`application/failure.go:47`).

```go
// grpc/status.go, ao lado de IdempotencyStatus
func StatusOf(err error) error // IdempotencyStatus → ports.Err* → redact.Categorized → Internal
```

- **Riscos:** a comparação por string de categoria (`NotFound`, `Conflict`…)
  precisa de um teste de contrato no kernel `app`, que importa os dois módulos.
  A SPEC-JJKWG4JP cita o `errors.go` por contexto. O arquivo sai do template.

**P2 — `unary`, `method`, `Methods`, `FullMethod` e `executionOf` no
`kernelgrpc`.** *Generics + Template Method · natureza.*

- **Evidência:** `{orders,reservations}/app/rpc/service.go:23-118` e
  `bookings/app/rpc/service.go:28-127` (as quatro funções nos três), mais uma
  quarta cópia de `unary` em `bff/app/api/testing_test.go:115`.
- **Varia:** só o tipo do servidor e o descritor. Os onze handlers chamam
  `_, err = executionOf(ctx)` e descartam o valor: a checagem pode virar passo
  fixo do esqueleto.

```go
func Unary[S, Req any, PReq interface{ *Req }, Resp any](
	service, method string, call func(S, context.Context, PReq) (Resp, error)) grpc.MethodDesc
func MethodNames(desc *grpc.ServiceDesc) []string
func FullMethod(service, method string) string // hoje inline em WithCommands, MethodLimits e ownMethods
```

- **Riscos e gates:** manter a checagem por `protoreflect` no kernel exige
  declarar o entrypoint `google.golang.org/protobuf/reflect/protoreflect` no
  `external` de `kernel/provider-grpc` (merge no manifesto); a alternativa é
  deixar `method()` local. Levar `executionOf` ao esqueleto inverte a
  precedência entre `Internal` e `InvalidArgument`. Os `service_test` que chamam
  o servidor direto deixam de exercitar a checagem.

**P3 — `classify` vira `application.CategoryOf`, `subject` vira
`ports.SubjectOf`.** *Extração · natureza · contraria uma frase do ADR-048.*

- **Evidência:** `orders/app/telemetry.go:64-92` e
  `{reservations,bookings}/app/telemetry.go:28-56` são idênticos.
- **Evidência nova contra o ADR-048:** a reversão por `DMPF-D001` aconteceu
  porque o destino tentado era `observability/usecase`, bloco `provider`. O
  contrato do `obsusecase.Classifier` já diz que resposta vazia vira
  `CategoryUnclassified` (`observability/usecase/instrumentation.go:46-48,155-161`).
  Então `classify` pode viver no kernel `application` — que já importa `ports`
  e é bloco `application` — devolvendo `""` quando não classifica, sem importar
  `observability`. `subject` só usa `ports.ExecutionContextFrom`.

```go
func CategoryOf(err error) string          // kernel application; "" quando não classifica
func SubjectOf(ctx context.Context) string // kernel ports
```

- **Exige:** addendum ao ADR-048, porque a frase "fica no contexto: `classify`,
  `subject`" deixa de valer. Não há unidade nova. Um contexto com sentinelas
  próprias compõe por fora.

**P4 — Helpers de composição do wiring.** *Extração · natureza.* São iguais nos
três `app/wiring.go`: `startPurge` (`orders:83`, `bookings:88`,
`reservations:83`), `RelayConfig` (`152`/`157`/`238`), `topicOf`
(`162`/`167`/`248`), os closures de purge e o corpo de `NewCatalog` em
`app/catalog.go:20-27`.

```go
func (c Catalog) AddressOf(destination string) string             // transport/channel
func NewCatalog(chs ...Channel) (Catalog, error)                  // transport/channel
func Instrument(c Config, rt *otelboot.Runtime, address func(string) string) Config // app/relay
func PurgeOutbox(pool *pgxpool.Pool) PurgeFunc                    // app; idem PurgeCommandInbox, PurgeMessageInbox
```

Receber a função `address` em vez do `channel.Catalog` evita um `require` novo
de `transport` no `go.mod` do `app`. Nenhum desses itens está na lista do que
"fica" do ADR-048; o precedente é `kafka.NewConfig(ctx, rt, catalog, …)`.

**P5 — Seções comuns de `Config` com leitor no kernel.** *Extração + Template
Method no `Validate` · natureza.*

- **Igual nos três contextos e no `config.go__tmpl__`:** as constantes
  `GRPC_*`, os campos e o `FromEnv` de gRPC e Kafka, `transport()`
  (`orders:246`, `reservations:276`, `bookings:231`), `kafkaClientAuth()`,
  `roleList()`, `policies()`, os defaults de relay, admissão e políticas e os
  blocos de atributos de `settings()`.
- **Varia:** as variáveis de domínio (`ITEM_LIMIT`, tópicos, `ORDERS_SOURCE`)
  e o `InboxRetention` de quem consome. Precedentes: `kafka.ReadClientAuth`
  (`kafka/auth.go:88`) e `boot.SignalsFromEnv`.

```go
type APIEnv struct{ Addr string; APIServer }                        // grpc
func ReadAPIEnv(lookup func(string) string, addr string) (APIEnv, error)
func (e APIEnv) Missing() []string                                  // IDN-03, GRP-15
func (a ClientAuth) Missing(insecure bool) []string                 // kafka, IDN-04
type Policies struct{ /* esperas, retenções, PurgeInterval, PurgeBatch */ }
func (p Policies) Validate(consumes bool) error                     // app
```

`requirements()` continua local e compõe os `Missing()`, como o ADR-048 manda.
O `telemetry_fields_test` fixa chaves planas (`grpc_addr`), então os nomes de
atributo se mantêm. Os nomes das variáveis de ambiente não mudam (ADR-053).

**P6 — Template Method para `serveAPI` e `runRelay`.** *Template Method ·
natureza · contraria uma frase do ADR-048.*

- **Evidência:** com os nomes normalizados, o `serveAPI` de
  `orders/app/wiring.go:88-150` e o de `bookings/app/wiring.go:93-155` não têm
  nenhuma linha diferente; `reservations:88-151` segue a mesma forma. O
  `runRelay` do `reservations` só difere na construção do catálogo e em dois
  atributos de log.
- **Leitura:** "diverge por papel" vale para o `RunWith` — o `reservations` tem
  o papel `consumer` —, não para o corpo de cada papel.

```go
type API struct {
	Desc *grpc.ServiceDesc; Commands []string
	Service func(pool *pgxpool.Pool) (any, error) // gancho: construtor tipado, fica no contexto
}
func ServeAPI(ctx context.Context, rt *otelboot.Runtime, cfg APIRole, api API) error
func RunRelay(ctx context.Context, rt *otelboot.Runtime, cfg RelayRole, catalog channel.Catalog) error
```

- **Exige (o maior rito da lista):** ADR novo que supersede a frase do
  ADR-048; package novo `libs/backend/go/app/serve` como unidade, com
  classificação, revisão e `--write-baseline` humano; o `go.mod` do `app` passa
  a requerer `grpc` e `transport`; o `dmpf-generator-check` precisa refletir o
  template novo.

**P7 — `postgres.SnapshotTable`.** *Generics · natureza.*
`orders/provider/order_repository.go:32-58`, `reservations/provider/reservation_repository.go:26-44`
e `bookings/provider/resource_repository.go:23-41` repetem o ciclo
`json.Marshal(state)` → `[]any{raw}` na ida e `scan(&raw)` → `Unmarshal` na
volta.

```go
func SnapshotTable[ID ~string, S, J any](name, idColumn string,
	to func(S) J, from func(J) S, withID func(S, ID) S) Table[ID, S]
```

`J` continua sendo o struct privado do provider com tags estáveis (ADR-053). A
`bookingTable`, híbrida com a coluna `resource_id`, fica manual. Vale incluir o
molde no template do provider, que hoje não traz repositório.

**P8 — Esqueleto de handler gRPC de comando.** *Template Method local ·
natureza · baixo valor.* Os oito handlers de comando repetem `err → statusOf`,
`Rejection() → rejectionOf` e `Response()`. Fica local a cada contexto (e no
template), porque `servicev1.Rejection` é do contrato de cada um.

```go
func command[R, Resp any](out usecase.Outcome[R], err error,
	rejected func(*servicev1.Rejection) Resp, accepted func(R) Resp) (Resp, error)
```

### 4.3 `bff`

Duas premissas valem para o grupo. Os stubs gRPC não são gerados (ADR-015),
então os wrappers de cliente escritos à mão são necessários — e o `invoke[Resp]`
já é genérico. O kernel `http` exclui a forma da API REST
(`libs/backend/go/http/doc.go:13-14`), então o esqueleto de handler fica local
ao `bff`, consumidor único (ADR-044).

**B1 — Esqueleto único de handler REST.** *Template Method + Generics ·
natureza dentro do `bff`.*

- **Evidência:** `bff/app/api/handlers_orders.go:41,73,93`,
  `handlers_reservations.go:33,63,83`, `handlers_bookings.go:42,68,88,114,132` e os
  helpers de `handlers.go:44-96`.
- **Igual:** `pathID` ou `decodeBody` → chamada gRPC → `writeFailure` no erro →
  oneof (sucesso; `Rejection` com 422; `default` com `errMissingResult`) ou
  consulta de status → `writeJSON`.
- **Varia:** a origem da entrada (path, corpo ou query), a validação, a RPC, o
  status de sucesso (201 ou 200) e a view.

```go
type decoder[Req any] func(http.ResponseWriter, *http.Request) (*Req, bool)
type outcome struct{ status int; body any; refused refusal; fault error }
func endpoint[Req, Resp any](d decoder[Req], call func(context.Context, *Req) (*Resp, error),
	present func(*Resp) outcome) http.HandlerFunc
func fromPath[Req any](name string, build func(id string) *Req) decoder[Req]
func fromBody[Body, Req any](idName, schema string, check func(Body) string, build func(string, Body) *Req) decoder[Req]
```

O oneof não é genérico: `present` mantém o type switch e só some o `default`.
Os testes de handler usam gRPC real via bufconn e servem de rede de segurança.

**B2 — Fábrica de rotas e ligação rota → handler.** *Template Method +
Extração · natureza (RST-02, RST-04).* `bff/app/api/routes.go:88-98` repete
onze vezes `Budget`, `Requires`, `IdempotencyKey`, `Permission` e
`ContractRef`; o mapa `serve` por string (`:136-157`) deixa um nome ausente
virar handler nil só em runtime. O `ContractRef` sai de método e path com
escape `~1`, todo `POST` leva `IdempotencyKey` e a permissão é
`<ctx>:write|read`. Ganho de linhas ≈0; o valor é tornar RST-02 verdadeiro por
construção e mover o erro de ligação para a compilação.

```go
type surface struct{ ctx, ref string; budget deadline.Budget }
func (s surface) command(name, path string) kernelhttp.Route
func (s surface) query(name, path string) kernelhttp.Route
type binding struct{ route kernelhttp.Route; serve http.HandlerFunc }
```

`Routes(budget)` é pública e usada em teste; tem de continuar funcionando sem
clientes, o que pede função literal em vez de method value (method value sobre
interface nil entra em pânico na ligação).

**B3 — Lado cliente do protocolo de hop copiado do kernel.** *Extração ·
natureza (os dois lados do mesmo fio).*

- **Evidência:** `bff/app/rpc/metadata.go:12-16` repete as cinco chaves que o
  kernel já exporta (`grpc/interceptor_context.go:35-53`); o `carrier`
  (`metadata.go:65-83`) é o `metadataCarrier` do kernel
  (`interceptor_context.go:381-398`); `bff/app/api/middleware.go:32,239,315`
  (`correlationFormat`, `localeFormat`, `newIdentifier`) copia o kernel; e
  `bff/app/api/routes.go:36` copia `kernelgrpc.DefaultLocale`.
- **Risco do status quo:** o servidor valida com o mesmo regex que o `bff` usa
  para gerar; se o formato mudar no kernel, os dois lados divergem em silêncio.
- **Proposta:** usar já as constantes exportadas e exportar do kernel
  `MetadataCarrier`, `NewID() string`, `ValidCorrelation(string) bool` e
  `ValidLocale(string) bool`. É API pública nova no módulo `grpc`; os sete usos
  em teste viram renomeação.

**B4 — `requireIdempotencyKey` ignora o `Route`.** *Extração · natureza.*
`bff/app/api/middleware.go:262-275` decide pela comparação `r.Method == POST`
com o header fixo, e `Route.IdempotencyKey` só é lido em teste. Proposta:
`requireIdempotencyKey(route kernelhttp.Route, next http.Handler)`, usando
`route.IdempotencyKey`. É divergência latente, não bug ativo; o comportamento
atual (GET com chave inválida recebe 400) precisa ser preservado de forma
explícita.

**B5 — Access log e resposta 500.** *Template Method local · natureza.*
`finishRequest` (`middleware.go:158-178`), `withAccessLog` (`:200-215`) e
`healthAccess.ServeHTTP` (`health.go:60-72`) repetem severidade, `Enabled`,
`accessAttrs` e `LogAttrs("http request")`; o `writeRejection(…, 500,
"internal-failure", …)` aparece três vezes e duplica `rpc.internalFailure`.
Proposta: `logRequest(ctx, logger, r, route string, status int, extra
...slog.Attr)`, `newRecorder(rw)` e `writeInternal(w, r)`. O `finishRequest`
também grava atributos de span; a extração cobre só o log.

**B6 — `withoutValues` duplicada no kernel.** *Extração com Strategy pelo
regex · natureza.* `bff/app/wiring.go:197-204` e
`observability/boot/libraries.go:119-126` têm o mesmo corpo; só o regex muda
(`serverStatement` contra `grpcStatement`). Proposta: `redact.WithoutValues(message
string, statement *regexp.Regexp) string`. É invariante de redação (DAT-02,
DAT-23); `server_errors_test.go` e `libraries_internal_test.go` já cobrem.

**B7 — `rpc.Category` duplica `categoryOf` do kernel.** *Extração · natureza.*
`bff/app/rpc/errors.go:81-112` e `grpc/classifier.go:56-86` têm a mesma tabela
FND-07; o `bff` trata a mais `deadline.ErrNoDeadline` e `ErrDeadlineExhausted`.
Proposta: exportar `kernelgrpc.Category(err error) string` com os dois casos.
**Muda comportamento:** a categoria do span cliente do kernel para esses erros
deixa de ser `_OTHER`; a decisão é do dono do kernel.

**B8 — Tabela de contextos no wiring.** *Extração local · parcialmente
coincidência (só três contextos hoje).* `bff/app/wiring.go:65-101`,
`api/routes.go:161-169`, `app/config.go` e `app/telemetry.go:28-38` repetem, por
contexto, target, caminho do contrato, conn, readiness e `serveContract`.
Recomendação: só o laço de dial, close e readiness sobre `[]backend{name,
target string; config func(rpc.Options) kernelgrpc.Config}`, e um laço de
`serveContract` em `NewHandler`; `Config` e os nomes de variável ficam como
estão, porque são públicos e testados.

**B9 — Clientes gRPC com handle genérico.** *Generics · coincidência ·
último da fila.* `bff/app/rpc/clients.go` tem onze constantes `Method*`, três
mapas de política e onze wrappers de uma linha sobre `invoke[Resp]`.

```go
type Call[Req, Resp any] func(context.Context, *Req) (*Resp, error)
func unaryOf[Req, Resp any](conn grpc.ClientConnInterface, method string) Call[Req, Resp]
```

As interfaces `OrdersClient` e afins viram structs de campos func, o que muda
`api.NewHandler` e os testes. Derivar a política do descritor tiraria a decisão
explícita por método — um RPC novo não idempotente herdaria retry —, então a
política continua declarada por método.

### 4.4 Kits e testes

**O argumento do ADR-046 ainda vale?** Em parte. Ele caducou para `Process`,
`createTopics`, `WaitFor`, `Role`, `Verdict` e o vetor do relay: com os nomes
normalizados, `orders/distkit/{harness,roles,verdict}.go` e os do `bookings`
não têm nenhuma linha diferente, e o `reservations/distkit/harness.go` divide
cerca de 107 linhas de código com eles. Continua valendo para o que só o
`reservations` usa — `Plan`, `Effects`, o `DMPF-R004`, o sink, `Ack`/`Deliver`.
O papel também não é exclusivo: o `reservations` declara `api`, `relay` e
`consumer` (`reservations/app/config.go:22-28`), mas o seu `distkit` só prova o
`DMPF-R004`; os `DMPF-P001` a `P003` do relay dele não são provados por nenhum
harness.

**T1 — Núcleo do `distkit` como Template Method.** *Template Method + Extração
· natureza (a convenção de nomes de variável é derivada; o R004 é por papel).*

```go
type Spec struct {
	Pool   pg.Options
	AppEnv func(Channel) []string                         // gancho: KAFKA_<CTX>_TOPIC e afins
	Roles  map[Role]func(t *testing.T, ctx context.Context)
}
func New(t testing.TB, s Spec) *Harness
func (h *Harness) Start(t testing.TB, r Role) *Process     // Wait, Stop e Output sincronizados
func RunRole(t *testing.T, s Spec)
func RelayVector(h *Harness) (settled map[string]string, published []Published, v Verdict) // P001 a P003
```

- **Destino:** `testkit/tb/dist`, novo. Cada `<ctx>/distkit` fica fino
  (`Spec`, papéis e, no `reservations`, o `Plan`/R004). Corrige D2 por
  construção e dá ao `reservations` a prova P001 a P003.
- **Ganho:** ≈ −380 linhas líquidas; um contexto novo passa de cerca de 330
  linhas copiadas para cerca de 60.
- **Exige:** o `go.mod` do `testkit` passa a requerer o franz-go (`kgo`,
  `kadm`), com entrada `external` no manifesto; o package novo entra num
  `include` (`DMPF-U001`) e precisa de `public_integration_surface` como o
  `kernel/testkit-tb`; os subjects de evidência `dist` e `provider` mudam; o
  `--write-baseline` é humano. O `dmpf-context-check.sh` ainda exige a unidade
  `<ctx>/distkit`, que fica, fina. Pede ADR que supersede a decisão do ADR-046.

**T2 — Suíte golden de contrato.** *Generics + Extração · natureza.*
`orders/contract/golden/fixture_test.go` (222 linhas) é igual ao do
`reservations` exceto pela linha dos `specs`, e quase igual ao de
`libs/backend/go/contracts/golden/fixture_test.go`; `golden_test.go` só difere
nos testes de enum. O `bookings/contract` tem três protos de evento e nenhuma
pasta `golden`.

```go
type Spec[M proto.Message] struct {
	Path string; Identity golden.Identity; New func() M
	FromFields func(map[string]string) (M, error)
}
func GoldenSuite[M proto.Message](t *testing.T, specs ...Spec[M]) // sync, forma, ida e volta, hash, GOLDEN_UPDATE
```

Destino `testkit/tb`, porque o package `golden` é bloco `contract` e não pode
importar `testing`; o `protobuf` já está no `go.mod` do `testkit`. O
`bookings` ganha golden de eventos com cerca de 60 linhas por spec. Cuidados:
o gerador `TestUpdateGolden` e os caminhos relativos `../../../../../`.

**T3 — Suíte de manifesto de telemetria e coletor OTLP.** *Generics + Extração
· natureza.* `app/telemetry_manifest_test.go` (559 linhas) e
`app/telemetry_tenant_test.go` (89) são iguais nos três contextos a menos do
nome do contexto; `telemetry_internal_test.go` e `subject_test.go` também, e
`telemetry_fields_test.go` e `wiring_test.go` ficam entre 97% e 99%. O coletor
OTLP (`otlpExports`, `startCollector`) aparece ainda em
`bff/app/telemetry_manifest_test.go:32-123`.

```go
type Subject[C any, R ~string] struct {
	Name string; Roles []R
	FromEnv   func(R, func(string) string) (C, error)
	Telemetry func(C) boot.Config
}
func Manifest[C any, R ~string](t *testing.T, s Subject[C, R])
func StartCollector(t *testing.T) (addr string, got *Exports) // reaproveitável pelo bff
```

Destino `testkit/obstest`, novo, que puxa `grpc`, `otlp-proto` e `otelboot`
para o `go.mod` do `testkit`. Se isso for inaceitável, a alternativa é o
template em `tools/dmpf-plugin/.../files/app/app/`, que hoje só traz
`config_test.go` e `errors_internal_test.go` — menos ganho, sem grafo novo.

**T4 — `serviceskit` adotável por generics.** *Generics · natureza (o `bind`
é por contexto).* Nenhum contexto importa `serviceskit` nem `providerkit`. O
motivo: `Fakes.UnitOfWork` devolve `UnitOfWork[any]` (`testkit/serviceskit/fakes.go:42`),
não há `CommandInbox`, e o `Ledger` não registra relógio, ids e autorização.
Os contextos reescrevem os dublês à mão — `recordingClock`, `recordingIDs`,
`recordingOutbox`, `recordingUnitOfWork[R]`, `recordingAuthorize` e
`foldDigest` em três cópias (`orders/application/doubles_test.go`,
`reservations/application/sync_doubles_test.go`, `bookings/application/doubles_test.go`).

```go
func UnitOfWork[R any](f *Fakes, bind func(Tx) R) ports.UnitOfWork[R]
func (t Tx) CommandInbox(consumer string) ports.Inbox
func Authorize[C any](f *Fakes, inner func(context.Context, C) error) func(context.Context, C) error
```

O `Authorize` genérico não nomeia `application.Authorize`, o que evita
`DMPF-D001` no `serviceskit` (bloco `provider`). A ordem dos nove passos é um
oráculo mais fino que o `Ledger`; o `recorder` continua opção.

**T5 — Levar os dublês do `bookings` ao `memory`.** *Extração · natureza.*
Fecha D3: trocar `memStore`, `memTx`, `txCommands` e `memUoW` por
`memory.Table`, `NewUnitOfWork`, `tx.CommandInbox` e `Table.Reader`, como
`orders` (`doubles_test.go:14-17,176-205`) e `reservations` já fazem, e
corrigir `testOccurred` para nanossegundos. Risco baixo: só testes, e nenhum
teste do `bookings` usa concorrência.

**T6 — Fixture de `ExecutionContext`.** *Extração · natureza.* Vinte e cinco
`_test.go` montam o mesmo literal `ExecutionContextSpec` (subject `s-test`,
tenant `acme`), dez deles como helper em `execution_test.go`; a única variação
é `WithIdempotencySlot`. Proposta em `testkit/tb`: `WithExecution(t, ctx,
opts ...ExecOption) context.Context`, `WithKey(t, ctx, key)` e a opção
`WithSlot()`.

**T7 — Asserções de domínio.** *Generics · natureza.* `requireRejected[R]` e
`requireAccepted[R]` são iguais byte a byte em `*/domain/helpers_test.go`;
`sameSequence`, `collect` e o laço da fixture de projeção se repetem em
`projection_test.go` e `determinism_test.go`. Proposta: `tb.RequireRejected[R
comparable]`, `tb.RequireAccepted`, `tb.RequireSameEvents`,
`tb.RunProjection(t, fixture, run) domainkit.Verdict` e `Verdict.Merge` no
`domainkit` (que é bloco `domain` e não importa `testing`). A restrição
`comparable` troca o `any(resp) != any(zero)` — que entra em pânico em runtime
para tipo não comparável — por checagem em compilação. Os `determinism_test.go`
(73 + 73 + 53 linhas) duplicam o `domainkit.ReadTwice` (ORA-34); removê-los
passa de 30 linhas e pede confirmação à parte.

**T8 — Leitores das tabelas do kernel e composição única.** *Extração ·
natureza nos leitores, coincidência na composição.* `Outbox()` e `Enqueued`
(`orders/appkit/harness.go:79-120` e o equivalente no `bookings`), o `Settled`
do `distkit` e o `InboxRow`/`Effects` do `reservations` leem as mesmas tabelas
`outbox`, `inbox` e `quarantine`. Proposta: `pg.Outbox(t, pool) []Enqueued`,
`pg.Settled` e `pg.Counts(t, pool, tables...)` em `testkit/tb/pg`. À parte,
`orders` e `bookings` reimplementam no `appkit` o `bind` que o próprio `app` já
tem não exportado (`orders/app/wiring.go:54`); exportar `app.NewService(pool,
clock, ids, waits)`, como o `reservations` já faz (`reservations/app/consumer.go:52`),
alinha os três.

**T9 — `provider/e2e_test.go` reimplementa o `appkit`.** *Extração ·
coincidência.* `orders/provider/e2e_test.go:30-80` e
`bookings/provider/e2e_test.go:30-76` trazem `fixedClock`, `sequenceIDs` (o
mesmo formato `m-%06d` de `ids.Sequence`), `outboxRow` e `newService`, num
package que já importa o `appkit`. Proposta: usar `appkit.New*`, `h.Outbox`,
`ids.Sequence` e `clock.New`, acrescentando `SchemaVersion` a `Enqueued`.

**T10 — `providerkit.Repository` por agregado.** *Generics já existentes ·
prioridade média-baixa.* Os testes de repositório dos três contextos (de 128 a
190 linhas, mais `concurrency_test.go`) repetem as cláusulas mecânicas de
versão e conflito que o `providerkit.Repository[ID, S]` já cobre — o kernel o
roda em `postgres/conformance_test.go:55`. Proposta: uma chamada por
`postgres.Table`, mantendo só o teste de ida e volta do codec. As cláusulas de
tenant (IDN-12 a IDN-15) já são certificadas no kernel; o ganho é a fiação e o
codec do contexto.

**T11 — Helpers de teste do `bff`.** *Extração · baixo valor.* O supervisor
`process`/`start` de `bff/app/harness_test.go:148-200` repete o `Process` de
T1, que o cobre. `fakeContexts` e `unary[Req, PReq]` têm 66 linhas iguais
entre `bff/app/api/testing_test.go:48-138` e `bff/app/rpc/testing_test.go:23-80`;
cabem num helper local.

## 5. Ordem sugerida

A ordem prioriza o que fecha deriva e o que não pede rito; o que contraria um
ADR fica por último, porque depende de decisão registrada antes do código.

| Onda | Itens | Por que nessa posição |
|------|-------|-----------------------|
| 0 — corrigir deriva | D1 (auditar no `bookings`), D2 (sincronizar `Output()`), T5 | Correções locais, sem abstração nova; D1 fecha um item de spec já marcada como concluída |
| 1 — sem ADR nem baseline | P1, A7, A2, A4, P4, P7, B3, B6, B4, B5, B8, T6, T7, T8, T9 | Funções novas em packages existentes ou mudança local; nenhuma unidade nova |
| 2 — Template Method estrutural | A1 (com A3 e A5), P2, P5, B1, B2, T2, T4, T10, P8, B9 | Mudam a forma dos casos de uso, das bordas e dos testes; A1 muda comportamento (auditoria) e reescreve o mapa de passos do `application/README.md` |
| 3 — exige ADR ou classificação | P3 (addendum ao ADR-048), P6 (ADR novo + unidade), T1 (ADR + unidade + franz-go no `testkit`), T3 na variante `obstest`, A6 (tipo publicado), B7 (decisão do dono do kernel) | Contrariam uma frase de ADR ou mexem em superfície pública e no baseline |

Em toda onda que promover algo ao kernel:

- retirar a cópia dos templates do generator
  (`tools/dmpf-plugin/templates/bounded-context/`) e ajustar o
  `dmpf-generator-check`;
- atualizar a skill `.agents/skills/dmpf-bounded-context/` e as instruções do
  `/dmpf-new-context` — o `dmpf-harness-check.sh` regenera o golden `bookings`
  por agente, e um agente sem a instrução volta a gerar cópias;
- registrar os digests de evidência que mudarem e, quando houver unidade nova,
  deixar o `--write-baseline` para a pessoa responsável (`DMPF-T001`).

## 6. Descartados

| Candidato | Motivo |
|-----------|--------|
| Codec genérico para respostas de um campo `string` | O layout é contrato persistido por operação (ADR-056); deixá-lo explícito vale mais que três linhas |
| União `Operation` com `isOperation()` no kernel | A união é fechada por contexto; marcador não exportado não pode vir do kernel |
| `Resources` genérico | UOW-03/04; o ADR-034 veda `tx.Repository("name")` |
| Snapshot, `clone` e `Equal` no kernel | Os campos divergem e a solução exigiria reflexão |
| Esqueleto de consumo (§6.3), `ports.Finder[K, S]`, `Sink` e `Handler` genéricos do consumidor | Um único consumidor; abstração sem cliente (ADR-046) |
| `instrumentation()` com `NoInstrumentation` como default no kernel | Contradiz `ports/instrumentation.go:189-190` ("never a silent default") |
| `AggregateType`, `Destination`, códigos de rejeição, `EventName`, funções `XChannel` | Vocabulário do contexto |
| `requirements()` e `RunWith` | Divergem por papel, como o ADR-048 decidiu |
| `Main` genérico para `cmd/main.go` | O template já mantém os três iguais; economizaria ~35 linhas e o `main_test` passaria a testar o kernel |
| `provider/mapper.go` | O type switch com `ErrUnmappedEvent` é o ponto idiomático; o payload é do contexto |
| `New*Reader`, `New*Repository` | Já são uma linha sobre `postgres.Table` |
| `schema.go` | É do template, e `go:embed` não atravessa módulos |
| Regex de identificador `^[A-Za-z0-9._:-]{1,128}$` (quatro cópias, com o `bff`) | Contrato publicado por campo de cada contexto; evolui de forma independente |
| `permissionOf` | O esqueleto já é `usecase.Permitted`; o switch é a declaração do contexto (IDN-16) |
| Reflexão para extrair `rejection` do oneof no `bff` | Esconde o contrato e quebra com renomeação |
| Nome de status por `enum.String()` | Os valores do fio são contrato (`cancelled` no `bookings`, `canceled` no `reservations`) |
| Esqueleto de handler, `withExecutionContext` ou `outcomeOf` no `kernelhttp` | Consumidor único (ADR-044) e `http/doc.go` exclui a forma da API; reavaliar se surgir segunda borda REST |
| `Classify`, `codeNames`, `Probe`, `readiness` e `main` do `bff` | Já usam o kernel ou não têm repetição interna |
| `Fields` por reflexão nos testes de projeção | O nome do campo é contrato cross-stack (ORA-33) |
| `want` derivado em `appkit/schema_test.go` | O literal é o oráculo de isolamento (ADR-053); derivá-lo de `Tables` o tornaria tautológico |
| Cenários de produtor do `appkit`, `idempotency_test` e `sequence_test` genéricos | As closures ficariam maiores que o código; esperar T4 |
| `memory.FixedClock` contra `testkit/clock` | Troca cosmética |
| `rpc/testing_test.go` dos contextos no `testkit` | Exigiria `kernelgrpc` e o SDK OTel no `testkit`; candidato a template do generator |

## 7. Notas laterais

Itens fora do escopo de abstração, encontrados durante a análise:

- O `libs/backend/go/testkit/README.md` (linhas 41-46 e 92) e
  `.agents/skills/dmpf-testkit/references/package-routing.md` (linhas 15-16)
  ainda dizem que `appkit` e `distkit` só existem no `reservations`; o ADR-048
  os tornou obrigatórios nos três.
- O template `files/app/appkit/pool.go__tmpl__` não corresponde aos três
  contextos, que têm o `OpenPool` dentro de `harness.go`.
- O comentário de doc do `Harness` em `reservations/appkit/harness.go:35-41`
  está solto, acima de `Tables`.
- Os manifestos de `orders` e `bookings` não declaram o franz-go, que os
  respectivos `distkit` importam; o do `reservations` declara. Falta confirmar
  se o verificador deveria acusar essa ausência.
