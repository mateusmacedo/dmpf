# transport

Primitivas de transporte compartilhadas pelos quatro providers do kernel DMPF
(FND-06, `docs/dmpf/politicas-transporte.md`): o orçamento de prazo por método
(`deadline`), o contrato de catalogação de canal com as fórmulas de
`janela_redelivery` (`channel`), o metadado de tentativa lateral ao envelope (`attempt`), a
admissão por rota e tenant (`admission`, RES-16/RES-17), as posições de
observabilidade de toda composição (`observe`) e a montagem da composição de
RES-22 que os quatro providers repetiam (`compose`).

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`).

## Por que um módulo próprio

gRPC e HTTP compartilham a abstração de prazo; Kafka e SQS/SNS compartilham a
catalogação de canal e as fórmulas de janela. Nenhum dos quatro providers é
lugar neutro para isso, e `observability` é nomeado por FND-08, não por
transporte. O módulo é bloco `provider`, sem I/O. A única dependência externa é a API do OpenTelemetry (`otel/trace`, `otel/metric`), que o package `observe` usa para os três decorators que RES-23 exige de toda composição.

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/transport` | `provider` | raiz do módulo, `deadline`, `channel`, `attempt`, `admission`, `observe`, `compose` |

## Packages

| Package | Conteúdo | Fase do plano |
| --- | --- | --- |
| `deadline` | `Budget`, `Require`, `Outgoing`, `ErrNoDeadline`, `ErrDeadlineExhausted` | 1.2 |
| `channel` | `Channel`, `Catalog`, `RedeliveryWindow`, `KafkaWindow`, `SQSWindow`, `SNSSQSWindow` | 1.3 |
| `attempt` | `Header`, `Encode`, `Decode`, `WithContext`, `FromContext` — o consumidor grava a tentativa no contexto antes de `Sink.Handle`, e a DLQ a lê para o header ou atributo `dmpf-attempt` (TRP-52) | 1.4 |
| `observe` | `Config`, `Slots`, `Tracing`, `Metrics`, `Logging` — as posições de observabilidade de RES-22 que o KRN-09 deixa ao chamador; a categoria de falha é o único ponto por transporte. Os conjuntos de atributos são memorizados por `(método, categoria)` — espaço limitado por MET-07, com teto de 4096 entradas — para o caminho quente não os reconstruir e reordenar a cada chamada | 3.2 |
| `admission` | `Limit`, `Config`, `Controller.Admit` — bucket por `(rota, tenant)`, teto de chaves (`MaxKeys`) e evicção LRU do ocioso | 2.1b |
| `compose` | `Config`, `Build`, `Shared`, `Retry`, `Operation` — a ordem dos decorators de RES-22 (observe → breaker → bulkhead → timeout → retry), `Timeout` reservando `Backoff.Base`, `Retry` declarado `false` como identidade, `RateLimit` recusado; o provider passa só a sheet, o classificador e a categoria de falha | code review |

## Admissão: bucket real por tenant, allowlist só no rótulo

`Controller.Admit(route, tenant)` chaveia o bucket pelo par `(route, tenant)`
literal — todo tenant, declarado na allowlist ou não, tem o próprio bucket, e o
esgotamento de um não recusa nenhum outro. A allowlist de `Config.Tenants`
(`metrics.Tenants`) não colapsa a chave do bucket: ela só limita a
cardinalidade do **rótulo de métrica** (`MET-07`) — um tenant fora dela
continua com bucket próprio, mas aparece na métrica como `"other"` via
`Tenants().Resolve(tenant)`. Quem impede a explosão de chaves é `MaxKeys`, com
evicção LRU do bucket ocioso (`DefaultMaxKeys = 64`, `NewController`). Uma
allowlist vazia é configuração válida — todo tenant cai em `"other"` no
rótulo, sem que isso afete a admissão.

## Do documento de canal ao `channel.Channel`

FND-06 §16 fixa **o que** o provider precisa encontrar na catalogação de um canal
(`ASY-02`) e dá um exemplo AsyncAPI ilustrativo. Este módulo não lê AsyncAPI: o
`channel.Channel` é o conteúdo de `ASY-02` em processo, montado pelo composition
root a partir do documento, campo a campo:

