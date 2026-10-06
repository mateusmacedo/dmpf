# ADR-057: Log das apps por OTLP; Alloy só para infraestrutura

## Status

Aceito — 2026-10-01. Implementa [SPEC-1TFW24WV](../specs/SPEC-1TFW24WV-observabilidade-ponta-a-ponta.md), nos requisitos RF-A1 e RF-A8. Supersede parcialmente o [ADR-054](./054-apps-autocontidos-e-infras-separadas.md): o item 6 e a alternativa descartada «Exportar por OTLP também o log do app em container».

## Contexto

O item 6 do ADR-054 deu ao runtime local a observabilidade completa e deixou o log das apps em dois caminhos, conforme o lugar onde o processo rodava:

1. **No host.** Com `OTLP_LOGS=true`, o processo exportava o log por OTLP ao Collector, que o entregava ao Loki.
2. **Em container.** O processo escrevia JSON próprio no stdout (`logging/handler.go`), e o Alloy seguia o container, aplicava `stage.json`, promovia `level` e `service` a label e levava o `trace_id` a structured metadata. O OTLP no container foi recusado porque o Alloy já coletava o stdout e o Loki guardaria cada registro duas vezes.

O mesmo registro tinha, assim, duas formas. O JSON carregava como atributo o que o OTel Logs Data Model guarda em campo próprio (`time`, `level`, `msg`, `service`, `version`, `instance`, `trace_id`, `span_id`), e os labels promovidos pelo Alloy colidiam com as chaves do JSON: a consulta com `| json` devolvia campos `*_extracted`. A revisão canônica da SPEC-1TFW24WV fixou o princípio de usar a forma canônica do OpenTelemetry onde ela existe, inclusive nas chaves e no formato do log, e a decisão 1 da auditoria de canonicalização levou o log das apps a um caminho só, por OTLP.

## Decisão

**Todo processo de app exporta o log só por OTLP, no Logs Data Model, pelo mesmo caminho no host, no Compose e no Kubernetes. O Alloy deixa de ler a saída das apps e coleta só os containers de infraestrutura.**

