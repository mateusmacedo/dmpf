# app

Bloco `app` do kernel DMPF: o consumer adapter que fica entre a entrega do transporte e o application service de consumo (FND-04 §6.3, passos 1, 2 e 7). Primeiro módulo do bloco `app` no workspace.

Criado por `KRN-07` (ARQ-526, `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md`; decisões em `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md`). A reconstrução do contexto de execução no adapter é da SPEC-9B6SHEH8 (ADR-049, ADR-052).

## Por que um módulo próprio

O adapter precisa de duas arestas ao mesmo tempo: `contract` (decodificar o envelope e calcular o `payload_hash`) e `application` (invocar o caso de uso). `application → contract` (célula 12) e `provider → application` (célula 26) são proibidas pela matriz de blocos e têm vetor negativo em `tools/dmpf-cell-check.sh`. Só a linha `app` permite ambas.

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/app-consumer` | `app` | raiz do módulo |
| `kernel/app-relay` | `app` | `relay` — o relay da outbox de `KRN-08` (ADR-038) |

A composition root do consumidor de exemplo, que este módulo carregava em
`example/reservations`, é hoje o bloco `app` do contexto `reservations` — o
package raiz de `apps/backend/reservations` (unidade `reservations/app`,
ADR-046). Aqui fica só o que todo contexto consumidor importa.

Desde o `KRN-12` (ADR-041), `Consumer.Consume` põe no contexto do `Handler` o contexto de mensagem da consumição — o `correlationid` do envelope, o `id` recebido como `causationid` e o `traceparent` e o `tracestate` do span `process` corrente, não os do envelope — por `ports.WithMessageContext`, para que o application service de consumo o copie em cada `OutboxEntry` que autorar (FND-07 §8.6 item 3): a linha de outbox escrita durante o consumo carrega o contexto do consumo (RF-B9). Quem cabeia este adapter e o relay em processos reais são os composition roots `apps/backend/reservations` (relay e consumer) e `apps/backend/orders` (relay), conforme o ADR-044.

O módulo não declara dependência externa (`external: []`). O código de produção importa a API do OpenTelemetry — `otel`, `otel/attribute`, `otel/trace`, `otel/metric`, `otel/log`, `otel/baggage`, `otel/propagation` e as constantes de `semconv/v1.43.0`, com `messagingconv` — para os spans, as métricas de mensageria e os registros do relay e do consumo, e não importa o SDK; o relay importa ainda `pgx/v5/pgxpool` (`NewOverPostgres`) e `protobuf/types/known/timestamppb` (`record.go`). O adapter não importa `google.golang.org/protobuf` — a decodificação de wire fica em `contracts/envelope.Unmarshal`, e o adapter recebe os bytes brutos em `Delivery.Raw`.

## O adapter

`Consumer` declara o que a spec exige (`ErrIncompleteConsumer` recusa a partida sem qualquer um): `Name`, `MaxAttempts` (`<= 0` desliga o teto), `Handle` (o `Handler` — o application service de consumo), `Containment`, `Clock`, `Timeout` (o orçamento de prazo por tentativa, `CTX-28`), `Boundary` e `Locale` (a declaração de `CTX-01` que uma consumição não tem caller para fornecer).

`Consumer.Consume(ctx, Delivery, Acknowledger)` realiza os passos 1, 2 e 7 da sequência de consumo (FND-04 §6.3):

1. `envelope.Unmarshal(Raw)` valida o envelope. Envelope inválido **não** recebe classificação de recepção: vai para a contenção e o `Handler` nunca roda (`INB-10`).
2. `Boundary.admits(env.Source)` recusa a mensagem cuja origem não está na fronteira confiável declarada — `Boundary{Transport, Sources}`, onde `Transport` é `verified` (o broker autentica o workload produtor, mTLS/SASL, ADR-052) ou `development-only` (o opt-out de desenvolvimento do transporte). Fora da fronteira vai para a contenção como `untrusted-boundary`, sem produzir contexto (`CTX-27`, `IDN-04`).
3. `executionOf` reconstrói o `ports.ExecutionContext` da consumição a partir do envelope: tenant, correlação, causação e trace vêm de `env` (`CTX-24`); **nenhum sujeito** entra, porque proveniência não é identidade (`CTX-25`); `RequestID` e `Deadline` são do próprio consumidor — cunhados por tentativa (`Attempt`) e derivados de `Timeout` — porque a consumição é a execução do consumidor, não a do produtor (`CTX-28`). Se a reconstrução falhar, a mensagem é contida como `invalid-envelope` em vez de subir o erro sozinho, o que a deixaria para reentrega eterna.
4. Monta o `Receipt` com `payload_hash = payloadhash.Sum(Payload)` sobre os bytes transportados (`ENV-18`), deposita o contexto de execução e o de mensagem no `ctx` (`ports.WithExecutionContext`, `ports.WithMessageContext`) e invoca o `Handler`, que devolve a disposição. Um panic do `Handler` vira o `Unexpected` não retentável que `application.Classify` resolve para R1×D4, sem o valor do panic nem a pilha: a mensagem é contida como `terminal-failure` e confirmada, com o mesmo `message consumed`, o mesmo `process` e o mesmo gesto de um `Handler` que devolve `Failure(Unexpected)`, acrescidos só de `dmpf.panic.type`, o tipo Go do valor recuperado (`string`, `*errors.errorString`, `runtime.boundsError`), e nada sai no stderr (`ERR-22`, `ERR-23`, `ERR-24`, RF-A1). Um panic na quarentena ou no ACK/Release vira o mesmo `Unexpected`: o `Consume` o devolve como erro, com o `Unexpected` à frente da causa do `Handler` (que segue na árvore, para `errors.Is`), e o registro do worker Kafka ou SQS leva `error.type` = `Unexpected`, sem confirmar mensagem cuja quarentena não concluiu, e o `message consumed` e o `process` saem como falha, com o mesmo `dmpf.panic.type`.
5. Aplica o efeito de broker **depois** do retorno do `Handler` (`INB-08`), conforme a tabela abaixo.

| Disposição | Efeito no broker | Contenção |
| --- | --- | --- |
| R1×D1, R1×D2, R2, R3 | `Ack` | — |
| R1×D3 com `Attempt < MaxAttempts` | `Release` (redelivery a cargo do transporte) | — |
| R1×D3 com `Attempt ≥ MaxAttempts` | `Ack` após conter | `attempts-exhausted` (`GAR-08`) |
| R1×D4 | `Ack` após conter | `terminal-failure` |
| R4 | `Ack` após conter | `collision` |
| envelope inválido | `Ack` após conter | `invalid-envelope` |
| origem fora da fronteira | `Ack` após conter | `untrusted-boundary` |

A contenção sempre grava `Delivery.Raw` — nunca um re-marshal do envelope decodificado, que descartaria campos desconhecidos e quebraria a identidade byte a byte exigida por `GAR-07`. Se a quarantine falhar, a mensagem **não** é confirmada: contida ou nada.

`Attempt{RequestID, Number}` (`attempt.go`) é o identificador que o adapter cunha por tentativa e a contagem de entrega do transporte; nenhum dos dois vem do envelope. Só o `Handler` o lê, para montar o contexto de execução — dali para baixo o contexto viaja pelo `ctx`, nunca por parâmetro explícito (`CTX-03`).

Depois de `Boundary.admits`, a consumição abre a raiz `process {messaging.destination.name}` CONSUMER (RF-B9), com link ao contexto de criação do envelope e o nome do endereço físico de `Channel.Address`, não o do canal lógico. O span leva `messaging.system` (de `System`), `messaging.operation.type=process`, `messaging.consumer.group.name` (de `Channel.Group`), `messaging.message.id`, `messaging.message.conversation_id`, `cloudevents.event_id`, `cloudevents.event_source`, `cloudevents.event_type`, `dmpf.inbox.attempt`, o `dmpf.request_id` próprio da tentativa e o `dmpf.tenant_id` do envelope e, ao fim, `dmpf.inbox.disposition`, `dmpf.inbox.gesture`, `dmpf.containment.reason` quando contida e `dmpf.outcome_category`. O baggage de execução entra depois do `Start`, e por isso o `process` não leva `dmpf.correlation_id` — os registros e os spans descendentes levam (RF-B8). Fora da fronteira, a raiz é nova, com link ao contexto recebido e classe `error`, para que o link não decida a amostragem (`TRC-07`). Cada mensagem sai num `message consumed`, depois do gesto, com as chaves de mensageria, `dmpf.inbox.attempt`, `dmpf.inbox.disposition`, `dmpf.inbox.gesture` e `dmpf.outcome_category`, no nível do outcome FND-07 da falha: `info` no aceite e na recusa de negócio, `warn` na recusa de autorização, `error` na falha técnica ou com o gesto em erro (RF-A5); a duração vai a `messaging.process.duration`, com `error.type` na falha (RF-D2). R1×D3 liberado para reentrega é o caminho saudável do retry e não marca erro. `Tracer`, `MeterProvider`, `LoggerProvider`, `System` e `Channel` são opcionais: sem `Tracer`, a consumição fica sem span, e sem provider vale o global.

## Mapeamento situação → mecanismo (`GAR-11`)

Declarado em código como dado revisável (`ContainmentMap`) e conferido por teste de totalidade.

| Situação (`ports.Reason`) | Mecanismo nesta entrega | Quando o `KRN-10` existir |
| --- | --- | --- |
| `invalid-envelope` | quarantine | quarantine |
| `untrusted-boundary` | quarantine | quarantine |
| `terminal-failure` (R1×D4) | quarantine | quarantine |
| `collision` (R4) | quarantine | quarantine |
| `attempts-exhausted` (R1×D3 esgotado) | quarantine | dead-letter queue do transporte |

Quarantine e DLQ são mecanismos distintos: a primeira retém para inspeção sem reentrega automática; a segunda é o destino terminal do transporte. O `KRN-07` realiza apenas a quarantine (sobre Postgres, em `postgres`); a DLQ depende de broker e é do `KRN-10`.

## A política de idempotência e a purga

`IdempotencyPolicy(wait, retention)` (`idempotency.go`) é a política que o
composition root entrega ao application service dos comandos: o teto de espera
(`IDM-07`), a retenção da entrada (`IDM-09`) e o SHA-256 que transforma o
fingerprint canônico no que a inbox compara (`IDM-04`). Fica neste bloco porque o
`depguard` do bloco `application` não admite `crypto/sha256`. Espera ou retenção
não positiva é `ErrInvalidIdempotencyPolicy`.

`RunPurge(ctx, cfg, clock, logs, fn)` (`purge.go`) é o laço que mantém uma
tabela do kernel dentro da retenção, no processo dono dela: `serve-api` purga as
entradas de comando vencidas, `serve-relay` a outbox publicada e
`serve-consumer` as entradas de mensagem da inbox. `logs` é o
`log.LoggerProvider` do processo, sobre o qual o laço cria o próprio logger. A
cada `Interval`, o laço chama `fn` com o corte `agora − Retention` e o `Batch`,
e um lote cheio é seguido do próximo na hora; cada lote removido sai em
`purged`, com `dmpf.purge.name`, `db.collection.name`, `dmpf.purge.removed` e
`dmpf.purge.before`. Uma falha é registrada em `purge cycle failed`, com
`dmpf.purge.name`, `db.collection.name` e o erro por `redact.Error`, e tentada
de novo no intervalo seguinte. `Name` identifica o laço e vai em
`dmpf.purge.name`; `Table` é a tabela que ele apaga e vai em
`db.collection.name`, o mesmo valor do span `DELETE` da purga. Sem `Table`, o
registro sai sem `db.collection.name`, nunca com o nome do laço no lugar. O laço
devolve `nil` quando o contexto termina, `ErrInvalidPurgeConfig` para
configuração que ele não conseguiria rodar e `ErrPurgePanicked`, sem o valor do
panic, quando `fn` ou o próprio laço entram em pânico, em vez de derrubar o
processo pela pilha do runtime.
`StartPurge(ctx, abort, cfg, clock, logs, fn)` roda o mesmo laço numa goroutine
própria e devolve `stop`, que só retorna depois que o laço terminou: o
composition root o chama antes de fechar o pool que a purga usa. O erro que
encerra o laço vai a `abort`, o `context.CancelCauseFunc` do contexto em que o
papel roda, para que o laço principal do papel (servidor gRPC, relay ou
consumer) termine, e `stop` o devolve, para que o `Run` da app o entregue ao
`cmd/main.go`, que o escreve no stderr e sai com código 1 (RF-A1). Sem `abort`,
`StartPurge` recusa com `ErrInvalidPurgeConfig`.

## O relay

`relay/postgres.go` traz `NewOverPostgres(pool, publisher, component, config)`, que monta o dreno de FND-04 §5.4 sobre a outbox de `postgres` com a identidade do kernel (`idclock.SystemClock`, `idclock.NewClaimIDs(component)`) — antes, `orders` e `reservations` repetiam essa montagem e cada um trazia o próprio `RandomClaimIDs`. O relay drena de todos os tenants sem distinção: a outbox fica fora do escopo de tenant (ADR-050), e é `record.go` quem lê o `tenantid` da `metadata` que `postgres` escreveu e o coloca no envelope publicado, para que o consumidor o leia de volta em `executionOf`. O `traceparent` e o `tracestate` da `metadata` vão ao envelope sem alteração: são o contexto de criação da mensagem (`ENV-08`).

Depois de um claim não vazio, o relay abre a raiz `outbox drain {destino}` INTERNAL (só `outbox drain` num lote de destinos mistos), com início no instante do claim, o evento `claimed`, `messaging.batch.message_count`, `dmpf.outbox.claim_id` e um link por mensagem ao contexto de criação, com `messaging.message.id` no link; um contexto ilegível na `metadata` custa só o link e deixa o evento `invalid_creation_context` (RF-B7). Para cada mensagem, abre `send {messaging.destination.name}` CLIENT, filho do drain, com link ao contexto de criação, o tópico físico de `Config.Address`, `messaging.system` de `Config.System`, `messaging.operation.type=send`, `messaging.message.id`, `messaging.message.conversation_id`, `cloudevents.event_id`, `cloudevents.event_source`, `cloudevents.event_type`, `dmpf.tenant_id`, um `dmpf.request_id` próprio e `dmpf.outbox.attempt`, a partir de 1. O `send` é o span dono da publicação (`tracing.WithOwnedSpan`): a composição do publisher grava nele a resiliência, em vez de abrir `dmpf.resilience`. A tentativa falha sai em `outbox publish failed`, `warn`, com `messaging.message.id`, `dmpf.outbox.attempt` e `error.type`, e a falha de claim, em `outbox claim failed`, `error`. O `send` grava `messaging.client.operation.duration` e, quando a publicação chega a ser tentada, `messaging.client.sent.messages`, com `error.type` na falha — o registro que falha antes, na montagem do envelope, não conta como enviado (RF-D2). Span, log e métricas do mesmo `send` levam um único `error.type`: o que a composição do publisher gravou no span, quando gravou, e o do próprio erro nos demais casos. `Config.Tracer`, `MeterProvider` e `LoggerProvider` são opcionais: sem `Tracer`, o relay fica sem span, e sem provider vale o global.

Um panic no laço não encerra o processo pela pilha do runtime. Na entrega de uma mensagem, as demais do scan terminam; num panic na abertura do drain ou na entrega, os claims não concluídos voltam ao pool (OBX-13), e o `outbox drain` termina mesmo quando o panic vem da devolução; em qualquer desses pontos e no claim, `Run` devolve só `ErrPanicked`, sem o valor do panic, que o `cmd/main.go` de cada app escreve no stderr como erro de término antes de sair com código 1 (ERR-20, ERR-22, ERR-23, RF-A1). A devolução que o banco recusa por erro, e não por o claim já ter sido substituído (OBX-10), sai em `warn` como `outbox release failed`, com `messaging.message.id` e o erro por `redact.Error`.

## Referências

- `docs/specs/SPEC-ANZX2WPG-dmpf-inbox-consumo.md` — spec do `KRN-07`
- `docs/adr/036-classificacao-de-recepcao-e-fronteira-pending.md` — decisões do adapter e da inbox
- `docs/adr/038-drenagem-da-outbox-lease-e-envelope-na-publicacao.md` — o relay
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier que `executionOf` popula
- `docs/adr/050-tabelas-de-infraestrutura-fora-do-escopo-de-tenant.md` — por que o relay drena sem tenant
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — `Boundary.Transport` e a fronteira confiável
- `docs/specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md` — os `RF-*` citados aqui: `outbox drain`, `send`, `process`, `message consumed` e as métricas de mensageria
- `libs/backend/go/ports/README.md` — `ExecutionContext`, `Acknowledger`, `Containment`, `MessageContext`
- `libs/backend/go/postgres/README.md` — a outbox, a inbox e a quarantine que este adapter usa
