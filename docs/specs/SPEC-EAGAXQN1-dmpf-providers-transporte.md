---
id: SPEC-EAGAXQN1
slug: dmpf-providers-transporte
title: DMPF KRN-10 — Providers de transporte: gRPC, REST, Kafka e SNS/SQS
stage: done
priority: P1
depends_on: [SPEC-WYX5GW87, SPEC-NYD18TGD, SPEC-ANZX2WPG, SPEC-CGPX20NP]
ticket_url: null
subtask_urls: []
created: 2026-09-06
---

# SPEC-EAGAXQN1: DMPF KRN-10 — Providers de transporte: gRPC, REST, Kafka e SNS/SQS

## Resumo

O kernel já publica atomicamente (`KRN-06`, `KRN-08`) e consome com
deduplicação (`KRN-07`), mas ninguém realiza o transporte: o relay entrega os
bytes a um `Publisher` que só existe em memória, e o consumer adapter recebe um
`Acknowledger` cujo `Release` o próprio godoc deixa "a concrete gesture left to
KRN-10". Esta spec entrega os providers de transporte do bloco `provider` —
gRPC e HTTP no síncrono, Kafka e SNS/SQS no assíncrono — com o governo do tempo,
o gesto de ACK e a byte-preservação **realizados em código**, e o endereço
concreto de cada canal apenas na configuração do `provider`.

Como operador do sistema, quero que a política de transporte de FND-06 seja
verificável por teste — deadline que decresce a cada salto, ACK que só sai
depois do commit local, `payload_hash` idêntico após o hop —, para que uma
regra de transporte não dependa de revisão manual para ser cumprida.