1. **Um caminho (RF-A1).** O registro segue `slog` → `otelslog` `v0.21.0` → `sdk/log` `v1.47.0` → `otlploggrpc` `v0.23.0` → Collector → Loki. O `boot` pede o exportador ao `autoexport` (`autoexport.NewLogExporter`), que lê `OTEL_LOGS_EXPORTER` e escolhe o gRPC por `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`; o default dele, `http/protobuf`, não é atendido pelo receiver do Collector, que só fala OTLP/gRPC na porta 4317. Os `deploy/.env.example` das quatro apps, o `infra/local/compose/app-base.yml` e os ConfigMaps de `deploy/k8s/base` declaram `OTEL_LOGS_EXPORTER=otlp`, `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` e o endpoint: `http://localhost:4317` no host, `http://otel-collector:4317` no Compose e no Kubernetes.
2. **Registro no Logs Data Model.** Cada logger é um `otelslog.Handler` (`logging.NewLogger`) sobre o `LoggerProvider` do processo envolvido por `otelboot.Leveled` (`otelboot/logger.go`), tanto no logger do `boot` quanto no provider que o `Runtime` entrega (`otelboot/start.go`): o `LOG_LEVEL` é aplicado no provider, porque o `Enabled` do `otelslog` só consulta o provider, e nunca barra a auditoria. O scope é o import path do pacote emissor: o composition root pede o logger a `Runtime.LoggerFor`, e cada lib que emite log recebe `rt.LoggerProvider()` e cria o logger com o próprio import path; o `Runtime.Logger`, cujo scope era o do módulo `observability`, saiu. `service.*`, `deployment.environment.name` e `dmpf.process.role` vêm só do Resource, e `TraceId` e `SpanId`, do `ctx` (RF-A2). O processor DMPF do `sdk/log` (`otelboot/logprocessor.go`) impõe antes do lote a allowlist de atributos por papel (as chaves semconv do papel, `error.type`, `db.collection.name` e toda chave `dmpf.*`) e aplica a amostragem de `LOG-12` (`logging/sampling.go`): abaixo de `error`, o registro de um trace não amostrado, inclusive o de um `traceparent` remoto não amostrado, segue a taxa da classe do processo na tabela de `TRC-13`; o registro de um trace amostrado fica, e o registro sem span válido, como os da partida e do encerramento, não é tráfego e não passa pela amostragem. A auditoria segue pelo mesmo `LoggerProvider`, identificada pelo `EventName` `dmpf.audit` e fora da amostragem do processor (RF-A7).
3. **Sem exportador, sem coleta.** O handler JSON de `logging/handler.go` saiu. Com `OTEL_LOGS_EXPORTER=none`, como nos harnesses de teste, ou com o SDK desligado por `OTEL_SDK_DISABLED=true`, o `boot` não monta o pipeline de log e descarta os registros num `slog.DiscardHandler` (`boot/telemetry.go`): nada do log vai ao stdout. Sem `OTEL_LOGS_EXPORTER`, vale o `otlp`, default do `autoexport`. A configuração recusada e o erro com que `app.Run` termina, inclusive o da partida da telemetria, saem no stderr, pelo `cmd/main.go` de cada app.
4. **Collector e Loki (RF-A8).** O pipeline `logs` do Collector (`otel-collector.yaml`) recebe por `otlp` e exporta por `otlp_http/loki` ao `http://loki:3100/otlp`. O `loki.yaml` não declara `otlp_config`: o Loki 3.7.7 aplica a promoção default de resource attributes e deriva `detected_level`. O painel de logs do `reference.json` consulta `{service_name=~"$service"}`, exclui a auditoria por `scope_name` e filtra por `detected_level`, sem `| json`, e o Grafana lê `trace_id` e `dmpf_correlation_id` do structured metadata (RF-C1, RF-C3). Nenhum identificador de correlação vira index label.
5. **Alloy só para infraestrutura (RF-A8).** A primeira regra do `discovery.relabel` descarta as apps antes da coleta: em `alloy-docker.alloy`, o container cujo serviço Compose casa `dmpf-.*`, nome que só os serviços das apps usam; em `alloy-kubernetes.alloy`, o pod cujo `app.kubernetes.io/name` é `bff`, `orders`, `reservations` ou `bookings`, o label que o `deploy/k8s/base/kustomization.yaml` de cada app aplica. Nenhum dos dois aplica `stage.json` nem promove `level` ou `service`; o Alloy grava só os labels de origem (`container`, `compose_service`, `compose_project` e `platform` no Compose; `namespace`, `pod`, `container` e `app` no Kubernetes). O Collector, o Loki, o Tempo, o Prometheus, o Postgres e o Redpanda seguem coletados.
6. **O que fica do item 6 do ADR-054.** O runtime local continua com observabilidade completa, por outras variáveis: `LOG_LEVEL=debug` no `app-base.yml` e nos `.env.example` das apps, a cabeça do sampler em `OTEL_TRACES_SAMPLER_ARG=1.0` e o `tail_sampling` do Collector a 100% em todas as classes no Compose (`TAIL_SAMPLING_READ_PERCENTAGE` e `TAIL_SAMPLING_UNCLASSIFIED_PERCENTAGE` em `infra/local/compose/otel-collector.yml`), conforme RF-E5 e RF-E7. `TRACE_SAMPLE_RATE` e `OTLP_LOGS` saem dos manifestos das apps (RF-E1). O pipeline de logs que o ADR-054 acrescentou ao Collector passa a ser o caminho de todo processo de app. O nível do registro de acesso passa à tabela única de `logging/severity.go` (RF-A5): no servidor e no consumo, `accepted` e `rejected` ficam em `info`, `denied` em `warn` e a falha em `error`; na chamada de cliente, `accepted` fica em `debug` e o resto em `warn`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Manter os dois caminhos do ADR-054 | O mesmo registro teria duas formas conforme o lugar do processo, e os labels promovidos pelo Alloy colidiam com as chaves do JSON |
| Stdout com as chaves canonizadas no JSON, coletado pelo Alloy | O `stdoutlog` do OTel Go (`v0.23.0`) se declara para teste e depuração, sem garantia de estabilidade nem de compatibilidade do formato; o Alloy continuaria a fazer parse e a escolher labels |
| Exportar por OTLP sem restringir o Alloy | Qualquer linha que a app ainda escrevesse no stdout ou no stderr chegaria ao Loki por um segundo caminho, com outra forma; a spec exige que o Alloy nunca raspe o stdout de app |
| `otlp_config` próprio no Loki | Labels escolhidos pelo DMPF voltariam a ser forma própria; a promoção default já indexa o serviço, e os identificadores de correlação não podem virar index label |

## Consequências

**Positivas:**

- O registro tem uma forma só, onde quer que o processo rode, e a consulta filtra por `service_name` e `detected_level` sem `| json`.
- Log e trace se ligam pelo `TraceId` do próprio registro, sem extração pelo coletor.
- Nenhum registro de app chega ao Loki por dois coletores.
- O log do Collector continua coletado pelo Alloy: o erro do Collector ao exportar chega ao Loki por um caminho que não passa por ele.

**Negativas:**

- O Loki convive com dois conjuntos de labels: os resource attributes promovidos, nas apps, e os labels de origem do Alloy, na infraestrutura.
- O Collector é o único caminho do log das apps e roda com uma réplica, por causa do `tail_sampling` (RF-E7).
- **Custo aceito:** `docker logs` e `kubectl logs` de uma app mostram só o stderr do processo, onde o `cmd/main.go` escreve a configuração recusada e o erro com que ele termina; o log dela fica só no Loki.
- **Custo aceito:** com o Collector fora do ar, o `BatchProcessor` do `sdk/log` guarda até 2048 registros (`OTEL_BLRP_MAX_QUEUE_SIZE`) e sobrescreve os mais antigos; a app não bloqueia, mas o registro se perde.
