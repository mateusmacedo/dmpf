---
id: SPEC-1TFW24WV
slug: observabilidade-ponta-a-ponta
title: Observabilidade ponta a ponta — logs padronizados, trace contínuo no salto assíncrono e correlação log↔trace
stage: done
priority: P1
depends_on: []
ticket_url: https://linear.app/mmda/issue/DEVS-19/spec-1tfw24wv-observabilidade-ponta-a-ponta-logs-padronizados-trace
subtask_urls: []
created: 2026-09-30
---

# SPEC-1TFW24WV: Observabilidade ponta a ponta — logs padronizados, trace contínuo no salto assíncrono e correlação log↔trace

## Resumo

Como operador da topologia DMPF, quero seguir um `place-order` da borda do BFF até o
consumo em `reservations` — log, trace, métrica e o caminho entre eles — sem buraco e sem
registro repetido, para diagnosticar uma falha a partir de qualquer sinal. A spec fecha as
lacunas medidas contra a baseline FND-08 (logs divergentes, salto outbox → relay → broker
fora do trace, link log↔trace vazio) e leva os cinco sinais à forma canônica do
OpenTelemetry: logs no OTel Logs Data Model por OTLP, spans e métricas na semconv
`v1.43.0`, configuração do SDK por `OTEL_*`. O DMPF só nomeia o que é do seu contexto
(`dmpf.*`).

## Revisão canônica (2026-09-30)

**Decisão do usuário.** Princípio "OTel puro": onde há conceito, prática ou convenção de
mercado (semconv `v1.43.0`, spec do SDK, OTel Logs Data Model, `OTEL_*`, contrib
`v0.72.0`, CloudEvents, W3C), usa-se a forma canônica, inclusive chaves e formato de log.
Invariantes DMPF que eram opinião contrária ao canônico caem; ficam as de contexto DMPF e
as exigências verificáveis (DAT-02/DAT-06, LOG-09, TRC-16/LOG-11). A restrição "nenhuma
dependência nova" cai: dependências entram nomeadas e pareadas, só em `provider`/`app`,
com inversão de dependência. Biblioteca pronta substitui código próprio, salvo quando
emite forma não canônica (tracer do `otelpgx` em semconv `v1.40.0` e `pgxpool.*`, tracer
do `kotel` em `v1.18.0`): aí o código próprio é mínimo e usa as constantes de
`semconv/v1.43.0`. As decisões 1–12 da auditoria de canonicalização estão resolvidas e
aplicadas abaixo; a continuidade F1→F2→F3 sob `tail_sampling` no Kubernetes, consequência
das decisões 8 e 11, foi resolvida pelo usuário com `write` a 100% na cauda.

| Sinal | Atual | Canônico |
|-------|-------|----------|
| Log | JSON próprio em stdout raspado pelo Alloy (`logging/handler.go`) | `otelslog` → `sdk/log` → OTLP → Collector → Loki `/otlp` |
| Log | `time`, `level`, `msg`, `service`, `version`, `instance`, `trace_id`, `span_id` como atributo | `Timestamp`, `SeverityNumber`/`SeverityText`, `Body`, Resource `service.*`, `TraceId`/`SpanId`/`TraceFlags` |
| Log | `operation`, `status`, `code`, `channel`, `message_id`, `error_category`, `duration_ms` | `http.*`, `rpc.*`, `messaging.*`, `cloudevents.event_type`, `error.type`; sem duração |
| Log | auditoria com `kind: audit` e serializador próprio | Logs API com `EventName` `dmpf.audit` e scope próprio |
| Trace | `dmpf.db.query` só com `dmpf.dependency` | `{db.operation.name} {db.collection.name}` com `db.*` e `error.type` = SQLSTATE |
| Trace | `dmpf.grpc.server <FullMethod>`; CLIENT acima dos retries | `otelgrpc`: `{rpc.method}` SERVER e CLIENT por tentativa; resiliência em INTERNAL |
| Trace | `HTTP <pattern>` próprio no BFF | `otelhttp`: `{http.request.method} {http.route}` + decorador de privacidade |
| Trace | `dmpf.kafka.publish`, `dmpf.sqs.*` raiz avulsa | `send {messaging.destination.name}` CLIENT filho do drain, link a F1 |
| Trace | consumo filho do envelope no `sink.go` | `process {messaging.destination.name}` CONSUMER raiz com link a F1 |
| Trace | `dmpf.service`, `dmpf.version`, `dmpf.error.category`, `dmpf.operation`, `dmpf.dependency` no span | Resource `service.*`, `error.type`, `rpc.method`/`http.route`/`db.operation.name`, `db.system.name`/`messaging.system`/`rpc.system.name` |
| Trace | `dmpf.correlation_id` em mensageria | `messaging.message.conversation_id` |
| Métrica | `dmpf_<comp>_<sinal>_<unidade>` com `_total`, unit `1` | nome com ponto, sem unidade nem `_total`, UCUM no instrumento |
| Métrica | RED próprio `dmpf_service_request_*` | `http.server.request.duration`, `rpc.{server,client}.call.duration`, `messaging.*.duration` |
| Métrica | sem runtime nem pool | `go.*` (contrib runtime), `db.client.connection.*` (`dbconv`) |
| SDK | `OTLP_ENDPOINT`, `OTLP_INSECURE`, `OTLP_LOGS`, `SERVICE*`, `INSTANCE_ID`, `TRACE_SAMPLE_RATE`; `WithEndpoint` forçado | `OTEL_EXPORTER_OTLP_*`, `OTEL_*_EXPORTER` (autoexport), `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER_ARG`, `OTEL_BSP_*` |
| SDK | `classSampler` escrito do zero; `ClassAwareProcessor` | `AlwaysRecord(ParentBased(root))`; `BatchSpanProcessor`; TRC-14 por `tail_sampling` |

**Continua `dmpf.*`** porque a semconv não tem chave para o conceito: `dmpf.correlation_id`
fora de mensageria, `dmpf.request_id` e `dmpf.tenant_id` (CTX-06/07/28);
`dmpf.outcome_category` (FND-07); `dmpf.error.code` (LOG-03); `dmpf.traffic_class` e as
classes de TRC-13; `dmpf.outbox.claim_id`, `dmpf.outbox.attempt` e os eventos `claimed` e
`invalid_creation_context`; `dmpf.inbox.{attempt,disposition,gesture}`,
`dmpf.containment.reason`, `dmpf.panic.type` (tipo Go do valor de panic recuperado no consumer, nunca o valor); `dmpf.idempotency_key`, `dmpf.idempotency_key.derived` e
`dmpf.idempotency_key.invalid` nos logs (IDM-01, IDM-03) e `dmpf.idempotency_outcome` no span
do caso de uso (IDM-08); `dmpf.usecase.<operação>`,
`outbox drain` e o span INTERNAL de resiliência (TRC-05, TRC-11); `dmpf.dependency` só no
span de resiliência e como label de métrica (nome da ficha RES); `dmpf.operation` como label do
RED do caso de uso e das séries de dependência do catálogo (RF-D2, `MET-04`); `dmpf.process.role` no
resource; `dmpf.audit.*`; `dmpf.config.*` no registro de partida.

As chaves de idempotência seguem o padrão `{objeto}.{propriedade}` da regra de nomes da
semconv, com `dmpf.idempotency_key` como objeto e a derivação e a invalidez como
propriedades, como em `dmpf.outbox.attempt`; `dmpf.idempotency_outcome` mantém o nome que a
SPEC-JJKWG4JP gravou. Os headers `Idempotent-Replayed` (HTTP) e `idempotent-replayed` (gRPC)
não entram em span nem log, nem como presença: o replay já está em
`dmpf.idempotency_outcome`, no mesmo trace.

**O que caiu da versão anterior (E1–E13):**

- E5 (fronteira recusada → raiz **sem** link e `dmpf.provenance.traceparent`): superada pelo
  padrão *public endpoint* de `otelhttp`/`otelgrpc` (`WithNewRoot` + link ao contexto
  remoto); a chave de proveniência sai.
- E8, na parte `time` UTC com milissegundo e `error_category`/`error_code`: superada pelo
  Logs Data Model (decisão 1) e por `error.type`/`dmpf.error.code`; a tabela de severidade
  continua.
- E9 (`channel=app` no Collector, `channel` como label, Alloy restrito a `dmpf-*` com IDs
  em structured metadata): superada pela decisão 1 (log só por OTLP, Alloy só para infra) e
  pela decisão 12 (defaults do Loki).
- E10 (`tenant_id` reservado renomeado pelo handler): superada pela decisão 5; o caso de uso
  deixa de gravar tenant e a chave é `dmpf.tenant_id`, vinda do baggage.
- E11 (envelope regravado com o contexto do `send` PRODUCER; consumo ligado ao `send`):
  superada pela decisão 6 e por `messaging-spans.md:242,266-274,337` (o contexto de criação
  "SHOULD NOT be modified"; o `send` "SHOULD always link" a ele e é CLIENT).
- E12 (processor próprio de herança no `OnStart`): superada pela decisão 5 (W3C Baggage
  in-process + `baggagecopy`).
- E13 (stream `channel="audit"`): superada pelo `EventName` `dmpf.audit` com scope próprio.
- E1 fica fora de mensageria e cede a `messaging.message.conversation_id` nos spans `send` e
  `process` (decisão 3). E2, E3, E4, E6 (sem `duration_ms`, decisão 10) e E7 continuam.

## Contexto

- **Problema**: sintomas medidos na topologia local em 2026-09-30.
  1. **Logs.** Cada escrita gera 5 linhas em 4 formas (`http request`, `transport: call`,
     `grpc request`, `grpc call`, `audit`); o `http request` sai com `correlation_id`,
     `request_id` e `tenant_id` vazios em 3.802 linhas; `request_id` só existe no BFF; o
     opt-out de TLS do Kafka gera dois `warn` no mesmo milissegundo; o Alloy promove
     `level`, `service` e `trace_id`, que colidem com o JSON (`*_extracted` em `| json`).
  2. **Traces.** O relay não abre span nem lê o `traceparent` do outbox; o
     `dmpf.kafka.publish` nasce raiz avulsa (200 de 200 traces em `*-relay`); o consumo
     pendura no span gRPC do `orders`, com 118 ms sem span; só F1 carrega
     `dmpf.correlation_id`; `dmpf.db.query` só tem `dmpf.dependency`, sem SQL, SQLSTATE nem
     linhas (`postgres/dbtrace.go:23-40`).
  3. **Grafana.** O derived field do Loki ficava com `url: ""` (`${…}` expandido no
     provisionamento); trace→log dependia do mapeamento padrão.
  4. **Métricas e SDK.** Sem métricas de runtime nem de pool; RED próprio fora da semconv;
     `WithEndpoint` forçado em `otelboot/otlp/exporters.go:30,48,70` anula
     `OTEL_EXPORTER_OTLP_*`; o `WithSampler` de `otelboot/start.go:136` sobrescreve
     `OTEL_TRACES_SAMPLER` em silêncio.
- **Impacto**: um log abre o trace; do trace de `place-order` chega-se, por
  `{ link:traceID = "<F1>" }` ou pela correlação, à drenagem e ao consumo; cada salto
  servidor produz um registro de acesso com as mesmas chaves; painéis e alertas usam nomes
  que qualquer ferramenta OTel reconhece.
- **Inspiração**:
  - Semconv de mensageria `v1.43.0` (`docs/messaging/messaging-spans.md:242,266-274,299-345`)
    — contexto de criação imutável, `send` com link, link como default produtor→consumidor.
  - Extensão CloudEvents Distributed Tracing — o envelope carrega o contexto do início da
    transmissão, não o de cada salto.
  - OTel Logs Data Model e ingestão OTLP nativa do Loki 3.7.7.
