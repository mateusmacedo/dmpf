# observability

Bloco `provider` do kernel DMPF em Go: a realização de resiliência e
observabilidade que os blocos `domain`, `port` e `application` declaram, mas não
podem conter. Realiza as regras de `FND-08` — resiliência de saída (`RES-*`),
tracing (`TRC-*`), métricas (`MET-*`) e log com auditoria (`LOG-*`) — sobre
OpenTelemetry `v1.47.0` e `semconv/v1.43.0`, com a Logs API e o `sdk/log` na
mesma linha `v1.47.0`, as bibliotecas contrib `v0.72.0` (`autoexport`,
`instrumentation/runtime`), `otelslog v0.21.0` e `baggagecopy v0.17.0`.

Aqui está tudo que faz I/O: relógio de parede, socket para o Collector — traces,
métricas e logs —, leitura de variável de ambiente. É por isso que o módulo é
`provider` e não outra coisa — o gancho de instrumentação que o application
service usa vive em `ports`, e é este módulo que o realiza.

Projeto Nx `observability`, tags `type:lib`, `scope:backend`,
`stack:go`. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/observability`.

## O que o módulo contém

Uma unidade DMPF, `kernel/observability`, com `block: provider` e
`bounded_context: kernel`. Catorze packages, todos no mesmo `include` do
`dmpf-units.json`:

| Package | Conteúdo |
| --- | --- |
| raiz (`observability`) | `OTelVersion`, `OTelLogsVersion`, `OTelPrometheusExporterVersion`, `SemconvVersion` — os pins que o `version_test.go` prova contra o grafo real de build; `ShutdownGrace` (10 s), a janela de dreno que o transporte gRPC e o `boot` compartilham |
| `audit` | `Sink`, `Event`, `NewLogSink`, `NewJSONSink`, `Recording`, `EventName`, `Scope` — a trilha de auditoria, emitida pela Logs API com `EventName` `dmpf.audit` e scope próprio |
| `boot` | `Boot`, `StartTelemetry`, `Telemetry`, `Signals`, `SignalsFromEnv`, `StartRuntimeMetrics` — lê o ambiente, monta o `LoggerProvider`, o error handler do SDK e os pipelines na ordem que todo composition root precisa, registra o `process configured` e fecha tudo em qualquer saída |
| `clock` | `Clock`, `System()`, `Fake` avançável com `Advance`, `WithTimeout`, `NewTimer` |
| `envconfig` | `Hostname`, `OrDefault`, `SplitList`, `ParseBool`, `ParsePositive`, `ParseFraction`, `ParseLevel` — leitura e parsing de variável de ambiente, com `ErrInvalidVariable` como sentinela comum |
| `idclock` | `SystemClock` (realiza `ports.Clock` sobre `time.Now`) e o gerador de identificadores pré-transação, os dois sobre os tipos do domínio — nunca sobre `time.Time` |
| `logging` | `NewLogger` (`otelslog` com o scope do pacote emissor), as chaves `Key*` dos registros, `Severity` por papel e desfecho, `NewSampler` de `LOG-12` |
| `metrics` | `Catalog()` com as doze séries de RF-D1 — oito por dependência (`dmpf.dependency.*`), `dmpf.operation.duration` e as três de `MET-11`/`MET-12` que os providers de transporte gravam —, `Labels` fechado — `dmpf.tenant_id` só por `TenantWithin` com allowlist declarada —, `Instruments` construído uma vez |
| `otelboot` | `Config`, `Start`, `Runtime`, `NewClassSampler`, `NewPrivacyExporter`, `NewLoggerProvider`, `NewLogProcessor`, `Leveled`, `NewMetricView` |
| `redact` | `Attr`, `Error`, `IsSecret`, `Placeholder`, `WithoutValues` — o que sai de um erro é `error.type` e `dmpf.error.code`, nunca a mensagem; `WithoutValues(message, statement)` guarda o texto que uma biblioteca escreveu antes do primeiro valor formatado, conforme o regex de cada chamador, e troca o resto por `Placeholder` (DAT-02, DAT-23) |
| `resilience` | `Sheet`, `Compose`, e os decorators de timeout, breaker, bulkhead, degradação e retry |
| `retry` | `Evaluate` por conjunção, `Budget`, `Backoff`, `Classifier` |
| `tracing` | `Attributes` fechado (`dmpf.*` ou constante semconv), `RecordError`, os eventos de span, `WithExecutionBaggage`, `WithOwnedSpan`/`OwnsSpan`, a taxonomia de classes |
| `usecase` | `Instrumentation`: realiza o gancho de `ports` sobre span, métricas e trilha, incluindo o registro do acesso entre tenants (`ActionCrossTenantAccess`) |

`otelboot` é o mecanismo, com os componentes explícitos em `Config`: o exportador
de spans, o reader de métricas e o `LoggerProvider` chegam montados, e por isso a
suíte roda o pipeline inteiro em memória — das `OTEL_*`, o código dele só lê as
que a spec do SDK atribui ao resource, ao propagador e aos limites de atributo, e
as `OTEL_BSP_*` ficam com o processor do SDK que ele monta. `boot` é o
ambiente e o ciclo de vida do processo: escolhe os exportadores pelas `OTEL_*`
via `autoexport`, lê `LOG_LEVEL`, `OTEL_SDK_DISABLED` e `OTEL_TRACES_SAMPLER*`,
liga as métricas de runtime, registra o `process configured` e fecha os
pipelines em toda saída.

## Onde cada regra está realizada

| Regra | Código |
| --- | --- |
| `RES-21`, `RES-40` | `resilience/sheet.go` — dez campos, cada um valor **ou** ausência declarada com motivo; `Effective()` alimenta o registro `resilience sheet in effect` |
| `RES-22` | `resilience/compose.go` — nove posições, de fora para dentro: tracing, métricas, log, bulkhead, breaker, rate limit, retry, timeout, chamada |
| `RES-05`, `RES-06`, `RES-07` | `resilience/timeout.go` — o prazo efetivo é o mínimo entre o do chamador, o do método e o remanescente |
| `RES-10`, `RES-12` | `resilience/breaker.go` — taxa de falha na janela, com piso de amostras antes de abrir; `CountsAsFailure` restringe o que conta contra a dependência (sem classificador, todo erro conta; cancelamento do chamador nunca conta); cada transição sai em `warn` com `dmpf.dependency` e `dmpf.breaker.state`, no `LoggerProvider` de `LogsTo`; o `NewBreaker` grava `dmpf.dependency.breaker.state` = 0 (fechado) ao ser criado, sem log, para a série existir antes da primeira transição |
| `RES-13`, `RES-14` | `resilience/bulkhead.go` — pool e fila; saturado é rejeição rápida, nunca espera |
| `RES-25`, `RES-34` | `resilience/retry_decorator.go` — `Compose` recusa retry em volta de uma unidade de trabalho |
| `RES-27` a `RES-31`, `RES-36` | `retry/evaluate.go`, `retry/budget.go` — a conjunção de fatores e o orçamento por execução |
| `RES-32`, `RES-33` | `retry/backoff.go` — exponencial com jitter total e teto |
| `RES-37`, `RES-38` | `resilience/degrade.go` — falhar, degradar ou omitir; adiar é da outbox e é recusado; a resposta degradada deixa o evento `dmpf.degraded` no span corrente. Ela e a recusa de `Defer` (`ErrDeferIsOutbox`) levam o mesmo par, `error.type` `_OTHER` e `dmpf.error.code` `RES-37`: no log, as duas não se distinguem, e só o evento marca a resposta degradada |
| `TRC-09` | `otelboot/config.go` — sem propagador W3C, `Start` não boota; `OTEL_PROPAGATORS` só pode faltar ou ser `tracecontext` |
| `TRC-13` | `otelboot/sampler.go` — `AlwaysRecord(ParentBased(root por classe))`; ver "A regra do erro sempre amostrado" |
| `TRC-14` | fora do processo: o `tail_sampling` do Collector (`infra/observability/otel-collector/otel-collector.yaml`) decide sobre o trace completo |
| `TRC-04`, `TRC-15` | `tracing/attributes.go` — construtor fechado, um método por chave permitida; `otelboot/privacy.go` — o decorador do exportador tira as chaves vedadas antes do envio |
| `TRC-11`, `TRC-12` | `tracing/record.go` — a tentativa é evento; o erro é status e `error.type`, sem mensagem |
| `MET-02` a `MET-05`, `MET-07` | `metrics/catalog.go`, `metrics/labels.go` — nome, unidade UCUM, fórmula e dono por série; labels por allowlist; `otelboot/views.go` — a View aplica a allowlist a cada instrumento |
| `MET-08` a `MET-10` | `usecase/instrumentation.go` — `dmpf.operation.duration` do caso de uso, com `dmpf.operation`, `dmpf.outcome_category` e `error.type` só na falha; vazão e erro saem do `_count` |
| `MET-28`, `MET-29` | `resilience/retry_decorator.go`, `breaker.go`, `bulkhead.go`, `timeout.go`, `degrade.go` — cada decorator conta o que só ele sabe; `observe.go` traduz os labels |
| `LOG-01`, `LOG-02` | `logging/logger.go`, `otelboot/start.go` — registro no OTel Logs Data Model, pela Logs API, exportado por OTLP |
| `LOG-05`, `LOG-09` | `otelboot/logprocessor.go`, `redact/secret.go` — allowlist de chaves por papel do processo; chave de segredo sai com o valor trocado por `<redacted>` |
| `LOG-12` | `logging/sampling.go`, `otelboot/logprocessor.go` — amostragem por classe só do registro de trace não amostrado; erro nunca é amostrado |
| `LOG-13`, `LOG-14` | `audit/envelope.go`, `otelboot/logger.go`, `otelboot/logprocessor.go`, `usecase/instrumentation.go` — a trilha sai com `EventName` `dmpf.audit` e scope próprio, fora do `LOG_LEVEL` e da amostragem |
| `IDN-12` | `usecase/instrumentation.go`, `audit/envelope.go` — todo caso de uso fecha registrando o acesso entre tenants que o provider reportou, com o tenant do chamador e o do dado alcançado |
| `IDN-20` | `usecase/instrumentation.go`, `audit/envelope.go` — o sujeito do registro vem do contexto de execução da chamada, e o atributo sem valor fica ausente: nenhum sujeito ou tenant é inventado |

## Defaults da plataforma

`resilience.Defaults(dependency)` devolve a ficha da plataforma, e é dela que
todo serviço parte:

| Campo | Valor | Origem |
| --- | --- | --- |
| Prazo por método | 2 s | `FND-08` §3.3 |
| Retry | habilitado | §4.3 |
| Orçamento | metade do prazo remanescente, fixado na primeira falha | §4.4, `RES-30` |
| Backoff | 100 ms × 2ⁿ, jitter total, teto de 5 s | `RES-32` |
| Teto de tentativas | 3 no caminho síncrono, 5 no assíncrono | `RES-33` |
| Breaker | janela de 30 s, 50 % de falha, piso de 20 amostras, 30 s de espera, 1 sonda | `RES-10` |
| Bulkhead | pool de 16, fila do tamanho do pool, 100 ms para adquirir | `RES-13` |
| Rate limit | não se aplica — admissão por rota e tenant é do `KRN-10` | `RES-21` |
| Cache | não se aplica — modelagem encaminhada em `FND-08` §1.4 | `RES-21` |
| Degradação | falhar | `RES-37` |

Amostragem, por classe de tráfego (`tracing.DefaultRates()`), a tabela de
`TRC-13`: erro `1.0`, escrita `1.0`, leitura `0.01`, manutenção `1.0` e
`unclassified` `0.01`. Essa tabela não roda na cabeça: no processo ela é a do
`LOG-12`, onde uma classe ausente dela resolve para a taxa mais restritiva
(`0.01`). A decisão sobre o trace completo é do `tail_sampling` do Collector,
com a tabela de `TRC-13` (RF-E7). Na cabeça,
escrita, leitura e `unclassified` seguem a taxa uniforme de
`OTEL_TRACES_SAMPLER_ARG` (`tracing.UniformRates`; ausente, `1.0`), e erro e
manutenção ficam em `1.0`. Todo span local sem classe declarada recebe
`dmpf.traffic_class = "unclassified"`, para que a omissão apareça em vez de
passar por leitura legítima.

Um campo que a ficha não declara — nem valor, nem motivo — reprova em
`Sheet.Validate()`. Em branco não é default: é política que ninguém decidiu.

## A regra do erro sempre amostrado

`TRC-14` pede que um erro seja sempre amostrado, e a decisão da cabeça acontece
no **início** do span, antes de o desfecho ser conhecido. A realização é a
decisão tardia que a própria regra admite, fora do processo: o processo registra
todo span, e o `tail_sampling` do Collector decide sobre o trace completo
(RF-E7).

- `NewClassSampler` é `AlwaysRecord(ParentBased(root))`. O filho segue o pai,
  local ou remoto. A raiz de classe `write` ou `read` é amostrada quando algum
  link válido está amostrado — é assim que o `outbox drain` e o `process`
  acompanham o contexto de criação —, e qualquer outra raiz passa por
  `TraceIDRatioBased` à taxa da classe. A raiz de classe `error`, a da fronteira
  recusada do consumo, não segue link, para que um produtor externo não force a
  amostragem. Nenhum span é descartado: fora da taxa, ele existe como
  `RecordOnly`.
- O sampler grava `dmpf.traffic_class` nos atributos do `SamplingResult` de todo
  span local sem classe, inclusive quando segue um pai, para que a cauda
  classifique todo trace.
- O `BatchSpanProcessor` do SDK, ajustado por `OTEL_BSP_*`, exporta só o span
  amostrado, e o `baggagecopy` roda antes dele.

Os manifestos das apps declaram `OTEL_TRACES_SAMPLER_ARG=1.0`: a cabeça amostra
tudo, e o Collector aplica `TRC-13` e `TRC-14`. Cabeça e cauda com a mesma
tabela multiplicariam as taxas e descartariam erro antes da cauda.

Sob saturação, a fila do `BatchSpanProcessor` descarta, e o próprio SDK conta a
perda em `otel.sdk.processor.span.processed` com `error.type=queue_full` — a
autoinstrumentação experimental que `OTEL_GO_X_OBSERVABILITY=true` liga. Para que
ela grave no `MeterProvider` do processo, `Start` o instala como global antes de
montar o processor, porque um buraco silencioso nos traces é pior que um buraco
medido.

## O que a telemetria garante

A entrega é **at-least-once, e sob perda declarada**. O exportador OTLP repete
tentativas, então o collector pode receber o mesmo span mais de uma vez. Na
direção oposta, a fila do processor descarta sob saturação e a amostragem
descarta por desenho — telemetria não é registro contábil. Falha de exportação
nunca falha a publicação, o consumo nem a requisição: o erro do SDK vira um
`warn` `telemetry export failed`, só com `error.type`.

Entrega exatamente-uma-vez (*exactly-once*) é **vedada** como promessa em
qualquer artefato do DMPF (P0-3, `GAR-01`), aqui como em toda parte.

A **trilha de auditoria é outra coisa**. Ela vai ao `audit.Sink` — em produção,
`audit.NewLogSink`, que emite pela Logs API com `EventName` `dmpf.audit` e scope
próprio —, e não passa pelo `LOG_LEVEL` nem pela amostragem de `LOG-12`,
exatamente porque nível e amostragem de log podem derrubar um registro — e um
fato auditável não pode depender disso (`LOG-13`, `LOG-14`). A separação é
lógica: o canal regulatório, com retenção, controle de acesso e integridade
próprios, não é derivado deste pipeline (`LOG-13`). Quando o sink recusa um
registro, o módulo escreve no log a **categoria** da falha e a ação, jamais o
conteúdo do evento (`fix`: `3557f54` registra o evento inteiro nessa linha, não
só a categoria, porque um evento de segurança perdido sem o próprio conteúdo no
log não dá o que investigar).

## Auditoria de acesso entre tenants (IDN-12)

`audit.Event` tem dois campos de tenant — `Tenant` e `DataTenant` — e os dois só
são preenchidos por um acesso entre tenants: um caso de uso cujo resultado
alcançou dado de um tenant diferente do que a chamada resolveu. Fora desse caso,
os campos ficam vazios, porque não são o normal do registro.

`usecase.Instrumentation` fecha todo caso de uso registrando esse acesso quando
o `ports.Result` o reporta — a exceção deliberada é a trajetória de
carregar-ou-criar no próprio tenant, que nunca é cruzamento. A ação vai como
`usecase.ActionCrossTenantAccess` (`"security.cross_tenant_access"`) na trilha.

`audit.NewLogSink` emite a trilha pela Logs API do `LoggerProvider` do processo:
`EventName` `dmpf.audit`, scope
`github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit`, `Timestamp`
do instante do evento, corpo `audit` e os atributos `dmpf.audit.subject`,
`dmpf.audit.object`, `dmpf.audit.action` e `dmpf.audit.outcome` — no acesso entre
tenants, também `dmpf.tenant_id` e `dmpf.audit.data_tenant_id`. Atributo sem
valor fica ausente. A correlação da chamada vem do `baggagecopy` e o trace, do
`ctx`; nada do evento vai ao span (`DAT-25`). O painel de logs do
`reference.json` e o caminho trace→logs do Grafana excluem a trilha pelo
`scope_name`. No acesso entre tenants, o sujeito vem do contexto de execução da
chamada (`ports.ExecutionContextFrom`, ADR-049) e fica ausente numa cadeia sem
sujeito, e o tenant é o que o provider relatou como o do contexto — nunca um
valor da identidade do processo (`IDN-20`).

## O registro de log

O log de aplicação é registro do OTel Logs Data Model, emitido pela Logs API e
exportado por OTLP (`LOG-01`, `LOG-02`): `Timestamp`, `ObservedTimestamp`,
`SeverityNumber`/`SeverityText`, `Body` com a mensagem, o resource do processo
(`service.name`, `service.version`, `service.instance.id`, `dmpf.process.role`
e, das `OTEL_*`, `deployment.environment.name`) e
`TraceId`/`SpanId`/`TraceFlags` lidos do `ctx`. Nada disso é atributo: o registro não tem `time`, `level`,
`service`, `version`, `instance`, `trace_id`, `span_id` nem `duration_ms`, e não
há chave reservada nem prefixo `app.` — o que identifica o processo é do
resource, e o que identifica o trace é do registro (RF-A2).

Os atributos seguem a semconv quando ela tem chave para o conceito (`http.*`,
`rpc.*`, `messaging.*`, `cloudevents.event_type`, `db.collection.name`,
`server.*`) e `dmpf.*` quando o conceito é do DMPF (RF-A3). O erro entra só por
`redact.Error`: `error.type` com a categoria FND-07 — ou `_OTHER`, quando o erro
não a declara — e `dmpf.error.code` quando o erro implementa `Categorized`;
nunca a mensagem (`DAT-23`). O detalhe do transporte — o código do broker ou
do SDK, o status HTTP — não tem chave própria: o cliente grava só a categoria,
em `error.type` e `dmpf.outcome_category` (`transport/observe`), e o status
HTTP fica em `http.response.status_code`, no CLIENT do `otelhttp`.

## O pipeline de log

```text
slog.Logger — logging.NewLogger: otelslog, scope = import path do pacote emissor
  → Leveled — LOG_LEVEL aplicado no provider; a auditoria passa sempre
    → sdk/log LoggerProvider — resource do processo, valor de atributo até 1024 bytes
      → baggagecopy — dmpf.correlation_id, dmpf.request_id, dmpf.tenant_id
      → processor DMPF — LOG-12, allowlist por papel, segredo em <redacted>
        → BatchProcessor → exportador do autoexport (OTEL_LOGS_EXPORTER)
