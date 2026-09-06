# ADR-037: Realizar observabilidade e resiliência em Go sobre OpenTelemetry, com sampler e processor próprios e retry por conjunção

## Status

Aceito — 2026-09-05. Implementa SPEC-NYD18TGD.

## Contexto

O FND-08 (`docs/dmpf/resiliencia-observabilidade.md`) fixa quatro famílias de
regra que nenhum bloco anterior do kernel podia conter: resiliência de saída
(`RES-*`), tracing (`TRC-*`), métricas (`MET-*`) e log com auditoria (`LOG-*`).
O ADR-026 já decidira a plataforma — OpenTelemetry — mas sem realização em Go,
e a norma seguia sem vetor executável.

A restrição que mais moldou o desenho vem da matriz de blocos. `domain` e `port`
não admitem capability de observabilidade, e a seta `provider → application` é
célula proibida. O gancho pelo qual um caso de uso reporta o que está fazendo
precisava então ser declarado em algum lugar que os dois lados alcançassem sem
violar a matriz.

O ADR-034 entregou a fronteira que este módulo instrumenta. O que faltava era
tudo que faz I/O: relógio de parede, socket para o collector, saída de log.

## Decisão

Um módulo `provider`, `dmpf-observability-go`, com doze packages e uma unidade
DMPF (`dmpf-kernel/observability`).

### O gancho de instrumentação vive em `dmpf-ports`, não no bloco `application`

A spec originalmente o punha em `dmpfapplication`. A implementação refutou isso
por medição: **Go satisfaz interface por assinatura idêntica, não por
estrutura**. Um provider que declarasse o próprio `Result` — mesmo com campos
iguais — não satisfaria a interface do bloco `application`, e importá-la seria a
aresta proibida.

A porta foi para `dmpf-ports` (`Instrumentation`, `Result`, `AuditEvent`,
`OutcomeCategory`, `EndOperation`, `ErrDenied`, `NoInstrumentation`), onde os
dois lados a alcançam legitimamente: declarada acima, realizada abaixo, como
toda porta. Erratas 7 e 8 da spec.

### `TRC-14` é realizado por sampler e processor próprios, não por composição

A decisão de amostragem do SDK acontece no **início** do span, antes de o
desfecho ser conhecido. Os samplers de fábrica devolvem `Drop` no ramo negativo
— o `TraceIDRatioBased` diretamente (`sdk/trace/sampling.go:71-116`) e o
`ParentBased` pelos seus defaults, que são `NeverSample()` (`:200-202`, e
`:156`) —, e um span descartado nunca é criado: nenhum processor poderia
retê-lo ao falhar depois. `BatchSpanProcessor` e
`SimpleSpanProcessor` completam o problema: ambos descartam span sem
`FlagsSampled` (`batch_span_processor.go:403`, `simple_span_processor.go:79`).

A saída é a "regra equivalente em processo" que a própria `TRC-14` admite, em
duas peças que só funcionam juntas:

- `classSampler` decide pela taxa da classe com os mesmos bits do `TraceID` que
  o `TraceIDRatioBased` usa, e devolve `RecordOnly` fora da taxa — **nunca**
  `Drop`. O span existe, não amostrado, e continua exportável.
- `classAwareProcessor` é o **único** `SpanProcessor` e dono exclusivo do
  exportador, e exporta todo span que termine amostrado **ou** com
  `codes.Error`. Nunca convive com outro processor sobre o mesmo exportador:
  `ExportSpans` é síncrono e não dá garantia de concorrência
  (`span_exporter.go:16-19`).

O que isso garante é o **span** em erro. O trace completo — ancestrais e irmãos
`RecordOnly` sem erro — é do tail sampling no collector. Nenhum artefato desta
entrega chama o fragmento de "trace completo".

**Política de descarte sob saturação**: a spec descreve o descarte do span "mais
antigo não amostrado e sem erro", mas esse span nunca entra na fila — o filtro
de entrada já é `sampled || codes.Error`. O critério que resta operante é
**preservar a falha**: sacrifica-se o mais antigo sem status de erro, e quando
toda a fila é falha, o span novo é o recusado. A perda é sempre contada em
`dmpf_otel_spans_dropped_total`.

