# dmpf-observability-go

Bloco `provider` do kernel DMPF em Go: a realização de resiliência e
observabilidade que os blocos `domain`, `port` e `application` declaram, mas não
podem conter. Realiza as regras de `FND-08` — resiliência de saída (`RES-*`),
tracing (`TRC-*`), métricas (`MET-*`) e log com auditoria (`LOG-*`) — sobre
OpenTelemetry `v1.46.0` e `semconv/v1.43.0`.

Aqui está tudo que faz I/O: relógio de parede, socket para o collector, saída de
log. É por isso que o módulo é `provider` e não outra coisa — o gancho de
instrumentação que o application service usa vive em `dmpf-ports`, e é este
módulo que o realiza.

Projeto Nx `dmpf-observability-go`, tags `type:lib`, `scope:backend`,
`stack:go`. Import path do módulo:
`github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability`.

## O que o módulo contém

Uma unidade DMPF, `dmpf-kernel/observability`, com `block: provider` e
`bounded_context: dmpf-kernel`. Doze packages, todos no mesmo `include` do
`dmpf-units.json`:

| Package | Conteúdo |
| --- | --- |
| raiz (`dmpfobservability`) | `OTelVersion`, `SemconvVersion` — os pins que o `version_test.go` prova contra o grafo real de build |
| `audit` | `Sink`, `Event`, `NewJSONSink`, `Recording` — a trilha de auditoria, canal separado do log |
| `clock` | `Clock`, `System()`, `Fake` avançável com `Advance`, `WithTimeout`, `NewTimer` |
| `logging` | `NewHandler` sobre `slog`: campos obrigatórios, allowlist de redação, amostragem por classe |
| `metrics` | `Catalog()` com as quinze séries (dez de MET-02, duas locais e as três de MET-11/MET-12 que os providers de transporte gravam), `Labels` fechado — `tenant` só por `TenantWithin` com allowlist declarada —, `Instruments` construído uma vez |
| `otelboot` | `Config`, `Start`, `Runtime`, o `classSampler` e o `classAwareProcessor` |
| `otelboot/otlp` | os exportadores OTLP/gRPC de traces e métricas |
| `redact` | `Attr`, `Error` — o que sai de um erro é categoria e código, nunca a mensagem |
| `resilience` | `Sheet`, `Compose`, e os decorators de timeout, breaker, bulkhead e degradação |
| `retry` | `Evaluate` por conjunção, `Budget`, `Backoff`, `Classifier` |
| `tracing` | `Attributes` fechado, `RecordError`, `AttemptEvent`, a taxonomia de classes |
| `usecase` | `Instrumentation`: realiza o gancho de `dmpf-ports` sobre span, métricas e trilha |

## Onde cada regra está realizada

| Regra | Código |
| --- | --- |
| `RES-21`, `RES-40` | `resilience/sheet.go` — dez campos, cada um valor **ou** ausência declarada com motivo; `Effective()` alimenta os atributos de recurso |
| `RES-22` | `resilience/compose.go` — nove posições, de fora para dentro: tracing, métricas, log, bulkhead, breaker, rate limit, retry, timeout, chamada |
| `RES-05`, `RES-06`, `RES-07` | `resilience/timeout.go` — o prazo efetivo é o mínimo entre o do chamador, o do método e o remanescente |
| `RES-10`, `RES-12` | `resilience/breaker.go` — taxa de falha na janela, com piso de amostras antes de abrir |
| `RES-13`, `RES-14` | `resilience/bulkhead.go` — pool e fila; saturado é rejeição rápida, nunca espera |
| `RES-25`, `RES-34` | `resilience/retry_decorator.go` — `Compose` recusa retry em volta de uma unidade de trabalho |
| `RES-27` a `RES-31`, `RES-36` | `retry/evaluate.go`, `retry/budget.go` — a conjunção de fatores e o orçamento por execução |
| `RES-32`, `RES-33` | `retry/backoff.go` — exponencial com jitter total e teto |
| `RES-37`, `RES-38` | `resilience/degrade.go` — falhar, degradar ou omitir; adiar é da outbox e é recusado |
| `TRC-09` | `otelboot/config.go` — sem propagador W3C, `Start` não boota |
| `TRC-13`, `TRC-14` | `otelboot/sampler.go`, `otelboot/processor.go` — ver "A regra do erro sempre amostrado" |
| `TRC-04`, `TRC-15` | `tracing/attributes.go` — construtor fechado, um método por chave permitida |
| `TRC-11`, `TRC-12` | `tracing/record.go` — a tentativa é evento; o erro é status e categoria, sem mensagem |
| `MET-02` a `MET-05`, `MET-07` | `metrics/catalog.go`, `metrics/labels.go` — nome, unidade, fórmula e dono por série; labels por allowlist |
| `MET-08` a `MET-10` | `usecase/instrumentation.go` — duração, requisições e erros por operação e categoria |
| `MET-28`, `MET-29` | `resilience/retry_decorator.go`, `breaker.go`, `bulkhead.go`, `timeout.go`, `degrade.go` — cada decorator conta o que só ele sabe; `observe.go` traduz os labels |
| `LOG-01`, `LOG-13` | `logging/handler.go` — campos obrigatórios na raiz do registro; redação por allowlist |
| `LOG-14` | `audit/sink.go`, `usecase/instrumentation.go` — a trilha vai ao sink, nunca ao handler de log |

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