```

O scope de cada registro é o import path do pacote que o emite, não o serviço:
as libs recebem o `log.LoggerProvider` na própria configuração e criam o logger
com `logging.NewLogger` (RF-A1). O composition root entrega
`Runtime.LoggerProvider()`; `Runtime.LoggerFor(scope)` é o atalho para o mesmo
logger.

`Leveled` aplica o `LOG_LEVEL` no provider, porque o `Enabled` do handler do
`otelslog` só consulta o provider; o registro com `EventName` `dmpf.audit` passa
qualquer que seja o nível. O processor DMPF envolve o `BatchProcessor` em vez de
precedê-lo, porque um processor do SDK não impede os registrados depois dele, e
o registro que o `LOG-12` amostrou fora não pode chegar ao lote. Ele faz três
coisas:

- **Amostragem de `LOG-12`.** O registro em `error` nunca é amostrado. O
  registro sem span context válido — partida, encerramento — não é tráfego e
  passa. O registro de um trace amostrado é mantido. O de um trace não
  amostrado, inclusive o de um `traceparent` remoto que chegou não amostrado,
  passa à taxa da classe do processo (`Telemetry.Class`) na tabela de
  `tracing.DefaultRates()`, sorteada por `math/rand/v2`. A auditoria nunca passa pela amostragem.
- **Allowlist de `LOG-05`.** Chave `dmpf.*` sempre passa; `error.type` e
  `db.collection.name` passam em todo papel. O resto depende do
  `dmpf.process.role` lido do resource do registro, e não da configuração,
  porque `OTEL_RESOURCE_ATTRIBUTES` pode sobrepô-lo: `api` admite
  `http.request.method`, `http.route`, `http.response.status_code`,
  `rpc.system.name`, `rpc.method`, `rpc.response.status_code`, `server.address`
  e `server.port`; `consumer` admite `messaging.system`,
  `messaging.operation.name`, `messaging.destination.name`,
  `messaging.consumer.group.name`, `messaging.message.id`,
  `cloudevents.event_type`, `messaging.destination.partition.id` e
  `messaging.kafka.offset`; `relay`, as mesmas, salvo
  `messaging.consumer.group.name`. Papel não declarado admite a união. Chave
  fora da lista sai do registro.
- **Segredo (`LOG-09`).** Chave cujas palavras incluem `password`, `passwd`,
  `secret`, `token`, `credential`, `credentials`, `authorization`, `dsn`,
  `apikey` ou `cookie`, ou que traz `key` logo depois de `private` ou de `api`,
  mantém a chave com o valor trocado por `redact.Placeholder` (`<redacted>`),
  também dentro de um grupo (`redact.IsSecret`). É a mesma regra para o tráfego
  e para o `process configured`.

Com `OTEL_LOGS_EXPORTER=none` ou `OTEL_SDK_DISABLED=true`, o registro é
descartado (`slog.DiscardHandler`). O log fica só no OTLP: a saída de erro do
processo recebe apenas a configuração recusada e o erro com que ele termina,
escritos pelo `cmd/main.go` de cada app (RF-A1, ADR-057).

## Baggage de execução

`tracing.WithExecutionBaggage` põe no baggage do `ctx` os identificadores do
`ports.ExecutionContext` — `dmpf.correlation_id`, `dmpf.request_id` e, com
tenant resolvido, `dmpf.tenant_id` —, e o `baggagecopy`, como SpanProcessor e
como LogProcessor, com o filtro restrito a `tracing.ExecutionBaggageKeys`, os
copia para todo span e todo registro de log do processo (RF-B8, `LOG-04`).

O propagador é só `propagation.TraceContext`, e `OTEL_PROPAGATORS` diferente de
`tracecontext` falha a partida: o baggage nunca vai ao fio, e o tenant não vaza
para terceiros. O `baggagecopy` grava no `OnStart`, então um span aberto antes do
baggage não o recebe — por isso o SERVER do `otelhttp` e o do `otelgrpc` levam
as três chaves por `tracing.ExecutionAttributes`, gravadas por quem monta o
contexto. Sem execução em curso — partida, encerramento —, as chaves ficam
ausentes, nunca com valor de preenchimento (`CTX-26`).

## Chaves `dmpf.*`

A semconv cobre o protocolo; `dmpf.*` fica com o que só existe no contexto do
DMPF. As chaves declaradas neste módulo:

| Chave | Onde aparece | Declarada em |
| --- | --- | --- |
| `dmpf.correlation_id`, `dmpf.request_id`, `dmpf.tenant_id` | baggage de execução, e daí todo span e todo registro com execução em curso; `dmpf.tenant_id` também é label de `dmpf.admission.rejections`, com valor próprio só para os tenants de `METRIC_TENANTS` e `other` para os demais (`MET-07`) | `tracing/attributes.go`, `tracing/baggage.go` |
| `dmpf.outcome_category` | desfecho da operação no span e nos registros de acesso; label de `dmpf.operation.duration` | `tracing/attributes.go` |
| `dmpf.traffic_class` | todo span local, pelo sampler (`unclassified` quando ausente) | `tracing/attributes.go`, `otelboot/sampler.go` |
| `dmpf.dependency` | span de resiliência, log do cliente e label das séries `dmpf.dependency.*` | `tracing/attributes.go` |
| `dmpf.operation` | label de `dmpf.operation.duration`, `dmpf.dependency.deadline_exceeded` e `dmpf.dependency.cancellations` | `tracing/attributes.go`, `metrics/labels.go` |
| `dmpf.retry.attempt`, `dmpf.retry.previous_category` | número da tentativa, contado a partir de 1, no log do cliente e no evento `dmpf.retry.attempt`; a categoria que causou a repetição, no evento | `tracing/attributes.go`, `tracing/record.go` |
| `dmpf.retry.budget_exhausted` | span da chamada que esgotou o orçamento de retry (`RES-36`) | `resilience/retry_decorator.go` |
| `dmpf.deadline.remaining_ms` | span de resiliência | `tracing/attributes.go` |
| `dmpf.outbox.claim_id`, `dmpf.outbox.attempt` | `dmpf.outbox.claim_id` no `outbox drain`; `dmpf.outbox.attempt`, que conta as reivindicações do registro, no `send` do relay e no `outbox publish failed` | `tracing/attributes.go` |
| `dmpf.inbox.attempt`, `dmpf.inbox.disposition`, `dmpf.inbox.gesture` | span `process` e registro `message consumed` | `tracing/attributes.go` |
| `dmpf.idempotency_key`, `dmpf.idempotency_key.derived`, `dmpf.idempotency_key.invalid` | registros de acesso do BFF e do servidor gRPC (`IDM-01`, `IDM-03`) | `tracing/attributes.go` |
| `dmpf.idempotency_outcome` | span do caso de uso (`IDM-08`) | `tracing/attributes.go` |
| `dmpf.error.code` | registro, por `redact.Error`, quando o erro implementa `Categorized`; evento `dmpf.degraded` | `tracing/attributes.go`, `redact/redact.go` |
| `dmpf.breaker.state` | transição do breaker | `logging/fields.go` |
| `dmpf.process.role` | resource: `api`, `relay` ou `consumer` | `otelboot/config.go` |
| `dmpf.audit.subject`, `dmpf.audit.object`, `dmpf.audit.action`, `dmpf.audit.outcome`, `dmpf.audit.data_tenant_id` | trilha de auditoria | `audit/envelope.go` |
| `dmpf.config.<campo>` | `process configured` | `boot/boot.go` |
| `dmpf.sheet.<campo>` | `resilience sheet in effect`, um registro por ficha, ao lado de `dmpf.dependency`; nunca no resource | `otelboot/config.go`, `otelboot/start.go` |
| `dmpf.sampler.declared` | o `warn` de `OTEL_TRACES_SAMPLER` ignorado | `boot/telemetry.go` |

Os eventos de span também são fechados, um por função em `tracing/record.go`:
`dmpf.retry.attempt`, `dmpf.breaker.rejected`, `dmpf.bulkhead.saturated`,
`dmpf.degraded`, `claimed` e `invalid_creation_context`. As chaves `dmpf.*` que
só um provider grava ficam documentadas no README dele.

## Bootstrap

```go
err := boot.Boot(ctx, boot.Telemetry{
    Service:  "orders",
    Role:     "api",
    Class:    tracing.ClassWrite,
    Settings: settings,
    Signals:  signals, // boot.SignalsFromEnv
}, func(ctx context.Context, rt *otelboot.Runtime) error {
    return serve(ctx, rt.Tracer(), rt.MeterProvider(), rt.LoggerProvider())
})
```

`boot.Boot(ctx, telemetry, work)` é o que todo composition root usa: chama
`StartTelemetry`, registra uma vez o `process configured` — cada
`Telemetry.Settings` achatado em `dmpf.config.<campo>`, segredo em `<redacted>`
pela regra do processor —, entrega o `*otelboot.Runtime` ao closure `work` e
chama `ShutdownGracefully` em **toda** saída, inclusive a que falhou. O
encerramento comum ao dreno de transporte e ao desligamento da telemetria é a
mesma janela, `observability.ShutdownGrace` (10 s), e os traces fecham antes das
métricas e dos logs.

`StartTelemetry` monta, nesta ordem:

1. o `LoggerProvider`, sobre o exportador de log que o `autoexport` escolhe —
   `nil` com `none`, e aí o registro é descartado;
2. o error handler do SDK, que escreve `telemetry export failed` em `warn` só com
   `error.type`, e um único `warn` com `dmpf.sampler.declared` quando
   `OTEL_TRACES_SAMPLER` está declarado — o erro de parse que o próprio SDK
   repassaria da mesma variável é descartado, para não virar segundo registro;
3. o exportador de spans e o reader de métricas do `autoexport`, com o producer
   de runtime registrado como fallback;
4. `otelboot.Start` e, sobre o `MeterProvider` resultante, as métricas de
   runtime de `contrib/instrumentation/runtime` (RF-D5).

Com `OTEL_SDK_DISABLED=true`, nada é exportado: o tracer nunca amostra e o log é
descartado.

`otelboot.Start` é o único lugar que toca o propagador e o `TracerProvider`
globais — e o `MeterProvider` global, que o `BatchSpanProcessor` precisa
encontrar instalado —, e uma configuração recusada não toca em nenhum deles. Sem
exportador de spans, fora do modo desligado, devolve `ErrExporterRequired`. Um
segundo `Start` no mesmo processo devolve `ErrAlreadyStarted`; `Shutdown` libera
o processo e é idempotente.

O resource é `resource.New` com o schema da semconv `v1.43.0`, os detectores
`WithTelemetrySDK`, `WithProcessRuntimeName` e `WithProcessRuntimeVersion`, os
atributos declarados e, por último, `WithFromEnv`: `OTEL_SERVICE_NAME` e
`OTEL_RESOURCE_ATTRIBUTES` prevalecem sobre o que `Telemetry` declara. Sem
`service.name`, `service.version` ou `service.instance.id` no resultado, a
partida falha com `ErrResourceIncomplete`.

O decorador de privacidade, `NewPrivacyExporter`, envolve o exportador de
spans — o `ReadOnlySpan` de `OnEnd` não aceita escrita — e limpa cada span antes
do envio (RF-B3, `TRC-15`): tira `client.address`, `network.peer.address`,
`network.peer.port` e `user_agent.original`; troca `url.path` pelo `http.route`
do mesmo span (sem rota, `REDACTED`); reescreve `url.full` só com esquema, host e
`url.template` (sem template, `/REDACTED`), sem query nem fragmento; zera a
descrição do status; e tira `exception.message` e `exception.stacktrace` dos
eventos. A View de métricas, `NewMetricView`, deixa cada série do catálogo só com
os labels que ela declara e tira das demais as mesmas chaves vedadas (RF-D3).
Nas séries de servidor HTTP (`http.server.request.duration` e
`http.server.{request,response}.body.size`), tira também `server.address` e
`server.port`, que o `otelhttp` lê do header `Host` enviado pelo cliente. A mesma
View dá as fronteiras de duração da semconv (0,005 a 10 s, RF-D4) a
`otel.sdk.exporter.operation.duration` e
`otel.sdk.metric_reader.collection.duration`, que o SDK só emite com
`OTEL_GO_X_OBSERVABILITY=true` (`otelboot/views.go`).

Todo valor de atributo de span e de registro de log é cortado em 1024 bytes,
salvo declaração das `OTEL_*_ATTRIBUTE_VALUE_LENGTH_LIMIT`.

Quando `otelboot.Config.Sheets` declara fichas, `Start` as valida e cada uma
sai uma vez no registro `resilience sheet in effect`, com `dmpf.dependency` e
`dmpf.sheet.<campo>` (`RES-40`). Elas ficam fora do resource, que identifica só
o processo. O `boot.Telemetry` não repassa fichas ao `otelboot`: nos processos
montados por `boot`, a partida não as carrega.

## Variáveis `OTEL_*`

A configuração do SDK é a da spec do OpenTelemetry, sem `WithEndpoint` nem
`WithInsecure` no código (RF-E1):

| Variável | Efeito | Onde |
| --- | --- | --- |
| `OTEL_SDK_DISABLED` | `true`, sem diferenciar maiúscula, desliga a exportação dos três sinais | `boot/telemetry.go` |
| `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER` | `otlp`, `console` ou `none` — nas métricas, também `prometheus` —, default `otlp`, escolhidos pelo `autoexport` | `boot/telemetry.go` |
| `OTEL_EXPORTER_OTLP_{ENDPOINT,PROTOCOL,INSECURE,HEADERS,TIMEOUT,COMPRESSION}` e as variantes por sinal | lidas pelos exportadores OTLP; o `autoexport` usa `http/protobuf` por default, e os manifestos das apps declaram `grpc` com endpoint `http://…:4317`, cujo esquema desliga o TLS do gRPC | `autoexport` |
| `OTEL_METRICS_PRODUCERS` | declarada, prevalece sobre o producer de runtime que `boot` registra como fallback | `boot/runtime.go` |
| `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` | sobrepõem o resource declarado; os manifestos das apps declaram `service.version`, `service.instance.id`, `deployment.environment.name` e `dmpf.process.role` | `otelboot/config.go` |
| `OTEL_PROPAGATORS` | ausente ou `tracecontext`; outro valor, `baggage` inclusive, falha a partida com `ErrPropagatorNotW3C` | `otelboot/config.go` |
| `OTEL_TRACES_SAMPLER_ARG` | taxa uniforme da cabeça para `write`, `read` e `unclassified`; ausente, `1.0`; fora de [0, 1] recusa a configuração | `boot/signals.go` |
| `OTEL_TRACES_SAMPLER` | ignorada: declarada, mesmo vazia ou inválida, gera um único `warn` | `boot/telemetry.go`, `boot/sampler.go` |
| `OTEL_BSP_*` | ajustam o `BatchSpanProcessor` do SDK | `otelboot/processor.go` |
| `OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT`, `OTEL_SPAN_ATTRIBUTE_VALUE_LENGTH_LIMIT`, `OTEL_LOGRECORD_ATTRIBUTE_VALUE_LENGTH_LIMIT` | limite de tamanho de valor; vale a variante do sinal antes da geral, e valor que não é inteiro volta aos 1024 bytes | `otelboot/limits.go` |
| `OTEL_GO_X_OBSERVABILITY` | `true` liga a autoinstrumentação experimental do SDK, de onde sai `otel.sdk.processor.span.processed` (RF-D7) | SDK `v1.47.0` |