### OTLP/gRPC para um OpenTelemetry Collector

O destino é o Collector, não um backend de vendor: trocar o backend passa a ser
configuração do collector, não mudança de código. Os exportadores ficam em
`otelboot/otlp`, separados do bootstrap, para que este permaneça testável sem
rede.

TLS é o default e desligá-lo exige **duas** declarações: `Transport{Insecure:
true}` e `AllowInsecure: true` na mesma `Config`. O endpoint vem da configuração
do deployment, nunca literal no código.

O `Config.Propagator` é obrigatório e precisa carregar W3C Trace Context —
verificado pelos campos que ele declara injetar (`traceparent` e `tracestate`),
não por asserção de tipo, para que um composite com `Baggage` passe. Sem ele,
`Start` recusa com `ErrPropagatorRequired` e **não toca** em nenhum global.

### `Budget` é o único valor que trafega no contexto

O orçamento de retry (`RES-30`) precisa ser compartilhado entre dependências de
uma mesma execução, e o contexto é o único canal com esse alcance. Ele é a
**única** exceção: nenhum outro estado da plataforma viaja em `context.Value`.

Pô-lo em `dmpfports` violaria `RES-24` pelo caminho mais discreto — o bloco
`port` passaria a conhecer política de retry.

### Retry por conjunção, com taxonomia injetada

`retry.Evaluate` decide por conjunção de fatores, e o primeiro deles é "o erro é
retentável". Não existe taxonomia de erro no kernel Go: a realização de FND-07
não é história deste épico. O avaliador recebe um `Classifier` injetado, e
classificação ausente ou indeterminada resolve para **não retentável** (`ERR-11`,
`RES-29`) — negar por omissão é o único default seguro.

O `usecase` recebe um segundo classificador, `func(error) string`, que dá a
`error_category` de `MET-10`. Categoria ausente **ou vazia** vira
`"unclassified"`; a mensagem do erro nunca vira label.

### `guardUnitOfWork` verifica por chamada, não em construção

A spec descreve a recusa de retry sobre unidade de trabalho como um gate de
construção. A implementação a faz **por chamada**, porque a `Operation` só chega
com a chamada: a composição de uma dependência não sabe qual método vai
decorar. O comportamento — recusar — é o mesmo; o momento é outro.

### Pins e piso

OpenTelemetry `v1.46.0` em todos os packages, `semconv/v1.43.0`, e piso Go
`1.26.6` — o mesmo do `develop`, e o **menor** que serve.

O piso importa aqui porque o `govulncheck v1.7.0` é obrigatório no CI e reprova
o módulo em `1.26.4` assim que os exportadores OTLP entram no grafo, por três
vulnerabilidades alcançáveis da stdlib (GO-2026-6090 e GO-2026-5856 em
`crypto/tls`, GO-2026-5972 em `encoding/asn1`). As três estão corrigidas em
`1.26.5`/`1.26.6`.

Esta entrega chegou a subir o piso a `1.26.8` antes de fechar essa conta: a
pesquisa mediu `1.26.4` reprovando e `1.26.8` passando, e nunca mediu o
`1.26.6` que estava entre os dois. Medido depois, `1.26.6` passa — inclusive com
`testcontainers-go`, desde que `github.com/moby/go-archive` esteja em `v0.3.0`
ou superior. A vulnerabilidade que resta em `1.26.6` é dessa dependência, e se
corrige por ela, não pelo toolchain. O piso ficou em `1.26.6`.

### Testcontainers fora da allowlist

O teste de integração sobe um Collector real por `testcontainers-go`, com a
imagem fixada por **digest** — uma tag é mutável, e uma suíte que fica vermelha
porque alguém republicou `0.160.0` é falha que ninguém reproduz. Roda no
`go test ./...` padrão, sem build tag e sem `testing.Short()`: pular em silêncio
criaria um caminho em que a integração nunca é exercida e ninguém percebe.