Amostragem de trace, por classe de tráfego: erro `1.0`, escrita `0.10`, leitura
`0.01`, manutenção `1.0`. Classe não declarada resolve para a taxa mais
restritiva (`0.01`) e o span recebe `dmpf.traffic_class = "unclassified"`, para
que a omissão apareça em vez de passar por leitura legítima.

Um campo que a ficha não declara — nem valor, nem motivo — reprova em
`Sheet.Validate()`. Em branco não é default: é política que ninguém decidiu.

## A regra do erro sempre amostrado

`TRC-14` pede que um erro seja sempre amostrado, e o SDK do OpenTelemetry não
entrega isso pelos componentes de fábrica. A decisão de amostragem acontece no
**início** do span, antes de o desfecho ser conhecido, e os samplers de fábrica
devolvem `Drop` no ramo negativo — um span descartado nunca é criado, então
nenhum processor poderia retê-lo quando ele falhasse depois.

A realização é a "regra equivalente em processo" que a própria `TRC-14` admite,
e tem duas peças que só funcionam juntas:

- `classSampler` decide pela taxa da classe usando os mesmos bits do `TraceID`
  que o `TraceIDRatioBased` usa, e devolve `RecordOnly` fora da taxa em vez de
  `Drop`. O span existe, não amostrado, e pode ser exportado depois.
- `classAwareProcessor` é o **único** `SpanProcessor` e o dono exclusivo do
  exportador. Exporta todo span que termine amostrado **ou** com
  `codes.Error`. Ele substitui o `BatchSpanProcessor`, que descarta o não
  amostrado, e nunca convive com outro processor sobre o mesmo exportador —
  `ExportSpans` é síncrono e não dá garantia de concorrência.

O que isso garante é o **span** em erro, não o trace inteiro. Os ancestrais e
irmãos `RecordOnly` sem erro ficam por conta do tail sampling no collector. Nenhum
artefato deste módulo chama o fragmento de "trace completo".

Sob saturação, a fila do processor sacrifica primeiro o span mais antigo **sem**
status de erro; se toda a fila for falha, é o span novo que é recusado. Em
qualquer caso a perda é contada em `dmpf_otel_spans_dropped_total`, porque um
buraco silencioso nos traces é pior que um buraco medido.

## O que a telemetria garante

A entrega é **at-least-once, e sob perda declarada**. O exportador OTLP repete
tentativas, então o collector pode receber o mesmo span mais de uma vez. Na
direção oposta, a fila do processor descarta sob saturação e a amostragem
descarta por desenho — telemetria não é registro contábil.

Entrega exatamente-uma-vez (*exactly-once*) é **vedada** como promessa em
qualquer artefato do DMPF (P0-3, `GAR-01`), aqui como em toda parte.

A **trilha de auditoria é outra coisa**. Ela vai ao `audit.Sink`, nunca ao
handler de log, exatamente porque retenção, nível e amostragem de log podem
derrubar um registro — e um fato auditável não pode depender disso (`LOG-14`).
Quando o sink recusa um registro, o módulo escreve no log a **categoria** da
falha e a ação, jamais o conteúdo do evento.

## As oito chaves reservadas do log

A plataforma escreve oito chaves na raiz de todo registro: `trace_id`,
`span_id`, `service`, `version`, `instance`, `correlation_id`, `request_id` e
`tenant_id`.

Elas são reservadas porque JSON tolera chave duplicada e o leitor fica com a
última. Um atributo do autor chamado `service` tomaria em silêncio o lugar do
serviço a que o registro se refere.

O que o autor puser numa dessas chaves é **renomeado** para `app.<chave>`, e não
descartado — o valor continua no registro, e a chave reservada continua sendo da
plataforma. A renomeação é recursiva dentro de um grupo (`WithGroup`), para que
um atributo aninhado não reentre por ali.

## Bootstrap