| Item de `ASY-02` | No exemplo AsyncAPI de §16 | Em `channel.Channel` | Validação |
| --- | --- | --- | --- |
| Endereço concreto e transporte | `channels.propostaAprovada.address`, `bindings.kafka` | `Address`, `Transport` (`kafka`, `sqs`, `sns-sqs`) | não vazio; Kafka: alfabeto, 249 caracteres, prefixo de ambiente (KFK-01/01c/02) |
| Tipo de evento e major do contrato | `components.messages.*.payload.schema.$ref` | `EventType`, `ContractMajor`, `ContractRef` | os três presentes; o contrato é referenciado, nunca copiado (ASY-03) |
| Chave de ordenação, ou a declaração de que não há | `x-dmpf.chaveOrdenacao` | `Ordering.Key` | `Unit == none` exige `Key == ""`; unidade exige chave (KFK-06, SQS-04) |
| Unidade de ordenação | `x-dmpf.unidadeOrdenacao` | `Ordering.Unit` (`partition`, `group`, `none`) | Kafka ordena por partição, SQS por grupo |
| `janela_redelivery` e a sua fórmula | `x-dmpf.janelaRedelivery` | `Redelivery` (um `RedeliveryWindow` dos construtores abaixo) | `Formula` conhecida, parâmetros completos, `UpperBound > 0` (TRP-23, TRP-24, TRP-24b) |
| Destino de contenção | `x-dmpf.contencao` | `Containment` | não vazio (KFK-12, SQS-11) |
| Estratégia de retry e o efeito sobre a ordem | `x-dmpf.retry` | `Retry{Strategy, MaxAttempts}` | limite positivo (TRP-31); canal ordenado não usa `separate-channel` (TRP-32, KFK-11) |
| Topologia Kafka | `bindings.kafka.partitions`, `operations.*.bindings.kafka.groupId` | `Partitions`, `Partitioner`, `KeyEncoding`, `Group` | os quatro presentes (KFK-04, KFK-07) |

O nome lógico (`Name`) é a chave do `Catalog`: o `Resolve(destino)` do provider
devolve `ErrUnknownChannel` para o que não está catalogado — o canal não
catalogado não é operado (ASY-01). Um `Catalog.Validate` também reprova dois
endereços Kafka que difiram só por `.`/`_` (KFK-01c).

## Fórmulas de `janela_redelivery`

`TRP-22` distingue três relógios — lease de claim (do relay, FND-04), intervalo
entre tentativas e **horizonte de redelivery** — e `TRP-23`/`TRP-24` exigem que
o canal declare o horizonte com a sua fórmula e parâmetros nomeados. Os três
construtores de `channel` produzem o `RedeliveryWindow` que `INB-14`
(`retenção_inbox >= janela_redelivery`) verifica:

| Transporte | Construtor | Parâmetros obrigatórios (TRP-24b) | `UpperBound` |
| --- | --- | --- | --- |
| Kafka | `KafkaWindow(KafkaRetention{...})` | `RetentionByTime`, `RetentionBySize` (−1 = sem limite), `CleanupPolicy`, `RemoteStorage`, `InitialOffset` | retenção por tempo; `SizeHorizon`, quando o operador o mediu menor, prevalece. Política sem `delete` não expira registro → bound 0 → `Validate` recusa |
| SQS | `SQSWindow(maxReceiveCount, visibilityBase, retention)` | os três | `min(maxReceiveCount × min(visibilityBase, 12h), retention)` — o teto de 12h é `SQS-08b` |
| SNS → SQS | `SNSSQSWindow(subscriptionDeliveryWindow, sqs)` | a janela da assinatura e um `SQSWindow` válido | `subscriptionDeliveryWindow + sqs.UpperBound` — a soma das **duas** janelas; só a da fila subestima (TRP-24) |

Exemplos: `SQSWindow(5, 30s, 4d)` → 150 s; com `retention` de 60 s → 60 s;
`SNSSQSWindow(1h, SQSWindow(5, 30s, 4d))` → 1 h 2 min 30 s; `KafkaWindow` com
`RetentionByTime: 7d, CleanupPolicy: "delete"` → 7 d. Replay operacional
(`TRP-25`) não entra em nenhuma das três.

## Referências

- `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md` — spec do `KRN-10`
- `docs/adr/039-providers-de-transporte-sink-e-gesto-de-release.md` — decisões deste módulo
- `docs/adr/051-escopo-de-tenant-por-choke-point-em-go.md` — o tenant que chaveia o bucket de admissão
- `libs/backend/go/ports/README.md` — `ExecutionContext.Tenant()`, a fonte do tenant que a borda passa a `Admit`