`testcontainers-go` **não** entra na `external[]` do manifesto: é dependência de
teste, e o verificador analisa o grafo de produção. Pela mesma razão, o
`govulncheck` não cobre o que só o teste importa — limite conhecido, não
descuido.

Os dois exportadores OTLP e o `google.golang.org/grpc` entram na allowlist com
capability **`io.network`**, e não `observability` como o resto do SDK: são os
únicos packages do módulo que abrem socket, e declará-los pelo propósito
apagaria do manifesto onde a rede entra.

## Alternativas descartadas

**Compor `ParentBased(TraceIDRatioBased(...))`** — os ramos negativos de ambos
devolvem `Drop`, e um span descartado não pode ser retido depois. É exatamente o
que `TRC-14` proíbe.

**Dois processors, um para amostrados e outro para erros** — dois donos do mesmo
exportador, cujo `ExportSpans` é síncrono e sem garantia de concorrência.

**Exportador de vendor em vez do Collector** — acopla ao backend a decisão que o
ADR-026 quis manter reversível.

**`cenkalti/backoff` ou `failsafe-go`** — trariam retry por biblioteca, com
política própria, onde a norma pede conjunção de fatores explícita e auditável.

**Realizar a taxonomia de erro aqui** — invade FND-07 e o `KRN-10`.

**Estender `dmpfports.Clock` para cobrir timer e sleep** — violaria `RES-24`,
levando política de resiliência ao bloco `port`.

**Tag `integration` ou `testing.Short()` para o teste do collector** — cria
caminho silencioso de não execução. O Docker no runner foi confirmado.

## Consequências

O kernel passa a ter observabilidade e resiliência executáveis. O `KRN-06`
(outbox Postgres, ADR-035) e o `KRN-07` (inbox, ADR-036) mergearam em paralelo a
esta entrega e ainda não são instrumentados por ela: ligá-los ao gancho e às
fichas de resiliência é trabalho de quem os consumir. O `KRN-10` (admissão)
encontra pronta a posição de rate limit que a ficha declara não aplicável aqui.

A suíte do módulo passa a **exigir Docker**: sem daemon acessível, o package
`otelboot/otlp` falha. É o preço declarado de não ter caminho silencioso.
`TESTCONTAINERS_RYUK_DISABLED=true` existe como escape para ambiente onde o Ryuk
não sobe — e é escape, não default: sem ele, containers órfãos ficam para trás
se o processo de teste morrer.

O módulo é o primeiro do kernel a abrir socket, e o primeiro cuja allowlist tem
capability `io.network`. Um serviço que o consuma herda essa fronteira.

A garantia de telemetria é **at-least-once e sob perda declarada**: o exportador
repete, a fila descarta sob saturação e a amostragem descarta por desenho.
Telemetria não é registro contábil. A trilha de auditoria, por isso mesmo, vai a
um sink próprio e nunca ao handler de log (`LOG-14`) — retenção, nível e
amostragem de log podem derrubar um registro, e um fato auditável não pode
depender disso.

`go mod tidy` não roda neste módulo: ele tenta resolver os módulos irmãos pelo
proxy, e quem os resolve é o `go.work`. Dependência direta nova é promovida à
mão do bloco `// indirect`.

O `dmpf-gate-check.sh` e o `nx affected` não devem rodar ao mesmo tempo: o gate
cria arquivos `zz_gate_*.go` temporários nos outros módulos, e um lint
concorrente os encontra no meio do caminho.

## Referências

- `docs/specs/SPEC-NYD18TGD-dmpf-resiliencia-observabilidade-go.md` — spec do `KRN-09`
- `docs/adr/026-baseline-resiliencia-observabilidade-opentelemetry.md` — a decisão de plataforma
- `docs/adr/034-fronteira-de-uow-em-go.md` — a fronteira que este módulo instrumenta
- `docs/adr/030-granularidade-modulo-go-e-bom.md` — o piso Go e o BOM
- `docs/dmpf/resiliencia-observabilidade.md` — FND-08
- `docs/dmpf/rfc-dmpf-foundation-v0.1.md` — §6.2, §6.3, §10.2
- `libs/backend/go/dmpf-observability/README.md` — o módulo em detalhe