`LOG_LEVEL`, que não é da spec do SDK, continua: é o nível mínimo do log,
aplicado no provider e nunca à auditoria.

## Leitura de configuração (`envconfig`)

`envconfig` é o parser comum a todo composition root e a `authn`: `Hostname()`
nunca falha a partida (responde `"local"` sem nome de host do SO), `OrDefault`
substitui vazio por um default declarado, `SplitList` separa uma lista por
vírgula descartando entradas vazias, `ParseFraction` lê uma taxa em [0, 1]
separando ausência de zero explícito, `ParseLevel` lê um nível do `slog`, e
`ParseBool`/`ParsePositive` devolvem `ErrInvalidVariable` — sempre com o nome da
variável — em vez de silenciar um valor mal formado atrás de um default.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p observability
bash tools/dmpf-plugin/scripts/dmpf-gate-check.sh
go run ./tools/dmpf-conformance/cmd/conformance --root .
```

O `dmpf-gate-check.sh` e o `nx affected` **não devem rodar ao mesmo tempo**: o
gate cria arquivos `zz_gate_*.go` temporários nos outros módulos para provar que
o `depguard` reprova o que deve, e um lint concorrente os encontra no meio do
caminho.

### O receiver OTLP dos testes

Nenhum teste do módulo sobe container. `otelboot` roda o pipeline com
exportadores em memória, injetados pela `Config`. `boot` exercita o caminho de
produção — `autoexport` e OTLP/gRPC de verdade — contra um receiver OTLP no
próprio processo (`startReceiver`, em `boot/telemetry_test.go`), que escuta em
`127.0.0.1:0` com os serviços de trace, métrica e log; o teste aponta para ele
as `OTEL_EXPORTER_OTLP_{PROTOCOL,ENDPOINT}`, com `grpc` e `http://<endereço>`.
Tudo roda no `go test ./...` padrão, sem build tag e sem `testing.Short()`:
pular em silêncio criaria um caminho em que a integração nunca é exercida e
ninguém percebe.