```go
runtime, err := otelboot.Start(ctx, otelboot.Config{
    Propagator: propagation.TraceContext{},          // obrigatório e W3C
    Resource:   otelboot.Resource{ServiceName: "orders", ServiceVersion: "1.4.2", ServiceInstanceID: id},
    Sampling:   tracing.DefaultRates(),
    Sheets:     []resilience.Sheet{resilience.Defaults("payments")},
    Transport:  otelboot.Transport{Endpoint: endpoint},
    Logger:     slog.New(logging.NewHandler(os.Stdout, logging.Config{ /* ... */ })),
})
```

`Start` é o único lugar que toca o propagador e os providers globais, e uma
configuração recusada não toca em nenhum deles. Um segundo `Start` no mesmo
processo devolve `ErrAlreadyStarted`; `Shutdown` libera o processo e é
idempotente.

O endpoint vem da configuração do deployment, nunca literal no código. TLS é o
default: `Transport{Insecure: true}` só passa com `AllowInsecure` declarado na
mesma `Config`, para que telemetria em texto claro seja sempre uma decisão
escrita.

As fichas entram no `resource` **na criação** do provider, como
`dmpf.sheet.<dependency>.<campo>` (`RES-40`) — um `resource` não recebe atributo
depois de criado, por isso elas são insumo do bootstrap e não algo registrado
mais tarde.

## Validação

```bash
pnpm nx run-many -t fmt-check,vet,lint,build,test,test-race,govulncheck -p dmpf-observability-go
bash tools/dmpf-gate-check.sh
go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance --root . --base origin/develop
```

O `dmpf-gate-check.sh` e o `nx affected` **não devem rodar ao mesmo tempo**: o
gate cria arquivos `zz_gate_*.go` temporários nos outros módulos para provar que
o `depguard` reprova o que deve, e um lint concorrente os encontra no meio do
caminho.

### O teste do collector precisa de Docker

`otelboot/otlp` sobe um OpenTelemetry Collector real por `testcontainers-go`,
com a imagem fixada por digest — uma tag é mutável, e uma suíte que fica
vermelha porque alguém republicou `0.160.0` é uma falha que ninguém reproduz. O
teste roda no `go test ./...` padrão, sem build tag e sem `testing.Short()`:
pular em silêncio criaria um caminho em que a integração nunca é exercida e
ninguém percebe.

O collector sobe **uma vez por package**, em `TestMain`, e a suíte fecha em
cerca de 8 s.

Em ambiente onde o container Ryuk do testcontainers não pode subir,
`TESTCONTAINERS_RYUK_DISABLED=true` é o escape — e é escape declarado, não
default: sem o Ryuk, containers órfãos ficam para trás se o processo de teste
morrer, e a limpeza passa a ser sua.

`testcontainers-go` **não** está na allowlist do `dmpf-units.json`, e é
deliberado: ela é dependência de teste, e o verificador analisa o grafo de
produção. O `govulncheck`, pela mesma razão, não cobre o que só o teste importa.

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
YAML

docker run --rm -p 4317:4317 \
  -v /tmp/otelcol.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector:0.160.0

# noutro terminal, com o serviço apontado para localhost:4317 e Insecure declarado
docker logs -f <container>
```

O exportador `debug` imprime cada span e cada métrica que chega, com o resource
junto. Se o span não aparece, o problema está antes do collector: propagador,
amostragem ou transporte — nessa ordem de suspeita.

## Governança

Criar um package novo aqui é criar membership de unidade DMPF: o import path
exato entra no `include` do `dmpf-units.json` e o baseline em
`tools/dmpf-baseline/units-baseline.json` precisa ser regravado com
`--write-baseline`. Essa mudança vai em commit separado do código (RFC §10.2);
misturar os dois reprova no CI com `DMPF-T002`.

Dependência externa nova entra na `external[]` com os quatro elementos — pacote,
faixa de versão, entrypoints e capability — e a capability é **declarada**, nunca
inferida do nome. Os exportadores OTLP estão como `io.network`, e não
`observability` como o resto do SDK, porque são os únicos packages do módulo que
abrem socket: esconder isso sob a capability do propósito apagaria do manifesto
onde a rede entra.

`go mod tidy` não roda neste módulo: ele tenta resolver os módulos irmãos pelo
proxy, e quem os resolve é o `go.work`. Dependência direta nova é promovida à
mão do bloco `// indirect` para o `require` principal.

## Referências

- `docs/specs/SPEC-NYD18TGD-dmpf-resiliencia-observabilidade-go.md` — spec do `KRN-09`
- `docs/adr/037-observabilidade-otel-e-retry-por-conjuncao-em-go.md` — decisões desta realização
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que este módulo instrumenta
- `docs/adr/026-baseline-resiliencia-observabilidade-opentelemetry.md` — a decisão de plataforma que originou o `FND-08`
- `docs/dmpf/resiliencia-observabilidade.md` — FND-08: as regras `RES-*`, `TRC-*`, `MET-*` e `LOG-*`
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §6.2, §6.3, §10.2