A entrega fecha o incremento 4 — Fronteiras — junto com o `KRN-09`: serviço
observável, resiliente e falando gRPC e Kafka (SPEC-YRJRADY9, "Sequenciamento
sugerido").

## Contexto

- **Problema**: o ADR-024 decidiu REST na borda externa e gRPC entre serviços
  nossos com o tempo governado pela borda; o ADR-025 decidiu Kafka como
  transporte-alvo do evento de domínio e SNS/SQS normatizado no acervo. Nenhuma
  das duas decisões tem realização Go. Sem elas, cada serviço que adotar o
  kernel reinventará deadline, retry, ACK e contenção — e as formas de acertar
  por acidente (deadline reiniciado, `enable.auto.commit`, SNS sem *raw
  delivery*) são exatamente as que FND-06 §5.2 e §6.2 catalogam como violação
  silenciosa.
- **Impacto**: um serviço compõe o transporte a partir de módulos cuja
  conformidade é provada por suíte; o composition root declara canais, prazos e
  políticas de retry, e o `provider` recusa operar o que não estiver declarado.
- **Inspiração**: `google.golang.org/grpc` já transmite `grpc-timeout` como
  duração restante e reconstrói o instante no receptor — é `GRP-06` nativo; o
  que falta é a disciplina em volta (recusar chamada sem prazo, nunca substituir
  o prazo recebido, retry só por idempotência declarada). No assíncrono,
  `franz-go` expõe `DisableAutoCommit`, `OnPartitionsRevoked`/`OnPartitionsLost`,
  `BlockRebalanceOnPoll`, `PauseFetchPartitions` e `SetOffsets`
  ([docs oficiais](https://github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md)),
  que são as primitivas de `TRP-28`, `TRP-47`, `TRP-48` e `TRP-29`.
- **Links relevantes**:
  - `docs/specs/SPEC-YRJRADY9-dmpf-kernel-sdk-go.md` — spec guarda-chuva; linha
    `KRN-10` da decomposição (`:395`) e requisito P1 (`:191-198`).
  - `docs/specs/SPEC-YWFGNPG5-dmpf-politicas-transporte.md` — spec que produziu
    FND-06, `done`.
  - `docs/specs/SPEC-NYD18TGD-dmpf-resiliencia-observabilidade-go.md` — `KRN-09`;
    deixa a admissão por rota e tenant e as métricas `MET-11`/`MET-12` para esta
    spec (`:182`, `:185`), e a posição de rate limiting do `Compose` vazia.
  - `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md` — `KRN-07`; consumer
    adapter que esta spec alimenta.
  - `docs/specs/SPEC-CGPX20NP-dmpf-relay-outbox.md` — `KRN-08`; relay cujo
    `Publisher` esta spec realiza.
  - `docs/dmpf/politicas-transporte.md` — FND-06, fonte normativa primária.
  - `docs/adr/024-rest-externo-grpc-interno-governo-do-tempo.md`,
    `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md`,
    `docs/adr/023-autoridade-de-validacao-e-registry.md`.
  - `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` —
    decorators e retry que o transporte compõe.

### Divergências entre o ticket e o repositório

O ticket `ARQ-529` foi escrito em 30/08/2026, antes de `KRN-07`, `KRN-08` e
`KRN-09` entrarem. Dez pontos precisam de reconciliação, e a spec prevalece
sobre o ticket em todos.

| Ticket `ARQ-529` diz | Repositório em 06/09/2026 | Esta spec adota |
| --- | --- | --- |
| "Quatro módulos do bloco `provider`" | A linha `provider` da matriz de blocos permite `provider → provider` (`conformance/internal/rule/matrix.go:52`). gRPC e HTTP compartilham a abstração de prazo (ticket, §5 passo 2); Kafka e SQS compartilham o contrato de catalogação e as fórmulas de `janela_redelivery` (ticket, entregável 3). Nenhum dos quatro é lugar neutro para isso, e `observability` é nomeado por FND-08, não por transporte | **Cinco módulos**: `transport` (primitivas compartilhadas, sem I/O; a API OpenTelemetry é a única dependência externa, pelo package `observe`) mais os quatro providers. A unidade `kernel/transport` é bloco `provider` |
| "Depende de `KRN-05`, `KRN-09`"; "Consome as portas de `KRN-06` e `KRN-07`" | `ports/acknowledger.go:15` reserva o gesto: "Release does not confirm; the transport redelivers with backoff, a concrete gesture left to KRN-10 (D3)". `app/relay/ports.go:32-42` declara `Publisher.Publish(ctx, destination string, message []byte) error` **no consumidor**, satisfeito estruturalmente | Realiza `ports.Acknowledger` e `ports.Containment` (bloco `port`, célula permitida) e satisfaz `relay.Publisher` **sem importar** `app` — a célula `provider → app` é proibida (`matrix.go:52`), e a tipagem estrutural de Go dispensa o import, como `postgres.OutboxStore` já faz com `relay.Store` |
| Silente sobre como o provider entrega ao consumer adapter | `app.Consumer.Consume(ctx, Delivery, Acknowledger)` recebe `Delivery{Raw, Attempt}` (`consumer.go:34-37`) — tipo do bloco `app`, que o `provider` não pode importar | Cada provider de consumo expõe uma interface `Sink` própria, `Handle(ctx, raw []byte, attempt int, ack ports.Acknowledger) error`; a **composition root** (bloco `app`) adapta para `app.Consumer.Consume`. Nenhum provider conhece `app` |
| "Chave de partição vem do envelope" (`TRP-11`) | `relay.Publisher.Publish` não recebe chave de partição: recebe o destino lógico e os bytes do envelope já serializado por `envelope.Marshal` | O provider **decodifica o envelope para ler** `PartitionKey`, `ID`, `Source` e o `payload_hash`, e publica **os bytes originais** — observar não é substituir (`TRP-17`). `Unmarshal` é do bloco `contract`, célula `provider → contract` permitida |
| "DLQ publicada antes do avanço do offset" | `app.Consumer.contain` (`consumer.go:118-129`) já **quarantena primeiro e confirma depois**, via `ports.Containment`; o destino da contenção é escolha do composition root, e `ContainmentMap` roteia por `Reason` | A DLQ do broker é uma realização de `ports.Containment` (`kafka.DLQ`, `sqs.DLQ`) que publica o envelope **byte a byte** com o diagnóstico sanitizado. A ordem de `TRP-30` é do adapter e permanece lá; a fusão DLQ/quarantine de `TRP-36` é declaração do canal |
| "Admissão por rota e tenant de `KRN-09`" | `SPEC-NYD18TGD:182` deixa a admissão por rota e tenant para o `KRN-10`; `:185` deixa `MET-11` e `MET-12`; `resilience/sheet.go` marca `RateLimitPolicy` como "belongs to another story" | Middleware de admissão no provider HTTP e no servidor gRPC, por rota e tenant, recusando **antes** do decode (`RES-16`, `RES-17`), com `dmpf_service_admission_rejections_total` (`MET-12`) acrescentado ao catálogo de `observability/metrics`. `MET-11` é realizado pelos pools de consumo de Kafka e SQS |
| Silente sobre a estabilidade do binding | `TRP-09`/`TRP-46` exigem que o endereço concreto (ou a revisão do binding) seja **persistido junto ao item da outbox antes do primeiro I/O**. O schema de `dmpf_outbox` (`KRN-06`) guarda só o destino lógico, e alterá-lo é evolução de outro módulo | **Fora do escopo**, encaminhado (ver "Escopo fora"). O risco é limitado enquanto cada canal tiver um único binding por deploy, e `COE-04` absorve a segunda entrega como R2. Registrado como pendência com owner |
| "Catalogação em AsyncAPI, sem a qual o provider não opera" | FND-06 §16 encaminha **a forma do documento** — schema, versão, onde vive — a ANC-03, e normatiza só o **conteúdo operacional** (`ASY-02`) | O conteúdo de `ASY-02` é um tipo Go (`channel.Channel`) validado na construção do provider; o composition root o declara. O exemplo AsyncAPI de §16 é reproduzido na documentação do módulo como mapeamento campo a campo. Um decodificador do documento é de ANC-03 |
| "TLS obrigatório em produção" (`GRP-15`) | O provider não sabe em que ambiente roda | TLS é o **default sem opção**; plaintext exige a opção nomeada `InsecureForDevelopmentOnly`, que o construtor registra em log de aviso. Não há terceiro estado |
| Inclui `KFK-13` a `KFK-18` por referência a §11 | As seis regras são **condicionais** à existência de registry em runtime (`KFK-13`), decisão de `ADR-DMPF-N` ainda pendente | Nenhum cliente de registry. `KFK-15` é satisfeita por construção. As demais ficam fora até o ADR |

### Fontes normativas

| Fonte | O que fixa para esta spec |
| --- | --- |
| FND-06 §4.1 (`TRP-07`, `TRP-08`) | Endereço concreto só na configuração do provider; recusa **na construção** |
| FND-06 §4.2 (`TRP-11`) | Chave de partição lida do envelope, nunca inferida do payload |
| FND-06 §4.3 (`TRP-12`) | Instante de publicação é metadado de transporte, nunca atributo do envelope |
| FND-06 §5.1 (`TRP-13` a `TRP-15`) | Bytes de `Any.value` idênticos ponta a ponta; gesto proibido nomeado; hash sobre os bytes recuperados |
| FND-06 §5.2 (`TRP-16` a `TRP-21`) | Matriz de hops: conforme / não conforme por linha; interceptor observa, não substitui; codificação textual exatamente uma vez; claim-check vedado |
| FND-06 §6.1 (`TRP-22` a `TRP-25`) | Três relógios; `janela_redelivery` por canal com fórmula e parâmetros |
| FND-06 §6.2 (`TRP-26` a `TRP-32`, `TRP-47`, `TRP-48`, `TRP-52`) | ACK depois do commit local; gesto por disposição; não commitar não reentrega em Kafka; `auto.commit=false`; offset contíguo `n+1`; revogação cancela; DLQ antes do avanço; contagem de tentativas observável |
| FND-06 §9 (`RST-01` a `RST-04`) | REST externo; sem retry de não idempotente; timeout do deadline; canal aponta contrato |
| FND-06 §10 (`GRP-01` a `GRP-18`) | gRPC interno; deadline obrigatório, propagado, relativo no fio; prazo por método com folga; retry por idempotência; backoff com jitter; balanceamento e health explícitos; erro gRPC e tabela HTTP; TLS |
| FND-06 §11 (`KFK-01` a `KFK-12`, `KFK-19`) | Nome de tópico; um tipo por tópico; partições declaradas; grupo estável; `consumer_name` ≠ grupo; um processador por partição; retry inline limitado; DLQ nomeada; prazo < intervalo de poll |
| FND-06 §12 (`SQS-01` a `SQS-13`) | Base64 uma vez; SNS raw delivery; ≤ 10 atributos; FIFO por chave; dedup id composto e resumido; extensão de visibilidade com teto 12h; delete por receipt handle; contenção por disposição; sem claim-check |
| FND-06 §13 (`TRP-42` a `TRP-44`) | Dedup nativa é otimização; o provider não é lugar de idempotência de efeito |
| FND-06 §16 (`ASY-01` a `ASY-04`) | Provider não opera canal não catalogado; sete itens obrigatórios; referência ao contrato; sem atributo novo |
| FND-08 §3.5 (`RES-16`, `RES-17`), `MET-11`, `MET-12` | Admissão por rota e tenant antes do decode, com sinal próprio |
| FND-07 `CTX-18`, `CTX-19`, `CTX-21`, `ERR-09`, `MAP-07` | Prazo é instante absoluto no contexto; monotônico; I/O respeita cancelamento; retryability → disposição |
| ADR-024, ADR-025 | Decisões de transporte que esta spec realiza |
| RFC §7.3 (matriz de blocos), `matrix.go`, `capability.go` | `provider → {domain, port, provider, contract}` permitido; `provider → {application, app}` proibido; `provider` é bloco permissivo em capabilities |

<constraints>
- [P0] NUNCA reiniciar o deadline recebido: o prazo de saída é `min(recebido − folga, orçamento do método)` e nunca maior que o recebido (`GRP-05`, `GRP-17`).
- [P0] NUNCA confirmar no broker antes do retorno do `Sink`: `Ack` e `Release` são invocados pelo adapter depois do commit local (`TRP-26`, `INB-08`); o provider nunca confirma por conta própria.
- [P0] NUNCA desserializar e reserializar o payload no caminho de publicação, consumo, retry ou contenção: o provider publica os bytes que recebeu (`TRP-13`, `TRP-14`, `TRP-17`).
- [P0] NUNCA importar `app` nem `application` de um provider: células 26 e 27 da matriz são proibidas; `relay.Publisher` é satisfeito estruturalmente.
- [P0] NUNCA retentar automaticamente método ou rota sem idempotência declarada; lista de códigos retentáveis vazia significa nenhum retry (`GRP-08`, `GRP-09`, `RST-02`).
- [P0] NUNCA operar canal sem os sete itens de `ASY-02` resolvidos, e recusar na construção, não na primeira publicação (`TRP-08`, `ASY-01`).
- [P0] Nenhum tópico, fila, ARN ou URL fora da configuração do provider (`TRP-07`); nenhum artefato declara exactly-once fim a fim (`TRP-38`).
- [P1] `enable.auto.commit` é `false` e o offset commitado é o do registro **seguinte** ao último contíguo disposto (`TRP-28`, `TRP-29`).
- [P1] Toda dependência externa nova entra em `dmpf-units.json` com as quatro chaves (pacote, faixa, entrypoints, capability), como `capability.go:113` exige.
</constraints>

## Requisitos

### Funcionais

#### `transport` — primitivas compartilhadas

- [ ] **[P0] Orçamento de prazo por método** (`deadline`): `Budget{Method, Limit,
  Slack}` e `Outgoing(ctx, now, budget) (time.Time, error)` que devolve
  `min(deadline(ctx) − Slack, now + Limit)`.
  - `ctx` sem deadline → `ErrNoDeadline` (`GRP-04`); nunca há default.
  - `deadline(ctx) − now ≤ Slack` → `ErrDeadlineExhausted`, sem I/O (`CTX-21`).
  - O resultado nunca excede `deadline(ctx)` (`GRP-05`).
  - `Limit` e `Slack` são declarados por método; `Slack > 0` obrigatório
    (`GRP-16`, `GRP-17`).
- [ ] **[P0] Contrato de catalogação de canal** (`channel`): `Channel` com os
  sete itens de `ASY-02` — `Name` (destino lógico), `Transport`, `Address`,
  `EventType` + `ContractMajor` + `ContractRef`, `Ordering{Key, Unit}`,
  `Redelivery RedeliveryWindow`, `Containment` e `Retry{Strategy, MaxAttempts}`
  — e `Validate() error` que reprova qualquer item ausente.
  - `Catalog` indexado por `Name`; `Resolve(destination)` devolve `ErrUnknownChannel`.
  - `Ordering.Unit ∈ {partition, group, none}`; `Unit == none` exige `Key == ""`
    (`KFK-06`, `SQS-04`).
  - `Ordering.Unit != none` **e** `Retry.Strategy == separate-channel` →
    `ErrOrderedChannelWithSeparateRetry` (`TRP-32`, `KFK-11`).
  - Kafka: `Address` casa `^[a-z0-9._-]+$`, tem ≤ 249 caracteres e não colide
    com outro canal por troca de `.`/`_` (`KFK-01`, `KFK-01c`); `Partitions > 0`
    e `Partitioner` declarados (`KFK-04`); `Group` não vazio (`KFK-07`).
  - `Address` não contém `prod`, `staging`, `hml` como segmento salvo quando
    `EnvironmentPrefix` está declarado (`KFK-02`).
- [ ] **[P0] Fórmulas de `janela_redelivery`** (`channel`): três construtores
  que produzem `RedeliveryWindow{Formula, Params, UpperBound}` (`TRP-23`,
  `TRP-24`, `TRP-24b`):
  - `KafkaWindow(retentionByTime, retentionBySize, cleanupPolicy, remoteStorage, initialOffset)`
    → `UpperBound = retentionByTime` quando a retenção por tamanho não for
    declarada como menor; todos os cinco parâmetros obrigatórios.
  - `SQSWindow(maxReceiveCount, visibilityBase, retention)` →
    `UpperBound = min(maxReceiveCount × min(visibilityBase + extensões, 12h), retention)`.
  - `SNSSQSWindow(subscriptionDeliveryWindow, sqs SQSWindow)` →
    `UpperBound = subscriptionDeliveryWindow + sqs.UpperBound` — a soma das duas
    janelas, nunca só a da fila.
  - `Validate` recusa `UpperBound == 0` e `Params` incompletos: "depende da
    configuração" não é fórmula (`TRP-24`).
- [ ] **[P1] Metadado de tentativa** (`attempt`): chave de header/atributo
  `dmpf-attempt` e funções `Encode(int)`/`Decode(string)`, para o transporte
  que não mantém contagem (`TRP-52`), sempre lateral ao envelope (`TRP-18`).

#### `grpc`

- [ ] **[P0] Cliente com governo do tempo**: `Dial(target, Config)` devolve
  `*grpc.ClientConn` com interceptors unário e de stream que aplicam
  `deadline.Outgoing` por método full (`/pkg.Service/Method`).
  - Método sem `Budget` declarado → `ErrMethodNotDeclared` na chamada
    (`GRP-16`); chamada sem deadline no contexto → `ErrNoDeadline` (`GRP-04`).
  - O interceptor deriva um contexto filho com o prazo calculado; nunca cria
    prazo maior que o recebido (`GRP-05`).
  - Teste de dois saltos por `bufconn`: A → B → C; o prazo observado em C é
    menor que em B, que é menor que em A, cada um pela folga declarada
    (`GRP-06`, `GRP-17`).
- [ ] **[P0] Retry por idempotência declarada**: `MethodPolicy{Idempotent bool,
  RetryableCodes []codes.Code, Backoff}`; retry só se `Idempotent` **e** o
  código estiver na lista (`GRP-08`, `GRP-09`).
  - Lista vazia ou `Idempotent == false` → zero tentativas adicionais.
  - Backoff exponencial com jitter via `observability/retry` (`GRP-10`),
    consumindo o orçamento de `RES` e nunca ultrapassando o prazo restante.
  - Métodos de stream nunca são retentados pelo interceptor.
- [ ] **[P0] TLS por default**: `Config.TLS *tls.Config` obrigatório; a única
  alternativa é `Config.InsecureForDevelopmentOnly = true`, que emite aviso em
  log estruturado na construção (`GRP-15`). Sem nenhum dos dois → `ErrTLSRequired`.
- [ ] **[P1] Balanceamento e health explícitos**: `Dial` aplica *service
  config* com `round_robin` e `healthCheckConfig{serviceName}` (`GRP-11`,
  `GRP-12b`); `NewServer(Config)` registra `grpc.health.v1.Health` com estado
  **por serviço** (`GRP-12`) e devolve `*grpc.Server` com TLS pela mesma regra.
- [ ] **[P1] Cancelamento nos dois sentidos**: o servidor expõe `ctx.Done()` do
  gRPC ao handler e os interceptors de cliente propagam o cancelamento ao
  contexto filho (`GRP-07`); teste que cancela o cliente e observa o servidor
  encerrar.
- [ ] **[P1] Tabela de status**: `HTTPStatus(codes.Code) int` com as dez
  linhas de `GRP-14`, incluindo `CANCELLED → 499`; código fora da tabela → `500`.
- [ ] **[P1] Admissão no servidor**: interceptor `Admission(limits)` por método
  e tenant, recusando com `RESOURCE_EXHAUSTED` **antes** do handler e
  incrementando `MET-12` (`RES-16`, `RES-17`).

#### `http`

- [ ] **[P0] Cliente com timeout derivado do deadline**: `Client.Do(ctx,
  route, req)` exige deadline no contexto e aplica `deadline.Outgoing` com o
  `Budget` da rota como timeout da requisição (`RST-03`). Sem deadline →
  `ErrNoDeadline`.
- [ ] **[P0] Retry só de método idempotente**: `GET`, `HEAD`, `PUT`, `DELETE`
  são retentáveis nos status declarados na rota; `POST` só quando
  `Route.IdempotencyKey != ""` e o header é enviado (`RST-02`). `PATCH` nunca.
  Backoff via `observability/retry`.
- [ ] **[P0] Canal externo aponta contrato**: `Route{Name, Method, Path,
  ContractRef, Budget, RetryableStatus}`; `ContractRef == ""` →
  `ErrContractRequired` na construção do `Client` (`RST-04`).
- [ ] **[P1] Admissão por rota e tenant no servidor**: `Admission(limits,
  TenantFunc) func(http.Handler) http.Handler` — *token bucket* por
  `(rota, tenant)`, recusa `429` **antes** de ler o corpo (`RES-17`),
  incrementa `dmpf_service_admission_rejections_total{route, tenant}` (`MET-12`).
  Tenant ausente é label `""`, não um tenant (`CTX-26`).
- [ ] **[P2] Sem superfície de API**: o módulo não define recurso, paginação,
  corpo de erro nem versionamento (`TRP-03`; ADR-024 os deixa fora).

#### `kafka`

- [ ] **[P0] Publisher estrutural**: `Publisher.Publish(ctx, destination,
  message)` resolve `destination` no `Catalog` (`TRP-07`, `TRP-08`),
  desserializa o envelope só para **ler** `PartitionKey`, publica `Record{Key:
  PartitionKey, Value: message}` com os bytes intactos (`KFK-05`, `TRP-13`).
  - `Address` do canal é o tópico; nunca derivado do `Type` em runtime
    (`KFK-01b`: a decomposição é declarada no canal).
  - Produtor idempotente habilitado (`TRP-43`); nenhum atributo do envelope vai
    para header (`TRP-18`); header `dmpf-published-at` opcional com o instante
    da publicação (`TRP-12`).
  - Hook `Observer func(Record)` sem retorno de registro: interceptor pode ler,
    não substituir (`TRP-17`), garantido pelo tipo.
- [ ] **[P0] Consumer com ACK depois do commit local**: `Consumer{Channel,
  Sink, MaxInlineAttempts, Backoff, ProcessingDeadline}.Run(ctx)` sobre
  `franz-go` com `DisableAutoCommit` (`TRP-28`), `BlockRebalanceOnPoll` e uma
  goroutine por partição (`KFK-09`).
  - Para cada registro: `Sink.Handle(ctx, record.Value, attempt, ack)`;
    `ack.Ack` marca o registro; o commit é do **sucessor do último contíguo**
    (`TRP-29`), nunca de um registro ainda não disposto.
  - `ack.Release` **não** devolve ao broker (`TRP-47`): o laço reentrega o mesmo
    registro ao `Sink` com `attempt+1` após `Backoff(attempt)`, até
    `MaxInlineAttempts` (`KFK-10`); o orçamento do laço respeita
    `ProcessingDeadline < max.poll.interval` (`KFK-19`).
  - Esgotado `MaxInlineAttempts`, o `Sink` recebe `attempt == MaxInlineAttempts`
    e o adapter contém (`ReasonAttemptsExhausted`) e confirma — a DLQ sai
    **antes** do avanço do offset (`TRP-30`, `KFK-12`).
  - `OnPartitionsRevoked`/`OnPartitionsLost`: cancela o contexto da partição,
    espera o trabalho em curso encerrar, commita só o contíguo já disposto e
    **não** commita o restante (`TRP-48`).
  - Canal com `Ordering.Unit == partition` recusa `Retry.Strategy ==
    separate-channel` na construção (`KFK-11`).
- [ ] **[P0] DLQ como `Containment`**: `DLQ{Channel}.Quarantine(ctx,
  ports.Contained)` publica em `Channel.Containment` o `Envelope` byte a
  byte, com headers `dmpf-reason`, `dmpf-consumer`, `dmpf-error` (sanitizado,
  `ERR-20`) e `dmpf-attempt` (`TRP-52`). Nunca reserializa (`TRP-13`).
- [ ] **[P1] Métricas do pool**: `dmpf_service_pool_utilization` e
  `dmpf_service_queue_depth` por consumidor (`MET-11`), via
  `observability/metrics`.
- [ ] **[P1] Nome de tópico**: `Validate` do canal aplica `KFK-01`, `KFK-01c` e
  `KFK-02`; `Partitions`, `Partitioner` e `KeyEncoding` obrigatórios (`KFK-04`).
  Aumento de partições **não** é operação do provider (`KFK-04b`).

#### `sqs`

- [ ] **[P0] Publisher SQS estrutural**: `Publisher.Publish(ctx, destination,
  message)` codifica `message` em Base64 **uma vez** como corpo (`SQS-01`,
  `TRP-19`).
  - FIFO: `MessageGroupId = hex(sha256(PartitionKey))` e
    `MessageDeduplicationId = hex(sha256(Source + "\x00" + ID + "\x00" + payload_hash))`
    (`SQS-05`, `SQS-06`, `SQS-06b` — 64 caracteres hexadecimais, dentro do teto
    de 128 e do alfabeto).
  - Standard: sem grupo e sem dedup id; `Ordering.Unit` deve ser `none`
    (`SQS-04`).
  - Tamanho final do corpo codificado > limite do caminho (`256 KiB`, ou o menor
    entre tópico e fila quando `Transport == sns-sqs`) → `ErrMessageTooLarge`
    sem enviar; **não há claim-check** (`SQS-12`, `SQS-12b`).
  - Mais de 10 atributos → `ErrTooManyAttributes` antes do envio (`SQS-03b`).
- [ ] **[P0] Publisher SNS com raw delivery verificado**: `NewSNSPublisher`
  consulta, **na construção**, `GetSubscriptionAttributes` de cada assinatura
  declarada no canal e recusa quando `RawMessageDelivery != "true"` (`SQS-02`,
  `TRP-08`); em tópico FIFO recusa assinatura com `FilterPolicy` (`SQS-03c`).
- [ ] **[P0] Consumer com delete depois do commit local**: `Consumer{Channel,
  Sink, VisibilityBase, ProcessingDeadline, HeartbeatEvery}.Run(ctx)`:
  - `ReceiveMessage` com `ApproximateReceiveCount` → `attempt` (`TRP-52`);
    `ProcessingDeadline < VisibilityBase` obrigatório na construção (`SQS-13`).
  - Enquanto o `Sink` executa, `ChangeMessageVisibility` a cada
    `HeartbeatEvery`, nunca além de **12h** do recebimento (`SQS-08`,
    `SQS-08b`); ao atingir o teto, cancela o contexto do `Sink`.
  - `ack.Ack` = `DeleteMessage(receiptHandle)` da **tentativa corrente**
    (`SQS-09`); `ack.Release` = `ChangeMessageVisibility(backoff(attempt))`
    (`SQS-10`), nunca delete.
  - O adapter roda com `MaxAttempts == 0` em canal SQS: a exaustão de D3 é do
    *redrive* gerenciado por `maxReceiveCount` (`SQS-11`, linha 1); D4 e R4 vão
    por publicação explícita seguida de delete (`SQS-11`, linha 2). Os dois
    mecanismos nunca se aplicam à mesma disposição (`SQS-11b`), e a construção
    recusa `Consumer` com `MaxInlineAttempts > 0`.
- [ ] **[P0] DLQ como `Containment`**: `DLQ{Channel}.Quarantine` faz
  `SendMessage` na fila de contenção com o corpo Base64 do envelope original e
  os atributos `dmpf-reason`, `dmpf-consumer`, `dmpf-error`, `dmpf-attempt`
  (≤ 10, `SQS-03b`); em fila FIFO, grupo e dedup id derivados como na publicação.
- [ ] **[P1] Decodificação do corpo**: `DecodeBody(body string) ([]byte, error)`
  desfaz **exatamente um** Base64 (`TRP-19`); corpo que seja um JSON de
  notificação SNS (campos `Type`, `MessageId`, `TopicArn`, `Message`) →
  `ErrSNSEnvelopeNotRaw` — a linha não conforme de §5.2 vira erro nomeado.
- [ ] **[P1] Métricas do pool**: `MET-11` como em Kafka.

#### Transversais

- [ ] **[P0] Composição com `KRN-09`**: toda chamada de saída de cada provider
  passa por `resilience.Compose(sheet, slots)` com a `Sheet` da dependência
  declarada pelo composition root; o provider não redeclara timeout, breaker
  nem retry fora do `Compose`.
- [ ] **[P0] Manifesto e baseline**: cada módulo tem `dmpf-units.json` com a
  unidade e as dependências externas em `external[]` com as quatro chaves; o
  baseline é regravado por `--write-baseline`, nunca à mão.
- [ ] **[P1] Documentação**: `README.md` por módulo, no molde de
  `postgres/README.md`, com o mapeamento do exemplo AsyncAPI de
  FND-06 §16 para `channel.Channel` e a tabela de fórmulas de
  `janela_redelivery` (entregável 3 do ticket).

### Não-funcionais

- [ ] **Conformidade**: `conformance` aprova as cinco unidades novas; o
  `tools/dmpf-cell-check.sh` continua provando as células 26 e 12; nenhum
  provider importa `app` nem `application`.
- [ ] **Cadeia Go verde**: `fmt-check`, `vet`, `lint`, `build`, `test-race`,
  `govulncheck` nos cinco módulos; `test-race` com `-count=1`, `-p 1` e
  `-tags=integration` onde houver infraestrutura.
- [ ] **Testes com infraestrutura**: Kafka e SQS têm testes sob
  `//go:build integration` que exigem `DMPF_KAFKA_BROKERS` e
  `DMPF_SQS_ENDPOINT`; o job `main` do CI sobe **Redpanda** e **floci** (SQS e SNS
  no mesmo endpoint) por `docker run --network container:$(hostname)`, no mesmo
  gesto do Postgres (`ci.yml:44-66`, ADR-035). Filas e tópicos são criados pelo
  próprio teste. Sem a variável, o teste faz `t.Skip` com motivo.
- [ ] **Sem rede em teste unitário**: gRPC por `bufconn`, HTTP por `httptest`,
  Kafka e SQS por fakes das interfaces de cliente que o provider declara.
- [ ] **Segurança**: nenhum byte do payload em log, métrica ou erro
  (`ERR-20`, `ERR-21`); `dmpf-error` na DLQ é o texto sanitizado que o adapter
  já produz.
- [ ] **Compatibilidade**: `google.golang.org/grpc >=1.83.1 <2`, versão já no
  workspace (`observability/go.mod:14`); `franz-go >=1.21.6 <2`;
  `aws-sdk-go-v2/service/sqs >=1.51.0 <2`, `service/sns >=1.46.0 <2`,
  `config >=1.33.3 <2` — versões verificadas em `proxy.golang.org` em
  06/09/2026.

## Camadas afetadas

| Camada | Afetada | O que muda |
| --- | --- | --- |
| `domain` | [ ] | Nada |
| `application` | [ ] | Nada |
| `port` | [ ] | Nada — `Acknowledger` e `Containment` são **realizados**, não alterados |
| `contract` | [ ] | Nada — `envelope.Unmarshal` e `payloadhash.Sum` são consumidos |
| `provider` | [x] | **Cinco módulos novos**: `transport`, `grpc`, `http`, `kafka`, `sqs`; `observability/metrics` ganha `MET-11`/`MET-12` |
| `app` | [ ] | Nada em código; o composition root de exemplo do `KRN-12` é quem ligará provider ao adapter |
| Workspace | [x] | `go.work` (+5 `use`), `tools/dmpf-baseline/units-baseline.json`, `.github/workflows/ci.yml` (+ Redpanda e floci), `AGENTS.md` (inventário de libs) |

## Localização de código

```text
libs/backend/go/transport/                      — NOVO módulo, bloco provider; única dependência externa: API OpenTelemetry (observe)
  deadline/
    deadline.go              — Budget, Outgoing, ErrNoDeadline, ErrDeadlineExhausted
    deadline_test.go
  channel/
    channel.go               — Channel, Ordering, Retry, Catalog, Validate, Resolve
    window.go                — RedeliveryWindow, KafkaWindow, SQSWindow, SNSSQSWindow
    naming.go                — regras KFK-01, KFK-01c, KFK-02 sobre Address
    channel_test.go, window_test.go, naming_test.go
  attempt/
    attempt.go               — Header, Encode, Decode
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/grpc/                  — NOVO módulo
  config.go                  — Config, MethodPolicy, Budgets, ErrTLSRequired, ErrMethodNotDeclared
  dial.go                    — Dial: TLS, service config (round_robin + health), interceptors
  interceptor_deadline.go    — unário e stream: deadline.Outgoing por método
  interceptor_retry.go       — unário: retry por MethodPolicy via observability/retry
  server.go                  — NewServer: TLS, health por serviço, Admission
  admission.go               — interceptor de admissão por método e tenant (MET-12)
  status.go                  — HTTPStatus(codes.Code)
  *_test.go                  — bufconn; two_hops_test.go prova GRP-05/06/17
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/http/                  — NOVO módulo (só stdlib + dmpf-*)
  route.go                   — Route, ErrContractRequired
  client.go                  — Client.Do: timeout do deadline, retry por método
  admission.go               — middleware por rota e tenant (MET-12)
  *_test.go                  — httptest
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/kafka/                 — NOVO módulo
  config.go                  — Config, Observer, erros
  publisher.go               — Publisher (estrutural a relay.Publisher)
  consumer.go                — Consumer.Run: poll, partição→goroutine, rebalance
  acknowledger.go            — realização de ports.Acknowledger por registro
  dlq.go                     — DLQ: realização de ports.Containment
  offsets.go                 — cursor contíguo por partição (TRP-29)
  client.go                  — interface mínima sobre *kgo.Client para fakes
  *_test.go                  — unitário com fake; integration_test.go (//go:build integration)
  hops_test.go               — linhas Kafka da matriz §5.2, par positivo e negativo
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/sqs/                   — NOVO módulo
  config.go                  — Config, erros (ErrMessageTooLarge, ErrTooManyAttributes, ErrSNSEnvelopeNotRaw)
  body.go                    — EncodeBody, DecodeBody (Base64 uma vez; detecção de notificação SNS)
  derive.go                  — MessageGroupId e MessageDeduplicationId (SQS-05, SQS-06, SQS-06b)
  publisher.go               — Publisher SQS (estrutural a relay.Publisher)
  sns.go                     — Publisher SNS com verificação de RawMessageDelivery na construção
  consumer.go                — Consumer.Run: receive, heartbeat de visibilidade, teto 12h
  acknowledger.go            — Ack = DeleteMessage; Release = ChangeMessageVisibility
  dlq.go                     — DLQ: realização de ports.Containment
  api.go                     — interfaces mínimas sobre os clientes sqs/sns para fakes
  *_test.go                  — unitário com fake; integration_test.go (//go:build integration, floci)
  hops_test.go               — linhas SQS/SNS da matriz §5.2, par positivo e negativo
  doc.go, README.md, go.mod, go.sum, project.json, package.json, dmpf-units.json

libs/backend/go/observability/
  metrics/instruments.go     — MODIFICAR: +AdmissionRejections (MET-12), +PoolUtilization, +QueueDepth (MET-11)
  metrics/instruments_test.go — MODIFICAR

go.work                      — MODIFICAR: +5 use
tools/dmpf-baseline/units-baseline.json — REGRAVAR via --write-baseline
.github/workflows/ci.yml     — MODIFICAR: +steps Redpanda e floci; +DMPF_KAFKA_BROKERS, DMPF_SQS_ENDPOINT, AWS_*
AGENTS.md                    — MODIFICAR: inventário de libs (+5), comandos
docs/adr/039-*.md            — NOVO: ADR desta realização (ver Decisões técnicas)
```

**Arquivos a modificar, e o que muda**:

- `libs/backend/go/observability/metrics/instruments.go` — acrescenta os
  três instrumentos que `SPEC-NYD18TGD:185` deixou para esta spec, no catálogo
  único de FND-08 §6; nomes e unidades **exatamente** os de `MET-11` e `MET-12`.
- `go.work` — cinco entradas `use`, em ordem alfabética como as oito atuais.
- `.github/workflows/ci.yml` — dois steps novos após "Subir Postgres", com o
  mesmo padrão de nome por `hostname`, `--network container:$(hostname)` e
  espera ativa; duas variáveis em `env`.
- `AGENTS.md` — seção "Libs" passa de oito para treze módulos; a linha de cada
  provider segue o formato das existentes (unidades, capability, o que o
  `test-race` exige).

## Design

### Arquitetura

```text
                       ┌─────────────────────────────────────────────┐
   bloco app           │  composition root (KRN-12 / serviço)        │
                       │  declara Catalog, Budgets, Sheets, TLS      │
                       │  adapta Sink → app.Consumer.Consume     │
                       └──────┬───────────────────────┬──────────────┘
                              │ relay.Publisher        │ Sink (interface do provider)
                              │ (estrutural)           │ ports.Acknowledger
                              │                        │ ports.Containment
   ┌──────────────────────────▼────────┐   ┌───────────▼───────────────────────┐
   │ grpc   http │   │ kafka   sqs │
   │ Dial/NewServer        Client/Admission │   │ Publisher · Consumer · DLQ · Ack        │
   └──────────┬─────────────────┬──────────┘   └──────────┬────────────────────┬─────────┘
              │                 │                          │                    │
              │      ┌──────────▼──────────────────────────▼──────┐             │
   bloco      │      │ transport                              │             │
   provider   │      │ deadline · channel (ASY-02, janela) · attempt│            │
              │      └──────────────────┬─────────────────────────┘             │
              │                         │                                       │
   ┌──────────▼─────────────────────────▼───────────────────────────────────────▼──┐
   │ observability (KRN-09): resilience.Compose · retry · metrics · clock      │
   └───────────────────────────────────────┬────────────────────────────────────────┘
                                           │
   bloco port / contract      ┌────────────▼──────────────┐   ┌────────────────────────┐
                              │ ports: Acknowledger, │   │ contracts:         │
                              │ Containment, Contained    │   │ envelope.Unmarshal,     │
                              └───────────────────────────┘   │ payloadhash.Sum         │
                                                              └────────────────────────┘
```

Fronteira que o desenho respeita: o `provider` guarda **tudo que é
tecnologia** — cliente gRPC, `net/http`, `kgo.Client`, `sqs.Client`, o
mapeamento de disposição para gesto de broker —, e nada de **processo de
consumo**: quem decide a disposição é o application service (`KRN-07`), quem
ordena "quarantena, depois confirma" é o adapter (`app.Consumer.contain`),
e quem liga um ao outro é o composition root. Nenhum provider importa
`app` nem `application`; as células 26 e 27 da matriz continuam
proibidas e o `dmpf-cell-check.sh` continua a prová-lo.

### Fluxo 1 — governo do tempo em dois saltos (gRPC)

```text
borda: ctx com deadline T0+2000ms
  A ── interceptor ──► Outgoing(ctx, now, Budget{B.Method, Limit 1500ms, Slack 100ms})
        = min(T0+2000−100, now+1500) = T0+1500 → grpc-timeout "1500m" no fio
  B recebe: reconstrói deadline = now_B + 1500ms (GRP-06, nativo do grpc-go)
  B ── interceptor ──► Outgoing(ctx_B, now, Budget{C.Method, Limit 1500ms, Slack 100ms})
        = min(deadline_B − 100, now+1500) = deadline_B − 100 → decresce (GRP-05, GRP-17)
  C recebe deadline_C < deadline_B < deadline_A   ← o teste afirma as duas desigualdades
  cancelamento do cliente em A → ctx.Done() em B e C (GRP-07)
```

O ponto que o teste fixa: `Limit` de `B → C` **maior** que o restante recebido
não estende o prazo — `Outgoing` toma o mínimo. É o anti-padrão de `CTX-19`
tornado impossível pela função, não pela disciplina.

### Fluxo 2 — publicação Kafka

```text
relay.Publish(ctx, "credito.proposta.aprovada", message)
  1. ch := catalog.Resolve(destination)          → ErrUnknownChannel se ausente (TRP-08)
  2. env := envelope.Unmarshal(message)           → só para LER; message não é tocado
  3. rec := Record{Topic: ch.Address, Key: env.PartitionKey, Value: message}
     headers: dmpf-published-at (TRP-12)         — nenhum atributo do envelope (TRP-18)
  4. observer(rec)                                — lê; o tipo não devolve Record (TRP-17)
  5. compose(sheet)(ctx, op, produce)             — timeout/breaker/retry do KRN-09
```

### Fluxo 3 — consumo Kafka e o gesto por disposição

```text
Run(ctx):
  poll → EachPartition → goroutine da partição (KFK-09)
    para cada record, attempt := 1
    loop:
      ack := &acknowledger{partition, offset, cursor}
      err := sink.Handle(ctxPart, record.Value, attempt, ack)   ← adapter decide
      switch ack.gesture:
        acked    → cursor.Mark(offset); commit(cursor.NextContiguous())   (TRP-26, TRP-29)
        released → if attempt == MaxInlineAttempts: break        ← adapter já conteve e confirmou
                   sleep(Backoff(attempt)) respeitando ProcessingDeadline (KFK-19)
                   attempt++; continue                            (TRP-47, KFK-10)
        none     → registro fica pendente; nada é commitado além dele (TRP-29)
  OnPartitionsRevoked/Lost:
    cancel(ctxPart); wait; commit(cursor.NextContiguous()); descarta o resto (TRP-48)
```

`TRP-27` fica inteiro no adapter: `R1×D1`, `R1×D2`, `R2`, `R3` chamam `Ack`;
`R1×D3` chama `Release`; `R1×D4`, `R4` e exaustão chamam `Containment.Quarantine`
(que aqui é a DLQ) **e depois** `Ack`. O provider só traduz `Ack`/`Release`
para o broker.

### Fluxo 4 — consumo SQS

```text
Run(ctx):
  msgs := Receive(WaitTimeSeconds, AttributeNames: [ApproximateReceiveCount])
  para cada msg:
    attempt := ApproximateReceiveCount                    (TRP-52)
    raw, err := DecodeBody(msg.Body)                      (TRP-19; ErrSNSEnvelopeNotRaw → Sink recebe body bruto e o adapter quarantena como envelope inválido)
    heartbeat := every(HeartbeatEvery) ChangeMessageVisibility(VisibilityBase) até 12h (SQS-08, SQS-08b)
    ack := &acknowledger{receiptHandle}
    sink.Handle(ctxMsg, raw, attempt, ack)
      Ack     → DeleteMessage(receiptHandle)               (SQS-09, TRP-26)
      Release → ChangeMessageVisibility(backoff(attempt))  (SQS-10)
    stop(heartbeat)
```

### Onde cada regra é provada

| Regra | Teste | Módulo |
| --- | --- | --- |
| `GRP-04` | chamada sem deadline → `ErrNoDeadline`, servidor nunca invocado | grpc |
| `GRP-05`, `GRP-06`, `GRP-17` | `two_hops_test`: `deadline_C < deadline_B < deadline_A`; `Limit` maior não estende | grpc |
| `GRP-07` | cancelar cliente → handler observa `ctx.Done()` | grpc |
| `GRP-08`, `GRP-09` | método não idempotente com `UNAVAILABLE` → 1 tentativa; idempotente → N | grpc |
| `GRP-11`, `GRP-12`, `GRP-12b` | service config contém `round_robin` e `healthCheckConfig`; servidor responde `SERVING` por serviço | grpc |
| `GRP-14` | tabela completa, `CANCELLED → 499` | grpc |
| `GRP-15` | sem TLS e sem `InsecureForDevelopmentOnly` → `ErrTLSRequired` | grpc |
| `RST-02` | `POST` sem chave → sem retry; com chave → header enviado e retry | http |
| `RST-03` | timeout da requisição igual ao restante menos folga | http |
| `RST-04` | rota sem `ContractRef` → erro na construção | http |
| `RES-16`, `RES-17`, `MET-12` | 429 antes de ler o corpo; contador por rota e tenant | http, grpc |
| `TRP-07`, `TRP-08`, `ASY-01`, `ASY-02` | canal sem item → erro na construção; destino desconhecido → `ErrUnknownChannel` | transport, kafka, sqs |
| `TRP-11`, `KFK-05` | `record.Key == env.PartitionKey`; chave nunca derivada do payload | kafka |
| `TRP-13`, `TRP-17` | `payloadhash.Sum` igual após publicar/consumir; `Observer` não altera bytes | kafka, sqs |
| `TRP-18`, `TRP-20` | headers/atributos só operacionais; hash ignora atributos | kafka, sqs |
| `TRP-19`, `SQS-01` | Base64 exatamente uma vez; duplo Base64 no fixture → hash diverge | sqs |
| `TRP-22` a `TRP-24b` | fórmulas produzem `UpperBound`; parâmetro ausente → `Validate` falha; SNS→SQS soma as duas janelas | transport |
| `TRP-26`, `TRP-29` | interrupção entre `Sink` e commit → nenhum offset além do contíguo; commit é `n+1` | kafka |
| `TRP-28` | `kgo` configurado com `DisableAutoCommit` | kafka |
| `TRP-30`, `KFK-12` | falha injetada entre DLQ e commit → DLQ tem a mensagem, offset não avançou | kafka |
| `TRP-47`, `KFK-10`, `KFK-19` | `Release` reentrega o mesmo registro com `attempt+1`; laço respeita `ProcessingDeadline` | kafka |
| `TRP-48` | revogação durante `Handle` → trabalho cancelado, resto não commitado | kafka |
| `TRP-32`, `KFK-11` | canal ordenado + `separate-channel` → erro | transport |
| `KFK-01`, `KFK-01c`, `KFK-02`, `KFK-04` | nomes inválidos, colisão `.`/`_`, ambiente sem prefixo, partições ausentes | transport |
| `SQS-02`, `SQS-03c` | assinatura sem raw delivery → construção falha; FIFO com filtro → falha | sqs |
| `SQS-03b` | 11 atributos → `ErrTooManyAttributes` | sqs |
| `SQS-05`, `SQS-06`, `SQS-06b` | grupo e dedup id de 64 hex; chaves iguais → grupos iguais; payload diferente → dedup id diferente | sqs |
| `SQS-08`, `SQS-08b`, `SQS-13` | heartbeat observado; teto 12h cancela; `ProcessingDeadline ≥ VisibilityBase` → erro | sqs |
| `SQS-09`, `SQS-10` | `Ack` → delete com o receipt handle corrente; `Release` → visibilidade, nunca delete | sqs |
| `SQS-11`, `SQS-11b` | `MaxInlineAttempts > 0` em SQS → erro na construção | sqs |
| `SQS-12`, `SQS-12b` | corpo acima do limite → `ErrMessageTooLarge`, nada enviado | sqs |
| Matriz §5.2 | `hops_test.go`: par positivo e negativo por linha aplicável | kafka, sqs, grpc |

## Decisões técnicas

- **Cinco módulos, não quatro**: `provider → provider` é célula permitida
  (`matrix.go:52`), e a abstração de prazo é compartilhada por gRPC e HTTP
  enquanto o contrato de canal e as fórmulas são compartilhados por Kafka e
  SQS. Alternativa descartada: colocar as primitivas em `observability` —
  módulo nomeado por FND-08, cuja spec fechou o escopo sem catalogação de canal;
  ou em `ports` — bloco de superfície fechada (treze identificadores,
  `KRN-04`) e capability `pure`, onde `time.Duration` sequer entra.

- **`franz-go v1.21.6` para Kafka**: é o único dos três clientes puros em Go
  com API v1 estável **e** as primitivas que `TRP-47` e `TRP-48` exigem —
  `PauseFetchPartitions`, `SetOffsets`, `OnPartitionsRevoked`/`Lost`,
  `BlockRebalanceOnPoll`, `DisableAutoCommit`, commit por `Offset.At =
  record.Offset+1` (`kadm.NewOffsetFromRecord`). `segmentio/kafka-go` está em
  `v0.4.51` (pré-1.0) e `IBM/sarama v1.60.2` não expõe pausa por partição sem
  contornos. `confluent-kafka-go` exige cgo, que a cadeia Go do workspace não
  prevê.

- **`aws-sdk-go-v2` para SQS e SNS**: é o SDK oficial e o já presente no
  acervo (a análise AS-IS de sistemas legados, inventário de `legado-golibs` e `legado-rendas-services`).
  Versões pinadas por módulo de serviço, como o SDK é publicado.

- **Publisher decodifica o envelope para ler a chave**: `relay.Publisher`
  recebe só destino e bytes; `TRP-11` manda ler a chave **do envelope**. A
  alternativa — mudar a assinatura do `Publisher` para receber a chave —
  reabriria o `KRN-08` e faria o relay conhecer um atributo que `OBX-14` o
  proíbe de interpretar. Decodificar para ler e publicar os bytes originais é
  exatamente a distinção de `TRP-17` ("pode ler, não substituir").

- **DLQ é `ports.Containment`, não um segundo caminho**: o adapter já
  ordena contenção antes de confirmação (`consumer.go:118-129`) e roteia por
  `Reason` (`containment.go`). Realizar a DLQ como `Containment` reaproveita
  essa ordem — `TRP-30` sai de graça — e deixa a fusão DLQ/quarantine de
  `TRP-36` como escolha do canal, não do provider. Alternativa descartada: DLQ
  interna ao provider, acionada por erro do `Sink`, que duplicaria a decisão de
  disposição fora do application service.

- **`Sink` é interface do provider; a ponte é da composition root**: a célula
  `provider → app` é proibida, então o provider não pode produzir
  `app.Delivery`. Declarar a interface no provider e adaptar no `app` é o
  idioma "accept interfaces, return structs" que `relay.Store` já usa no
  sentido inverso.

- **Kafka `Release` reentrega inline; SQS `Release` altera visibilidade**: o
  mesmo `Acknowledger` tem gestos opostos porque `TRP-47` afirma que "em Kafka,
  não commitar não reentrega" e `SQS-10` que "a ausência de delete **é** o
  gesto". O `MaxAttempts` do adapter é `MaxInlineAttempts` em Kafka e **zero**
  em SQS — em SQS a exaustão é do redrive gerenciado (`SQS-11`, linha 1), e
  aplicar os dois mecanismos à mesma disposição produz duplicata na fila de
  contenção (`SQS-11b`).

- **Verificação de raw delivery na construção**: `SQS-02` sem verificação é
  uma linha da matriz que só se descobre em produção. `GetSubscriptionAttributes`
  na construção é o gesto de `TRP-08` ("recusa na construção, não na primeira
  publicação") aplicado ao hop. Custo: uma chamada de API por assinatura ao
  subir; alternativa descartada: confiar na configuração declarada.

- **Sem decodificador AsyncAPI**: FND-06 §16 encaminha a forma do documento a
  ANC-03. Um parser aqui fixaria schema e versão que não são desta âncora e
  traria `yaml.v3` como dependência de um bloco que não precisa dela. O
  `channel.Channel` **é** o conteúdo de `ASY-02`; o README mostra o mapeamento
  a partir do exemplo de §16.

- **Redpanda e floci no CI, nunca LocalStack**: Redpanda é Kafka-compatível
  em um binário sem ZooKeeper e sobe em segundos; floci (`floci/floci`, ~90 MB,
  arranque em milissegundos) emula SQS **e** SNS no mesmo endpoint — FIFO,
  visibilidade, `ApproximateReceiveCount`, redrive, `Subscribe` com
  `RawMessageDelivery` —, o que torna o hop SNS → SQS testável de fato nas duas
  linhas (com e sem raw delivery). LocalStack está vetado neste workspace.
  ElasticMQ foi a primeira escolha e passou no probe, mas não cobre SNS; a
  decisão pelo floci é do usuário (2026-09-06). A validação contra SNS real
  continua com a revisão de infraestrutura de `TRP-41` e com o `KRN-11`.

- **Métricas `MET-11`/`MET-12` no catálogo de `observability/metrics`**:
  FND-08 §6 é um catálogo único; instrumentos declarados em outro módulo
  criariam segunda autoridade sobre nome e unidade. O `KRN-09` os deixou
  nominalmente para esta spec (`SPEC-NYD18TGD:185`).

- **ADR-039 desta realização**: registra as três decisões que FND-06 não
  antecipou — cinco módulos, `Sink`/`Containment` como ponte com o adapter, e
  o gesto assimétrico de `Release` entre Kafka e SQS — mais a pendência de
  `TRP-46`. Segue o molde do ADR-038.

## Regras relacionadas

Cobertas nesta entrega: `TRP-07`, `TRP-08`, `TRP-11` a `TRP-20`, `TRP-22` a
`TRP-24b`, `TRP-26` a `TRP-32`, `TRP-42` a `TRP-44`, `TRP-47`, `TRP-48`,
`TRP-52`, `TRP-53` (por composição, sem tabela própria), `RST-01` a `RST-04`,
`GRP-01`, `GRP-04` a `GRP-18`, `KFK-01` a `KFK-12`, `KFK-19`, `SQS-01` a
`SQS-13`, `ASY-01` a `ASY-04`, `RES-16`, `RES-17`, `MET-11`, `MET-12`.

Declaradas, sem código: `GRP-02` e `GRP-03` (perfis Connect e transcodificação
não são realizados — o módulo gRPC só expõe `application/grpc`, e a ausência do
caminho é a conformidade); `TRP-16` (caminho não conforme não existe para
mensagem de inbox); `TRP-21`, `SQS-12b` (claim-check vedado: `ErrMessageTooLarge`
é o gesto); `TRP-25` (replay operacional é `GAR-09`, do runbook); `TRP-37` a
`TRP-41` (matriz de decisão é norma de desenho); `KFK-04b`, `TRP-10`, `COE-01`
a `COE-08` (migração e coexistência são operação, não código do provider —
`COE-04` é satisfeita pelo adapter, cujo `Name` é o `consumer_name`).

Fora, com destino: `TRP-09`, `TRP-46` (ver "Escopo fora"); `KFK-13` a `KFK-18`
(condicionais a `ADR-DMPF-N`); `TRP-33` a `TRP-36`, `TRP-49` a `TRP-51`, `TRP-54`
(validação do envelope e limites de entrada são do adapter, `KRN-07`, e do
`envelope`, `KRN-05`).

## Verificação e testes

### Critérios de aceite

Os seis primeiros são os do ticket `ARQ-529`, verbatim; os seguintes derivam
das fontes normativas e das divergências reconciliadas acima.

- [x] Uma chamada gRPC com deadline em vigor faz trafegar a duração restante, o
  receptor reconstrói o instante e o prazo não é reiniciado — comprovado em dois
  saltos.
- [x] Método sem semântica idempotente comprovada não recebe retry automático,
  em gRPC nem em HTTP.
- [x] O ACK só sai depois do commit local, e a DLQ é publicada antes do avanço
  do offset — comprovados por testes que interrompem a execução entre os passos.
- [x] O `payload_hash` recomputado após o hop é idêntico ao publicado; um
  interceptor que reserialize e o hop sem *raw message delivery* reprovam.
- [x] Nenhum tópico, fila ou ARN aparece fora da configuração do provider, e
  canal sem catalogação resolvível ou sem `janela_redelivery` com fórmula não é
  operado.
- [x] A desserialização de qualquer contrato funciona sem acesso de rede a um
  registry, e nenhum artefato declara exactly-once fim a fim.
- [x] Nenhum dos cinco módulos importa `app` nem `application`;
  `conformance --root . --base develop` aprova e o `dmpf-cell-check.sh`
  passa.
- [x] `relay.Publisher` é satisfeito por `kafka.Publisher` e `sqs.Publisher` em
  teste de compilação (`var _ interface{ Publish(...) error } = ...`) sem import
  de `app`.
- [x] `ports.Acknowledger` e `ports.Containment` são satisfeitos pelas
  realizações Kafka e SQS.
- [x] Em Kafka, com `enable.auto.commit` desabilitado, um registro cujo `Sink`
  não retornou nunca tem offset commitado, e o commit após três registros
  contíguos é `offset(3)+1`.
- [x] Em Kafka, `Release` reentrega o mesmo registro ao `Sink` com `attempt+1`
  e, esgotado `MaxInlineAttempts`, o adapter contém e o offset avança **depois**
  da DLQ.
- [x] Em Kafka, revogação de partição durante `Handle` cancela o contexto e
  não commita o registro em curso.
- [x] Em SQS FIFO, `MessageGroupId` e `MessageDeduplicationId` têm 64
  caracteres hexadecimais; duas mensagens com o mesmo `id` e payloads
  diferentes têm dedup ids diferentes.
- [x] Em SQS, `Ack` faz delete com o receipt handle da tentativa corrente e
  `Release` altera a visibilidade sem deletar; a visibilidade é estendida
  durante o processamento e cessa às 12h.
- [x] `NewSNSPublisher` recusa assinatura sem `RawMessageDelivery=true` e
  tópico FIFO com `FilterPolicy`.
- [x] Um corpo SQS que seja notificação SNS (sem raw delivery) é reconhecido e
  produz `ErrSNSEnvelopeNotRaw`; um corpo com duplo Base64 produz hash
  divergente.
- [x] Corpo codificado acima do limite do caminho não é enviado
  (`ErrMessageTooLarge`); não existe caminho de claim-check.
- [x] Toda fórmula de `janela_redelivery` produz limite superior fechado, e a
  de SNS → SQS é a soma das duas janelas.
- [x] Canal ordenado com retry em canal separado é recusado na construção.
- [x] Rota HTTP sem `ContractRef` e cliente gRPC sem TLS (sem a opção
  nomeada) são recusados na construção.
- [x] A admissão recusa por rota e tenant antes de ler o corpo e incrementa
  `dmpf_service_admission_rejections_total{route, tenant}`.
- [ ] Cadeia Go verde nos cinco módulos; `test-race` de Kafka e SQS roda no CI
  contra Redpanda e floci; sem as variáveis, faz `t.Skip` nomeando-as.
- [x] `dmpf-units.json` de cada módulo declara as dependências externas com as
  quatro chaves; o baseline é regravado por `--write-baseline`.
- [x] `README.md` de cada módulo existe; o de `transport` traz o
  mapeamento do exemplo AsyncAPI de FND-06 §16 e a tabela de fórmulas.
- [x] ADR-039 registrado e indexado em `docs/adr/README.md`; `AGENTS.md`
  inventaria os treze módulos.

### Cenários de teste

**gRPC — governo do tempo (`two_hops_test.go`)**

- Dado três servidores em `bufconn` encadeados A → B → C, com `Budget{Limit:
  1500ms, Slack: 100ms}` em ambos os saltos e deadline de 2000ms na borda,
  quando A chama B e B chama C, então `deadline_C < deadline_B < deadline_A` e
  `deadline_B − deadline_C ≥ 100ms`.
- Dado `Budget{Limit: 10s}` em B → C e restante de 400ms em B, quando B chama
  C, então o prazo de C é `≤ 300ms` — o `Limit` maior não estende.
- Dado restante em B de 50ms e `Slack` de 100ms, quando B tenta chamar C,
  então `ErrDeadlineExhausted` sem abrir stream.
- Dado contexto sem deadline, quando o cliente chama qualquer método, então
  `ErrNoDeadline` e o servidor não registra invocação.

**gRPC — retry, TLS, health, admissão**

- Dado método `Idempotent: false` e servidor devolvendo `UNAVAILABLE` duas
  vezes, quando o cliente chama, então o servidor registra exatamente 1 invocação.
- Dado método `Idempotent: true, RetryableCodes: [UNAVAILABLE]`, então 3
  invocações e sucesso na terceira; os intervalos crescem e diferem entre si
  (jitter).
- Dado `Config` sem `TLS` e sem `InsecureForDevelopmentOnly`, então `Dial` →
  `ErrTLSRequired`; com a opção, `Dial` sucede e um aviso é registrado.
- Dado `NewServer` com dois serviços, quando um é marcado `NOT_SERVING`, então
  o health responde por nome e o outro segue `SERVING`.
- Dado limite de 2 req/s para `(método, tenant "a")`, quando chegam 3 em 100ms,
  então a terceira recebe `RESOURCE_EXHAUSTED` sem invocar o handler e o
  contador `MET-12` marca 1 com `route`/`tenant`.

**HTTP**

- Dado rota `POST` sem `IdempotencyKey` e servidor `httptest` devolvendo 503,
  então 1 requisição; com `IdempotencyKey: "Idempotency-Key"`, então o header é
  enviado com valor estável e há retry.
- Dado deadline de 800ms e `Budget{Limit: 5s, Slack: 50ms}`, então o
  `httptest` observa a conexão encerrada em ≈750ms.
- Dado `Route{ContractRef: ""}`, então `NewClient` → `ErrContractRequired`.
- Dado middleware de admissão e corpo de 1 MiB, quando a recusa ocorre, então
  o handler não foi invocado e o corpo não foi lido (contador de bytes do
  `io.Reader` é zero).

**`transport` — canal e janelas**

- Dado `Channel` sem `Redelivery`, então `Validate` → erro nomeando o item.
- Dado `Ordering{Unit: partition}` e `Retry{Strategy: separate-channel}`,
  então `ErrOrderedChannelWithSeparateRetry`.
- Dado dois canais `a.b_c.v1` e `a.b.c.v1`, então `Catalog.Validate` reprova
  a colisão (`KFK-01c`).
- Dado `SQSWindow(maxReceiveCount: 5, visibilityBase: 30s, retention: 4d)`,
  então `UpperBound == 150s`; com `retention: 60s`, então `UpperBound == 60s`.
- Dado `SNSSQSWindow(deliveryWindow: 1h, sqs)`, então
  `UpperBound == 1h + sqs.UpperBound`.
- Dado `KafkaWindow` sem `initialOffset`, então `Validate` falha (`TRP-24b`).

**Kafka — unitário com fake**

- Dado `Publish` de um envelope com `PartitionKey: "k1"`, então o registro tem
  `Key == "k1"`, `Value` idêntico byte a byte e `payloadhash.Sum(Unmarshal(Value).Payload)`
  igual ao original.
- Dado um `Observer` que tenta alterar o valor, então o registro publicado é o
  original — o tipo não permite substituição.
- Dado três registros contíguos `Ack`ados, então `commit == offset(3)+1`; com o
  segundo pendente, então `commit == offset(1)+1`.
- Dado `Release` no primeiro `Handle`, então o segundo `Handle` recebe o mesmo
  `Value` com `attempt == 2` após `Backoff(1)`.
- Dado falha injetada entre `Quarantine` (DLQ) e `Ack`, então a DLQ recebeu a
  mensagem e o offset não avançou; na retomada a inbox a veria de novo (R2/R3),
  nunca perdida.
- Dado `OnPartitionsRevoked` durante `Handle`, então `ctxPart.Err() != nil`, o
  registro em curso não é commitado e o commit anterior contíguo permanece.

**Kafka — integração (`//go:build integration`, Redpanda)**

- Publica 100 mensagens com 3 chaves em tópico de 3 partições; consome; a
  ordem por chave é preservada e o hash de cada payload é idêntico.
- Mata o consumidor após 50 `Ack`s sem esperar o commit; retoma; nenhuma
  mensagem perdida e as duplicatas são ≤ o lote em curso.

**SQS/SNS — unitário com fake**

- Dado canal FIFO, então `MessageGroupId == hex(sha256("k1"))` e
  `MessageDeduplicationId` tem 64 hex; mesmo `id` com payload diferente → dedup
  ids diferentes.
- Dado corpo cuja codificação final tem 262 145 bytes, então
  `ErrMessageTooLarge` e o fake não recebe `SendMessage`.
- Dado 11 atributos, então `ErrTooManyAttributes`.
- Dado assinatura com `RawMessageDelivery: "false"`, então `NewSNSPublisher` →
  erro nomeando a assinatura.
- Dado `Body` igual a `{"Type":"Notification","MessageId":"...","TopicArn":"...","Message":"..."}`,
  então `DecodeBody` → `ErrSNSEnvelopeNotRaw`.
- Dado `Body` com Base64 aplicado duas vezes, então o hash do payload recuperado
  diverge do original — a linha negativa de `TRP-19`.
- Dado `Handle` que demora 3 × `HeartbeatEvery`, então o fake registra 3
  `ChangeMessageVisibility` com o mesmo receipt handle; com relógio avançado a
  12h, então o contexto do `Sink` é cancelado.
- Dado `Release`, então `ChangeMessageVisibility(backoff(attempt))` e nenhum
  `DeleteMessage`.
- Dado `Consumer{MaxInlineAttempts: 1}`, então a construção falha (`SQS-11b`).

**SQS/SNS — integração (`//go:build integration`, floci)**

- Fila FIFO com `maxReceiveCount: 2` e DLQ gerenciada: `Sink` que sempre
  `Release`a leva a mensagem à DLQ na terceira entrega, sem `Ack`.
- `Ack` após `Receive`: `ApproximateNumberOfMessages` da fila cai a zero.

**Matriz de hops (`hops_test.go`, por módulo)**

| Linha de §5.2 | Positivo | Negativo |
| --- | --- | --- |
| Kafka, publicação direta | hash igual | — |
| Kafka, interceptor | `Observer` lê, hash igual | fixture com valor reserializado por `proto.Marshal(Unmarshal(v))` de um `Any` reordenado → hash diverge |
| Kafka, headers | atributos do envelope ausentes dos headers | fixture com `partitionkey` em header e ausente do envelope → `Unmarshal` reprova |
| SNS → SQS com raw | corpo Base64 decodifica, hash igual | — |
| SNS → SQS sem raw | — | `ErrSNSEnvelopeNotRaw` |
| SQS direta | Base64 uma vez, hash igual | duplo Base64 → hash diverge |
| Atributos SQS | atributos não entram no hash | — |
| gRPC unário | payload atravessa `bufconn`, hash igual | — |
| Claim-check | — | `ErrMessageTooLarge`, sem caminho de referência |

<critical_constraints>
- [P0] NUNCA reiniciar o deadline recebido: `Outgoing` devolve `min(recebido − folga, orçamento)` e nunca mais que o recebido.
- [P0] NUNCA confirmar no broker antes do retorno do `Sink`; o provider só traduz `Ack`/`Release`.
- [P0] NUNCA desserializar e reserializar o payload; publicar os bytes recebidos, em publicação, retry e contenção.
- [P0] NUNCA importar `app` nem `application` de um provider.
- [P0] NUNCA retentar sem idempotência declarada; lista vazia é zero retry.
- [P0] NUNCA operar canal sem os sete itens de `ASY-02`; recusar na construção.
- [P0] Endereço concreto só na configuração; nenhum artefato declara exactly-once.
- [P1] `auto.commit=false`; commit é o sucessor do último contíguo disposto.
- [P1] Dependência externa nova sempre em `dmpf-units.json` com as quatro chaves.
</critical_constraints>

## Escopo fora

- **Persistência do binding por mensagem (`TRP-09`, `TRP-46`)**: exige coluna
  ou tabela nova em `dmpf_outbox` (`KRN-06`) e o gesto de migração de §15.
  Pendência registrada no ADR-039 com owner (esta linha do kernel) e prazo
  (antes do primeiro canal em coexistência). Enquanto cada canal tiver um
  binding por deploy, `COE-04` absorve a segunda entrega como R2 e o dano se
  limita a ordenação e trabalho.
- **Provisionamento de infraestrutura Kafka**: `TRP-41` — a política vale como
  norma de desenho; este módulo é código, não autorização de operação de canal.
- **Superfície da API REST**: recurso, paginação, corpo de erro, versionamento
  (`TRP-03`, ADR-024).
- **Registry de schemas em runtime**: `KFK-13` a `KFK-18`, condicionais a
  `ADR-DMPF-N`; `ENV-02`/`ENV-23` já garantem desserialização sem rede.
- **Forma do documento AsyncAPI e seu parser**: ANC-03.
- **Perfis Connect e transcodificação gRPC-JSON** (`GRP-02`, `GRP-03`): não
  realizados; a ausência é a conformidade para mensagem de inbox.
- **Claim-check** (`TRP-21`, `SQS-12b`): vedado até nova major do perfil.
- **CDC, replay operacional (`TRP-25`), runbook e limiares de FND-08**.
- **Composition root de exemplo ligando provider → `app.Consumer` →
  Postgres**: é do `KRN-12`; esta spec entrega os fakes e a interface `Sink`
  que ele usará.
- **Test kit de conformidade de provider e vetores para o catálogo**: `KRN-11`
  recebe os `hops_test.go` como insumo.
- **Migração do acervo SQS para Kafka**: adoção organizacional, ANC-09.