### Smoke manual do collector

Para conferir a exportação sem passar pela suíte — depurando uma configuração de
deployment, por exemplo — suba o collector à mão e aponte o serviço para ele:

```bash
cat > /tmp/otelcol.yaml <<'YAML'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    traces:  {receivers: [otlp], exporters: [debug]}
    metrics: {receivers: [otlp], exporters: [debug]}
    logs:    {receivers: [otlp], exporters: [debug]}
YAML

docker run --rm -p 4317:4317 \
  -v /tmp/otelcol.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector:0.160.0

# noutro terminal, com o serviço declarando grpc e http://localhost:4317
# nas OTEL_EXPORTER_OTLP_{PROTOCOL,ENDPOINT}
docker logs -f <container>
```

O exportador `debug` imprime cada span, cada métrica e cada registro de log que
chega, com o resource junto. Se o sinal não aparece, o problema está antes do
collector: propagador, amostragem ou exportador (`OTEL_*_EXPORTER`, endpoint,
protocolo) — nessa ordem de suspeita.

## Governança

Criar um package novo aqui é criar membership de unidade DMPF: o import path
exato entra no `include` do `dmpf-units.json` e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`; sem isso, o CI reprova com `DMPF-T001`.

Dependência externa nova entra na `external[]` com os quatro elementos — pacote,
faixa de versão, entrypoints e capability — e a capability é **declarada**, nunca
inferida do nome. O `autoexport` está como `io.network`, e não `observability`
como o resto do SDK, porque é por ele que o módulo abre socket para o Collector.
Esconder isso sob a capability do propósito apagaria do manifesto onde a rede
entra.

O tidy do módulo roda pelo target Nx (`pnpm nx run observability:tidy`), que
chama `tools/dmpf-plugin/scripts/go-tidy.sh`: o `go mod tidy` puro ignora o `go.work` e buscaria os
módulos irmãos pelo proxy, e o script põe os `replace` do `go.work` no `go.mod`
só durante o tidy.

## Referências

- `docs/specs/SPEC-NYD18TGD-dmpf-resiliencia-observabilidade-go.md` — spec do `KRN-09`
- `docs/specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md` — a forma canônica dos sinais: Logs Data Model por OTLP, semconv `v1.43.0`, `OTEL_*`
- `docs/specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md` — spec do acesso entre tenants e do contexto de execução
- `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` — decisões desta realização
- `docs/adr/057-log-das-apps-por-otlp-e-alloy-so-para-infraestrutura.md` — o log das apps só por OTLP
- `docs/adr/049-contexto-de-execucao-viaja-no-context-context.md` — o carrier de que a instrumentação lê o sujeito
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que este módulo instrumenta
- `docs/adr/026-baseline-resiliencia-observabilidade-opentelemetry.md` — a decisão de plataforma que originou o `FND-08`
- `docs/dmpf/resiliencia-observabilidade.md` — FND-08: as regras `RES-*`, `TRC-*`, `MET-*` e `LOG-*`
- `docs/dmpf/contexto-erros-seguranca.md` — FND-07: `IDN-12`, `IDN-20`, o acesso entre tenants
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §6.2, §6.3, §10.2
