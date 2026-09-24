# kafka

Bloco `provider` do kernel DMPF para o transporte-alvo do evento de domínio em
Kafka (FND-06 §11, ADR-025), sobre `franz-go`: chave do registro = chave de
partição do envelope, valor byte a byte, ACK por offset contíguo depois do
commit local, retry inline com limite, DLQ publicada antes do avanço do
offset, e autenticação de cliente por SASL ou certificado (ADR-052).

Criado por `KRN-10` (ARQ-529, `docs/specs/SPEC-EAGAXQN1-dmpf-providers-transporte.md`);
a autenticação de cliente é de `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md`
(ADR-052).

## Unidades do manifesto

| Unidade | Bloco | Package |
| --- | --- | --- |
| `kernel/provider-kafka` | `provider` | raiz do módulo |

Dependências externas declaradas: `github.com/twmb/franz-go` `>=1.21.6`
(`pkg/kgo`, `pkg/kerr`, e `pkg/sasl`/`pkg/sasl/scram` para o SASL; `io.messaging`),
`go.opentelemetry.io/otel/trace` e `go.opentelemetry.io/otel/metric`
(`observability`). `pkg/kadm` é módulo separado e entra **só nos testes de
integração**, para criar os tópicos.

## O que o módulo contém

- **`config.go`** — `Config{Brokers, TLS, SASL, InsecureForDevelopmentOnly,
  Catalog, Sheet, Service, Clock, Tracer, Instruments, Logger, Rand}`.
  `Validate` recusa TLS que não verifica o par ou admite versão abaixo de 1.2
  (`ErrTLSTooWeak`), TLS ligado sem `SASL` e sem certificado de cliente
  (`ErrClientAuthRequired`), mecanismo SASL fora de SCRAM-SHA-256/512
  (`ErrSASLMechanism`) e SASL sem usuário ou senha (`ErrSASLCredentials`).
  `ClientAuthenticated()` reporta se o broker está verificado por TLS **e**
  este cliente se autentica a ele — o único chão sobre o qual uma fronteira de
  consumo pode se chamar verificada (IDN-03, IDN-04; ver `app.TransportVerified`
  em `libs/backend/go/app`). Todo nome de tópico vem do `channel.Catalog`
  (TRP-07, ASY-01): `Channel(destino)` recusa destino desconhecido e canal de
  outro transporte.
- **`auth.go`** — `SASL{Mechanism, Username, Password}` (`ScramSHA256`,
  `ScramSHA512`) e `ClientAuth{SASL, CertFile, KeyFile, CAFile}`: o que um
  processo declara sobre a própria identidade perante o broker — um principal
  SASL ou um certificado de cliente — mais a autoridade do certificado do
  broker quando ele é privado. `ReadClientAuth(lookup)` lê
  `DMPF_KAFKA_SASL_MECHANISM`, `DMPF_KAFKA_SASL_USERNAME`,
  `DMPF_KAFKA_SASL_PASSWORD`, `DMPF_KAFKA_CLIENT_CERT_FILE`,
  `DMPF_KAFKA_CLIENT_KEY_FILE` e `DMPF_KAFKA_CA_FILE` do ambiente, para que os
  três processos que falam com o Kafka (`api`, `relay`, `consumer`) leiam a
  declaração do mesmo jeito. A ACL do broker que liga o principal ao `source`
  que o consumer admite é da plataforma, não deste módulo (ADR-052).
- **`newconfig.go`** — `NewConfig(ctx, rt, catalog, brokers, service,
  insecure, auth)`: monta o `Config` de um processo a partir do runtime de
  observabilidade e da declaração de `ClientAuth`, com TLS 1.2+ como default —
  o opt-out (`DMPF_KAFKA_INSECURE`) só serve desenvolvimento e CI, e o log
  registra quando ele é usado.
- **`client.go`** — a interface `client` sobre `*kgo.Client`, o que permite o
  fake dos testes. O commit é `CommitRecords`: síncrono, commita `offset + 1`
  do registro (TRP-29) e não exige os tipos `kmsg` que `CommitOffsets` exige.