- **Links relevantes**:
  - `docs/dmpf/resiliencia-observabilidade.md` (FND-08) §5 (`TRC-*`), §6 (`MET-*`), §7 (`LOG-*`).
  - ADR-026, ADR-037, ADR-038, ADR-039, ADR-041, ADR-053, ADR-054.
  - SPEC-Z2HM6NAP (backlog) — confiança do `traceparent` na borda HTTP; mesmo sampler.
  - SPEC-JJKWG4JP — mergeada em `develop` (PR #6, `9ec83534`) e integrada nesta branch; os
    arquivos que ela alterou (†) partem do código dela.

<constraints>
- [P0] O envelope CloudEvents é o único portador do contexto de trace no salto assíncrono (`ENV-08`), e o `traceparent`/`tracestate` dele é o contexto de criação da mensagem, gravado uma vez e nunca reescrito pelo relay. Nenhum header Kafka nem message attribute SQS carrega contexto W3C.
- [P0] Todo salto assíncrono (F1→F2, lote→mensagem, produtor→consumidor, fronteira recusada) é link, nunca parentesco (`TRC-08` revisado).
- [P0] Nenhum payload, corpo, envelope inteiro, valor de parâmetro SQL, `client.address`, `network.peer.address`, `user_agent.original`, `url.path` com identificador ou `url.full` com query ou identificador sai do processo em span, log ou métrica; status de erro sem descrição e sem `exception.message` (`TRC-12`, `TRC-15`, `DAT-02`, `DAT-06`, `DAT-23`).
- [P0] Nenhum segredo, DSN, token, senha ou caminho de chave privada entra em sinal algum (`LOG-09`).
- [P0] Nenhuma instrumentação em `domain` ou `port` (`TRC-16`, `LOG-11`); `ports.MessageContext` só ganha campo de dado; bibliotecas OTel só em módulos `provider`/`app`, atrás de ports.
- [P0] Dependências só as nomeadas e pareadas: core `v1.47.0`, `semconv/v1.43.0`, contrib `v0.72.0` (`otelhttp`, `otelgrpc`, `instrumentation/runtime`, `exporters/autoexport`) e `processors/baggagecopy v0.17.0`. Tracer do `kotel`, tracer e `RecordStats` do `otelpgx` e `otelaws` não entram; lib que emite forma não canônica é substituída por código próprio mínimo com constantes `semconv/v1.43.0`.
- [P0] `trace_id`, `span_id`, `dmpf.correlation_id`, `dmpf.request_id`, `dmpf.tenant_id` e `messaging.message.id` nunca viram index label do Loki.
- [P1] O log das apps segue um único caminho, OTLP; o Alloy nunca raspa stdout de app.
- [P1] O contrato de wire (`contracts/envelope`) não muda de forma.
</constraints>

## Requisitos

### Funcionais

#### Frente A — Logs

- [x] **[P0] RF-A1 Log das apps só por OTLP**: `slog` → `otelslog v0.21.0` → `sdk/log v1.47.0` → `otlploggrpc v0.23.0` (escolhido pelo `autoexport` com `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`, RF-E1) → Collector → Loki `/otlp`. O handler JSON de `logging/handler.go` e o `WithoutKeys` de `logging/sinks.go` saem; com `OTEL_LOGS_EXPORTER=none` ou `OTEL_SDK_DISABLED=true`, o registro é descartado (`slog.DiscardHandler`, `boot/telemetry.go:80-81`). O log fica só no OTLP: o stderr recebe apenas a configuração recusada e o erro com que o processo termina, inclusive o da partida da telemetria, escritos pelo `cmd/main.go` de cada app. Um panic não chega ao stderr pela pilha do runtime: o servidor gRPC o recupera em `kernelgrpc.NewServer` (`grpc/recover.go`) e o relay o converte em `relay.ErrPanicked`, só a sentinela, sem o valor do panic (`app/relay/panic.go:17`, `app/relay/relay.go:76`), que o `Run` devolve ao `cmd/main.go`, e o `kernelapp.Consumer` converte o panic do `Handler` no `Unexpected` não retentável (`app/consumer.go`, `Consumer.handle`), e o `message consumed` e o `process` levam só o tipo Go do valor recuperado em `dmpf.panic.type`, nunca o valor nem a pilha (`app/consumer.go:231,242`, `app/process.go:147`, `app/log.go:62`), e o da quarentena e do ACK/Release na mesma falha `Unexpected`, que o `Consume` devolve ao worker como erro, não como panic, à frente da causa do `Handler`, que segue na árvore; por isso o registro do próprio worker (`kafka: sink returned without a gesture`, `sqs: sink failed`) leva `error.type` = `Unexpected`, já que o `errorAttr` dele toma a primeira categoria da árvore (`kafka/classifier.go:68-74`, `sqs/classifier.go:82-88`); o panic do próprio `Sink` vira `ErrSinkPanicked`, só a sentinela, sem o valor (`kafka/worker.go:206`, `sqs/consumer.go:290`) (`app/consumer.go:238-266,317-320`, `app/process.go:123-133`); a purga converte o panic em `kernelapp.ErrPurgePanicked`, sem o valor (`app/purge.go:102`), que `StartPurge` entrega ao `abort` (o `context.CancelCauseFunc` do contexto do papel, que encerra o laço principal) e que o `stop` devolve ao `Run` da app; o heartbeat e o worker do consumer SQS e o worker e a parada do consumer Kafka o convertem em `sqs.ErrPanicked` e `kafka.ErrPanicked`, também só a sentinela (`sqs/panic.go:23`, `kafka/panic.go:31`), que o `Run` do consumer devolve (`app/purge.go`, `sqs/panic.go`, `kafka/panic.go`) (decisões do usuário de 2026-10-03). Os erros do próprio `net/http` saem pelo `ErrorLog` do servidor do BFF com scope `net/http`, em `warn`, só com o texto anterior ao primeiro valor; endereço do par, valor do panic, pilha e texto do erro viram `<redacted>` (`bff/app/wiring.go:184-204`). O scope de cada registro é o import path do pacote emissor (`logging.NewLogger`), não `t.Service`: as libs recebem o `LoggerProvider` e criam o próprio logger. Os loggers de bibliotecas de terceiros ligados ao da plataforma usam o import path da raiz do módulo, porque o adaptador não sabe qual subpacote emitiu o registro: `google.golang.org/grpc` e `go.opentelemetry.io/otel` (`boot/libraries.go:25-26`) e `github.com/aws/aws-sdk-go-v2` (`sqs/awslog.go:14`).
- [x] **[P0] RF-A2 Registro no Logs Data Model**: `Timestamp`, `ObservedTimestamp`, `SeverityNumber`/`SeverityText`, `Body` = mensagem, Resource (`service.name`, `service.version`, `service.instance.id`, `deployment.environment.name`, `dmpf.process.role`) e `TraceId`/`SpanId`/`TraceFlags` do `ctx`. Nenhum atributo `time`, `level`, `service`, `version`, `instance`, `trace_id`, `span_id`, `duration_ms`; sai o prefixo `app.` de chave reservada.
- [x] **[P0] RF-A3 Chaves de atributo**: protocolo em semconv — `http.request.method`, `http.route`, `http.response.status_code`; `rpc.system.name`, `rpc.method`, `rpc.response.status_code`; `messaging.system`, `messaging.operation.name`, `messaging.destination.name`, `messaging.consumer.group.name`, `messaging.message.id`, `cloudevents.event_type`; `db.collection.name`. Contexto DMPF em `dmpf.*` (seção Revisão). Erro por `redact.Error`: `error.type` (categoria FND-07 ou `_OTHER`; o detalhe do transporte nunca entra nele) e `dmpf.error.code` quando o erro implementa `Categorized`; `Record.SetErr` e `slog.Any` com `error` são proibidos (`DAT-23`). A allowlist de `LOG-05` é imposta por um `sdk/log` Processor DMPF, que também aplica a amostragem por classe de `LOG-12` ao registro de trace não amostrado, inclusive o de um trace que segue um `traceparent` remoto não amostrado; registro sem span context válido, como os de partida e encerramento, não é tráfego e não passa por ela (`logging/sampling.go:28-30`).
- [x] **[P0] RF-A4 Correlação automática** (`LOG-02`, `LOG-04`): `dmpf.correlation_id`, `dmpf.request_id` e `dmpf.tenant_id` chegam a todo registro com execução em curso pelo `baggagecopy` LogProcessor (RF-B8). O `http request` do BFF sai com o contexto já montado (desde a SPEC-JJKWG4JP, o `defer` de `apps/backend/bff/app/api/middleware.go:53` lê o `ctx` por closure, já com o `ExecutionContext` de `:102` no caminho aceito, e o baggage é montado junto dele); o registro de consumo sai com o contexto da mensagem, não o do worker. Edge case: sem execução (partida, encerramento), as chaves ficam ausentes — nunca default (`CTX-26`).
- [x] **[P0] RF-A5 Um registro de acesso por salto, sem `duration_ms`**, com nível pela tabela única de `logging/severity.go` (servidor e consumo: `accepted`/`rejected` → `info`, `denied` → `warn`, `failed` → `error`, salvo o R1×D3 liberado do consumo; no consumo, o outcome vem da categoria FND-07 do erro, como no cliente (`Validation`, `DomainRejection`, `NotFound`, `Conflict` → `rejected`; `Forbidden`, `Unauthenticated` → `denied`; as demais e `_OTHER` → `failed`), e é `failed` sempre que o gesto não se concluiu, com a quarentena ou o ACK em erro (`app/log.go:67-93`); o nível do registro mede a urgência operacional e não muda o span nem a métrica: o `process` de uma mensagem recusada fecha com erro e o `messaging.process.duration` leva `error.type`, porque a mensagem não foi processada (`app/process.go:151-152`, `app/metrics.go:55-56`; decisão do usuário de 2026-10-03); cliente: `accepted` e `rejected` → `debug`, `denied` e `failed` → `warn`, `logging/severity.go:18-25`; decisão do usuário de 2026-10-03T02:05Z, na revisão de código):
  - BFF `http request`: `http.request.method`, `http.route`, `http.response.status_code`, `dmpf.outcome_category`, `dmpf.idempotency_key` (chave do cliente, se válida) ou `dmpf.idempotency_key.invalid` (sem o valor) e `dmpf.idempotency_key.derived` (chave escopada ao sujeito que segue ao contexto, IDM-03). Nas rotas de saúde (`/livez`, `/readyz`), servidas só na porta de administração (`ADMIN_ADDR`, default `:8090`, fora do Service público; a porta pública responde `404` do `ServeMux` a elas, com o registro da borda; decisão do usuário de 2026-10-03; `bff/app/api/health.go:27-36`, `bff/app/wiring.go:111-121`), o mesmo `http request`, com as quatro primeiras chaves, sai no máximo uma vez a cada 5 min por rota e processo quando a resposta é 2xx; a resposta fora de 2xx sai sempre, no nível da tabela (o `503` do `/readyz` em `error`), e não mexe na janela do sucesso (`LOG-10`; decisão do usuário de 2026-10-02, mantida na porta de administração; `bff/app/api/health.go:60-72`). Toda resposta com SERVER sai com exatamente um `http request`: o registro de acesso envolve o CORS e o `ServeMux` (`bff/app/api/middleware.go:200-215`, `bff/app/api/routes.go:170`, `bff/app/api/health.go:35`) e só cede o registro quando o handler que escreve o próprio de fato rodou e marcou o `statusRecorder` (`bff/app/api/middleware.go:39,131-135`, `bff/app/api/health.go:61`); o padrão casado não basta, porque o `ServeMux` o grava em `r.Pattern` também no 307 da limpeza de path (`net/http/server.go:2687-2694,2824` do go1.26.6 de `go.work`). As rotas do contrato e as de saúde escrevem o próprio registro; o preflight `OPTIONS` respondido pelo CORS, o 404 e o 405 do `ServeMux`, o 307 da limpeza de path (`//livez`, `/orders/./o-1`, em `info`) e os documentos OpenAPI recebem o da borda, com `http.request.method`, `http.response.status_code`, `dmpf.outcome_category` e `http.route` só quando um padrão do `ServeMux` atendeu (sem padrão, a chave fica ausente, como no SERVER). Em todo `http request` do BFF, `http.request.method` é o valor que o `otelhttp` grava no SERVER e na métrica: um dos nove métodos do HTTP em maiúsculas, ou `_OTHER`, nunca o valor enviado pelo cliente (`bff/app/api/middleware.go:181,191-198`; `otelhttp@v0.72.0/internal/semconv/util.go:118-126`).
  - Servidor gRPC: `grpc request` e `grpc call` fundidos em um `grpc call` com `rpc.system.name`, `rpc.method`, `rpc.response.status_code`, `dmpf.outcome_category` e `dmpf.idempotency_key` ou, com a chave fora do formato, `dmpf.idempotency_key.invalid` sem o valor. O protocolo `grpc.health.v1` e a reflection não geram `grpc call`: a cadeia de `ServerInterceptors` só intercepta os métodos do próprio serviço (`grpc/interceptor_context.go:106-116`), e o `stats.Handler` do registro usa o filtro do `otelgrpc` (`grpc/server.go:59,68`). Toda chamada com SERVER sai com exatamente um `grpc call`, emitido só no `stats.End` pelo `stats.Handler` que `kernelgrpc.NewServer` registra depois do `otelgrpc` (`grpc/access.go`), com o status com que o `grpc-go` fechou a chamada: código, nível e categoria vêm desse status, não do que o handler devolveu, porque a resposta é enviada depois da cadeia (`server.go:1283-1288` do `google.golang.org/grpc` v1.85.0-dev.0.20260825072537-93e31b48545e). A cadeia e a recuperação de panic só anotam na marca da chamada o contexto de execução, a chave de idempotência e o erro do handler (`grpc/interceptor_context.go:154`, `grpc/recover.go:46`), e esse erro só define a categoria e o `dmpf.error.code` quando o código dele é o do status final. A resposta acima de `grpc.MaxSendMsgSize` sai `RESOURCE_EXHAUSTED`; o handler que devolve OK depois do prazo sai com o `CANCELLED` ou o `DEADLINE_EXCEEDED` que o SERVER registra (o `grpc-go` fecha o stream pelo `RST_STREAM` do cliente ou pelo próprio timer, o que chegar antes), em `error`. A chamada recusada antes da cadeia sai sem as chaves `dmpf.*` de execução: o corpo que não decodifica (`INTERNAL`), a mensagem acima de `grpc.MaxRecvMsgSize` (`RESOURCE_EXHAUSTED`) e o peer recusado pelo `TrustedPeers` (`PERMISSION_DENIED`, em `warn`); o stream concluído também sai (decisões do usuário de 2026-10-03). A chamada que entra em panic também gera o seu `grpc call`, em `error`, com `rpc.response.status_code` = `INTERNAL`, `error.type` e `dmpf.outcome_category` = `Unexpected` (ERR-22), sem o valor nem a pilha do panic; o SERVER fecha com erro e `error.type` = `Unexpected`.
  - Cliente: `transport: call` em `debug` no sucesso; `transport: call failed` uma vez por tentativa, em `debug` quando a categoria FND-07 da resposta é de rejeição de negócio (`Validation`, `DomainRejection`, `NotFound`, `Conflict` → `rejected`) e em `warn` na recusa de autorização (`Forbidden`, `Unauthenticated` → `denied`) e na falha (`TransientDependency`, inclusive a recusa do breaker e do bulkhead, `RateLimited`, `DeadlineExceeded`, `Cancelled`, `Unexpected` e `_OTHER` → `failed`), com `dmpf.dependency`, `dmpf.retry.attempt`, `dmpf.outcome_category`, `error.type` e, quando o primeiro erro da cadeia que implementa `Categorized` tem código, `dmpf.error.code` (`RES-12` do breaker, `RES-14` do bulkhead; `transport/observe/observe.go:186-223`, `transport/README.md:33`). O `NotFound` que o polling da jornada `reservations` do load recebe é resposta esperada, não desvio: saía em `warn`, 1906 registros numa hora, e passa a `debug` (decisão do usuário de 2026-10-03T02:05Z, na revisão de código; `transport/observe/observe.go:202-212`).
  - Consumo: um `message consumed` por mensagem, emitido pelo `kernelapp.Consumer` depois do gesto de ACK (o envelope que não decodifica é contido antes de abrir o `process` e não gera `message consumed`, `app/consumer.go:129-138`), com as chaves de mensageria de RF-A3, `dmpf.inbox.disposition`, `dmpf.inbox.gesture` e `dmpf.outcome_category`; `kafka: record processed` sai (antes em `kafka/worker.go:168` de `develop`; a ausência é afirmada em `kafka/consumer_test.go:175-191`); `delivery of another event type acknowledged` (`reservations/app/sink.go:30`) passa a `debug`. Partição e offset ficam só no span. A mensagem R1×D3 liberada para nova tentativa (gesto `Release` sem erro e sem contenção) é o caminho saudável de retentativa, não falha: o `message consumed` sai em `info`, com a categoria da falha transitória em `dmpf.outcome_category`, e o `process` também não marca erro (`app/process.go:156-165`, `app/log.go:67-80`; decisão do usuário de 2026-10-02, na revisão de código). Um panic do `Handler` é recuperado pelo `kernelapp.Consumer` e convertido em `Failure(Unexpected)` não retentável, que `application.Classify` resolve para R1×D4 (ERR-22, ERR-23, ERR-24): a mensagem é contida como `terminal-failure` e confirmada, o `message consumed` sai em `error` com `dmpf.outcome_category` = `Unexpected` e o `process` fecha com erro e `error.type` = `Unexpected`, sem o valor nem a pilha do panic, iguais aos de um `Handler` que devolve `Failure(Unexpected)` (`app/consumer.go`; decisão do usuário de 2026-10-03). Um panic em `Containment.Quarantine` ou no `Acknowledger` (ACK ou Release) tem o mesmo destino: o `kernelapp.Consumer` o recupera, nada é confirmado depois de uma quarentena que não concluiu, e o `message consumed` sai em `error`, sem `dmpf.inbox.gesture`, com `dmpf.outcome_category` = `Unexpected`, que prevalece sobre a categoria do `Handler`; o `process` fecha com erro e `error.type` = `Unexpected`, e `messaging.process.duration` leva o mesmo `error.type`, sem o valor nem a pilha do panic (`app/consumer.go:238-266,317-320`, `app/process.go:123-133,171-173`; decisão do usuário de 2026-10-03).
- [x] **[P1] RF-A6 Registros operacionais** (detalhe operacional):
  - Partida: um único `process configured` por processo, com a config efetiva que varia por ambiente em `dmpf.config.<campo>`; segredo = `redact.Placeholder`; DSN decomposto em host, porta, banco e `sslmode`; usuário, username SASL e caminho de chave privada = `set`/`unset`; `METRIC_TENANTS` = contagem (`LOG-07`, `LOG-09`). Constantes de código não entram.
  - Opt-out de TLS: um `warn` por processo e papel — cliente Kafka (`kafka/newconfig.go:28`), cliente gRPC (`grpc/config.go:172-173`), servidor gRPC (`grpc/server.go:89-90`), cliente SQS (`sqs/api.go:65-66`) e exportação OTLP com endpoint efetivo `http://` em sinal de exporter `otlp`, um por processo (`boot/telemetry.go:102,151-185`).
  - Resiliência: transição de breaker (`resilience/breaker.go:277-279`) em `warn` com `dmpf.dependency` e `dmpf.breaker.state`.
  - Contenção: `message contained` em `warn` com `messaging.message.id` e `dmpf.containment.reason` em `postgres/quarantine.go` e `kafka/dlq.go`; o header `HeaderError` do DLQ (`kafka/dlq.go:110`) leva a categoria, não o texto do erro (`DAT-03`).
  - Relay: `outbox publish failed` em `warn` por tentativa falha, com `messaging.message.id`, `dmpf.outbox.attempt` e `error.type`; falha de claim em `error`; um panic no claim, na abertura do drain, na entrega ou na devolução encerra o `Run` com `ErrPanicked`, sem o valor do panic; depois do claim, a devolução dos claims não concluídos (OBX-13) e o fim do `outbox drain` rodam em `defer` (`app/relay/relay.go:117-123`), e o `cmd/main.go` o escreve como erro de término. A devolução que o banco recusa por erro, e não por claim substituído (OBX-10), sai em `warn` como `outbox release failed` com `messaging.message.id` e o erro por `redact.Error` (`app/relay/shutdown.go:66-68`, `app/relay/log.go:51-55`).
  - Kafka: atribuição, revogação e perda de partição (`kafka/consumer.go:107-109`) em `info` com `messaging.consumer.group.name` e `messaging.destination.name`, partições no `Body`.
  - Autenticação: recusa de token em `warn` com `error.type` = `Unauthenticated` e o motivo em `dmpf.auth.refusal_reason` ∈ {`token_expired`, `audience_mismatch`, `issuer_mismatch`, `signature_invalid`, `malformed`} (`authn/oidc.go:100-143`; a assinatura é conferida antes de `iss`, `aud` e `exp` — go-oidc v3.21.0, `oidc/verify.go:200,334` —, então token forjado sai `signature_invalid` mesmo vencido ou de outra audiência), inclusive o token que passa pela biblioteca sem `sub` (`ErrSubjectUnresolved`), o de `nbf` futuro e a credencial com esquema diferente de `Bearer`, como `Token` ou `Basic` (comparação sem caixa, `authn/oidc.go:64-65`), todos com `malformed`; credencial ausente (`ErrCredentialAbsent`), inclusive o esquema que chega sem valor (`authn/identity.go:28-30`, `ports/authn.go:20`), não registra recusa.
  - `not ready` de `bff/app/api/health.go:122` por `redact.Error`; a chave `table` de `postgres/wait.go:35` vira `db.collection.name`; em `app/purge.go:120,125` (SPEC-JJKWG4JP), `purge cycle failed` leva o erro por `redact.Error`, `table` vira `db.collection.name`, e `removed`/`before` viram `dmpf.purge.removed`/`dmpf.purge.before`.
- [x] **[P1] RF-A7 Auditoria** (separação lógica de `LOG-13`, `LOG-14`): `observability/audit` emite pela Logs API com `EventName` `dmpf.audit`, scope = import path do pacote e atributos `dmpf.audit.{subject,object,action,outcome}` mais a correlação; o subject nunca é copiado para span (`DAT-25`). Consultas e painéis operacionais excluem esse scope por `scope_name`.
- [x] **[P0] RF-A8 Coleta**: o Collector exporta logs por `otlp_http` ao `/otlp` do Loki; o Loki usa a promoção default de resource attributes (sem `otlp_config` próprio) e deriva `detected_level`. O Alloy só processa containers de infraestrutura, sem `stage.json`: exclui os serviços `dmpf-*` no Compose e `app.kubernetes.io/name` ∈ {`bff`, `orders`, `reservations`, `bookings`} no Kubernetes, igual em `alloy-docker.alloy` e `alloy-kubernetes.alloy`. `tools/otel-env-check.sh` (com `--self-test`), no CI, reprova manifesto ou template de app com `OTEL_LOGS_EXPORTER` diferente de `otlp`, `OTEL_EXPORTER_OTLP_PROTOCOL` ausente ou diferente de `grpc`, `OTEL_EXPORTER_OTLP_ENDPOINT` fora de `http://<host>:4317` ou de `https://<host>:4317` com `OTEL_EXPORTER_OTLP_CERTIFICATE` (geral ou do sinal) no mesmo arquivo, `http://` em arquivo de `k8s/overlays/hmg`, overlay `hmg` sem `OTEL_EXPORTER_OTLP_ENDPOINT` e `OTEL_EXPORTER_OTLP_CERTIFICATE`, certificado que não seja caminho absoluto, `OTEL_BSP_EXPORT_TIMEOUT` ou `OTEL_BLRP_EXPORT_TIMEOUT` ausente ou diferente de `3000`, variante por sinal `OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_{PROTOCOL,ENDPOINT}` declarada fora das regras da variável geral correspondente, `OTEL_TRACES_SAMPLER_ARG` ausente ou diferente de `1.0` (RF-E5, RF-E7) e `OTEL_GO_X_OBSERVABILITY` diferente de `true` (RF-D7), e config do Alloy sem a exclusão das apps.

#### Frente B — Traces

- [x] **[P0] RF-B1 Nome e atributo semconv** (`TRC-03` revisado): todo span com equivalente usa o nome e as chaves de `semconv/v1.43.0`; `dmpf.service`, `dmpf.version`, `dmpf.error.category`, `dmpf.operation` e `dmpf.dependency` (fora do span de resiliência) saem do span. Nas gravações do DMPF, `error.type` recebe a categoria FND-07, o SQLSTATE ou `_OTHER`, nunca o fallback por reflexão de `semconv.ErrorType`. Os spans das instrumentações contrib seguem a semconv delas: o CLIENT do `otelhttp.NewTransport` (`http/client.go:102`, `authn/oidc.go:45`) grava em `error.type` o código de status da resposta de erro (`otelhttp@v0.72.0/internal/semconv/client.go:176-179`) ou, no erro de transporte, o tipo Go do erro, por reflexão quando o erro não declara `ErrorType()` (`otelhttp@v0.72.0/transport.go:301`; na métrica, `internal/semconv/client.go:283-284,302-324`), e a categoria FND-07 fica nos spans do DMPF, como o INTERNAL de RF-B5 (`TRC-12` revisado; decisão do usuário de 2026-10-02 sobre `TRC-12` e `MET-08`). O conjunto fechado de `tracing/attributes.go` aceita "`dmpf.*` ou constante semconv". O `dmpf.retry.previous_category` do evento `dmpf.retry.attempt` e o `error.type` de `dmpf.dependency.retries` recebem, com o classificador declarado pelo provedor (`compose.Config.Category`, presente em todos os provedores do kernel), a mesma categoria do `error.type` do `dmpf.resilience`: o `ErrorCategory()` do erro categorizado, senão a categoria do classificador do transporte (`resilience/retry_decorator.go:177-190`, `transport/compose/compose.go:98`), e `_OTHER` só sem nenhuma das duas (decisão do usuário de 2026-10-03).
- [x] **[P0] RF-B2 Borda HTTP do BFF por `otelhttp v0.72.0`**: um SERVER por requisição, nome `{http.request.method} {http.route}`, 4xx com status `Unset`; headers só `content-type` e `accept` em `http.request.header.*`. RED `http.server.request.duration` vem do mesmo handler (RF-D2). Exceção (decisão do usuário de 2026-10-02): na porta de administração, `GET` e `HEAD` de `/livez` e `/readyz` não abrem SERVER nem ponto de `http.server.request.duration`, porque o filtro de `otelhttp.WithFilter` pergunta ao `ServeMux` dela que handler atenderia a requisição e só pula as rotas de saúde montadas (`bff/app/api/health.go:34,38-44`; `otelhttp@v0.72.0/handler.go:89-95`); o resto, inclusive o `405` nesses paths (o de `OPTIONS` incluído, porque a porta não tem CORS) e o `404` que a porta pública dá a `/livez` e `/readyz`, continua com SERVER e ponto. A extração aceita o `traceparent` do cliente e descarta o `tracestate`: nenhum span do BFF nem chamada gRPC aos contextos o carrega (decisão do usuário de 2026-10-03; `bff/app/api/traceparent.go:12-25`, `bff/app/api/routes.go:177`); a confiança do `traceparent` continua com a SPEC-Z2HM6NAP. O registro delas segue RF-A5.
- [x] **[P0] RF-B3 Decorador de privacidade**: `otelboot` envolve o `SpanExporter` (o `ReadOnlySpan` de `OnEnd` não aceita escrita) e, antes do envio, remove `client.address`, `network.peer.address`, `network.peer.port` e `user_agent.original`, troca `url.path` pelo template de `http.route` do mesmo span (sem `http.route`: `REDACTED`), reescreve `url.full` sem query nem fragmento e com o path trocado por `url.template` do mesmo span (sem `url.template`: `/REDACTED`), porque o `otelhttp.NewTransport` grava o URL inteiro (`otelhttp@v0.72.0/internal/semconv/client.go:114-123`), zera `Status.Description` e retira `exception.message` e `exception.stacktrace` dos eventos. O valor nunca sai do processo (`TRC-15`).
- [x] **[P0] RF-B4 gRPC por `otelgrpc v0.72.0`**: `NewServerHandler` em `kernelgrpc.NewServer` (`grpc/server.go:62`) e `NewClientHandler` em `Dial` (`grpc/dial.go:46`); um SERVER por chamada e um CLIENT por tentativa, nome `{rpc.method}`, `rpc.system.name=grpc`, `rpc.response.status_code`; health e reflection fora por `WithFilter`. O `serverSpan` de `grpc/interceptor_context.go:112-127` sai; o interceptor mantém o `ExecutionContext` e o registro de acesso.
- [x] **[P0] RF-B5 Resiliência em INTERNAL** (`TRC-05`, `TRC-11`): `transport/observe` abre `dmpf.resilience {dmpf.dependency}` INTERNAL sobre as tentativas, com `dmpf.dependency`, `dmpf.deadline.remaining_ms`, `dmpf.retry.max_attempts` e `dmpf.outcome_category`, e eventos `dmpf.retry.attempt` (tentativa e `dmpf.retry.previous_category`), `dmpf.breaker.rejected`, `dmpf.bulkhead.saturated` e `dmpf.degraded` (com `dmpf.error.code`). No HTTP de saída (`libs/backend/go/http/client.go`, discovery e JWKS de `authn/oidc.go`), `otelhttp.NewTransport` abre o CLIENT por tentativa com `http.request.resend_count`. No caminho do relay, o `send` (RF-B7) é o span dono (`tracing.OwnsSpan`): `transport/observe` não abre `dmpf.resilience` sob ele e grava os mesmos atributos e eventos no `send`.
- [x] **[P0] RF-B6 Postgres por código próprio** (`postgres/dbtrace.go`): nome `{db.operation.name} {db.collection.name}`, recuo para `{db.operation.name}` e depois `postgresql`; `db.system.name=postgresql`, `db.namespace`, `server.address`, `server.port`, `db.query.text` parametrizado sem `Args`, `db.operation.name` em maiúsculas, `db.collection.name` só em comando único com alvo único, `db.response.status_code` e `error.type` = SQLSTATE, `db.response.returned_rows` (SELECT) e `dmpf.db.rows_affected` (DML). BEGIN, COMMIT e ROLLBACK levam só `db.operation.name`. Nunca `PgError.Message`/`Detail`/`Hint`, DSN nem usuário. A supressão de query sem pai (`dbtrace.go:72-74`) continua.
- [x] **[P0] RF-B7 F1 → F2** (`TRC-01`, `TRC-08`):
  - `ports.MessageContext` ganha `Tracestate`; o interceptor gRPC o preenche com o `tracestate` do span ativo até 512 bytes, o piso da W3C Trace Context §3.3.1.5, e acima disso o descarta inteiro, sem cortar membro (`grpc/interceptor_context.go:300,320-330`); `postgres/outbox.go` grava `traceparent` e `tracestate` do span ativo em `metadata`, e `relay/record.go` (`Assemble`) os leva ao envelope sem alteração.
  - Depois de claim não vazio, o relay abre a raiz `outbox drain {destino}` INTERNAL com início no instante do claim, evento `claimed`, `messaging.batch.message_count`, `dmpf.outbox.claim_id` e um link por mensagem ao contexto de criação, com `messaging.message.id` no link; lote de destinos mistos abre `outbox drain`. `MarkPublished`, `Reschedule`, `Fail` e `release` rodam sob o drain; a query do claim não abre span.
  - Por mensagem, `send {messaging.destination.name}` CLIENT, filho do drain, com link ao contexto de criação e `messaging.system` (de `relay.Config.System`), `messaging.operation.type=send`, `messaging.operation.name=send`, `messaging.destination.name` (tópico físico), `messaging.message.id`, `messaging.message.conversation_id`, `cloudevents.event_id`, `cloudevents.event_source`, `cloudevents.event_type`, `dmpf.tenant_id`, `dmpf.request_id` próprio da tentativa, `dmpf.outbox.attempt` (a partir de 1), `dmpf.outcome_category` e `error.type`. O `send` abre de um `ctx` sem o membro `dmpf.request_id` do baggage da drenagem: o `baggagecopy` grava no `OnStart` cada membro do baggage do `ctx` pai (`baggagecopy@v0.17.0/processor.go:42-50`) e sobrescreveria o atributo de início. O publisher Kafka grava `messaging.destination.partition.id` e `messaging.kafka.offset` no `send`, que é o span ativo (RF-B5).
  - Os spans `dmpf.kafka.publish` (`kafka/publisher.go:119-122`), `dmpf.sqs.*` e SNS deixam de existir; o record Kafka ganha o header `content-type: application/cloudevents+protobuf` do binding CloudEvents, sem header W3C por salto.
  - Edge case: `traceparent` inválido no `metadata` → a mensagem publica sem link e o drain registra `invalid_creation_context`; telemetria nunca falha a drenagem.
- [x] **[P0] RF-B8 Herança por W3C Baggage in-process**: a borda do BFF, o interceptor do servidor gRPC e o `kernelapp.Consumer`, a partir de `ports.ExecutionContext` (`tracing.WithExecutionBaggage`), e o relay (cada `send`, com os dados do `metadata`; o drain só com o `dmpf.request_id` dele, `app/relay/drain.go:64-66`) montam baggage com `dmpf.correlation_id`, `dmpf.request_id` e `dmpf.tenant_id`; `baggagecopy v0.17.0` (SpanProcessor e LogProcessor, `Filter` restrito às três chaves) os copia para spans e logs. `OTEL_PROPAGATORS=tracecontext`: baggage nunca vai ao fio. Em `send` e `process` o baggage é montado depois do `Start`, e esses spans não recebem `dmpf.correlation_id` (decisão 3); os logs e os spans descendentes recebem. No relay, a correlação do span fica só em `messaging.message.conversation_id` do `send` (`app/relay/send.go:98-100`): nem o drain nem o `UPDATE outbox` sob ele levam `dmpf.correlation_id` (decisão do usuário de 2026-10-02). Os SERVER de `otelhttp` e `otelgrpc` também começam antes do baggage; por isso o middleware do BFF e o interceptor do servidor gRPC gravam as três chaves no span SERVER corrente com `tracing.ExecutionAttributes`. `dmpf.tenant_id` fica ausente em cadeia de plataforma sem sujeito.
- [x] **[P0] RF-B9 F3 no kernel** (`TRC-07`, `TRC-10`):
  - Depois de `Boundary.admits` (`app/consumer.go:140`), o `kernelapp.Consumer` abre a raiz `process {messaging.destination.name}` (tópico físico, não o canal lógico) CONSUMER com link ao contexto de criação do envelope, `messaging.system`, `messaging.operation.type=process`, `messaging.destination.name`, `messaging.consumer.group.name` (de `Channel.Group`, não de `Consumer.Name`), `messaging.message.id`, `messaging.message.conversation_id`, `cloudevents.event_id`, `cloudevents.event_source`, `cloudevents.event_type`, `dmpf.inbox.attempt`, `dmpf.inbox.disposition`, `dmpf.inbox.gesture`, `dmpf.containment.reason` quando contida e `dmpf.request_id` próprio; o baggage de RF-B8 é montado depois do `Start`. Extração e span saem de `reservations/app/sink.go:40-43`.
  - Fora da fronteira confiável: raiz nova com link ao contexto recebido, classe `error`; `dmpf.provenance.traceparent` sai.
  - Consumo em cadeia: a linha de outbox escrita durante o consumo grava o contexto do span ativo (`process` ou descendente, `app/consumer.go:180-187`), não `env.TraceParent`.

#### Frente C — Grafana

- [x] **[P0] RF-C1 Log→trace**: o derived field `TraceID` lê o structured metadata `trace_id` e usa `url: '$${__value.raw}'`; o datasource efetivo expõe `${__value.raw}`.
- [x] **[P1] RF-C2 Trace→logs**: `tracesToLogsV2` com `tags: [{key: service.name, value: service_name}]`, `filterByTraceID: true`, `spanStartTimeShift: -5m`, `spanEndTimeShift: 5m` e `customQuery` sobre structured metadata `{$${__tags}} | trace_id="$${__trace.traceId}" | scope_name != "github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"`, que exclui a auditoria pelo `scope_name` (RF-A7).
- [x] **[P1] RF-C3 Da correlação a todos os traces**: o derived field `correlation_id` lê o structured metadata `dmpf_correlation_id` (forma normalizada pelo Loki da chave `dmpf.correlation_id`, conferida na Tarefa 16.2 do plano) e abre `{ span.dmpf.correlation_id = "<v>" || span.messaging.message.conversation_id = "<v>" }`, que lista F1, F2 e F3.
- [x] **[P1] RF-C4 Gate de provisionamento**: `tools/grafana-provisioning-check.sh` reprova `${` sem `$` à esquerda em `infra/observability/grafana/*.yaml`, inclusive na `customQuery`, e o `tracesToLogsV2` de `datasources.yaml` ausente, sem `customQuery: true` ou cuja consulta não exclui o `scope_name` da auditoria (RF-C2), e roda em `.github/workflows/ci.yml`.
- [x] **[P1] RF-C5 Painéis**: `reference.json` consulta `{service_name=~"$service"}` sem `| json` nem `line_format`; `load-bff.json` tira o filtro `span_name=~"HTTP .*"`: o único painel que ainda lê as séries de spanmetrics do Tempo filtra só por `span_kind="SPAN_KIND_SERVER"` (o valor do label nessas séries, `load-bff.json:596`; decisão do usuário de 2026-10-02, na revisão de código), e os painéis por rota passam ao histograma `http.server.request.duration` do `otelhttp`, por `http.request.method` e `http.route` (`load-bff.json:480`, `:529` e `:652`); todos os painéis usam os nomes de RF-D1/RF-D2, e o breaker sai sem `_ratio` (`load-bff.json:948`).
- [x] **[P2] RF-C6 Roteiro**: `infra/README.md` documenta log→trace, trace→logs e F1 → F2/F3 por `{ link:traceID = "<F1>" }` e pela correlação.

#### Frente D — Métricas

- [x] **[P0] RF-D1 Catálogo DMPF na forma OTel** (`MET-02`/`MET-03` revisados): nome com ponto, sem unidade nem `_total`, unidade UCUM em `WithUnit` (`s`, `{retry}`, `{execution}`, `{call}`, `{response}`, `{request}`, `{message}`, `{state}`; `1` só para razão). `metrics/catalog.go`, com os nomes da síntese da auditoria (`keep`): `dmpf.operation.duration` (`s`, caso de uso), `dmpf.dependency.{retries,budget.exhausted,breaker.state,deadline_exceeded,cancellations,bulkhead.rejections,degraded,omitted}` (antes `dmpf_service_{degraded,omitted}_total`), `dmpf.admission.rejections` (`{request}`), `dmpf.consumer.pool.utilization` (`1`) e `dmpf.consumer.queue.depth` (`{message}`) (MET-11, MET-12). `prometheus.yaml` declara `otlp.translation_strategy: UnderscoreEscapingWithSuffixes`.
- [x] **[P0] RF-D2 RED canônico**: `http.server.request.duration` (`otelhttp` handler), `http.client.request.duration` (`otelhttp.NewTransport`, HTTP de saída de RF-B5), `rpc.server.call.duration` e `rpc.client.call.duration` (`otelgrpc`), `messaging.client.operation.duration` e `messaging.client.sent.messages` no `send`, `messaging.process.duration` no `process` (código próprio com `messagingconv`). `dmpf_service_request_duration_seconds`, `dmpf_service_requests_total` e `dmpf_service_errors_total` (`transport/observe/observe.go:121-127` e `observability/usecase/instrumentation.go:149-171`) saem; o RED do caso de uso vira `dmpf.operation.duration` (`s`), com `dmpf.operation`, `dmpf.outcome_category` e `error.type` (conceito DMPF, RF-D1); vazão e erro vêm do `_count` com `error.type`.
- [x] **[P0] RF-D3 Labels**: semconv (`error.type`, `http.request.method`, `http.route`, `http.response.status_code`, `rpc.method`, `rpc.response.status_code`, `messaging.operation.name`, `messaging.destination.name`, `db.operation.name`) ou `dmpf.dependency`, `dmpf.operation` (RED do caso de uso e séries de dependência do catálogo, RF-D2 e `MET-04`; `metrics/catalog.go:101,109,117`), `dmpf.outcome_category` e `dmpf.tenant_id` (só os de `METRIC_TENANTS`, `MET-07`); `service` sai (é resource). A View de `otelboot` dá a cada instrumento um `AttributeFilter` (`otelboot/views.go:18-63`): nas séries do catálogo DMPF (`metrics.Catalog()`), a allowlist dos labels declarados, que descarta toda outra chave; nas séries semconv fora do catálogo — das instrumentações contrib (`otelhttp`, `otelgrpc`, runtime), das que o DMPF grava por `messagingconv` e `dbconv` e das `otel.sdk.*` do SDK —, o conjunto de atributos da instrumentação canônica, menos as chaves de RF-B3 (`client.address`, `network.peer.address`, `network.peer.port`, `user_agent.original`, `url.full`, `url.path`, `exception.message`, `exception.stacktrace`) e, nas séries HTTP do servidor (`http.server.request.duration`, `http.server.request.body.size`, `http.server.response.body.size`), também `server.address` e `server.port`, que o `otelhttp` deriva do `Host` do cliente (`otelhttp@v0.72.0/internal/semconv/server.go:368-369`) (decisão do usuário de 2026-10-02, na revisão de código). O valor do label semconv tem a forma que a instrumentação canônica grava (decisão do usuário de 2026-10-02): em `dmpf.admission.rejections`, `http.route` é a chave da rota a partir da primeira `/`, sem o método (`/orders/{id}/place`; `http/admission.go:63,80-85`, como o `http.route` do `otelhttp`, `otelhttp@v0.72.0/internal/semconv/util.go:78-83`), e `rpc.method` é o `FullMethod` sem a barra inicial (`company.orders.service.v1.OrdersService/PlaceOrder`; `grpc/admission.go:40`, como o `otelgrpc`, `otelgrpc@v0.72.0/internal/parse.go:21-28`); a recusa de rota HTTP sem limite declarado fica em `undeclared` (`http/admission.go:64-66`).
- [x] **[P1] RF-D4 Buckets**: todo histograma de duração em segundos usa as fronteiras advisory da semconv (0,005; 0,01; 0,025; 0,05; 0,075; 0,1; 0,25; 0,5; 0,75; 1; 2,5; 5; 7,5; 10), salvo `go.schedule.duration`, cujas fronteiras vêm do histograma do runtime Go (`runtime@v0.72.0/producer.go:86`, RF-D5).
- [x] **[P1] RF-D5 Runtime Go**: `contrib/instrumentation/runtime v0.72.0`, com `runtime.Start` sobre o `MeterProvider` do processo e `runtime.NewProducer()` registrado por `autoexport.WithFallbackMetricProducer`, chamado antes de `autoexport.NewMetricReader` (`autoexport@v0.72.0/metrics.go:85-95`; `OTEL_METRICS_PRODUCERS` declarado prevalece), fonte de `go.schedule.duration`; saem `go.memory.used`, `go.memory.allocated`, `go.memory.allocations`, `go.memory.gc.goal`, `go.goroutine.count`, `go.processor.limit`, `go.config.gogc`, `go.schedule.duration` e, com `GOMEMLIMIT`, `go.memory.limit`.
- [x] **[P1] RF-D6 Pool Postgres por `dbconv`** (`postgres/pool.go`): `db.client.connection.count` (`db.client.connection.state` = `idle`|`used`), `db.client.connection.max`, `db.client.connection.pending_requests` e `db.client.connection.timeouts` por callback (`postgres/pool.go:157-171`): `count` e `max` de `pgxpool.Stat()`; `pending_requests` de um contador que o `pgxpool.AcquireTracer` soma no início da aquisição e subtrai no fim (`pool.go:180-186`), porque `pgxpool.Stat()` não expõe as aquisições em espera (`pgx@v5.10.0/pgxpool/stat.go:18-91`); `timeouts` de `Stat().CanceledAcquireCount()`, que conta toda aquisição cancelada por contexto, inclusive pelo cancelamento do chamador, e não só pelo prazo excedido (`pgxpool/stat.go:33-37`). `db.client.connection.wait_time` sai do mesmo `AcquireTracer`, só na aquisição concluída sem erro (`pool.go:185-192`); `db.client.connection.pool.name` = `{server.address}:{server.port}/{db.namespace}` (decisão do usuário de 2026-10-02, na revisão de código).
- [x] **[P1] RF-D7 Autoinstrumentação do SDK**: `otel.sdk.processor.span.processed` com `error.type=queue_full` substitui `dmpf_otel_spans_dropped_total`. No SDK `v1.47.0` a métrica só sai com a feature experimental `OTEL_GO_X_OBSERVABILITY=true` (`sdk@v1.47.0/internal/x/features.go:25-31`), declarada nos manifestos e templates das apps; por ser experimental, o nome e a ativação podem mudar em upgrade do core.

#### Frente E — SDK e configuração

- [x] **[P0] RF-E1 Env `OTEL_*`**: sem `WithEndpoint`/`WithInsecure`, os exporters leem `OTEL_EXPORTER_OTLP_{ENDPOINT,PROTOCOL,INSECURE,HEADERS,TIMEOUT,COMPRESSION}` e as variantes por sinal. O `autoexport` usa `http/protobuf` por default (`autoexport@v0.72.0/metrics.go:52,119-121`) e o Collector só recebe OTLP/gRPC em `0.0.0.0:4317` (`otel-collector.yaml`); os manifestos e templates das apps declaram `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`, `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317` (no Kubernetes, o host do Service do Collector na mesma porta) e `OTEL_BSP_EXPORT_TIMEOUT` e `OTEL_BLRP_EXPORT_TIMEOUT` em `3000` (ms; o default do SDK é 30000, acima da fatia de cada sinal na janela de encerramento), e o esquema `http://` desliga o TLS do gRPC no Compose e em `dev`. Em `hmg`, o patch do ConfigMap de cada app declara `OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector:4317` e `OTEL_EXPORTER_OTLP_CERTIFICATE` com a CA montada — `/etc/dmpf/otel/ca.crt` (Secret `otel-collector-ca`) nos contextos, `/etc/dmpf/grpc/ca.crt` (Secret `bff-grpc-ca`) no BFF (`apps/backend/*/deploy/k8s/overlays/hmg/configmap-*-patch.yaml:18-19`, bff `:14-15`) —, que o SDK lê sem `WithTLSCredentials` (`otlptracegrpc@v1.47.0/internal/otlpconfig/envconfig.go:84,112-118`, `otlpmetricgrpc@v1.47.0/internal/oconf/envconfig.go:86`, `otlploggrpc@v0.23.0/config.go:61-62,581-583`). `OTLP_ENDPOINT`, `OTLP_INSECURE`, `OTLP_LOGS`, `SERVICE`, `SERVICE_VERSION`, `INSTANCE_ID` e `TRACE_SAMPLE_RATE` saem do código, de `infra/local/compose/app-base.yml`, dos `.env*` e `k8s/base/configmap.yaml` das apps e dos templates do `tools/dmpf-plugin`. `LOG_LEVEL` continua.
- [x] **[P0] RF-E2 Seleção de exporter por `autoexport v0.72.0`**: `OTEL_{TRACES,METRICS,LOGS}_EXPORTER` ∈ {`otlp`, `console`, `none`}, default `otlp`; o modo em memória por endpoint vazio (ADR-041) sai; os harnesses de teste declaram `none`. `otelboot/otlp` é removido.
- [x] **[P0] RF-E3 Resource**: `resource.New` com `WithFromEnv`, `WithTelemetrySDK`, `WithProcessRuntimeName`, `WithProcessRuntimeVersion` e `WithSchemaURL` da semconv `v1.43.0`; `OTEL_SERVICE_NAME`; `OTEL_RESOURCE_ATTRIBUTES` com `service.version`, `service.instance.id`, `deployment.environment.name` e `dmpf.process.role` ∈ {`api`, `relay`, `consumer`}, declarados por papel nos manifestos (Compose e Kubernetes) e nos targets `serve-<papel>`, `docker:run-relay` e `docker:run-consumer`; no `docker:run` do papel `api`, inferido pelo `@nx/docker` em `nx.json` (o `CMD` da imagem é `--role api`), o papel entra pelo `--role` (`otelboot/config.go:145,168`), com o mesmo valor no resource. Sem `service.version` ou `service.instance.id` depois do `WithFromEnv`, a partida falha com `ErrResourceIncomplete`. `WithHost`, `WithContainerID`, `WithProcessCommandArgs` e `WithProcessOwner` não entram.
- [x] **[P0] RF-E4 Propagador** (`TRC-09`, decisão 4): `OTEL_PROPAGATORS` ausente ou `tracecontext` monta `propagation.TraceContext`; qualquer outro valor falha a partida (`baggage` vazaria o tenant; `none` e propagadores de terceiros violam o W3C Trace Context explícito de `TRC-09`).
- [x] **[P0] RF-E5 Sampler** (`TRC-13`): `AlwaysRecord(ParentBased(root))`, sem a aritmética de `otelboot/sampler.go`. O sampler decorado grava `dmpf.traffic_class` (ausente → `unclassified`) nos atributos do `SamplingResult` de todo span local, inclusive quando `ParentBased` segue um pai remoto ou local, para que a cauda (RF-E7) classifique todo trace; o `root` amostra se um link válido está amostrado quando a classe é `write` ou `read`, e senão aplica `TraceIDRatioBased` à taxa da classe: `OTEL_TRACES_SAMPLER_ARG` como taxa uniforme para `write`, `read` e `unclassified` (`error` e `maintenance` em 1), ausente → `1.0`, o default de `traceidratio` na spec do SDK. A tabela de TRC-13 vive só na cauda (RF-E7): cabeça e cauda com a tabela multiplicariam as taxas e descartariam erro na cabeça, contra TRC-14. `OTEL_TRACES_SAMPLER` declarado gera um único `warn` e é ignorado (decisão 11). `LOG-12` usa a tabela de TRC-13 (`tracing.DefaultRates`, `tracing/sampling.go:28-38`: `error`, `write` e `maintenance` em 1, `read` e `unclassified` em `0.01`), não o `OTEL_TRACES_SAMPLER_ARG`.
- [x] **[P0] RF-E6 Processadores**: `sdktrace.NewBatchSpanProcessor` (lê `OTEL_BSP_*`) sobre o exporter decorado de RF-B3, precedido do `baggagecopy`; o encerramento fecha traces, métricas e logs nessa ordem, cada sinal com parte igual do que resta do prazo (`otelboot/start.go:243-244,260`), e `Runtime.ForceFlush` esvazia os três (`otelboot/start.go:223`); os manifestos declaram `OTEL_BSP_EXPORT_TIMEOUT=3000` e `OTEL_BLRP_EXPORT_TIMEOUT=3000`, abaixo da parte de cada sinal na janela de 10 s (`observability.ShutdownGrace`); o `ClassAwareProcessor` e `DefaultQueueSize`/`DefaultBatchSize` de `otelboot/processor.go` saem.
- [x] **[P0] RF-E7 TRC-14 por tail sampling**: o Collector passa a `otel/opentelemetry-collector-contrib:0.160.0` (`infra/local/compose/otel-collector.yml`, `infra/observability/otel-collector/deployment.yaml`, uma réplica) com `tail_sampling` no pipeline de traces: `status_code` `ERROR` e `dmpf.traffic_class` ∈ {`error`, `maintenance`} sempre; `write` sempre (100%), para que F1, F2 e F3 da mesma escrita nunca se separem; `read` e `unclassified` por `probabilistic` à taxa da classe; por último, trace sem `dmpf.traffic_class` ou com classe fora da taxonomia (origem fora do DMPF) por `probabilistic` à taxa de `unclassified`, para nunca ser descartado em silêncio. Os manifestos das apps declaram `OTEL_TRACES_SAMPLER_ARG=1.0` (RF-A8 reprova outro valor); o Compose declara 100% em todas as classes do tail (ADR-054 §6), o Kubernetes a taxa de TRC-13 só para `read` e `unclassified`. No `hmg`, o receiver OTLP/gRPC do Collector serve TLS com o certificado do Secret `otel-collector-tls`, emitido pela CA dos certificados de servidor dos `api`, com `otel-collector` no SAN: `infra/k8s/overlays/hmg/otel-collector-tls.yaml` entra como segundo `--config`, que o Collector funde ao `config.yaml` da base, e `otel-collector-patch.yaml` monta o Secret em `/etc/otelcol-tls`; o Compose e o `dev` seguem em texto claro. Nos dois overlays, a NetworkPolicy `otel-collector` (`infra/observability/otel-collector/networkpolicy.yaml`) só admite a 4317 de pods com `app.kubernetes.io/part-of: dmpf` e `app.kubernetes.io/component` e a 8888 do Prometheus (decisão do usuário de 2026-10-03).

### Não-funcionais

- [x] Performance: `pnpm nx run load:smoke` verde; `pnpm nx run load:average` com `reservation_convergence_timeouts` = 0, p95 da borda ≤ 90,5 ms e `reservation_convergence` p95 ≤ 651 ms (110% do baseline `average-20260930T153425`: 82,3 ms e 592 ms), na mesma máquina.
- [x] Cardinalidade: nenhum stream de app no Loki tem index label fora da promoção default do Loki 3.7.7; nenhum label de métrica tem ID de entidade.
- [x] Segurança: o consumo só adota o contexto do envelope como link confiável depois de `Boundary.admits` (`TRC-07`); nenhum sinal exportado contém as chaves proibidas de RF-B3.
- [x] Degradação: falha de exportação nunca falha publicação, consumo nem requisição; Collector fora do ar descarta pela fila do batch, sem bloquear.
- [x] Estabilidade: messaging e cloudevents são Development, rpc é Release Candidate, db e http são Stable nos spans e, no HTTP, nas durações `http.server.request.duration` e `http.client.request.duration`, com exceções em Development que a plataforma emite: os atributos `db.client.connection.pool.name` e `db.client.connection.state` (`otel@v1.47.0/semconv/v1.43.0/attribute_group.go:3817,3828`; `postgres/pool.go:158-160`), `db.response.returned_rows` (`:4000`; `postgres/dbtrace.go:102`), `http.request.body.size` e `http.response.body.size` (`:7208,7306`; SERVER do `otelhttp`, `otelhttp@v0.72.0/internal/semconv/server.go:337-347`) e as métricas `db.client.connection.{count,max,pending_requests,timeouts,wait_time}` (`postgres/pool.go:137-192`), `http.server.request.body.size`, `http.server.response.body.size` e `http.client.request.body.size` (do `otelhttp`, `internal/semconv/server.go:270-271` e `client.go:335`), todas `development` no modelo da semconv (`semantic-conventions@v1.43.0/model/db/metrics.yaml:46,90,104,118,146`, `model/http/metrics.yaml:96,112,140`), registradas em `TRC-03`; upgrade de semconv é migração governada (`TRC-03`, `schema_url`; decisão do usuário de 2026-10-02, na revisão de código).
- [x] Compatibilidade: Go `1.26.6`+, OTel `v1.47.0`, `semconv/v1.43.0`, contrib `v0.72.0`, `baggagecopy v0.17.0`, `otelslog v0.21.0`, `sdk/log v1.47.0`, Collector contrib `0.160.0`, Grafana 13.2.1, Loki 3.7.7, Tempo 3.0.0, Prometheus v3.14.0, Alloy v1.19.2.

## Camadas afetadas

| Camada | Afetada? | Descrição |
|--------|----------|-----------|
| `domain` | [ ] | Sem mudança (`TRC-16`, `LOG-11`) |
| `port` (`libs/backend/go/ports`) | [x] | `MessageContext.Tracestate`, campo de dado |
| `application` | [x] | `message_context.go` copia `Tracestate`; `Failure` implementa `Categorized` |
| `provider` (`observability`, `postgres`, `kafka`, `sqs`, `grpc`, `http`, `authn`, `transport`) | [x] | SDK canônico, decorador de privacidade, spans e métricas semconv, logs OTLP, baggage |
| `app` do kernel (`app/relay`, `app/consumer.go`) | [x] | drain, `send`, `process`, métricas de mensageria, registros de relay e consumo |
| Apps (`bff`, `orders`, `reservations`, `bookings`) | [x] | `otelhttp`, access log, `OTEL_*`, relay e consumo ligados ao kernel |
| Infra (Collector, Loki, Alloy, Prometheus, Grafana, manifestos, CI) | [x] | Collector contrib com tail sampling, Alloy só infra, datasources, painéis, gates |
| Generator (`tools/dmpf-plugin`) | [x] | Templates com `OTEL_*` e o wiring novo |
| Contrato de wire (`contracts/`) | [ ] | Sem mudança de forma |

## Localização de código

```text
libs/backend/go/
  observability/boot/        — autoexport, OTEL_*, runtime, registro de partida
  observability/otelboot/    — resource, sampler, BSP, decorador de privacidade, baggagecopy, Views, Processor DMPF
  observability/logging/     — otelslog (NewLogger), severidade, amostragem de LOG-12
  observability/tracing/     — conjunto fechado dmpf.* ou semconv, error.type, baggage
  observability/metrics/     — catálogo com ponto e UCUM, labels, buckets
  observability/audit/       — EventName dmpf.audit
  observability/resilience/  — eventos e log de breaker e degradação
  observability/redact/      — error.type e dmpf.error.code
  app/relay/ app/consumer.go — drain, send, process, métricas de mensageria
  postgres/                  — spans db.*, pool por dbconv, tracestate no outbox
  grpc/ http/ authn/         — otelgrpc, otelhttp.NewTransport, recusa de token
  kafka/ sqs/                — sem span próprio de publish, content-type, DLQ, rebalance, TLS
  transport/observe/         — span INTERNAL de resiliência, log do cliente
apps/backend/                — bff (otelhttp, access log, health na porta de administração), relay e consumo das apps
infra/observability/         — otel-collector, loki, alloy, prometheus, grafana
tools/                       — otel-env-check.sh, grafana-provisioning-check.sh, dmpf-plugin
```

**Arquivos a modificar** († = alterado também pela SPEC-JJKWG4JP, já integrada):

- `libs/backend/go/observability/{go.mod,go.sum}` — `autoexport`, `runtime`, `baggagecopy`.
- `libs/backend/go/observability/boot/{telemetry.go,signals.go,boot.go}` — RF-A1, RF-A6 (partida), RF-D5, RF-E1, RF-E2.
- `libs/backend/go/observability/otelboot/{start.go,config.go,sampler.go,processor.go,shutdown.go}` e arquivos novos — RF-B3, RF-B8, RF-D3, RF-D7, RF-E3..E6; `otelboot/otlp/` removido.
- `libs/backend/go/observability/logging/{fields.go,logger.go,severity.go,sampling.go}` — RF-A1..A5; `logging/handler.go` e `logging/sinks.go` removidos.
- `libs/backend/go/observability/tracing/{attributes.go,record.go,owned.go,sampling.go}` — RF-B1; sai `KeyProvenanceTraceparent`.
- `libs/backend/go/observability/{redact,audit,metrics,resilience}/`, inclusive `resilience/sheet.go` — RF-A3, RF-A7, RF-D1..D4, RF-A6, RF-B5.
- `libs/backend/go/observability/usecase/instrumentation.go` † — chaves `tracing.Key*`; sem tenant do autor.
- `libs/backend/go/ports/message_context.go`, `libs/backend/go/application/{message_context.go,failure.go}` — `Tracestate`; `Categorized`.
- `libs/backend/go/postgres/{outbox.go,dbtrace.go,pool.go,wait.go,claim.go,signals.go,table.go}`; `postgres/{quarantine.go,inbox.go,purge.go}` † — RF-B6, RF-B7, RF-D6, RF-A6.
- `libs/backend/go/app/{go.mod,consumer.go,purge.go}`, `libs/backend/go/app/relay/{config.go,relay.go,publish.go,record.go,panic.go}` — RF-A1, RF-A5, RF-A6, RF-B7, RF-B9, RF-D2.
- `libs/backend/go/transport/observe/observe.go` — RF-B5, RF-A5 (cliente), RF-D2.
- `libs/backend/go/grpc/{config.go,server.go,dial.go,observe.go,recover.go,access.go}`; `grpc/{go.mod,interceptor_context.go}` † — RF-B4, RF-B5, RF-A5, RF-A6.
- `libs/backend/go/http/{client.go,go.mod}`, `libs/backend/go/authn/{oidc.go,config.go,go.mod}` — RF-B3, RF-B5, RF-A6.
- `libs/backend/go/kafka/{worker.go,newconfig.go,client.go,consumer.go,dlq.go,auth.go,panic.go}`; `kafka/publisher.go` † — RF-A1, RF-A5, RF-A6, RF-B7.
- `libs/backend/go/sqs/{publisher.go,sns.go,dlq.go,api.go,consumer.go,heartbeat.go,panic.go}` — RF-A1, RF-A3, RF-B7, RF-A6.
- `apps/backend/bff/app/{api/health.go,api/traceparent.go,wiring.go,config.go,probe.go,telemetry.go,harness_test.go}`, `apps/backend/bff/deploy/{compose.yml,.env.example,k8s/base/deployment.yaml}`, `apps/backend/load/{src/params.js,src/main.js,src/lib/guard.js,scripts/run.sh}`; `bff/app/api/{middleware.go,routes.go,handlers*.go}` †, `bff/app/rpc/clients.go` † — RF-B2, RF-A4, RF-A5, RF-A6.
- `apps/backend/{orders,reservations,bookings}/app/{telemetry.go,wiring.go,config.go}` † e `apps/backend/reservations/app/consumer.go` †, `apps/backend/reservations/app/sink.go` — RF-A1, RF-B7, RF-B9, RF-E1.
- `apps/backend/*/deploy/{.env,.env.example,compose.yml,k8s/base/configmap.yaml}`, `infra/local/compose/app-base.yml` — RF-E1, RF-E5, RF-E7.
- `tools/dmpf-plugin/src/generators/bounded-context/files/app/app/telemetry.go__tmpl__`; `files/app/app/{wiring,config}.go__tmpl__` † e `files/app/appkit/pool.go__tmpl__` †; `files/deploy/**__tmpl__`; `generator.spec.ts` † — RF-A1, RF-E1.
- `infra/observability/otel-collector/{otel-collector.yaml,deployment.yaml,networkpolicy.yaml}`, `infra/k8s/overlays/hmg/{otel-collector-tls.yaml,otel-collector-patch.yaml}`, `infra/local/compose/otel-collector.yml`, os patches `hmg` de ConfigMap e de montagem da CA das apps (`apps/backend/*/deploy/k8s/overlays/hmg/`) e os templates equivalentes do `tools/dmpf-plugin` — RF-A8, RF-E7.
- `infra/observability/loki/loki.yaml`, `infra/observability/alloy/{alloy-docker.alloy,alloy-kubernetes.alloy}` — RF-A8.
- `infra/observability/prometheus/prometheus.yaml` — RF-D1.
- `infra/observability/grafana/{datasources.yaml,dashboards.yaml,dashboards/plataforma/reference.json,dashboards/carga/load-bff.json}` e os dashboards de `dashboards/{plataforma,kernel,infra}/` — RF-C1..C5.
- `tools/grafana-provisioning-check.sh`, `tools/otel-env-check.sh` (novo), `.github/workflows/ci.yml` — RF-C4, RF-A8.
- `libs/backend/go/observability/README.md`, `infra/README.md`, `apps/backend/*/README.md` — chaves, pipeline, `OTEL_*`, RF-C6.
- `docs/dmpf/resiliencia-observabilidade.md`, `docs/adr/` — revisões listadas em "Decisões técnicas".

## Design

### Arquitetura

```text
Trace A (F1)                                 Trace B (F2)
bff    POST /orders/{id}/place    SERVER     orders  outbox drain orders.events  INTERNAL raiz
 └ bff dmpf.resilience orders     INTERNAL           │ links → contexto de criação de cada mensagem
    └ bff orders.v1.../PlaceOrder CLIENT             ├ UPDATE outbox (MarkPublished…)   CLIENT
       └ orders .../PlaceOrder    SERVER             └ send orders.events               CLIENT
          └ dmpf.usecase.PlaceOrder INTERNAL             │ link → contexto de criação (A); resiliência no send
             └ INSERT outbox      CLIENT         envelope: traceparent = contexto de A, inalterado
   metadata: traceparent + tracestate (A) ───────────────┘
                                             Trace C (F3)
                                             reservations process orders.events  CONSUMER raiz
                                              │ link → contexto de criação (A)
                                              └ inbox / dmpf.usecase / db
                                                 └ INSERT outbox reservation-confirmed
                                                    metadata: contexto do span ativo em C
Processo: slog → otelslog → sdk/log (baggagecopy, Processor DMPF) ─┐
          spans → baggagecopy → BatchSpanProcessor → decorador ────┼→ Collector contrib
          métricas (Views, runtime, dbconv) → PeriodicReader ──────┘   tail_sampling → Tempo
                                                                       logs → Loki /otlp
                                                                       métricas → Prometheus
```

### Fluxo principal

1. **F1** — `otelhttp` abre o SERVER no BFF; o middleware monta `ExecutionContext` e
   baggage; `otelgrpc` abre CLIENT por tentativa sob o INTERNAL de resiliência; no `orders`,
   o interceptor monta `MessageContext` com `traceparent` e `tracestate` do span ativo, e a
   UoW grava a linha do outbox.
2. **F2** — o relay reivindica o lote; se não vier vazio, abre `outbox drain` com início no
   claim e um link por mensagem; para cada uma abre `send` CLIENT com link à criação,
   publica o envelope inalterado e marca como publicada.
3. **F3** — o `kernelapp.Consumer` checa a fronteira, abre `process` CONSUMER em raiz com
   link à criação, monta o baggage e executa inbox, caso de uso, commit e ACK; o outbox em
   cadeia grava o contexto do span ativo; sai um `message consumed`.
4. **Exportação** — o decorador limpa as chaves proibidas; o Collector aplica
   `tail_sampling` e envia a Tempo, Loki e Prometheus.
5. **Navegação** — log→trace por `TraceID`; log→todos os traces da correlação por
   `correlation_id`; trace→logs por `tracesToLogsV2`; A → B e A → C por
   `{ link:traceID = "<A>" }`.

### Pseudocódigo

```text
drain(lote, início_do_claim):
  se lote vazio: retornar
  links = [link(ctx_de(m.metadata), {messaging.message.id: m.id}) para m válido]
  ctx, dreno = raiz("outbox drain " + destino_único_ou_vazio, INTERNAL,
                    início = início_do_claim, links, {batch.message_count, dmpf.outbox.claim_id})
  ctx = com_baggage(ctx, {dmpf.request_id: novo()})
  dreno.evento("claimed")
  para m em lote:
    ctx_p = sem_membro_baggage(ctx, dmpf.request_id)
    ctx_m, send = filho(ctx_p, "send " + tópico(m), CLIENT, [link(ctx_de(m.metadata))],
                        {messaging.*, cloudevents.*, messaging.message.conversation_id,
                         dmpf.tenant_id, dmpf.request_id: novo(), dmpf.outbox.attempt})
    ctx_m = com_baggage(ctx_m, {dmpf.correlation_id, dmpf.tenant_id: de(m.metadata), dmpf.request_id: send})
    erro = publicar(ctx_m, montar(m))
    send.desfecho(categoria(erro), error.type); send.fechar()
  dreno.fechar()

root.decidir(p):
  classe = p.atributo(dmpf.traffic_class) ou "unclassified"
  se classe ∈ {write, read} e algum p.link válido está amostrado: RecordAndSample
  senão: TraceIDRatioBased(taxa(classe)).decidir(p)

decorador.exportar(spans):
  para s em spans:
    remover(s, client.address, network.peer.address, network.peer.port, user_agent.original)
    s.url.path = s.http.route se presente, senão "REDACTED"
    s.url.full = origem(s.url.full) + (s.url.template se presente, senão "/REDACTED"), sem query nem fragmento
    s.status.descrição = ""; remover(eventos(s), exception.message, exception.stacktrace)
  exporter.exportar(spans)
```

## Decisões técnicas

- **Envelope com o contexto de criação inalterado; `send` CLIENT; `process` com link à
  criação** (decisão 6): é o que a semconv manda (`messaging-spans.md:242,266-274,337`) e o
  que a extensão CE Distributed Tracing define. Alternativa descartada: regravar o envelope
  com o contexto do `send` PRODUCER (E11) — modifica o contexto de criação.
- **Link em todo salto assíncrono**: link é o default produtor→consumidor da semconv
  (`messaging-spans.md:299-345`); parentesco é opt-in, não adotado. Fronteira recusada
  segue o padrão *public endpoint* (`otelhttp handler.go:104-109`).
- **Log só por OTLP** (decisão 1): única forma sem parse, chave nem label próprios.
  Alternativa descartada: stdout + Alloy com chaves canonizadas no JSON — `stdoutlog` não é
  formato estável de produção. Custo aceito: `docker logs`/`kubectl logs` deixam de
  mostrar o log coletado; Collector fora do ar descarta pela fila.
- **`otelgrpc` e `otelhttp` nos spans e nas métricas de borda** (decisões 2, 4 e 7): são
  canônicos em semconv `v1.43.0`; o `otelgrpc` grava `grpcStatus.Message()` em 6 códigos
  (`interceptor.go:86-97`) e o `otelhttp` emite `client.address`, `user_agent.original` e
  `url.path` sem opção (`internal/semconv/server.go:185-203`), e o decorador de RF-B3 os
  limpa antes do envio. Alternativa descartada: spans próprios com `rpcconv`/`httpconv`.
- **Código próprio em Postgres, mensageria e pool**: o tracer do `otelpgx` emite semconv
  `v1.40.0`, grava `err.Error()` no status (`tracer.go:205-215`) e `pgxpool.*` fora da
  semconv; o do `kotel` emite `v1.18.0`. As constantes `semconv/v1.43.0` já estão pinadas.
- **Baggage in-process + `baggagecopy`** (decisão 5), com `OTEL_PROPAGATORS=tracecontext`
  para o tenant não vazar a terceiros. Alternativa descartada: processor próprio no
  `OnStart` (E12).
- **Sampler e tail sampling** (decisões 8 e 11): `AlwaysRecord` existe em
  `sdk@v1.47.0/trace/sampling.go:316`, o que derruba a justificativa do ADR-037:42-66 para
  o `classSampler`. Com a cabeça em `1.0` (RF-E5) o processo classifica e exporta tudo, e
  o `tail_sampling` aplica TRC-13 e TRC-14 sobre o trace completo; cabeça abaixo de 1
  descartaria erro antes da cauda, contra TRC-14.
- **Continuidade F1→F2→F3 no Kubernetes — `write` a 100% na cauda** (decisão do usuário,
  2026-09-30): F1, F2 e F3 são traces distintos ligados por link, e o `tail_sampling`
  decide trace a trace; com `write` sempre amostrado, a escrita nunca perde F2 ou F3.
  Alternativa descartada: decisão da cauda por `dmpf.correlation_id` com `hash_seed`
  comum — depende de policy não verificada no contrib `0.160.0`. Custo aceito: volume de
  traces de escrita no Tempo; `read` e `unclassified` seguem a taxa de TRC-13.
- **Seleção de exporter por `autoexport`** (decisão 9); **sem `duration_ms`** (decisão 10);
  **`OTEL_*` e detectores** (decisão 11); **defaults do Loki, `dbconv`, `content-type` do
  binding e `dmpf.process.role`** (decisão 12).
- **Correlação**: `messaging.message.conversation_id` nos spans de mensageria e
  `dmpf.correlation_id` no resto, inclusive nos logs (decisão 3); o TraceQL de RF-C3 usa
  as duas.
- **Sequenciamento com SPEC-JJKWG4JP**: mergeada em `develop` (PR #6, `9ec83534`) e integrada
  nesta branch por fast-forward; os arquivos marcados com † partem do código dela. Livres primeiro: `grpc/server.go` e `grpc/dial.go` (a ligação do `otelgrpc`
  depende do `grpc/go.mod` †), `postgres/pool.go`, `postgres/dbtrace.go`, `kafka/client.go`,
  `telemetry.go__tmpl__`, o restante de `observability`, `infra/` e CI.

### Revisões no acervo e nos ADRs

- FND-08 `TRC-03`: nome e chave com equivalente na semconv `v1.43.0` são obrigatórios;
  `dmpf.*` só sem equivalente; registrar a estabilidade por domínio.
- FND-08 `TRC-04`/`TRC-12`: serviço, versão e instância são resource; a categoria FND-07 é
  o valor de `error.type` nas gravações do DMPF, e os spans e séries das instrumentações
  contrib mantêm o `error.type` da semconv delas (RF-B1); status de erro sem descrição.
- FND-08 `TRC-07`: fora da fronteira, raiz nova com link ao contexto recebido.
- FND-08 `TRC-08` e Exemplo 3: link é o default em todo salto assíncrono.
- FND-08 `TRC-11`: no HTTP, cada tentativa é CLIENT com `http.request.resend_count`; o span
  lógico de resiliência é INTERNAL.
- FND-08 `TRC-15`: exclusões nominais (`client.address`, `network.peer.address`,
  `user_agent.original`, `url.path` com identificador) e remoção no processo, antes do envio.
- FND-08 `MET-02`/`MET-03`/`MET-04`: forma OTel, UCUM, labels semconv ou `dmpf.*`.
- FND-08 `MET-08`..`MET-30`: nomes canônicos (`http.server.request.duration`,
  `rpc.*.call.duration`, `messaging.*`, `db.client.connection.*`) e os `dmpf.*` com ponto.
- FND-08 `LOG-01`/`LOG-02`: registro no Logs Data Model exportado por OTLP.
- FND-08 `LOG-13`/`LOG-14`: auditoria identificada por `EventName` `dmpf.audit` e scope.
- FND-05 `ENV-08`: o `traceparent` do envelope é o contexto de criação, imutável até o
  consumidor.
- ADR-037: sampler `AlwaysRecord(ParentBased(root))`, `BatchSpanProcessor`, TRC-14 no
  Collector; pins contrib `v0.72.0` e `baggagecopy v0.17.0`.
- ADR-041: seleção por `OTEL_*_EXPORTER` (supera o fallback por endpoint vazio e o "um
  envelope de log").
- ADR-053: exceção à config sem prefixo para as variáveis `OTEL_*` da spec do SDK.
- ADR-054 §6: superado pelo ADR novo.
- ADR-057: log das apps por OTLP; o Alloy só para containers de infraestrutura.

## Regras relacionadas

- FND-08 `TRC-01` a `TRC-16`, `MET-01` a `MET-30`, `LOG-01` a `LOG-14` (com as revisões acima).
- FND-05 `ENV-08`; FND-07 `CTX-06`, `CTX-07`, `CTX-26`, `CTX-27`, `CTX-28`, `DAT-02`, `DAT-03`, `DAT-06`, `DAT-22`, `DAT-23`, `DAT-25`.
- SPEC-Z2HM6NAP — mesmo sampler e `WithPublicEndpointFn` da borda; esta spec não muda a confiança da borda HTTP.
- SPEC-JJKWG4JP — mergeada e integrada; arquivos marcados com † partem do código dela.

## Verificação e testes

### Critérios de aceite

- [x] Para um `POST /orders/{id}/place` no Compose, `{ link:traceID = "<F1>" }` retorna o trace de `outbox drain orders.events` com `send orders.events` (CLIENT) e o trace de `process orders.events` (CONSUMER) de `reservations`; o `traceparent` do envelope consumido tem o trace id de F1.
- [x] Na janela de um `load:smoke` (1 min), `{ name =~ "dmpf\\.(kafka|sqs|grpc|db)\\..*" }` e `{ span.dmpf.service != "" }` retornam 0 spans; `{ span.db.system.name = "postgresql" && span.db.query.text != "" }` retorna spans de `orders` e `reservations`.
- [x] Na mesma janela, nenhum span exportado tem `client.address`, `network.peer.address`, `user_agent.original`, `url.path` diferente do template da rota nem `url.full` com query ou path concreto, e nenhum status de erro tem descrição.
- [x] Na mesma janela, `{ span.dmpf.correlation_id != "" }` retorna spans de `bff`, `orders` (papel `api`) e `reservations`; no relay, o critério aceita a correlação em `messaging.message.conversation_id`, que todo `send` e todo `process` levam (RF-B8, decisão 3; decisão do usuário de 2026-10-02).
- [x] A linha de outbox de `reservation-confirmed` tem `traceparent` com o trace id do `process`, não o de F1.
- [x] Com `LOG_LEVEL=info`, um place-order gera no Loki exatamente 1 `http request` (`bff`), 1 `grpc call` (`orders`) e 1 `message consumed` por mensagem (`reservations`), todos com `dmpf_correlation_id` e `trace_id` em structured metadata; o registro de auditoria tem `scope_name` do pacote `audit`.
- [x] `{service_name=~".+"}` no Loki não tem label `level`, `service` nem `channel`, e nenhum registro de app chega pelo Alloy.
- [x] `GET /api/datasources/uid/loki` devolve `TraceID` com `url` = `${__value.raw}`; `GET /api/datasources/uid/tempo` devolve a `customQuery` sem `$$`.
- [x] O Prometheus expõe `http_server_request_duration_seconds`, `rpc_server_call_duration_seconds`, `messaging_process_duration_seconds`, `go_goroutine_count`, `db_client_connection_count`, `otel_sdk_processor_span_processed_total` e `dmpf_dependency_breaker_state` (sem `_ratio`); `dmpf_service_requests_total` não existe.
- [x] Um processo com `OTEL_TRACES_SAMPLER=always_on` loga um único `warn`; com `OTEL_PROPAGATORS=baggage` a partida falha; sem `OTEL_TRACES_SAMPLER_ARG` a taxa efetiva da cabeça é `1.0`; sem `service.instance.id` a partida falha com `ErrResourceIncomplete`.
- [x] O `process configured` de cada processo não contém senha, DSN, token nem caminho de chave privada (teste com valores sentinela).
- [x] `tools/grafana-provisioning-check.sh` e `tools/otel-env-check.sh` aprovam o repositório e reprovam os casos do `--self-test`.
- [x] Testes com exportador em memória em `app/relay`, `app/consumer.go`, `postgres/dbtrace.go`, `transport/observe`, sampler e decorador; testes de `sdk/log` em memória para as chaves dos registros de acesso, consumo e partida.
- [x] Validação passando: `pnpm biome check .`, `pnpm nx affected -t lint,typecheck,test,build --exclude=@mateusmacedo/dmpf-source`, `fmt-check`, `vet`, `test-race` e `govulncheck` dos módulos Go tocados, `go run ./tools/dmpf-conformance/cmd/conformance --root . --base develop` e `go run ./tools/dmpf-conformance/cmd/modsync --root . --check`.

### Cenários de teste

```text
DADO a topologia local com OTEL_TRACES_SAMPLER_ARG=1.0, tail a 100% e LOG_LEVEL=info
QUANDO um cliente faz POST /orders/{id}/place com sucesso
ENTÃO { link:traceID = "<F1>" } retorna o trace do drain com send e o trace do process
  E o Loki tem 3 registros de acesso com o mesmo dmpf_correlation_id e trace_id não vazio
  E o TraceID de cada registro abre o trace correspondente no Tempo

DADO o root sampler com taxa 0 para write e um lote de 3 mensagens em que só o contexto
  de criação da segunda está amostrado
QUANDO o relay drena o lote
ENTÃO o drain é amostrado, carrega 3 links com messaging.message.id e os 3 send saem
  com o desfecho de cada mensagem

DADO uma linha de outbox com traceparent malformado em metadata
QUANDO o relay drena o lote que a contém
ENTÃO a mensagem é publicada com o envelope inalterado, o send não tem link
  E o drain registra o evento invalid_creation_context

DADO uma entrega Kafka por fronteira não confiável
QUANDO o kernelapp.Consumer recebe o envelope
ENTÃO o process abre raiz nova com link ao contexto recebido e classe error
  E nenhum parentesco com o produtor é criado e dmpf.provenance.traceparent não existe

DADO o BFF com otelhttp e uma requisição GET /orders/abc-123 com User-Agent e IP de cliente
QUANDO o span SERVER é exportado
ENTÃO url.path = "/orders/{id}", sem client.address, network.peer.address nem user_agent.original

DADO o broker indisponível durante a publicação
QUANDO o relay tenta enviar a mensagem
ENTÃO o send fecha com status Error sem descrição, error.type e dmpf.outcome_category
  E sai outbox publish failed em warn com dmpf.correlation_id e trace_id do send
  E a próxima tentativa abre um novo send com dmpf.outbox.attempt = 2
```

<critical_constraints>
- [P0] O envelope CloudEvents é o único portador do contexto de trace no salto assíncrono (`ENV-08`), e o `traceparent`/`tracestate` dele é o contexto de criação da mensagem, gravado uma vez e nunca reescrito pelo relay. Nenhum header Kafka nem message attribute SQS carrega contexto W3C.
- [P0] Todo salto assíncrono (F1→F2, lote→mensagem, produtor→consumidor, fronteira recusada) é link, nunca parentesco (`TRC-08` revisado).
- [P0] Nenhum payload, corpo, envelope inteiro, valor de parâmetro SQL, `client.address`, `network.peer.address`, `user_agent.original`, `url.path` com identificador ou `url.full` com query ou identificador sai do processo em span, log ou métrica; status de erro sem descrição e sem `exception.message` (`TRC-12`, `TRC-15`, `DAT-02`, `DAT-06`, `DAT-23`).
- [P0] Nenhum segredo, DSN, token, senha ou caminho de chave privada entra em sinal algum (`LOG-09`).
- [P0] Nenhuma instrumentação em `domain` ou `port` (`TRC-16`, `LOG-11`); `ports.MessageContext` só ganha campo de dado; bibliotecas OTel só em módulos `provider`/`app`, atrás de ports.
- [P0] Dependências só as nomeadas e pareadas: core `v1.47.0`, `semconv/v1.43.0`, contrib `v0.72.0` (`otelhttp`, `otelgrpc`, `instrumentation/runtime`, `exporters/autoexport`) e `processors/baggagecopy v0.17.0`. Tracer do `kotel`, tracer e `RecordStats` do `otelpgx` e `otelaws` não entram; lib que emite forma não canônica é substituída por código próprio mínimo com constantes `semconv/v1.43.0`.
- [P0] `trace_id`, `span_id`, `dmpf.correlation_id`, `dmpf.request_id`, `dmpf.tenant_id` e `messaging.message.id` nunca viram index label do Loki.
- [P1] O log das apps segue um único caminho, OTLP; o Alloy nunca raspa stdout de app.
- [P1] O contrato de wire (`contracts/envelope`) não muda de forma.
</critical_constraints>

## Escopo fora

- **Canal de auditoria regulatório** (retenção, controle de acesso e integridade de `LOG-13` pleno): exige infraestrutura própria; spec de auditoria.
- **Confiança do `traceparent` na borda HTTP e `WithPublicEndpointFn` do BFF**: SPEC-Z2HM6NAP.
- **Instrumentos ainda não emitidos do catálogo** (`dmpf.outbox.*`, `dmpf.relay.*`, `dmpf.inbox.*`, `dmpf.dlq.*`, `dmpf.quarantine.*`, saga; `MET-14`..`MET-19`, `MET-22`..`MET-27`): esta spec só fixa os nomes na revisão do acervo.
- **Métricas de processo** (RSS, FDs, CPU): o cAdvisor cobre o Compose; `bridges/prometheus` + `client_golang` traria 7 módulos de terceiros.
- **Collector com mais de uma réplica** (`loadbalancing` exporter por trace id): o tail sampling exige todos os spans de um trace na mesma instância.
- **Id de agregado em span** (`dmpf.aggregate.id`, `cloudevents.event_subject`, `messaging.kafka.message.key`): sem classificação declarada do id (`DAT-01`, `DAT-03`).
- **Excessos cortados**: `dmpf.deadline.budget_ms` e `dmpf.idempotent` por chamada, `dmpf.inbox.reception`, `dmpf.sheet.<dep>.<campo>` no resource, `host.name` e `container.id`, `dmpf.correlation.origin`, lag por span (`dmpf.consume.lag_ms`, `dmpf.outbox.lag_ms`). `network.protocol.version` e `http.*.body.size` não são acrescentados por código próprio; os que `otelhttp` e `otelgrpc` emitem por padrão ficam, na forma da lib.
- **Configuração declarativa** (`otelconf`) e header W3C por salto no record Kafka: fora da decisão 12.
- **Troca de código próprio por biblioteca no restante do kernel**: spec própria.
- **Logs dos containers de infraestrutura** (Loki, Tempo, Redpanda, Postgres): formato do fornecedor.