- **`publisher.go` / `record.go`** — `Publisher.Publish(ctx, destino, bytes)`
  realiza estruturalmente o `Publisher` do relay: `Key` = `PartitionKey` do
  envelope (KFK-05), `Value` = os bytes recebidos (TRP-13), header operacional
  `dmpf-published-at` (TRP-18). O `Observer` recebe um `Record` com cópias —
  pode ler, não pode substituir (TRP-17). Produção pela composição de RES-22
  (`transport/compose`), com acks de todas as réplicas em sincronia e
  escrita idempotente.
- **`consumer.go` / `worker.go`** — `Consumer.Run` entra no grupo do canal com
  `DisableAutoCommit` (TRP-28) e `BlockRebalanceOnPoll`. **Um worker
  persistente por partição** (KFK-09): o laço de poll só enfileira — sem
  bloquear, para nunca segurar a rebalance — e, quando a fila do worker atinge
  `QueuePerPartition`, pausa o fetch da partição até ela drenar; o worker
  processa em ordem, com deadline por tentativa e a tentativa gravada no
  contexto (`attempt.WithContext`). `Ack` marca o registro e commita o
  **prefixo contíguo** — um pendente no meio fixa o teto (TRP-29). **Só
  `Release`** repete: pausa a partição, espera `Backoff.Next`, retoma e tenta
  de novo com `attempt + 1` (KFK-10, TRP-47), até `Channel.Retry.MaxAttempts`.
  Retorno **sem gesto** (erro sem `Ack`/`Release`, ou nada) e limite atingido
  têm o mesmo desfecho: o registro fica pendente e a partição **para**
  (`stalled`, pausada, com log de erro dizendo o offset e quantos registros
  esperam atrás) — processar o próximo aplicaria efeitos fora de ordem; só a
  revogação devolve a partição. Um panic do `Sink` é tratado como tentativa sem
  gesto (`ErrSinkPanicked`, categoria `panic`), nunca como queda do consumer.
  Revogação ou perda da partição fecha o commit do worker, cancela a tentativa
  em curso sem drenar a fila pelo `Sink`, espera o worker sair, **levanta a
  pausa de fetch** — o kgo a mantém entre rebalances — e commita só o que já
  estava contíguo (TRP-48); o encerramento drena todos os workers sob
  `RebalanceTimeout`. Erro devolvido junto do `Ack` é logado com categoria.
  `MET-11` por poll: utilização = workers ocupados / partições atribuídas;
  profundidade = soma das filas.
- **`offsets.go`** — o `cursor` contíguo. **`acknowledger.go`** — o gesto
  terminal por entrega (`Ack` | `Release`; a segunda chamada é
  `ErrAlreadyDisposed`, TRP-26/27).
- **`dlq.go`** — `DLQ` realiza `ports.Containment`: publica o envelope
  **intacto** no `Containment` do canal (KFK-12, GAR-07), com a chave de
  partição quando ele ainda decodifica e os headers `dmpf-reason`,
  `dmpf-consumer`, `dmpf-message-id`, `dmpf-contained-at`, `dmpf-error` e, quando
  o contexto veio do consumer, `dmpf-attempt` (TRP-52); `dmpf-message-id` e
  `dmpf-error`, que vêm de fora do provider, são cortados em `HeaderValueLimit`
  (1 KiB). Quem chama commita o offset só depois do retorno (TRP-30).
- **`classifier.go`** — `Classifier` do produtor (`kerr.Error.Retriable`,
  `net.Error`) e a categoria de falha para sinais, nunca a mensagem.

### Autenticação de cliente (`ClientAuthenticated`, ADR-052)

O cliente se autentica por SASL SCRAM-SHA-256/512 **ou** por certificado de
cliente; a partida recusa TLS ligado sem uma das duas (`ErrClientAuthRequired`).
`Config.ClientAuthenticated()` é o predicado que uma fronteira de consumo lê
para se declarar verificada — `app.TransportVerified`, em
`libs/backend/go/app`, só vale com TLS **e** cliente autenticado. A ligação
entre o principal autenticado e o `source` que o consumer admite é a **ACL do
broker por principal**: só o principal de `orders` produz em `orders.events`.
Sem essa ACL — pré-requisito de todo ambiente com a fronteira verificada,
descrita em ADR-052 —, `TransportVerified` afirmaria uma ligação que o broker
não impõe; este módulo não a provisiona nem a confere.

No compose local o listener interno do Redpanda exige SASL, sem TLS no broker,
com as mesmas ACLs gravadas pelo `redpanda-init`; a fronteira do consumer fica
`development-only`. O listener externo continua sem autenticação para os
testes do host e não é usado pela topologia.

### Contrato com o adapter (`app`)

O adapter contém a mensagem quando `Delivery.Attempt >= Consumer.MaxAttempts`
dele; este provider deixa de retentar em `Channel.Retry.MaxAttempts`. **Os dois
tetos precisam ser iguais**, e quem os iguala é o composition root: com o teto
do adapter maior, o registro para de ser reentregue antes de ser contido, fica
pendente e a partição para (visível no log e no offset que não avança); com o
teto do adapter menor, a contenção acontece antes da última tentativa que o
canal declarou.

## O que o módulo não contém

Provisionamento de tópicos (TRP-41 — `kadm` aparece só no teste); registry de
schemas (KFK-13 a KFK-18); tópico de retry (vedado em canal ordenado por
KFK-11); a decisão de contenção — o `Sink` é o adapter, e é ele quem chama a
DLQ; o binding persistido de TRP-09/TRP-46; a ACL do broker que liga principal
a tópico (ADR-052, pré-requisito da plataforma).

## Como rodar os testes localmente

Os unitários rodam sobre um `FakeClient` (fetches roteirizados, commits,
produzidos, partições pausadas) e não precisam de broker. A `hops_test.go`
realiza as linhas Kafka da matriz FND-06 §5.2 com par positivo e negativo.

Os testes de integração levam a build tag `integration`, exigem
`DMPF_KAFKA_BROKERS` (sem ela fazem `t.Skip` nomeando-a) e criam tópicos com
sufixo único por execução:

```bash
docker run -d --name rp -p 9092:9092 redpandadata/redpanda:v26.2.2 \
  redpanda start --mode dev-container --smp 1 \
  --kafka-addr internal://0.0.0.0:9092 --advertise-kafka-addr internal://localhost:9092
export DMPF_KAFKA_BROKERS=localhost:9092
pnpm nx run kafka:test-race
```

`sasl_integration_test.go` é um segundo alvo, também sob `integration`, mas
independente: pede `DMPF_KAFKA_SASL_BROKERS`, o endereço de um broker que
**exige** SASL, e fica em `t.Skip` sem essa variável — o `DMPF_KAFKA_BROKERS`
comum não basta porque o Redpanda de desenvolvimento do teste acima não pede
autenticação.

O `test-race` roda com `cache: false` no Nx e `-count=1` no `go test`: nenhum
dos dois caches enxerga o estado do broker. No CI o Redpanda sobe no job
(`ci.yml`, step "Subir Redpanda").

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test-race,govulncheck -p kafka
go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop
```

O `dmpf-gate-check.sh` não alcança este módulo; o gate autoritativo é o
verificador.

## Referências

- `docs/dmpf/politicas-transporte.md` (FND-06) — §5.2, §6.2 (`TRP-26` a
  `TRP-32`, `TRP-47`, `TRP-48`), §11 (`KFK-01` a `KFK-12`, `KFK-19`).
- `docs/dmpf/resiliencia-observabilidade.md` (FND-08) — `RES-22`, `MET-11`.
- `docs/dmpf/contexto-erros-seguranca.md` (FND-07) — `IDN-03`, `IDN-04`.
- `docs/adr/025-kafka-transporte-alvo-sns-sqs-acervo.md` — Kafka como transporte-alvo.
- `docs/adr/052-identidade-de-workload-no-grpc-e-no-kafka.md` — SASL/certificado de cliente e a ACL do broker.
- `libs/backend/go/transport/README.md` — `channel`, `attempt`, `observe`.
- `libs/backend/go/app/README.md` — o adapter que realiza o `Sink` e `TransportVerified`.
